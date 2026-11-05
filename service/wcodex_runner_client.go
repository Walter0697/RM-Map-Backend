package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	RunnerJobStatusAccepted  = "accepted"
	RunnerJobStatusQueued    = "queued"
	RunnerJobStatusRunning   = "running"
	RunnerJobStatusDone      = "done"
	RunnerJobStatusFailed    = "failed"
	RunnerJobStatusCancelled = "cancelled"
	RunnerJobStatusTimedOut  = "timed_out"
)

type RunnerSubmitRequest struct {
	Prompt        string
	Application   string
	Operation     string
	Timeout       int
	OutputSchema  map[string]any
	AnalysisJobID string
	TraceParent   string
}

type RunnerJob struct {
	ID     string          `json:"id"`
	JobID  string          `json:"job_id,omitempty"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type wcodexRunnerClient struct {
	baseURL string
	client  *http.Client
}

func NewWcodexRunnerClient(baseURL string, client *http.Client) *wcodexRunnerClient {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &wcodexRunnerClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  client,
	}
}

func (client *wcodexRunnerClient) Submit(ctx context.Context, input RunnerSubmitRequest) (RunnerJob, error) {
	if client == nil || client.baseURL == "" {
		return RunnerJob{}, errors.New("wcodex runner URL is required")
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return RunnerJob{}, errors.New("runner prompt is required")
	}
	if strings.TrimSpace(input.Application) == "" || strings.TrimSpace(input.Operation) == "" {
		return RunnerJob{}, errors.New("runner application and operation are required")
	}
	if input.Timeout <= 0 {
		return RunnerJob{}, errors.New("runner timeout must be positive")
	}
	contextPayload := map[string]any{
		"application": strings.TrimSpace(input.Application),
		"operation":   strings.TrimSpace(input.Operation),
	}
	if strings.TrimSpace(input.AnalysisJobID) != "" {
		contextPayload["analysis_job_id"] = strings.TrimSpace(input.AnalysisJobID)
	}
	if strings.TrimSpace(input.TraceParent) != "" {
		contextPayload["traceparent"] = strings.TrimSpace(input.TraceParent)
	}
	payload := map[string]any{
		"prompt":        input.Prompt,
		"context":       contextPayload,
		"timeout_sec":   input.Timeout,
		"output_schema": input.OutputSchema,
	}
	return client.doJSON(ctx, http.MethodPost, "/jobs", payload)
}

func (client *wcodexRunnerClient) Get(ctx context.Context, runnerJobID string) (RunnerJob, error) {
	if client == nil || client.baseURL == "" {
		return RunnerJob{}, errors.New("wcodex runner URL is required")
	}
	if strings.TrimSpace(runnerJobID) == "" {
		return RunnerJob{}, errors.New("runner job ID is required")
	}
	return client.doJSON(ctx, http.MethodGet, "/jobs/"+url.PathEscape(strings.TrimSpace(runnerJobID)), nil)
}

func (client *wcodexRunnerClient) doJSON(ctx context.Context, method string, path string, payload any) (RunnerJob, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return RunnerJob{}, fmt.Errorf("encode runner request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, body)
	if err != nil {
		return RunnerJob{}, fmt.Errorf("create runner request: %w", err)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.client.Do(request)
	if err != nil {
		return RunnerJob{}, fmt.Errorf("runner request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 16*1024))
		return RunnerJob{}, fmt.Errorf("runner returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	var job RunnerJob
	if err := json.NewDecoder(response.Body).Decode(&job); err != nil {
		return RunnerJob{}, fmt.Errorf("decode runner response: %w", err)
	}
	if job.ID == "" {
		job.ID = strings.TrimSpace(job.JobID)
	}
	if job.ID == "" {
		return RunnerJob{}, errors.New("runner response did not include a job ID")
	}
	return job, nil
}

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWcodexRunnerClientSubmitUsesStructuredContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/jobs" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["timeout_sec"] != float64(120) {
			t.Fatalf("expected bounded timeout, got %#v", payload["timeout_sec"])
		}
		contextPayload, ok := payload["context"].(map[string]any)
		if !ok || contextPayload["application"] != "roroadmap" || contextPayload["operation"] != "social-media-analysis" {
			t.Fatalf("unexpected runner context: %#v", payload["context"])
		}
		if contextPayload["analysis_job_id"] != "analysis-1" || contextPayload["traceparent"] != "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01" {
			t.Fatalf("runner context did not preserve analysis correlation: %#v", contextPayload)
		}
		if _, ok := payload["output_schema"]; !ok {
			t.Fatal("expected strict output schema")
		}
		body, _ := json.Marshal(payload)
		if strings.Contains(string(body), "api-key") || strings.Contains(string(body), "password") {
			t.Fatalf("request contains secret-like content: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"runner-123","status":"accepted"}`))
	}))
	defer server.Close()

	client := NewWcodexRunnerClient(server.URL, server.Client())
	job, err := client.Submit(context.Background(), RunnerSubmitRequest{
		Prompt:        "analyze normalized post",
		Application:   "roroadmap",
		Operation:     "social-media-analysis",
		Timeout:       120,
		OutputSchema:  map[string]any{"type": "object"},
		AnalysisJobID: "analysis-1",
		TraceParent:   "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01",
	})
	if err != nil {
		t.Fatalf("submit runner job: %v", err)
	}
	if job.ID != "runner-123" || job.Status != RunnerJobStatusAccepted {
		t.Fatalf("unexpected runner job: %#v", job)
	}
}

func TestWcodexRunnerClientGetDecodesTerminalJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/jobs/runner-123" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"runner-123","status":"done","result":{"recommendation":"needs_input"}}`))
	}))
	defer server.Close()

	client := NewWcodexRunnerClient(server.URL, server.Client())
	job, err := client.Get(context.Background(), "runner-123")
	if err != nil {
		t.Fatalf("get runner job: %v", err)
	}
	if job.Status != RunnerJobStatusDone || string(job.Result) != `{"recommendation":"needs_input"}` {
		t.Fatalf("unexpected runner response: %#v", job)
	}
}

func TestValidateAgentAnalysisResultRejectsUnknownRecommendation(t *testing.T) {
	if err := ValidateAgentAnalysisResult(AgentAnalysisResult{Recommendation: "do_magic"}); err == nil {
		t.Fatal("expected unknown recommendation error")
	}
	if err := ValidateAgentAnalysisResult(AgentAnalysisResult{Recommendation: AgentRecommendationNeedsInput, Confidence: 0.8}); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

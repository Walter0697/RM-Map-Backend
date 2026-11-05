package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSocialMediaAnalysisResponseDoesNotExposeInternalFields(t *testing.T) {
	response := SocialMediaAnalysisResponse{
		JobID:       "analysis_123",
		Status:      "completed",
		Stage:       "validated",
		RunnerJobID: "runner_456",
		Prompt:      "secret prompt",
		Result:      json.RawMessage(`{"recommendation":"needs_input"}`),
	}

	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	body := string(payload)
	for _, forbidden := range []string{"runner_456", "secret prompt", "runner_job_id", "prompt"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaked internal field %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"job_id":"analysis_123"`) {
		t.Fatalf("response omitted public job ID: %s", body)
	}
}

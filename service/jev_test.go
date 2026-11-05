package service

import (
	"context"
	"testing"
)

func TestNoopJevClientDoesNotMakeJevRequired(t *testing.T) {
	client := NewJevClient("")
	decision, err := client.Decide(context.Background(), JevRequest{JobID: "analysis-1"})
	if err != nil {
		t.Fatalf("noop Jev decision returned error: %v", err)
	}
	if decision.Configured {
		t.Fatalf("expected Jev to remain unconfigured")
	}
}

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"

	"mapmarker/backend/config"
)

type observabilityTestSecretResolver struct {
	values map[string]string
}

func (resolver observabilityTestSecretResolver) Resolve(_ context.Context, name string) (string, error) {
	return resolver.values[name], nil
}

func TestSocialAnalysisTracerExportsPhoenixProjectAndAttributes(t *testing.T) {
	var mu sync.Mutex
	var payload []byte
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		payload = append([]byte(nil), body...)
		authorization = r.Header.Get("Authorization")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tracer, err := NewSocialAnalysisTracer(config.PhoenixSetting{
		Enable:    true,
		Endpoint:  server.URL,
		Project:   "roroadmap-social-analysis",
		APIKey:    "phoenix-key",
		BatchSize: 1,
		TimeoutMS: 1000,
	}, observabilityTestSecretResolver{})
	if err != nil {
		t.Fatalf("NewSocialAnalysisTracer returned error: %v", err)
	}
	_, span := tracer.Start(context.Background(), "apify.fetch", map[string]string{
		"analysis.job_id": "analysis-123",
		"provider.name":   "apify",
		"api.key":         "must-not-be-exported",
	})
	span.End()
	if err := tracer.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if authorization != "Bearer phoenix-key" {
		t.Fatalf("expected Phoenix authorization header, got %q", authorization)
	}
	decoded := &v1.ExportTraceServiceRequest{}
	if err := proto.Unmarshal(payload, decoded); err != nil {
		t.Fatalf("decode OTLP payload: %v", err)
	}
	if len(decoded.ResourceSpans) != 1 || len(decoded.ResourceSpans[0].Resource.Attributes) == 0 {
		t.Fatalf("expected resource attributes in OTLP payload: %#v", decoded)
	}
	encoded := string(payload)
	for _, expected := range []string{"roroadmap-social-analysis", "analysis-123", "apify"} {
		if !strings.Contains(encoded, expected) {
			t.Fatalf("OTLP payload missing %q", expected)
		}
	}
	if strings.Contains(encoded, "must-not-be-exported") {
		t.Fatal("OTLP payload exposed a secret-like attribute")
	}
}

func TestSocialAnalysisTracerDisabledIsNoop(t *testing.T) {
	tracer, err := NewSocialAnalysisTracer(config.PhoenixSetting{}, nil)
	if err != nil {
		t.Fatalf("NewSocialAnalysisTracer returned error: %v", err)
	}
	_, span := tracer.Start(context.Background(), "disabled", nil)
	span.SetAttribute("analysis.job_id", "analysis-123")
	span.End()
	if err := tracer.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
}

func TestSocialAnalysisTracerExporterFailureDoesNotPropagate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	tracer, err := NewSocialAnalysisTracer(config.PhoenixSetting{Enable: true, Endpoint: server.URL, BatchSize: 1, TimeoutMS: 100}, nil)
	if err != nil {
		t.Fatalf("NewSocialAnalysisTracer returned error: %v", err)
	}
	_, span := tracer.Start(context.Background(), "provider", nil)
	span.End()
	if err := tracer.Shutdown(context.Background()); err != nil {
		t.Fatalf("export failure should not propagate from Shutdown: %v", err)
	}
}

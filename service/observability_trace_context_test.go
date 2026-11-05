package service

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestSocialAnalysisTraceParentRoundTripsTraceAndSpanContext(t *testing.T) {
	original := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{16, 15, 14, 13, 12, 11, 10, 9},
		TraceFlags: trace.FlagsSampled,
		Remote:     false,
	})
	parent := trace.ContextWithSpanContext(context.Background(), original)
	traceParent := injectSocialAnalysisTraceParent(parent)
	resumed := extractSocialAnalysisTraceParent(context.Background(), traceParent)

	got := trace.SpanContextFromContext(resumed)
	if got.TraceID() != original.TraceID() || got.SpanID() != original.SpanID() || got.TraceFlags() != original.TraceFlags() {
		t.Fatalf("trace context changed during round trip: got %s/%s flags=%s, want %s/%s flags=%s", got.TraceID(), got.SpanID(), got.TraceFlags(), original.TraceID(), original.SpanID(), original.TraceFlags())
	}
}

func TestExtractSocialAnalysisTraceParentIgnoresMalformedValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "unchanged")
	resumed := extractSocialAnalysisTraceParent(ctx, "not-a-traceparent")
	if resumed != ctx {
		t.Fatal("expected malformed traceparent to leave context unchanged")
	}
}

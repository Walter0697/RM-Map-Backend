package service

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"mapmarker/backend/database/dbmodel"
)

type traceContextRecordingTracer struct {
	traceIDs []trace.TraceID
}

func (tracer *traceContextRecordingTracer) Start(ctx context.Context, _ string, _ map[string]string) (context.Context, SpanHandle) {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		spanContext = trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    trace.TraceID{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36},
			SpanID:     trace.SpanID{36, 35, 34, 33, 32, 31, 30, 29},
			TraceFlags: trace.FlagsSampled,
		})
	}
	tracer.traceIDs = append(tracer.traceIDs, spanContext.TraceID())
	return trace.ContextWithSpanContext(ctx, spanContext), noopSpanHandle{}
}

func TestStartSocialAnalysisRunReusesPersistedTraceID(t *testing.T) {
	tracer := &traceContextRecordingTracer{}
	firstJob := &dbmodel.SocialMediaAnalysisJob{JobID: "analysis-1"}
	firstContext, firstSpan := startSocialAnalysisRun(context.Background(), firstJob, tracer)
	firstSpan.End()
	if firstJob.TraceParent == "" {
		t.Fatal("expected first run to persist traceparent")
	}

	secondJob := &dbmodel.SocialMediaAnalysisJob{JobID: firstJob.JobID, TraceParent: firstJob.TraceParent}
	secondContext, secondSpan := startSocialAnalysisRun(context.Background(), secondJob, tracer)
	secondSpan.End()
	if len(tracer.traceIDs) != 2 || tracer.traceIDs[0] != tracer.traceIDs[1] {
		t.Fatalf("expected resumed run to reuse trace ID, got %v", tracer.traceIDs)
	}
	if trace.SpanContextFromContext(firstContext).TraceID() != trace.SpanContextFromContext(secondContext).TraceID() {
		t.Fatal("expected worker contexts to share the trace ID")
	}
}

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

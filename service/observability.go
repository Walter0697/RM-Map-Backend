package service

import (
	"context"
	"log"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"mapmarker/backend/config"
)

const socialAnalysisTracerName = "mapmarker/backend/social-analysis"

type SpanHandle interface {
	SetAttribute(string, string)
	AddEvent(string)
	RecordError(error)
	SetStatus(bool, string)
	End()
}

type SocialAnalysisTracer struct {
	provider *sdktrace.TracerProvider
	tracer   oteltrace.Tracer
}

type noopSpanHandle struct{}

func (noopSpanHandle) SetAttribute(string, string) {}
func (noopSpanHandle) AddEvent(string)             {}
func (noopSpanHandle) RecordError(error)           {}
func (noopSpanHandle) SetStatus(bool, string)      {}
func (noopSpanHandle) End()                        {}

func NewSocialAnalysisTracer(setting config.PhoenixSetting, secrets SecretResolver) (*SocialAnalysisTracer, error) {
	if !setting.Enable {
		return &SocialAnalysisTracer{}, nil
	}
	endpoint := strings.TrimRight(strings.TrimSpace(setting.Endpoint), "/")
	if endpoint == "" {
		return &SocialAnalysisTracer{}, nil
	}
	apiKey := strings.TrimSpace(setting.APIKey)
	if apiKey == "" && secrets != nil {
		resolved, err := secrets.Resolve(context.Background(), "PHOENIX_API_KEY")
		if err != nil {
			log.Printf("[observability] Phoenix API key unavailable; tracing disabled: %v", err)
			return &SocialAnalysisTracer{}, nil
		}
		apiKey = strings.TrimSpace(resolved)
	}
	if apiKey == "" {
		log.Printf("[observability] Phoenix API key is not configured; tracing disabled")
		return &SocialAnalysisTracer{}, nil
	}
	traceEndpoint := endpoint
	if !strings.HasSuffix(traceEndpoint, "/v1/traces") {
		traceEndpoint += "/v1/traces"
	}
	timeout := time.Duration(setting.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	batchSize := setting.BatchSize
	if batchSize <= 0 {
		batchSize = 64
	}
	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpointURL(traceEndpoint),
		otlptracehttp.WithHeaders(map[string]string{"Authorization": "Bearer " + apiKey}),
		otlptracehttp.WithTimeout(timeout),
	)
	if err != nil {
		log.Printf("[observability] Phoenix exporter unavailable; tracing disabled: %v", err)
		return &SocialAnalysisTracer{}, nil
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithMaxExportBatchSize(batchSize),
			sdktrace.WithBatchTimeout(timeout),
		),
		sdktrace.WithResource(resource.NewWithAttributes(
			"https://opentelemetry.io/schemas/1.21.0",
			attribute.String("service.name", "roroadmap-backend"),
			attribute.String("openinference.project.name", setting.Project),
			attribute.String("phoenix.project.name", setting.Project),
		)),
	)
	return &SocialAnalysisTracer{provider: provider, tracer: provider.Tracer(socialAnalysisTracerName)}, nil
}

func (tracer *SocialAnalysisTracer) Start(ctx context.Context, name string, attributes map[string]string) (context.Context, SpanHandle) {
	if tracer == nil || tracer.tracer == nil {
		return ctx, noopSpanHandle{}
	}
	otelAttributes := make([]attribute.KeyValue, 0, len(attributes))
	for key, value := range attributes {
		if isSensitiveObservabilityAttribute(key) {
			continue
		}
		otelAttributes = append(otelAttributes, attribute.String(key, value))
	}
	spanContext, span := tracer.tracer.Start(ctx, name, oteltrace.WithAttributes(otelAttributes...))
	return spanContext, &otelSpanHandle{span: span}
}

func (tracer *SocialAnalysisTracer) Shutdown(ctx context.Context) error {
	if tracer == nil || tracer.provider == nil {
		return nil
	}
	if err := tracer.provider.Shutdown(ctx); err != nil {
		log.Printf("[observability] Phoenix export failed during shutdown: %v", err)
	}
	return nil
}

type otelSpanHandle struct {
	span oteltrace.Span
}

func (span *otelSpanHandle) SetAttribute(key, value string) {
	if span != nil && span.span != nil && !isSensitiveObservabilityAttribute(key) {
		span.span.SetAttributes(attribute.String(key, value))
	}
}

func (span *otelSpanHandle) AddEvent(name string) {
	if span != nil && span.span != nil {
		span.span.AddEvent(name)
	}
}

func (span *otelSpanHandle) RecordError(err error) {
	if span != nil && span.span != nil && err != nil {
		span.span.RecordError(err)
	}
}

func (span *otelSpanHandle) SetStatus(ok bool, description string) {
	if span == nil || span.span == nil {
		return
	}
	if ok {
		span.span.SetStatus(codes.Ok, description)
		return
	}
	span.span.SetStatus(codes.Error, description)
}

func (span *otelSpanHandle) End() {
	if span != nil && span.span != nil {
		span.span.End()
	}
}

func isSensitiveObservabilityAttribute(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, marker := range []string{"key", "token", "secret", "password", "authorization", "credential"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

func init() {
	otel.SetTracerProvider(otel.GetTracerProvider())
}

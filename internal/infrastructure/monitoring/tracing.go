// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package monitoring

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	noop "go.opentelemetry.io/otel/trace/noop"
)

const tracerName = "green-api-mcp-gateway"

// TracerProvider wraps the OTel SDK TracerProvider for lifecycle management.
type TracerProvider struct {
	provider *sdktrace.TracerProvider
}

// NewTracerProvider initialises an OTel TracerProvider.
// When enabled=false it installs a no-op provider so callers need no special
// handling — traces simply produce no output.
func NewTracerProvider(enabled bool, serviceName string) *TracerProvider {
	if !enabled {
		// Install global no-op provider.
		otel.SetTracerProvider(noop.NewTracerProvider())
		return &TracerProvider{}
	}

	if serviceName == "" {
		serviceName = "green-api-mcp-gateway"
	}

	exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Printf("[tracing] failed to create stdout exporter: %v — tracing disabled", err)
		otel.SetTracerProvider(noop.NewTracerProvider())
		return &TracerProvider{}
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		res = resource.Default()
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		// Sample every request in dev; swap for ParentBased(TraceIDRatioBased(0.1)) in prod.
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	log.Printf("[tracing] OpenTelemetry tracing enabled (stdout exporter), service=%s", serviceName)

	return &TracerProvider{provider: tp}
}

// Shutdown flushes and closes the underlying SDK provider.
// Safe to call when provider is nil (no-op case).
func (tp *TracerProvider) Shutdown(ctx context.Context) {
	if tp == nil || tp.provider == nil {
		return
	}
	if err := tp.provider.Shutdown(ctx); err != nil {
		log.Printf("[tracing] shutdown error: %v", err)
	}
}

// Tracer returns the global tracer for the given component name.
// Callers that do not want a custom name can use StartToolSpan / StartAPISpan helpers.
func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// StartToolSpan creates an OTel span for an MCP tool call.
// Usage:
//
//	ctx, span := monitoring.StartToolSpan(ctx, "whatsapp_send_message", instanceID)
//	defer span.End()
func StartToolSpan(ctx context.Context, toolName string, instanceID uint64) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, "mcp.tool."+toolName,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("mcp.tool.name", toolName),
			attribute.Int64("greenapi.instance_id", int64(instanceID)),
		),
	)
	return ctx, span
}

// StartAPISpan creates an OTel span for an outbound Green API call.
// Usage:
//
//	ctx, span := monitoring.StartAPISpan(ctx, "sendMessage", instanceID)
//	defer span.End()
func StartAPISpan(ctx context.Context, method string, instanceID uint64) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, "greenapi."+method,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("greenapi.method", method),
			attribute.Int64("greenapi.instance_id", int64(instanceID)),
		),
	)
	return ctx, span
}

// EndSpanWithError records an error on the span and marks it as error status,
// then ends the span. Convenient for defer-based patterns:
//
//	defer monitoring.EndSpanWithError(span, &err)
func EndSpanWithError(span trace.Span, errPtr *error) {
	if errPtr != nil && *errPtr != nil {
		span.RecordError(*errPtr)
		span.SetStatus(codes.Error, (*errPtr).Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

// Package monitoring provides Prometheus metrics and OpenTelemetry tracing
// for the Green API MCP Gateway.
package monitoring

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metric collectors for the gateway.
type Metrics struct {
	// ToolCallsTotal counts MCP tool invocations, labelled by tool name and status.
	ToolCallsTotal *prometheus.CounterVec

	// ToolCallDuration observes how long each MCP tool call takes (seconds).
	ToolCallDuration *prometheus.HistogramVec

	// APIRequestsTotal counts outbound Green API HTTP requests.
	APIRequestsTotal *prometheus.CounterVec

	// APIRequestDuration observes outbound Green API request latency (seconds).
	APIRequestDuration *prometheus.HistogramVec

	// ErrorsTotal counts all errors, labelled by component and error kind.
	ErrorsTotal *prometheus.CounterVec

	// ActiveInstances is a gauge tracking how many Green API instances are registered.
	ActiveInstances prometheus.Gauge

	// ActiveWebhookConnections tracks the number of active webhook polling goroutines.
	ActiveWebhookConnections prometheus.Gauge

	// RateLimitedTotal counts rate limited requests, labeled by instance_id.
	RateLimitedTotal *prometheus.CounterVec
}

// NewMetrics registers and returns all Prometheus metrics.
// Uses promauto so that registration happens once on package init.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	factory := promauto.With(reg)

	return &Metrics{
		ToolCallsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "greenapi_mcp",
			Name:      "tool_calls_total",
			Help:      "Total number of MCP tool calls, labelled by tool name and status (success|error).",
		}, []string{"tool", "status"}),

		ToolCallDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "greenapi_mcp",
			Name:      "tool_call_duration_seconds",
			Help:      "Latency of MCP tool calls in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"tool"}),

		APIRequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "greenapi_mcp",
			Name:      "api_requests_total",
			Help:      "Total outbound requests to the Green API, labelled by method and status.",
		}, []string{"method", "status"}),

		APIRequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "greenapi_mcp",
			Name:      "api_request_duration_seconds",
			Help:      "Latency of outbound Green API requests in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method"}),

		ErrorsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "greenapi_mcp",
			Name:      "errors_total",
			Help:      "Total number of errors by component and kind.",
		}, []string{"component", "kind"}),

		ActiveInstances: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: "greenapi_mcp",
			Name:      "active_instances",
			Help:      "Number of registered Green API instances.",
		}),

		ActiveWebhookConnections: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: "greenapi_mcp",
			Name:      "active_webhook_connections",
			Help:      "Current number of active webhook polling goroutines.",
		}),

		RateLimitedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "mcp_gateway",
			Name:      "rate_limited_total",
			Help:      "Total number of rate limited requests, labeled by instance_id.",
		}, []string{"instance_id"}),
	}
}

// RecordToolCall is a convenience helper that records duration and increments
// the call counter. Call at the end of a tool handler:
//
//	defer m.RecordToolCall(toolName, start, err)
func (m *Metrics) RecordToolCall(tool string, start time.Time, err error) {
	m.ToolCallDuration.WithLabelValues(tool).Observe(time.Since(start).Seconds())
	status := "success"
	if err != nil {
		status = "error"
		m.ErrorsTotal.WithLabelValues("tool", tool).Inc()
	}
	m.ToolCallsTotal.WithLabelValues(tool, status).Inc()
}

// SetActiveWebhookConnections updates the active webhook connections gauge.
func (m *Metrics) SetActiveWebhookConnections(count float64) {
	m.ActiveWebhookConnections.Set(count)
}

// RecordRateLimit records a rate limited request for the given instance.
func (m *Metrics) RecordRateLimit(instanceID uint64) {
	m.RateLimitedTotal.WithLabelValues(fmt.Sprintf("%d", instanceID)).Inc()
}

// RecordAPIRequest records an outbound API call result.
func (m *Metrics) RecordAPIRequest(method string, start time.Time, err error) {
	m.APIRequestDuration.WithLabelValues(method).Observe(time.Since(start).Seconds())
	status := "success"
	if err != nil {
		status = "error"
		m.ErrorsTotal.WithLabelValues("api", method).Inc()
	}
	m.APIRequestsTotal.WithLabelValues(method, status).Inc()
}

// NoopMetricsProvider is a no-op MetricsProvider used when metrics are disabled.
// It satisfies the application.MetricsProvider interface.
type NoopMetricsProvider struct{}

func (NoopMetricsProvider) RecordToolCall(_ string, _ time.Time, _ error)   {}
func (NoopMetricsProvider) RecordAPIRequest(_ string, _ time.Time, _ error) {}
func (NoopMetricsProvider) SetActiveWebhookConnections(_ float64)           {}
func (NoopMetricsProvider) RecordRateLimit(_ uint64)                        {}

// MetricsServer is a lightweight HTTP server that exposes /metrics.
type MetricsServer struct {
	server *http.Server
}

// NewMetricsServer creates a MetricsServer on the given port using the
// provided (or default) Prometheus registry.
func NewMetricsServer(port int, reg prometheus.Gatherer) *MetricsServer {
	if reg == nil {
		reg = prometheus.DefaultGatherer
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return &MetricsServer{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}
}

// Start launches the metrics HTTP server in a background goroutine.
// It stops automatically when ctx is cancelled.
func (ms *MetricsServer) Start(ctx context.Context) {
	go func() {
		log.Printf("[metrics] Prometheus /metrics listening on %s", ms.server.Addr)
		if err := ms.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[metrics] server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := ms.server.Shutdown(shutCtx); err != nil {
			log.Printf("[metrics] shutdown error: %v", err)
		}
	}()
}

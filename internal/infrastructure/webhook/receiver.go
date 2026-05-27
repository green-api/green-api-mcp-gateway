// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

// Package webhook implements the WebhookBridge port for receiving Green API
// notifications via HTTP push (receiver mode).
//
// In receiver mode an HTTP server listens for POST requests on
// /webhook/{instanceId}. Each request body is expected to be a JSON object
// matching the notification format produced by your webhook producer (identical
// to the body returned by Green API's receiveNotification endpoint).
// The notification is forwarded to the registered NotificationHandler and an
// HTTP 200 OK is returned to the caller.
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/green-api/green-api-mcp-gateway/internal/domain"
)

// Receiver is an HTTP server that accepts webhook pushes from your webhook
// producer and forwards them to a NotificationHandler. It implements
// application.WebhookBridge.
type Receiver struct {
	port    int
	handler NotificationHandler

	server   *http.Server
	mu       sync.Mutex
	stopOnce sync.Once
	doneCh   chan struct{}
}

// NewReceiver creates a Receiver that will listen on the given TCP port and
// dispatch incoming webhook payloads to handler.
func NewReceiver(port int, handler NotificationHandler) *Receiver {
	return &Receiver{
		port:    port,
		handler: handler,
		doneCh:  make(chan struct{}),
	}
}

// Start launches the HTTP server in a background goroutine. It returns once
// the listener is ready (or immediately with an error if the port is
// unavailable). The server runs until ctx is cancelled or Stop is called.
func (r *Receiver) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/", r.handleWebhook)

	r.mu.Lock()
	r.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", r.port),
		Handler: mux,
	}
	srv := r.server
	r.mu.Unlock()

	// Create listener eagerly so we can return an error synchronously if the
	// port is already in use.
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("webhook receiver: listen %s: %w", srv.Addr, err)
	}

	go func() {
		defer close(r.doneCh)
		log.Printf("[webhook/receiver] listening on %s", srv.Addr)
		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Printf("[webhook/receiver] server error: %v", serveErr)
		}
		log.Println("[webhook/receiver] server stopped")
	}()

	// Honour context cancellation.
	go func() {
		select {
		case <-ctx.Done():
			r.Stop()
		case <-r.doneCh:
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server. Safe to call multiple times.
func (r *Receiver) Stop() {
	r.stopOnce.Do(func() {
		r.mu.Lock()
		srv := r.server
		r.mu.Unlock()

		if srv != nil {
			if err := srv.Close(); err != nil {
				log.Printf("[webhook/receiver] shutdown error: %v", err)
			}
		}
		// Wait for the serve goroutine to exit.
		<-r.doneCh
		log.Println("[webhook/receiver] stopped")
	})
}

// handleWebhook handles POST /webhook/{instanceId}.
func (r *Receiver) handleWebhook(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract instanceId from the path: /webhook/<instanceId>
	// TrimPrefix removes the leading "/webhook/" leaving only the id segment.
	idStr := strings.TrimPrefix(req.URL.Path, "/webhook/")
	// Reject empty or nested paths.
	if idStr == "" || strings.Contains(idStr, "/") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	instanceID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid instanceId", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("[webhook/receiver] instance %d: read body error: %v", instanceID, err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer func() { _ = req.Body.Close() }()

	// The payload from your webhook producer is the raw notification JSON. We wrap
	// it in a NotificationBody so the handler signature matches the polling bridge.
	notification := &domain.NotificationBody{
		Body: json.RawMessage(body),
	}

	if r.handler != nil {
		r.handler(instanceID, notification)
	}

	log.Printf("[webhook/receiver] instance %d: notification received and dispatched", instanceID)
	w.WriteHeader(http.StatusOK)
}

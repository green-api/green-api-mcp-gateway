// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

// Package webhook implements the WebhookBridge port for receiving Green API
// notifications via long-polling (polling mode).
//
// One goroutine is spawned per registered instance. Each goroutine issues a
// blocking ReceiveNotification call (timeout handled server-side, 20 s by
// default). When a notification arrives it is forwarded to the registered
// NotificationHandler and then deleted from the queue via DeleteNotification.
// All goroutines honour context cancellation for graceful shutdown.
package webhook

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/green-api/green-api-mcp-gateway/internal/application"
	"github.com/green-api/green-api-mcp-gateway/internal/domain"
)

// NotificationHandler is called for every notification received from Green API.
// The implementation must not block — offload heavy work to a separate goroutine
// if needed.
type NotificationHandler func(instanceID uint64, notification *domain.NotificationBody)

// Bridge polls all registered Green API instances for new notifications and
// dispatches them to a NotificationHandler. It implements application.WebhookBridge.
type Bridge struct {
	credentials application.CredentialStore
	client      application.WhatsAppClient
	handler     NotificationHandler
	metrics     application.MetricsProvider

	// retryDelay controls how long to wait after an error before the next poll.
	retryDelay time.Duration

	// activeConnections counts running polling goroutines for metrics.
	activeConnections int64

	cancel   context.CancelFunc
	wg       sync.WaitGroup
	mu       sync.Mutex
	stopOnce sync.Once
}

// NewBridge creates a Bridge with the given credentials store, WhatsApp client
// and notification handler. retryDelay is the back-off between error retries;
// pass 0 to use the default of 5 s.
func NewBridge(
	credentials application.CredentialStore,
	client application.WhatsAppClient,
	handler NotificationHandler,
	metrics application.MetricsProvider,
	retryDelay time.Duration,
) *Bridge {
	if retryDelay <= 0 {
		retryDelay = 5 * time.Second
	}
	return &Bridge{
		credentials: credentials,
		client:      client,
		handler:     handler,
		metrics:     metrics,
		retryDelay:  retryDelay,
	}
}

// Start launches one polling goroutine per registered instance. It returns
// immediately; the goroutines run until ctx is cancelled or Stop is called.
func (b *Bridge) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	ctx, b.cancel = context.WithCancel(ctx)

	instances := b.credentials.ListInstances()
	if len(instances) == 0 {
		log.Println("[webhook/bridge] no instances registered — bridge idle")
		return nil
	}

	for _, id := range instances {
		instanceID := id // capture for closure
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			count := atomic.AddInt64(&b.activeConnections, 1)
			b.metrics.SetActiveWebhookConnections(float64(count))
			defer func() {
				c := atomic.AddInt64(&b.activeConnections, -1)
				b.metrics.SetActiveWebhookConnections(float64(c))
			}()
			b.pollInstance(ctx, instanceID)
		}()
		log.Printf("[webhook/bridge] polling started for instance %d", instanceID)
	}
	return nil
}

// Stop signals all polling goroutines to stop and waits for them to finish.
// It is safe to call Stop more than once.
func (b *Bridge) Stop() {
	b.stopOnce.Do(func() {
		b.mu.Lock()
		cancel := b.cancel
		b.mu.Unlock()

		if cancel != nil {
			cancel()
		}
		b.wg.Wait()
		log.Println("[webhook/bridge] all polling goroutines stopped")
	})
}

// pollInstance is the per-instance polling loop. It runs until ctx is cancelled.
func (b *Bridge) pollInstance(ctx context.Context, instanceID uint64) {
	for {
		// Check for shutdown before every iteration.
		select {
		case <-ctx.Done():
			log.Printf("[webhook/bridge] instance %d: context cancelled, stopping", instanceID)
			return
		default:
		}

		notification, err := b.client.ReceiveNotification(ctx, instanceID)
		if err != nil {
			// Context was cancelled — clean exit.
			if ctx.Err() != nil {
				log.Printf("[webhook/bridge] instance %d: stopping (context done)", instanceID)
				return
			}
			log.Printf("[webhook/bridge] instance %d: ReceiveNotification error: %v — retrying in %s",
				instanceID, err, b.retryDelay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(b.retryDelay):
			}
			continue
		}

		// nil response means the queue was empty (long-poll timed out).
		if notification == nil {
			continue
		}

		// Dispatch to the handler.
		if b.handler != nil {
			b.handler(instanceID, notification)
		}

		// Acknowledge / remove the notification from the queue.
		if err := b.client.DeleteNotification(ctx, instanceID, notification.ReceiptID); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[webhook/bridge] instance %d: DeleteNotification(%d) error: %v",
				instanceID, notification.ReceiptID, err)
			// Continue polling — the notification will be re-delivered but that
			// is preferable to silently dropping it.
		}
	}
}

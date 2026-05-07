package services

import (
	"log/slog"
	"time"

	emailqueue "github.com/denisvmedia/inventario/email/queue"
	emailqueueinmemory "github.com/denisvmedia/inventario/email/queue/inmemory"
	emailqueueredis "github.com/denisvmedia/inventario/email/queue/redis"
)

type emailJob struct {
	ID           string            `json:"id"`
	TemplateType emailTemplateType `json:"template_type"`
	To           string            `json:"to"`
	Name         string            `json:"name,omitempty"`
	URL          string            `json:"url,omitempty"`
	ChangedAt    *time.Time        `json:"changed_at,omitempty"`
	Attempt      int               `json:"attempt"`
	CreatedAt    time.Time         `json:"created_at"`
	// Warranty-reminder fields. Optional and only populated by
	// AsyncEmailService.SendWarrantyReminderEmail — every other template
	// ignores them. Keep them on the job rather than introducing a free-form
	// `Data map[string]string` so the wire shape stays JSON-typed.
	CommodityName string `json:"commodity_name,omitempty"`
	CommodityURL  string `json:"commodity_url,omitempty"`
	ExpiryDate    string `json:"expiry_date,omitempty"`
	ThresholdDays int    `json:"threshold_days,omitempty"`
}

// newEmailQueue selects Redis-backed queueing when configured; otherwise it
// falls back to an in-memory queue suitable for single-process environments.
func newEmailQueue(redisURL string) emailqueue.Queue {
	if redisURL == "" {
		slog.Warn("Using in-memory email queue — not suitable for multi-instance deployments; set --email-queue-redis-url for production")
		return emailqueueinmemory.New(1024)
	}

	q, err := emailqueueredis.NewFromConfig(emailqueueredis.Config{
		RedisURL: redisURL,
	})
	if err != nil {
		slog.Error("Failed to create Redis email queue, falling back to in-memory", "error", err)
		return emailqueueinmemory.New(1024)
	}

	slog.Info("Using Redis-backed email queue")
	return q
}

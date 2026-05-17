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
	// Group-invite fields. Optional and only populated by
	// AsyncEmailService.SendGroupInviteEmail. See above re: free-form Data.
	InviterName string     `json:"inviter_name,omitempty"`
	GroupName   string     `json:"group_name,omitempty"`
	Role        string     `json:"role,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	// Storage-quota fields. Optional and only populated by
	// AsyncEmailService.SendStorageQuotaWarningEmail. GroupName
	// piggybacks on the group-invite field above. ThresholdPercent
	// is the matched tier (e.g. 90); UsagePercent is the rounded
	// actual percentage at send time (>= threshold). Human strings
	// are pre-formatted (e.g. "135 MiB") so the renderer doesn't
	// have to redo unit conversion.
	ThresholdPercent      int      `json:"threshold_percent,omitempty"`
	UsagePercent          int      `json:"usage_percent,omitempty"`
	StorageUsedHuman      string   `json:"storage_used_human,omitempty"`
	StorageQuotaHuman     string   `json:"storage_quota_human,omitempty"`
	StorageBreakdownLines []string `json:"storage_breakdown_lines,omitempty"`
	StorageFilesURL       string   `json:"storage_files_url,omitempty"`
	StorageSettingsURL    string   `json:"storage_settings_url,omitempty"`
	// Loan-reminder fields (#1509). Populated only by
	// AsyncEmailService.SendLoanReminderEmail. LoanKind is the
	// LoanReminderKind ("overdue"|"due_soon") which the renderer maps
	// to kind-aware subject + body copy. LoanDaysDelta is the positive
	// magnitude (days-until-due for due_soon, days-overdue for overdue).
	BorrowerName  string `json:"borrower_name,omitempty"`
	LentAt        string `json:"lent_at,omitempty"`
	DueBackAt     string `json:"due_back_at,omitempty"`
	LoanKind      string `json:"loan_kind,omitempty"`
	LoanDaysDelta int    `json:"loan_days_delta,omitempty"`
	// Feedback fields (#1387). Populated only by
	// AsyncEmailService.SendFeedbackEmail. FeedbackType is the human
	// label ("Bug", "Feature request", etc.); FromName/FromEmail/FromUserID
	// identify the submitter; ReplyToEmail is the optional reply-to;
	// FeedbackMessage is the free-form body (kept under a long-form
	// field name to avoid colliding with the existing Name field used
	// by the other transactional templates); DiagnosticsLines is the
	// pre-formatted bullet list the FE opts into.
	FeedbackType     string   `json:"feedback_type,omitempty"`
	FromName         string   `json:"from_name,omitempty"`
	FromEmail        string   `json:"from_email,omitempty"`
	FromUserID       string   `json:"from_user_id,omitempty"`
	ReplyToEmail     string   `json:"reply_to_email,omitempty"`
	FeedbackMessage  string   `json:"feedback_message,omitempty"`
	DiagnosticsLines []string `json:"diagnostics_lines,omitempty"`
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

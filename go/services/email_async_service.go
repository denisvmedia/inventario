package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	emailqueue "github.com/denisvmedia/inventario/email/queue"
	mailsender "github.com/denisvmedia/inventario/email/sender"
)

// AsyncEmailService is the orchestration layer between business flows and provider
// transports.
//
// Delivery pipeline:
//  1. API call enqueues a logical email job.
//  2. Worker goroutines dequeue jobs and render templates.
//  3. Rendered messages are sent through the selected provider sender.
//  4. Failures are re-scheduled with exponential backoff up to maxRetries.
//
// This separation keeps handlers fast and centralizes retry policy in one place.
type AsyncEmailService struct {
	queue    emailqueue.Queue
	renderer *emailTemplateRenderer
	sender   emailSender

	from    string
	replyTo string

	workers           int
	maxRetries        int
	queuePopTimeout   time.Duration
	retryPollInterval time.Duration
	retryBaseDelay    time.Duration
	retryMaxDelay     time.Duration
	sendTimeout       time.Duration

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewAsyncEmailService constructs a queue-backed EmailService using EmailConfig.
//
// It wires three composable components:
//   - queue backend (Redis or in-memory),
//   - template renderer (embedded templates),
//   - provider sender implementation.
func NewAsyncEmailService(cfg EmailConfig) (*AsyncEmailService, error) {
	cfg.normalize()

	renderer, err := newEmailTemplateRenderer()
	if err != nil {
		return nil, fmt.Errorf("create email template renderer: %w", err)
	}

	sender, err := newEmailSenderFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create email sender: %w", err)
	}

	if cfg.Provider != EmailProviderStub && strings.TrimSpace(cfg.From) == "" {
		return nil, errors.New("email FROM address is required for non-stub providers")
	}

	return &AsyncEmailService{
		queue:             newEmailQueue(cfg.QueueRedisURL),
		renderer:          renderer,
		sender:            sender,
		from:              strings.TrimSpace(cfg.From),
		replyTo:           strings.TrimSpace(cfg.ReplyTo),
		workers:           cfg.QueueWorkers,
		maxRetries:        cfg.QueueMaxRetry,
		queuePopTimeout:   cfg.QueuePopTimeout,
		retryPollInterval: cfg.RetryPollInterval,
		retryBaseDelay:    cfg.RetryBaseDelay,
		retryMaxDelay:     cfg.RetryMaxDelay,
		sendTimeout:       cfg.SendTimeout,
		stopCh:            make(chan struct{}),
	}, nil
}

// Start launches worker goroutines and the retry-promoter loop.
//
// Workers run until either the provided context is canceled or Stop is called.
func (s *AsyncEmailService) Start(ctx context.Context) {
	for i := 0; i < s.workers; i++ {
		workerID := i + 1
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runWorker(ctx, workerID)
		}()
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runRetryPromoter(ctx)
	}()

	slog.Info("Email service workers started",
		"workers", s.workers,
		"max_retries", s.maxRetries,
	)
}

// Stop terminates background workers and blocks until they exit.
//
// Stop is safe to call multiple times.
func (s *AsyncEmailService) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
	slog.Info("Email service workers stopped")
}

// SendVerificationEmail enqueues a verification email.
func (s *AsyncEmailService) SendVerificationEmail(ctx context.Context, to, name, verificationURL string) error {
	return s.enqueue(ctx, emailJob{
		TemplateType: emailTemplateVerification,
		To:           to,
		Name:         name,
		URL:          verificationURL,
	})
}

// SendPasswordResetEmail enqueues a password-reset email.
func (s *AsyncEmailService) SendPasswordResetEmail(ctx context.Context, to, name, resetURL string) error {
	return s.enqueue(ctx, emailJob{
		TemplateType: emailTemplatePasswordReset,
		To:           to,
		Name:         name,
		URL:          resetURL,
	})
}

// SendPasswordChangedEmail enqueues a password-changed security notification.
func (s *AsyncEmailService) SendPasswordChangedEmail(ctx context.Context, to, name string, changedAt time.Time) error {
	ts := changedAt
	return s.enqueue(ctx, emailJob{
		TemplateType: emailTemplatePasswordChange,
		To:           to,
		Name:         name,
		ChangedAt:    &ts,
	})
}

// SendWelcomeEmail enqueues a welcome email.
func (s *AsyncEmailService) SendWelcomeEmail(ctx context.Context, to, name string) error {
	return s.enqueue(ctx, emailJob{
		TemplateType: emailTemplateWelcome,
		To:           to,
		Name:         name,
	})
}

func (s *AsyncEmailService) enqueue(ctx context.Context, job emailJob) error {
	job.ID = uuid.NewString()
	job.To = strings.TrimSpace(job.To)
	job.Name = strings.TrimSpace(job.Name)
	job.CreatedAt = time.Now().UTC()
	job.Attempt = max(job.Attempt, 0)

	if job.To == "" {
		return errors.New("email recipient is required")
	}
	if _, ok := subjectByTemplateType(job.TemplateType); !ok {
		return fmt.Errorf("unsupported template type: %q", job.TemplateType)
	}

	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal email job: %w", err)
	}

	return s.queue.Enqueue(ctx, payload)
}

func (s *AsyncEmailService) runWorker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
		}

		payload, err := s.queue.Dequeue(ctx, s.queuePopTimeout)
		if err != nil {
			slog.Error("Email worker dequeue failed", "worker_id", workerID, "error", err)
			continue
		}
		if payload == nil {
			continue
		}

		var job emailJob
		if err := json.Unmarshal(payload, &job); err != nil {
			slog.Error("Email worker failed to decode job payload; dropping job",
				"worker_id", workerID,
				"error", err,
			)
			continue
		}
		s.processJob(job, workerID)
	}
}

func (s *AsyncEmailService) processJob(job emailJob, workerID int) {
	rendered, err := s.renderer.render(job)
	if err != nil {
		slog.Error("Email template rendering failed; dropping job",
			"worker_id", workerID,
			"job_id", job.ID,
			"template", job.TemplateType,
			"error", err,
		)
		return
	}

	sendCtx, cancel := context.WithTimeout(context.Background(), s.sendTimeout)
	defer cancel()

	err = s.sender.Send(sendCtx, mailsender.Message{
		To:      job.To,
		From:    s.from,
		ReplyTo: s.replyTo,
		Subject: rendered.Subject,
		HTML:    rendered.HTML,
		Text:    rendered.Text,
	})
	if err == nil {
		slog.Debug("Email sent",
			"worker_id", workerID,
			"job_id", job.ID,
			"template", job.TemplateType,
			"to", job.To,
		)
		return
	}

	nextAttempt := job.Attempt + 1
	if nextAttempt > s.maxRetries {
		slog.Error("Email delivery failed after retries; dropping job",
			"worker_id", workerID,
			"job_id", job.ID,
			"template", job.TemplateType,
			"to", job.To,
			"attempts", nextAttempt,
			"error", err,
		)
		return
	}

	job.Attempt = nextAttempt
	delay := s.retryDelay(nextAttempt)
	readyAt := time.Now().Add(delay)
	retryPayload, marshalErr := json.Marshal(job)
	if marshalErr != nil {
		slog.Error("Failed to encode email retry payload; dropping job",
			"worker_id", workerID,
			"job_id", job.ID,
			"template", job.TemplateType,
			"to", job.To,
			"attempt", nextAttempt,
			"send_error", err,
			"marshal_error", marshalErr,
		)
		return
	}

	if retryErr := s.queue.ScheduleRetry(context.Background(), retryPayload, readyAt); retryErr != nil {
		slog.Error("Failed to schedule email retry; dropping job",
			"worker_id", workerID,
			"job_id", job.ID,
			"template", job.TemplateType,
			"to", job.To,
			"attempt", nextAttempt,
			"send_error", err,
			"retry_error", retryErr,
		)
		return
	}

	slog.Warn("Email delivery failed; retry scheduled",
		"worker_id", workerID,
		"job_id", job.ID,
		"template", job.TemplateType,
		"to", job.To,
		"attempt", nextAttempt,
		"retry_in", delay.String(),
		"error", err,
	)
}

func (s *AsyncEmailService) retryDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return s.retryBaseDelay
	}

	delay := s.retryBaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= s.retryMaxDelay {
			return s.retryMaxDelay
		}
	}
	return min(delay, s.retryMaxDelay)
}

func (s *AsyncEmailService) runRetryPromoter(ctx context.Context) {
	ticker := time.NewTicker(s.retryPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			moved, err := s.queue.PromoteDueRetries(ctx, now, 200)
			if err != nil {
				slog.Error("Failed to promote due email retries", "error", err)
				continue
			}
			if moved > 0 {
				slog.Debug("Promoted due retry emails", "count", moved)
			}
		}
	}
}

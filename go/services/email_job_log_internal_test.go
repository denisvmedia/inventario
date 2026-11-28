package services

// White-box: the worker's log lines are written by the unexported processJob,
// and the only exported way to reach it is a running worker pool reading a
// real queue. Driving processJob directly with a sender that always fails
// makes the retry line appear exactly once, without timing.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	mailsender "go.5x5.cz/inventario/email/sender"
)

// failedDeliveryLine queues one email through send, fails its delivery once
// and returns the worker's "retry scheduled" log record.
func failedDeliveryLine(c *qt.C, send func(*AsyncEmailService) error) (record map[string]any, raw string) {
	c.Helper()
	renderer, err := newEmailTemplateRenderer()
	c.Assert(err, qt.IsNil)
	queue := &captureQueue{}
	svc := &AsyncEmailService{
		queue:          queue,
		renderer:       renderer,
		sender:         &flakyEmailSender{failUntil: 1},
		from:           "noreply@example.com",
		maxRetries:     3,
		retryBaseDelay: 10 * time.Millisecond,
		retryMaxDelay:  time.Second,
		sendTimeout:    time.Second,
	}
	c.Assert(send(svc), qt.IsNil)
	payloads := queue.EnqueuedPayloads()
	c.Assert(payloads, qt.HasLen, 1)
	var job emailJob
	c.Assert(json.Unmarshal(payloads[0], &job), qt.IsNil)

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	svc.processJob(context.Background(), job, 1)
	slog.SetDefault(prev)

	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		var rec map[string]any
		c.Assert(json.Unmarshal([]byte(line), &rec), qt.IsNil)
		if rec["msg"] == "Email delivery failed; retry scheduled" {
			return rec, buf.String()
		}
	}
	c.Fatalf("no retry line was logged: %s", buf.String())
	return nil, ""
}

func TestAsyncEmailService_RetryLineOfARecoveryEmailNamesTheAddress(t *testing.T) {
	c := qt.New(t)

	rec, _ := failedDeliveryLine(c, func(svc *AsyncEmailService) error {
		return svc.SendPasswordResetEmail(context.Background(), "reset@example.com", "Alex", "https://app.example.com/reset-password?token=abc")
	})

	c.Assert(rec["email"], qt.Equals, "reset@example.com")
}

func TestAsyncEmailService_RetryLineOfAReminderLeavesTheAddressOut(t *testing.T) {
	c := qt.New(t)

	rec, raw := failedDeliveryLine(c, func(svc *AsyncEmailService) error {
		return svc.SendWarrantyReminderEmail(context.Background(), "owner@example.com", "Alex", "Dishwasher", "2026-12-31", "https://app.example.com/g/home/commodities/c1", 30)
	})

	c.Assert(rec["template"], qt.Equals, string(emailTemplateWarrantyReminder))
	c.Assert(raw, qt.Not(qt.Contains), "owner@example.com")
}

// rejectingSender fails every send the way an SMTP server rejects a
// recipient: the error quotes the address.
type rejectingSender struct{}

func (rejectingSender) Send(_ context.Context, msg mailsender.Message) error {
	return fmt.Errorf("smtp RCPT TO: 550 5.1.1 <%s>: Recipient address rejected", strings.ToUpper(msg.To))
}

// rejectedDeliveryLogs queues one email through send, has the sender reject
// it and returns everything the worker logged. With maxRetries 0 the worker
// drops the job; otherwise it schedules a retry.
func rejectedDeliveryLogs(c *qt.C, maxRetries int, send func(*AsyncEmailService) error) string {
	c.Helper()
	renderer, err := newEmailTemplateRenderer()
	c.Assert(err, qt.IsNil)
	queue := &captureQueue{}
	svc := &AsyncEmailService{
		queue:          queue,
		renderer:       renderer,
		sender:         rejectingSender{},
		from:           "noreply@example.com",
		maxRetries:     maxRetries,
		retryBaseDelay: 10 * time.Millisecond,
		retryMaxDelay:  time.Second,
		sendTimeout:    time.Second,
	}
	c.Assert(send(svc), qt.IsNil)
	payloads := queue.EnqueuedPayloads()
	c.Assert(payloads, qt.HasLen, 1)
	var job emailJob
	c.Assert(json.Unmarshal(payloads[0], &job), qt.IsNil)

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	svc.processJob(context.Background(), job, 1)
	slog.SetDefault(prev)
	return buf.String()
}

func sendWarrantyReminder(svc *AsyncEmailService) error {
	return svc.SendWarrantyReminderEmail(context.Background(), "owner@example.com", "Alex", "Dishwasher", "2026-12-31", "https://app.example.com/g/home/commodities/c1", 30)
}

// The provider's error quotes the address, so the worker replaces it on the
// lines of an email that leaves the address out: on a retry and when it gives
// up.
func TestAsyncEmailService_RejectionOfAReminderLeavesTheAddressOut(t *testing.T) {
	c := qt.New(t)

	retry := rejectedDeliveryLogs(c, 3, sendWarrantyReminder)
	dropped := rejectedDeliveryLogs(c, 0, sendWarrantyReminder)

	c.Assert(strings.ToLower(retry), qt.Not(qt.Contains), "owner@example.com")
	c.Assert(retry, qt.Contains, "<recipient>")
	c.Assert(strings.ToLower(dropped), qt.Not(qt.Contains), "owner@example.com")
	c.Assert(dropped, qt.Contains, "<recipient>")
}

func TestAsyncEmailService_RejectionOfARecoveryEmailKeepsTheError(t *testing.T) {
	c := qt.New(t)

	logs := rejectedDeliveryLogs(c, 3, func(svc *AsyncEmailService) error {
		return svc.SendPasswordResetEmail(context.Background(), "reset@example.com", "Alex", "https://app.example.com/reset-password?token=abc")
	})

	c.Assert(logs, qt.Contains, "<RESET@EXAMPLE.COM>: Recipient address rejected")
}

package services

// White-box: the worker's log lines are written by the unexported processJob,
// and the only exported way to reach it is a running worker pool reading a
// real queue. Driving processJob directly with a sender that always fails
// makes the retry line appear exactly once, without timing.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
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

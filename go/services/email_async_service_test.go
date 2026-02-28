package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	emailqueueinmemory "github.com/denisvmedia/inventario/email/queue/inmemory"
	mailsender "github.com/denisvmedia/inventario/email/sender"
)

type flakyEmailSender struct {
	mu        sync.Mutex
	failUntil int
	calls     int
}

func TestAsyncEmailService_SendWelcomeEmail_EnqueuesJSONPayload(t *testing.T) {
	c := qt.New(t)

	queue := &captureQueue{}
	svc := &AsyncEmailService{
		queue: queue,
	}

	err := svc.SendWelcomeEmail(context.Background(), "user@example.com", "Alex")
	c.Assert(err, qt.IsNil)

	payloads := queue.EnqueuedPayloads()
	c.Assert(payloads, qt.HasLen, 1)

	var job emailJob
	err = json.Unmarshal(payloads[0], &job)
	c.Assert(err, qt.IsNil)
	c.Assert(job.ID, qt.Not(qt.Equals), "")
	c.Assert(job.TemplateType, qt.Equals, emailTemplateWelcome)
	c.Assert(job.To, qt.Equals, "user@example.com")
	c.Assert(job.Name, qt.Equals, "Alex")
	c.Assert(job.Attempt, qt.Equals, 0)
}

func TestAsyncEmailService_RunWorker_DropsMalformedPayload(t *testing.T) {
	c := qt.New(t)

	queue := &captureQueue{
		dequeueItems: [][]byte{[]byte("{not-json")},
	}
	sender := &flakyEmailSender{}
	svc := &AsyncEmailService{
		queue:           queue,
		sender:          sender,
		queuePopTimeout: 5 * time.Millisecond,
		stopCh:          make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.runWorker(ctx, 1)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	c.Assert(sender.CallCount(), qt.Equals, 0)
}

func TestAsyncEmailService_ProcessJob_SchedulesRetryWithEncodedPayload(t *testing.T) {
	c := qt.New(t)

	renderer, err := newEmailTemplateRenderer()
	c.Assert(err, qt.IsNil)

	queue := &captureQueue{}
	sender := &flakyEmailSender{failUntil: 100}
	svc := &AsyncEmailService{
		queue:          queue,
		renderer:       renderer,
		sender:         sender,
		from:           "noreply@example.com",
		replyTo:        "support@example.com",
		maxRetries:     3,
		retryBaseDelay: 10 * time.Millisecond,
		retryMaxDelay:  1 * time.Second,
		sendTimeout:    1 * time.Second,
	}

	job := emailJob{
		ID:           "job-1",
		TemplateType: emailTemplateVerification,
		To:           "user@example.com",
		Name:         "User",
		URL:          "https://example.com/verify?token=abc",
		Attempt:      0,
	}
	start := time.Now()
	svc.processJob(job, 1)

	payloads, readyAt := queue.RetryPayloads()
	c.Assert(payloads, qt.HasLen, 1)
	c.Assert(readyAt, qt.HasLen, 1)
	c.Assert(readyAt[0].After(start), qt.IsTrue)

	var retryJob emailJob
	err = json.Unmarshal(payloads[0], &retryJob)
	c.Assert(err, qt.IsNil)
	c.Assert(retryJob.ID, qt.Equals, "job-1")
	c.Assert(retryJob.Attempt, qt.Equals, 1)
	c.Assert(retryJob.TemplateType, qt.Equals, emailTemplateVerification)
}

func TestAsyncEmailService_WorkerConcurrency_ProcessesBatch(t *testing.T) {
	c := qt.New(t)

	renderer, err := newEmailTemplateRenderer()
	c.Assert(err, qt.IsNil)

	sender := &collectingEmailSender{}
	total := 120
	svc := &AsyncEmailService{
		queue:             emailqueueinmemory.New(total * 2),
		renderer:          renderer,
		sender:            sender,
		from:              "noreply@example.com",
		replyTo:           "support@example.com",
		workers:           6,
		maxRetries:        1,
		queuePopTimeout:   10 * time.Millisecond,
		retryPollInterval: 10 * time.Millisecond,
		retryBaseDelay:    10 * time.Millisecond,
		retryMaxDelay:     100 * time.Millisecond,
		sendTimeout:       1 * time.Second,
		stopCh:            make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)
	defer svc.Stop()

	for i := 0; i < total; i++ {
		email := "user" + fmt.Sprintf("%03d", i) + "@example.com"
		err := svc.SendWelcomeEmail(context.Background(), email, "User")
		c.Assert(err, qt.IsNil)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if sender.Count() >= total {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	c.Assert(sender.Count(), qt.Equals, total)

	msgs := sender.Messages()
	seen := make(map[string]struct{}, total)
	for _, msg := range msgs {
		seen[msg.To] = struct{}{}
	}
	c.Assert(seen, qt.HasLen, total)
}

func (s *flakyEmailSender) Send(_ context.Context, _ mailsender.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.calls <= s.failUntil {
		return errors.New("temporary send failure")
	}
	return nil
}

func (s *flakyEmailSender) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type collectingEmailSender struct {
	mu       sync.Mutex
	messages []mailsender.Message
}

func (s *collectingEmailSender) Send(_ context.Context, msg mailsender.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
	return nil
}

func (s *collectingEmailSender) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages)
}

func (s *collectingEmailSender) Messages() []mailsender.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]mailsender.Message, len(s.messages))
	copy(out, s.messages)
	return out
}

type captureQueue struct {
	mu sync.Mutex

	enqueued      [][]byte
	dequeueItems  [][]byte
	retryPayloads [][]byte
	retryReadyAt  []time.Time
}

func (q *captureQueue) Enqueue(_ context.Context, payload []byte) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.enqueued = append(q.enqueued, cloneBytes(payload))
	return nil
}

func (q *captureQueue) Dequeue(ctx context.Context, _ time.Duration) ([]byte, error) {
	q.mu.Lock()
	if len(q.dequeueItems) > 0 {
		payload := cloneBytes(q.dequeueItems[0])
		q.dequeueItems = q.dequeueItems[1:]
		q.mu.Unlock()
		return payload, nil
	}
	q.mu.Unlock()

	<-ctx.Done()
	return nil, ctx.Err()
}

func (q *captureQueue) ScheduleRetry(_ context.Context, payload []byte, readyAt time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.retryPayloads = append(q.retryPayloads, cloneBytes(payload))
	q.retryReadyAt = append(q.retryReadyAt, readyAt)
	return nil
}

func (q *captureQueue) PromoteDueRetries(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

func (q *captureQueue) EnqueuedPayloads() [][]byte {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([][]byte, len(q.enqueued))
	for i := range q.enqueued {
		out[i] = cloneBytes(q.enqueued[i])
	}
	return out
}

func (q *captureQueue) RetryPayloads() ([][]byte, []time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()

	payloads := make([][]byte, len(q.retryPayloads))
	for i := range q.retryPayloads {
		payloads[i] = cloneBytes(q.retryPayloads[i])
	}
	readyAt := make([]time.Time, len(q.retryReadyAt))
	copy(readyAt, q.retryReadyAt)

	return payloads, readyAt
}

func cloneBytes(in []byte) []byte {
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

func TestAsyncEmailService_RetrysFailedDelivery(t *testing.T) {
	c := qt.New(t)

	renderer, err := newEmailTemplateRenderer()
	c.Assert(err, qt.IsNil)

	sender := &flakyEmailSender{failUntil: 1}
	svc := &AsyncEmailService{
		queue:             emailqueueinmemory.New(16),
		renderer:          renderer,
		sender:            sender,
		from:              "noreply@example.com",
		replyTo:           "support@example.com",
		workers:           1,
		maxRetries:        3,
		queuePopTimeout:   10 * time.Millisecond,
		retryPollInterval: 10 * time.Millisecond,
		retryBaseDelay:    10 * time.Millisecond,
		retryMaxDelay:     50 * time.Millisecond,
		sendTimeout:       1 * time.Second,
		stopCh:            make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)
	defer svc.Stop()

	err = svc.SendVerificationEmail(context.Background(), "user@example.com", "User", "https://example.com/verify?token=abc")
	c.Assert(err, qt.IsNil)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sender.CallCount() >= 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("expected email to be retried and sent; call count=%d", sender.CallCount())
}

func TestNewAsyncEmailService_MailchimpRequiresAPIKey(t *testing.T) {
	c := qt.New(t)

	_, err := NewAsyncEmailService(EmailConfig{
		Provider: EmailProviderMailchimp,
		From:     "noreply@example.com",
	})
	c.Assert(err, qt.IsNotNil)
}

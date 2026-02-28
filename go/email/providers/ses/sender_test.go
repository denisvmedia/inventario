package ses

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/email/sender"
)

type mockSESClient struct {
	sendFn func(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
	input  *sesv2.SendEmailInput
	calls  int
}

func (m *mockSESClient) SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	m.calls++
	m.input = params
	if m.sendFn != nil {
		return m.sendFn(ctx, params, optFns...)
	}
	return &sesv2.SendEmailOutput{}, nil
}

func TestNew_RequiresRegionWhenClientMissing(t *testing.T) {
	c := qt.New(t)

	_, err := New(Config{})
	c.Assert(err, qt.IsNotNil)
}

func TestSend_Success(t *testing.T) {
	c := qt.New(t)

	mock := &mockSESClient{}
	s := &Sender{client: mock}

	err := s.Send(context.Background(), sender.Message{
		To:      "user@example.com",
		From:    "noreply@example.com",
		ReplyTo: "support@example.com",
		Subject: "hello",
		HTML:    "<p>hello</p>",
		Text:    "hello",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(mock.calls, qt.Equals, 1)
	c.Assert(aws.ToString(mock.input.FromEmailAddress), qt.Equals, "noreply@example.com")
	c.Assert(mock.input.Destination.ToAddresses, qt.DeepEquals, []string{"user@example.com"})
	c.Assert(mock.input.ReplyToAddresses, qt.DeepEquals, []string{"support@example.com"})
	c.Assert(mock.input.Content.Simple, qt.IsNotNil)
	c.Assert(aws.ToString(mock.input.Content.Simple.Subject.Data), qt.Equals, "hello")
	c.Assert(aws.ToString(mock.input.Content.Simple.Body.Html.Data), qt.Equals, "<p>hello</p>")
	c.Assert(aws.ToString(mock.input.Content.Simple.Body.Text.Data), qt.Equals, "hello")
}

func TestSend_PropagatesClientError(t *testing.T) {
	c := qt.New(t)

	mock := &mockSESClient{
		sendFn: func(_ context.Context, _ *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
			return nil, errors.New("boom")
		},
	}
	s := &Sender{client: mock}

	err := s.Send(context.Background(), sender.Message{
		To:      "user@example.com",
		From:    "noreply@example.com",
		Subject: "hello",
		HTML:    "<p>hello</p>",
		Text:    "hello",
	})
	c.Assert(err, qt.IsNotNil)
}

func TestSend_WithoutReplyTo(t *testing.T) {
	c := qt.New(t)

	mock := &mockSESClient{
		sendFn: func(_ context.Context, _ *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
			return &sesv2.SendEmailOutput{
				MessageId: aws.String("id-1"),
			}, nil
		},
	}
	s := &Sender{client: mock}

	err := s.Send(context.Background(), sender.Message{
		To:      "user@example.com",
		From:    "noreply@example.com",
		Subject: "hello",
		HTML:    "<p>hello</p>",
		Text:    "hello",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(mock.input.ReplyToAddresses, qt.IsNil)
	c.Assert(mock.input.Content, qt.IsNotNil)
	c.Assert(mock.input.Content.Simple.Body, qt.IsNotNil)
	c.Assert(mock.input.Content.Simple.Body.Html, qt.IsNotNil)
	c.Assert(mock.input.Content.Simple.Body.Text, qt.IsNotNil)
	c.Assert(aws.ToString(mock.input.Content.Simple.Subject.Charset), qt.Equals, "UTF-8")
}

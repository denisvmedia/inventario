package stub

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/email/sender"
)

func TestNew(t *testing.T) {
	c := qt.New(t)
	s := New()
	c.Assert(s, qt.IsNotNil)
}

func TestSender_Send(t *testing.T) {
	c := qt.New(t)
	s := New()

	err := s.Send(context.Background(), sender.Message{
		To:      "user@example.com",
		From:    "noreply@example.com",
		Subject: "hello",
		HTML:    "<p>hello</p>",
		Text:    "hello",
	})
	c.Assert(err, qt.IsNil)
}

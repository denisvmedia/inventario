package stub

import (
	"context"
	"log/slog"

	"github.com/denisvmedia/inventario/email/sender"
)

// Sender is a fake transport used when delivery should not leave the process.
//
// It enables callers to run the same orchestration pipeline (enqueue -> render ->
// send) while replacing external I/O with structured logging.
type Sender struct{}

// New constructs a stub transport sender.
func New() *Sender {
	return &Sender{}
}

// Send logs recipient/subject metadata and returns success without network I/O.
func (Sender) Send(_ context.Context, message sender.Message) error {
	slog.Info("STUB email sender: queued email sent",
		"to", message.To,
		"subject", message.Subject,
	)
	return nil
}

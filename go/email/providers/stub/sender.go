package stub

import (
	"context"
	"log/slog"

	"go.5x5.cz/inventario/email/sender"
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

// Send logs that a message was sent and returns success without network I/O.
// It logs neither the recipient nor the subject: the sender cannot tell a
// sign-in email from a reminder, and a subject can hold an item name or a
// feedback sender. The worker's own line names the job and its template.
func (Sender) Send(ctx context.Context, _ sender.Message) error {
	slog.InfoContext(ctx, "STUB email sender: queued email sent")
	return nil
}

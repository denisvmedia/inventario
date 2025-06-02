package bufwriter

import (
	"fmt"
	"log/slog"
	"strings"
)

type Writer struct {
	output strings.Builder
}

func (w *Writer) WriteLinef(format string, args ...any) {
	_, err := fmt.Fprintf(&w.output, format+"\n", args...)
	if err != nil {
		slog.Error("error writing line", "err", err)
	}
}

func (w *Writer) WriteLine(s string) {
	_, err := fmt.Fprint(&w.output, s+"\n")
	if err != nil {
		slog.Error("error writing line", "err", err)
	}
}

// Write writes a string to the output
func (w *Writer) Write(s string) {
	_, err := fmt.Fprint(&w.output, s)
	if err != nil {
		slog.Error("error writing line", "err", err)
	}
}

// Writef writes a formatted string to the output
func (w *Writer) Writef(format string, args ...any) {
	_, err := fmt.Fprintf(&w.output, format, args...)
	if err != nil {
		slog.Error("error writing line", "err", err)
	}
}

func (w *Writer) Output() string {
	return w.output.String()
}

func (w *Writer) Reset() {
	w.output.Reset()
}

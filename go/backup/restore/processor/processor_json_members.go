//go:build !legacy_xml_backup

package processor

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
)

// decodeJSONMember reads one JSON member of an INB archive into doc, refusing
// a member larger than maxJSONDocBytes. what names the member in errors.
func decodeJSONMember(hdr *tar.Header, r io.Reader, what string, doc any) error {
	if hdr.Size < 0 || hdr.Size > maxJSONDocBytes {
		return errx.Classify(ErrJSONDocTooLarge, errx.Attrs("member", hdr.Name, "size", hdr.Size, "max", maxJSONDocBytes))
	}
	data, err := io.ReadAll(io.LimitReader(r, hdr.Size))
	if err != nil {
		return errxtrace.Wrap("failed to read "+what+" member", err, errx.Attrs("member", hdr.Name))
	}
	if err := json.Unmarshal(data, doc); err != nil {
		return errxtrace.Wrap("failed to decode "+what, err, errx.Attrs("member", hdr.Name))
	}
	return nil
}

// restoreRecords runs apply on each of n records under one restore log step
// and records how many were restored, kept and failed. apply reports whether
// it created the record (false for one the group already has). A malformed
// record stops the restore; any other failure is counted and the rest go on.
// what names one record in the error list.
func (w *inbWalker) restoreRecords(step, what string, n int, apply func(i int) (bool, error)) error {
	l := w.proc
	l.createRestoreStep(w.ctx, step, models.RestoreStepResultInProgress, "")
	restored, kept, failed := 0, 0, 0
	for i := range n {
		created, err := apply(i)
		switch {
		case errors.Is(err, ErrMalformedEntity):
			l.updateRestoreStep(w.ctx, step, models.RestoreStepResultError, err.Error())
			return err
		case err != nil:
			failed++
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("failed to process %s: %v", what, err))
		case created:
			restored++
		default:
			kept++
		}
	}
	reason := fmt.Sprintf("%d restored, %d kept as they were, %d failed", restored, kept, failed)
	result := models.RestoreStepResultSuccess
	if failed > 0 {
		result = models.RestoreStepResultError
	}
	l.updateRestoreStep(w.ctx, step, result, reason)
	return nil
}

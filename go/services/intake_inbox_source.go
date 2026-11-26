package services

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// IntakeInboxSource flags every open batch of files of the group (#2810): a
// file waits for its scan or could not be scanned, or a product the scan
// found waits for a decision or for its draft. The batch leaves the inbox
// once every file is scanned and every product has a draft or is left out,
// or when it is removed.
type IntakeInboxSource struct{}

// Refs returns the ids of the open batches, from one query.
func (IntakeInboxSource) Refs(ctx context.Context, regSet *registry.Set) ([]InboxRef, error) {
	sessions, err := regSet.IntakeRegistry.ListOpenSessions(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("inbox: list open intake sessions", err)
	}
	refs := make([]InboxRef, 0, len(sessions))
	for _, sess := range sessions {
		refs = append(refs, InboxRef{Kind: InboxKindIntakeSession, ID: sess.ID, Reasons: []InboxReason{InboxReasonIntake}})
	}
	return refs, nil
}

// Collect returns the open batches, newest first, each with its progress
// and the number of its live files.
func (IntakeInboxSource) Collect(ctx context.Context, regSet *registry.Set) ([]InboxEntry, error) {
	reg := regSet.IntakeRegistry
	sessions, err := reg.ListOpenSessions(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("inbox: list open intake sessions", err)
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		ids = append(ids, sess.ID)
	}
	sources, err := reg.ListSources(ctx, ids...)
	if err != nil {
		return nil, errxtrace.Wrap("inbox: list intake sources", err)
	}
	candidates, err := reg.ListCandidates(ctx, ids...)
	if err != nil {
		return nil, errxtrace.Wrap("inbox: list intake candidates", err)
	}
	sourceIDs := make([]string, 0, len(sources))
	for _, src := range sources {
		sourceIDs = append(sourceIDs, src.ID)
	}
	files := map[string]int{}
	if len(sourceIDs) > 0 {
		if files, err = regSet.FileRegistry.CountLinkedTo(ctx, models.FileLinkIntake, sourceIDs); err != nil {
			return nil, errxtrace.Wrap("inbox: count intake files", err)
		}
	}
	summaries := make(map[string]*models.IntakeSessionSummary, len(sessions))
	for _, id := range ids {
		summaries[id] = &models.IntakeSessionSummary{}
	}
	for _, src := range sources {
		if sum, ok := summaries[src.SessionID]; ok {
			sum.AddSource(src, files[src.ID])
		}
	}
	for _, cand := range candidates {
		if sum, ok := summaries[cand.SessionID]; ok {
			sum.AddCandidate(cand)
		}
	}
	out := make([]InboxEntry, 0, len(sessions))
	for _, sess := range sessions {
		sum := summaries[sess.ID]
		out = append(out, InboxEntry{
			Kind:      InboxKindIntakeSession,
			ID:        sess.ID,
			Reasons:   []InboxReason{InboxReasonIntake},
			UpdatedAt: new(sess.CreatedAt),
			FileCount: sum.Files,
			Intake:    sum,
		})
	}
	return out, nil
}

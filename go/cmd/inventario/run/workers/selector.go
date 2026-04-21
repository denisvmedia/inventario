package workers

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// WorkerID identifies a single background worker family. The string values are
// the canonical identifiers accepted by --workers-only / --workers-exclude on
// `run workers` and are intended to double as the source of truth for logs,
// metrics labels and any future Helm values keyed off worker type.
type WorkerID string

// Canonical worker identifiers. Keep alphabetically ordered; tests and
// AllWorkerIDs rely on this being the authoritative list.
const (
	WorkerEmails       WorkerID = "emails"
	WorkerExports      WorkerID = "exports"
	WorkerImports      WorkerID = "imports"
	WorkerRestores     WorkerID = "restores"
	WorkerThumbnails   WorkerID = "thumbnails"
	WorkerTokenCleanup WorkerID = "token-cleanup"
)

var allWorkerIDs = []WorkerID{
	WorkerEmails,
	WorkerExports,
	WorkerImports,
	WorkerRestores,
	WorkerThumbnails,
	WorkerTokenCleanup,
}

var knownWorkerIDs = map[WorkerID]struct{}{
	WorkerEmails:       {},
	WorkerExports:      {},
	WorkerImports:      {},
	WorkerRestores:     {},
	WorkerThumbnails:   {},
	WorkerTokenCleanup: {},
}

var allWorkerIDStrings = []string{
	string(WorkerEmails),
	string(WorkerExports),
	string(WorkerImports),
	string(WorkerRestores),
	string(WorkerThumbnails),
	string(WorkerTokenCleanup),
}

// selectorAll is the explicit synonym for "every worker", accepted as a value
// of --workers-only so operators can make the intent explicit in systemd units
// and CI pipelines without relying on the empty string.
const selectorAll = "all"

// AllWorkerIDs returns the canonical, stable-ordered list of every worker ID.
// The returned slice is a fresh copy and safe for callers to mutate.
func AllWorkerIDs() []WorkerID {
	return slices.Clone(allWorkerIDs)
}

// Set is the resolved set of workers that should run in the current process.
// It is produced by ParseSelector and consumed by the run logic in workers.go.
type Set map[WorkerID]struct{}

// Has reports whether id is enabled in the set.
func (s Set) Has(id WorkerID) bool {
	_, ok := s[id]
	return ok
}

// Sorted returns the enabled worker IDs in canonical (alphabetical) order,
// suitable for logging or diffing.
func (s Set) Sorted() []WorkerID {
	out := make([]WorkerID, 0, len(s))
	for id := range s {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

// ParseSelector resolves the effective worker set from the raw --workers-only /
// --workers-exclude flag values. Both arguments are comma-separated lists of
// worker identifiers (case-insensitive, surrounding whitespace tolerated).
//
// Rules:
//   - --workers-only and --workers-exclude are mutually exclusive; supplying
//     both is an error.
//   - --workers-only="" or --workers-only="all" selects every worker.
//   - --workers-only="a,b" selects exactly those workers.
//   - --workers-exclude="a,b" selects every worker except those listed.
//   - Unknown identifiers produce an error that lists the valid values.
func ParseSelector(only, exclude string) (Set, error) {
	only = strings.TrimSpace(only)
	exclude = strings.TrimSpace(exclude)

	if only != "" && exclude != "" {
		return nil, errors.New("--workers-only and --workers-exclude are mutually exclusive")
	}

	if only != "" {
		if strings.EqualFold(only, selectorAll) {
			return allSet(), nil
		}
		ids, err := parseIDList(only, "--workers-only")
		if err != nil {
			return nil, err
		}
		return idsToSet(ids), nil
	}

	if exclude != "" {
		ids, err := parseIDList(exclude, "--workers-exclude")
		if err != nil {
			return nil, err
		}
		set := allSet()
		for _, id := range ids {
			delete(set, id)
		}
		return set, nil
	}

	return allSet(), nil
}

// allSet returns a fresh Set containing every canonical worker. The returned
// map is owned by the caller and safe to mutate.
func allSet() Set {
	set := make(Set, len(allWorkerIDs))
	for _, id := range allWorkerIDs {
		set[id] = struct{}{}
	}
	return set
}

// parseIDList splits raw on commas, trims and lowercases each element,
// validates it against the canonical list, and returns the parsed IDs. flag is
// the human-readable flag name used in error messages.
func parseIDList(raw, flag string) ([]WorkerID, error) {
	parts := strings.Split(raw, ",")
	out := make([]WorkerID, 0, len(parts))
	for _, p := range parts {
		token := strings.ToLower(strings.TrimSpace(p))
		if token == "" {
			return nil, fmt.Errorf("%s: empty identifier in list %q", flag, raw)
		}
		id := WorkerID(token)
		if !isKnownID(id) {
			return nil, fmt.Errorf("%s: unknown worker %q (valid: %s)", flag, token, strings.Join(idStrings(), ", "))
		}
		out = append(out, id)
	}
	return out, nil
}

func isKnownID(id WorkerID) bool {
	_, ok := knownWorkerIDs[id]
	return ok
}

func idStrings() []string {
	return slices.Clone(allWorkerIDStrings)
}

func idsToSet(ids []WorkerID) Set {
	set := make(Set, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

package workers

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// WorkerID identifies a single operational group of background workers. The
// string values are the canonical identifiers accepted by --workers-only /
// --workers-exclude on `run workers` and are intended to double as the source
// of truth for logs, metrics labels, and Helm values keyed off worker group.
//
// Each group bundles one or more individual worker families that share an
// operational profile (lifecycle, scaling signal, resource footprint). See
// the groups slice in workers.go for the composition of each group.
type WorkerID string

// Canonical worker group identifiers. Keep alphabetically ordered; tests and
// AllWorkerIDs rely on this being the authoritative list.
const (
	// WorkerArchive bundles exports, imports, and restores. They share the
	// archive-format code path, are long-running, and have matching I/O + DB
	// characteristics, so they scale on a single queue-depth signal.
	WorkerArchive WorkerID = "archive"
	// WorkerEmails owns the email delivery lifecycle. Kept as its own group
	// because SMTP / provider rate limits are a distinct scaling story.
	WorkerEmails WorkerID = "emails"
	// WorkerHousekeeping groups periodic maintenance tasks (refresh token
	// cleanup today; pending-deletion group purge and expired-invite GC in
	// follow-ups). replicaCount is expected to stay at 1.
	WorkerHousekeeping WorkerID = "housekeeping"
	// WorkerMedia bundles CPU/RAM-heavy media processing (thumbnails today,
	// future image resize / OCR). Scales on the media-queue depth signal.
	WorkerMedia WorkerID = "media"
)

var allWorkerIDs = []WorkerID{
	WorkerArchive,
	WorkerEmails,
	WorkerHousekeeping,
	WorkerMedia,
}

var knownWorkerIDs = map[WorkerID]struct{}{
	WorkerArchive:      {},
	WorkerEmails:       {},
	WorkerHousekeeping: {},
	WorkerMedia:        {},
}

var allWorkerIDStrings = []string{
	string(WorkerArchive),
	string(WorkerEmails),
	string(WorkerHousekeeping),
	string(WorkerMedia),
}

// deprecatedAliases maps the pre-#1311 worker-family names to the operational
// group that now owns them. Accepted on --workers-only / --workers-exclude for
// one release, with a structured deprecation warning surfaced by ParseSelector
// so operators can migrate their systemd units and CI pipelines.
var deprecatedAliases = map[string]WorkerID{
	"exports":       WorkerArchive,
	"imports":       WorkerArchive,
	"restores":      WorkerArchive,
	"thumbnails":    WorkerMedia,
	"token-cleanup": WorkerHousekeeping,
}

// selectorAll is the explicit synonym for "every group", accepted as a value
// of --workers-only so operators can make the intent explicit in systemd units
// and CI pipelines without relying on the empty string.
const selectorAll = "all"

// DeprecatedAlias records a legacy identifier that was accepted during
// selector parsing. Callers surface it as a deprecation warning so operators
// see one line per legacy token rather than a single bag of mappings.
type DeprecatedAlias struct {
	Alias     string
	Canonical WorkerID
}

// AllWorkerIDs returns the canonical, stable-ordered list of every worker
// group. The returned slice is a fresh copy and safe for callers to mutate.
func AllWorkerIDs() []WorkerID {
	return slices.Clone(allWorkerIDs)
}

// Set is the resolved set of worker groups that should run in the current
// process. It is produced by ParseSelector and consumed by run logic in
// workers.go.
type Set map[WorkerID]struct{}

// Has reports whether id is enabled in the set.
func (s Set) Has(id WorkerID) bool {
	_, ok := s[id]
	return ok
}

// Sorted returns the enabled group IDs in canonical (alphabetical) order,
// suitable for logging or diffing.
func (s Set) Sorted() []WorkerID {
	out := make([]WorkerID, 0, len(s))
	for id := range s {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

// ParseSelector resolves the effective worker-group set from the raw
// --workers-only / --workers-exclude flag values. Both arguments are
// comma-separated lists of identifiers (case-insensitive, surrounding
// whitespace tolerated). Legacy per-family names (exports, imports, restores,
// thumbnails, token-cleanup) are accepted and mapped to their owning group;
// each such mapping is returned in the second result so the caller can log it.
//
// Rules:
//   - --workers-only and --workers-exclude are mutually exclusive; supplying
//     both is an error.
//   - --workers-only="" or --workers-only="all" selects every group.
//   - --workers-only="a,b" selects exactly those groups.
//   - --workers-exclude="a,b" selects every group except those listed.
//   - Unknown identifiers produce an error that lists the valid groups.
func ParseSelector(only, exclude string) (Set, []DeprecatedAlias, error) {
	only = strings.TrimSpace(only)
	exclude = strings.TrimSpace(exclude)

	if only != "" && exclude != "" {
		return nil, nil, errors.New("--workers-only and --workers-exclude are mutually exclusive")
	}

	if only != "" {
		if strings.EqualFold(only, selectorAll) {
			return allSet(), nil, nil
		}
		ids, aliases, err := parseIDList(only, "--workers-only")
		if err != nil {
			return nil, nil, err
		}
		return idsToSet(ids), aliases, nil
	}

	if exclude != "" {
		ids, aliases, err := parseIDList(exclude, "--workers-exclude")
		if err != nil {
			return nil, nil, err
		}
		set := allSet()
		for _, id := range ids {
			delete(set, id)
		}
		return set, aliases, nil
	}

	return allSet(), nil, nil
}

// allSet returns a fresh Set containing every canonical worker group. The
// returned map is owned by the caller and safe to mutate.
func allSet() Set {
	set := make(Set, len(allWorkerIDs))
	for _, id := range allWorkerIDs {
		set[id] = struct{}{}
	}
	return set
}

// parseIDList splits raw on commas, trims and lowercases each element,
// resolves deprecated aliases to their owning group, validates the result
// against the canonical list, and returns the parsed IDs together with the
// list of deprecated identifiers that were encountered. flag is the
// human-readable flag name used in error messages.
func parseIDList(raw, flag string) ([]WorkerID, []DeprecatedAlias, error) {
	parts := strings.Split(raw, ",")
	out := make([]WorkerID, 0, len(parts))
	var aliases []DeprecatedAlias
	for _, p := range parts {
		token := strings.ToLower(strings.TrimSpace(p))
		if token == "" {
			return nil, nil, fmt.Errorf("%s: empty identifier in list %q", flag, raw)
		}
		if canonical, ok := deprecatedAliases[token]; ok {
			aliases = append(aliases, DeprecatedAlias{Alias: token, Canonical: canonical})
			out = append(out, canonical)
			continue
		}
		id := WorkerID(token)
		if !isKnownID(id) {
			return nil, nil, fmt.Errorf("%s: unknown worker %q (valid: %s)", flag, token, strings.Join(idStrings(), ", "))
		}
		out = append(out, id)
	}
	return out, aliases, nil
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

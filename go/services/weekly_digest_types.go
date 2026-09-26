package services

import "time"

// WeeklyDigestGroupCounts is one group's line in a digest: what happened in it
// over the covered week.
//
// Counts rather than lists, because a digest is a nudge and the app is where the
// detail lives. A group with nothing to report is left out of the digest
// entirely rather than shown as three zeroes.
type WeeklyDigestGroupCounts struct {
	GroupName string `json:"group_name"`
	// ItemsAdded counts commodities created in the window, read from the
	// commodity event log rather than from the commodities table, which has no
	// created_at column.
	ItemsAdded int `json:"items_added"`
	// ItemsChanged counts the commodities that saw any other recorded change —
	// an edit, a status flip, a price change, a move. One commodity changed
	// five times counts once: the number is meant to say "how much moved", not
	// how many rows the log gained.
	ItemsChanged int `json:"items_changed"`
	// FilesAdded counts files attached in the window.
	FilesAdded int `json:"files_added"`
}

// Empty reports whether this group has nothing worth a line.
func (c WeeklyDigestGroupCounts) Empty() bool {
	return c.ItemsAdded == 0 && c.ItemsChanged == 0 && c.FilesAdded == 0
}

// WeeklyDigestUpcomingKind names what a digest's upcoming entry is about. The
// value reaches the email templates, which branch on it for the wording.
type WeeklyDigestUpcomingKind string

const (
	// WeeklyDigestUpcomingWarranty is a warranty that expires soon.
	WeeklyDigestUpcomingWarranty WeeklyDigestUpcomingKind = "warranty"
	// WeeklyDigestUpcomingMaintenance is a maintenance task that comes due soon.
	WeeklyDigestUpcomingMaintenance WeeklyDigestUpcomingKind = "maintenance"
)

// WeeklyDigestUpcoming is one entry in the "coming up" section.
//
// It overlaps the warranty and maintenance reminder emails on purpose: those
// fire on their own thresholds and this is the weekly overview, so a user who
// has reminders switched off still sees what is approaching, and one who has
// them on gets no surprise. Date is pre-formatted because it is a date, which
// carries no grammar.
type WeeklyDigestUpcoming struct {
	Kind      WeeklyDigestUpcomingKind `json:"kind"`
	GroupName string                   `json:"group_name"`
	Name      string                   `json:"name"`
	Date      string                   `json:"date"`
	URL       string                   `json:"url,omitempty"`
}

// WeeklyDigestWindow is the span a digest covers: the seven days before the
// Monday it is sent on.
type WeeklyDigestWindow struct {
	Start time.Time
	End   time.Time
}

// WeekStartUTC returns the Monday 00:00 UTC of the week t falls in.
//
// It is the digest's idempotency key, so every tick has to derive the same value
// from any instant within the week — which is why it works in UTC and ignores
// the caller's location. A per-user timezone would move the boundary per user
// and is deliberately left to a follow-up (#1391).
func WeekStartUTC(t time.Time) time.Time {
	utc := t.UTC()
	// time.Weekday counts from Sunday; Monday has to come out as 0.
	offset := (int(utc.Weekday()) + 6) % 7
	day := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	return day.AddDate(0, 0, -offset)
}

// WeeklyDigestWindowFor returns the week a digest sent at t covers: the seven
// days ending at the start of t's week. Sent on a Monday it is the Monday to
// Sunday just gone.
func WeeklyDigestWindowFor(t time.Time) WeeklyDigestWindow {
	end := WeekStartUTC(t)
	return WeeklyDigestWindow{Start: end.AddDate(0, 0, -7), End: end}
}

// Contains reports whether ts falls in the window, start inclusive and end
// exclusive so consecutive weeks cannot both claim the same instant.
func (w WeeklyDigestWindow) Contains(ts time.Time) bool {
	utc := ts.UTC()
	return !utc.Before(w.Start) && utc.Before(w.End)
}

// WeeklyDigestEmail is what one recipient's digest carries into the email
// layer. It exists so the send site does not grow another ten positional
// arguments, and so the queue's job shape stays a flat mirror of it.
type WeeklyDigestEmail struct {
	// WeekStart and WeekEnd are the covered week's bounds, already formatted:
	// they are dates, which carry no grammar to localize.
	WeekStart string
	WeekEnd   string
	// Groups holds one entry per group with something to report. Never empty —
	// an empty digest is not sent.
	Groups []WeeklyDigestGroupCounts
	// Upcoming may be empty: a week with activity and nothing approaching is an
	// ordinary week.
	Upcoming []WeeklyDigestUpcoming
	// URL is where the app opens. SettingsURL is the notification settings of
	// the first group in the digest, which is where the toggle behind this email
	// lives. Either may be empty, and the templates drop the link when it is.
	URL         string
	SettingsURL string
}

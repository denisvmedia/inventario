package jsonapi

import (
	"net/http"
	"time"

	"github.com/go-chi/render"

	"go.5x5.cz/inventario/models"
)

// InboxEntry is one record of the inbox (#2794): something in the group that
// still needs attention, with every reason it does. Kind and id name the
// record: a draft is opened through /drafts/{id}, an item through
// /commodities/{id}, a batch of files through /intake/sessions/{id}.
type InboxEntry struct {
	Kind  string `json:"kind" example:"draft" enums:"draft,commodity,intake_session"`
	ID    string `json:"id" example:"5f0c8a7e-3f43-4f1e-9a43-6a1fbb8f2c11"`
	Title string `json:"title" example:"Cordless drill"`
	// Reasons holds at least one reason, each once, in display order.
	Reasons []string `json:"reasons" example:"draft" enums:"draft,item_draft,intake"`
	// UpdatedAt is when the record was last saved; drafts carry it, items
	// do not. A batch of files carries when it was added.
	UpdatedAt *time.Time `json:"updated_at,omitempty" extensions:"x-nullable=true"`
	// FileCount is the number of live files attached to a draft or dropped
	// in a batch; 0 for an item.
	FileCount int `json:"file_count" example:"1"`
	// Intake is the progress of a batch of files, the same as on
	// /intake/sessions; absent for other kinds.
	Intake *InboxIntake `json:"intake,omitempty"`
}

// InboxIntake is the progress of a batch of files in the inbox.
type InboxIntake struct {
	State   models.IntakeSessionState   `json:"state" enums:"processing,review,done" example:"review"`
	Summary models.IntakeSessionSummary `json:"summary"`
}

// InboxCounts counts the inbox. Total counts each record once; reasons
// counts, per reason, the records that have it, so a record with two
// reasons adds to both and the reasons can add up to more than total.
type InboxCounts struct {
	Total   int            `json:"total" example:"3"`
	Reasons map[string]int `json:"reasons"`
}

// InboxMeta is the count block of an inbox page. Entries is the number of
// entries the reason filter keeps; Counts describes the whole inbox.
type InboxMeta struct {
	Entries int         `json:"entries" example:"2"`
	Counts  InboxCounts `json:"counts"`
}

// InboxResponse is a page of the inbox.
type InboxResponse struct {
	Data []InboxEntry `json:"data"`
	Meta InboxMeta    `json:"meta"`
}

// InboxCountResponse is the payload of GET /inbox/count.
type InboxCountResponse struct {
	Data InboxCounts `json:"data"`
}

// NewInboxResponse builds the response for a page. entries is the number of
// entries the reason filter keeps.
func NewInboxResponse(data []InboxEntry, entries int, counts InboxCounts) *InboxResponse {
	if data == nil {
		data = []InboxEntry{}
	}
	return &InboxResponse{Data: data, Meta: InboxMeta{Entries: entries, Counts: counts}}
}

func (*InboxResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// NewInboxCountResponse builds the response of GET /inbox/count.
func NewInboxCountResponse(counts InboxCounts) *InboxCountResponse {
	return &InboxCountResponse{Data: counts}
}

func (*InboxCountResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

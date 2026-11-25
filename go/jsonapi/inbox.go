package jsonapi

import (
	"net/http"
	"time"

	"github.com/go-chi/render"
)

// InboxEntry is one record of the inbox (#2794): something in the group that
// still needs attention, with every reason it does. Kind and id name the
// record: a draft is opened through /drafts/{id}, an item through
// /commodities/{id}.
type InboxEntry struct {
	Kind  string `json:"kind" example:"draft" enums:"draft,commodity"`
	ID    string `json:"id" example:"5f0c8a7e-3f43-4f1e-9a43-6a1fbb8f2c11"`
	Title string `json:"title" example:"Cordless drill"`
	// Reasons holds at least one reason, each once, in display order.
	Reasons []string `json:"reasons" example:"draft" enums:"draft,item_draft"`
	// UpdatedAt is when the record was last saved; drafts carry it, items
	// do not.
	UpdatedAt *time.Time `json:"updated_at,omitempty" extensions:"x-nullable=true"`
	// FileCount is the number of live files attached to a draft; 0 for an
	// item.
	FileCount int `json:"file_count" example:"1"`
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

// InboxCheck is an optional quality check next to the inbox: the items that
// lack one piece of information. It is not part of the inbox or its counts.
// Query is the item query of the check, in the parameter names of
// GET /commodities, and Count is the total that list reports for it.
type InboxCheck struct {
	Key   string `json:"key" example:"no_photo" enums:"no_location,no_photo,no_purchase_price,no_purchase_date,untagged"`
	Query string `json:"query" example:"include_inactive=false&no_photo=true"`
	Count int    `json:"count" example:"4"`
}

// InboxChecksResponse is the payload of GET /inbox/checks.
type InboxChecksResponse struct {
	Data []InboxCheck `json:"data"`
}

// NewInboxChecksResponse builds the response of GET /inbox/checks.
func NewInboxChecksResponse(checks []InboxCheck) *InboxChecksResponse {
	if checks == nil {
		checks = []InboxCheck{}
	}
	return &InboxChecksResponse{Data: checks}
}

func (*InboxChecksResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

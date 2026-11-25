package jsonapi

import (
	"net/http"

	"github.com/go-chi/render"
)

// UpcomingItem is one entry of the group-wide Upcoming list.
type UpcomingItem struct {
	// Kind is the source of the entry.
	Kind string `json:"kind" enums:"reminder,warranty,loan,maintenance" example:"reminder"`
	// Date is the entry's date (YYYY-MM-DD).
	Date string `json:"date" format:"date" example:"2026-12-24"`
	// DaysLeft is the whole days from today to Date; negative when the
	// entry is overdue.
	DaysLeft int `json:"days_left" example:"12"`
	// Title is the reminder or maintenance title, or the borrower for a
	// loan. Empty for a warranty.
	Title string `json:"title,omitempty"`
	// SourceID is the reminder, loan or maintenance schedule id; for a
	// warranty it is the item id.
	SourceID  string                   `json:"source_id"`
	Commodity *MaintenanceCommodityRef `json:"commodity,omitempty"`
}

// UpcomingMeta describes the window the list covers.
type UpcomingMeta struct {
	Days  int `json:"days" example:"30"`
	Total int `json:"total" example:"7"`
}

// UpcomingResponse is the response of GET /g/{slug}/upcoming.
type UpcomingResponse struct {
	Data []*UpcomingItem `json:"data"`
	Meta UpcomingMeta    `json:"meta"`
}

func NewUpcomingResponse(items []*UpcomingItem, days int) *UpcomingResponse {
	if items == nil {
		items = []*UpcomingItem{}
	}
	return &UpcomingResponse{
		Data: items,
		Meta: UpcomingMeta{Days: days, Total: len(items)},
	}
}

func (*UpcomingResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

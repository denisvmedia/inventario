package types

import (
	"slices"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
)

// The item records an archive carries (format 3.9). Mirrors the types of the
// same names in backup/export/inb_types.go. Each names the UUID of the member
// who recorded it in AuthorID; the processor decides who records it in the
// destination group, so the conversions below leave the author unset, and
// they leave the ids unset too: the registries assign new ones.

// INBLoan is an archived loan of an item.
type INBLoan struct {
	ID                  string `json:"id"`
	AuthorID            string `json:"authorId,omitempty"`
	BorrowerName        string `json:"borrowerName"`
	BorrowerContact     string `json:"borrowerContact,omitempty"`
	BorrowerNote        string `json:"borrowerNote,omitempty"`
	LentAt              string `json:"lentAt"`
	DueBackAt           string `json:"dueBackAt,omitempty"`
	ReturnedAt          string `json:"returnedAt,omitempty"`
	ReminderSentDueSoon bool   `json:"reminderSentDueSoon,omitempty"`
	ReminderSentOverdue bool   `json:"reminderSentOverdue,omitempty"`
}

// INBServiceRecord is an archived service record of an item.
type INBServiceRecord struct {
	ID                  string `json:"id"`
	AuthorID            string `json:"authorId,omitempty"`
	ProviderName        string `json:"providerName"`
	ProviderContact     string `json:"providerContact,omitempty"`
	Reason              string `json:"reason,omitempty"`
	SentAt              string `json:"sentAt"`
	ExpectedReturnAt    string `json:"expectedReturnAt,omitempty"`
	ReturnedAt          string `json:"returnedAt,omitempty"`
	CostAmount          string `json:"costAmount,omitempty"`
	CostCurrency        string `json:"costCurrency,omitempty"`
	ReminderSentDueSoon bool   `json:"reminderSentDueSoon,omitempty"`
	ReminderSentOverdue bool   `json:"reminderSentOverdue,omitempty"`
}

// INBMaintenanceSchedule is an archived maintenance schedule of an item.
type INBMaintenanceSchedule struct {
	ID           string `json:"id"`
	AuthorID     string `json:"authorId,omitempty"`
	Title        string `json:"title"`
	IntervalDays int    `json:"intervalDays"`
	NextDueAt    string `json:"nextDueAt"`
	LastDoneAt   string `json:"lastDoneAt,omitempty"`
	Notes        string `json:"notes,omitempty"`
	Enabled      bool   `json:"enabled"`
}

// INBSupplyLink is an archived supply link of an item.
type INBSupplyLink struct {
	ID        string `json:"id"`
	AuthorID  string `json:"authorId,omitempty"`
	Label     string `json:"label"`
	URL       string `json:"url"`
	Notes     string `json:"notes,omitempty"`
	SortOrder int    `json:"sortOrder"`
}

// INBReminder is an archived reminder of an item.
type INBReminder struct {
	ID               string `json:"id"`
	AuthorID         string `json:"authorId,omitempty"`
	Title            string `json:"title"`
	DueOn            string `json:"dueOn"`
	RemindDaysBefore []int  `json:"remindDaysBefore,omitempty"`
	RepeatUnit       string `json:"repeatUnit"`
	RepeatInterval   int    `json:"repeatInterval"`
}

// inbPDate maps an optional archived date to the model's nullable date.
func inbPDate(s string) models.PDate {
	if s == "" {
		return nil
	}
	return new(models.Date(s))
}

// ConvertToLoan returns the loan of the item with DB id commodityID.
func (l *INBLoan) ConvertToLoan(commodityID string) *models.CommodityLoan {
	return &models.CommodityLoan{
		CommodityID:         commodityID,
		BorrowerName:        l.BorrowerName,
		BorrowerContact:     l.BorrowerContact,
		BorrowerNote:        l.BorrowerNote,
		LentAt:              models.Date(l.LentAt),
		DueBackAt:           inbPDate(l.DueBackAt),
		ReturnedAt:          inbPDate(l.ReturnedAt),
		ReminderSentDueSoon: l.ReminderSentDueSoon,
		ReminderSentOverdue: l.ReminderSentOverdue,
	}
}

// ConvertToService returns the service record of the item with DB id
// commodityID. An unparseable cost is an error, not a zero cost.
func (s *INBServiceRecord) ConvertToService(commodityID string) (*models.CommodityService, error) {
	rec := &models.CommodityService{
		CommodityID:         commodityID,
		ProviderName:        s.ProviderName,
		ProviderContact:     s.ProviderContact,
		Reason:              s.Reason,
		SentAt:              models.Date(s.SentAt),
		ExpectedReturnAt:    inbPDate(s.ExpectedReturnAt),
		ReturnedAt:          inbPDate(s.ReturnedAt),
		CostCurrency:        s.CostCurrency,
		ReminderSentDueSoon: s.ReminderSentDueSoon,
		ReminderSentOverdue: s.ReminderSentOverdue,
	}
	if s.CostAmount != "" {
		amount, err := decimal.NewFromString(s.CostAmount)
		if err != nil {
			return nil, errxtrace.Wrap("failed to parse the service cost", err, errx.Attrs("value", s.CostAmount))
		}
		rec.CostAmount = amount
	}
	return rec, nil
}

// ConvertToSchedule returns the maintenance schedule of the item with DB id
// commodityID.
func (m *INBMaintenanceSchedule) ConvertToSchedule(commodityID string) *models.MaintenanceSchedule {
	return &models.MaintenanceSchedule{
		CommodityID:  commodityID,
		Title:        m.Title,
		IntervalDays: m.IntervalDays,
		NextDueAt:    models.Date(m.NextDueAt),
		LastDoneAt:   inbPDate(m.LastDoneAt),
		Notes:        m.Notes,
		Enabled:      m.Enabled,
	}
}

// ConvertToSupplyLink returns the supply link of the item with DB id
// commodityID.
func (s *INBSupplyLink) ConvertToSupplyLink(commodityID string) *models.SupplyLink {
	return &models.SupplyLink{
		CommodityID: commodityID,
		Label:       s.Label,
		URL:         s.URL,
		Notes:       s.Notes,
		SortOrder:   s.SortOrder,
	}
}

// ConvertToReminder returns the reminder of the item with DB id commodityID.
func (r *INBReminder) ConvertToReminder(commodityID string) *models.CommodityReminder {
	return &models.CommodityReminder{
		CommodityID:      commodityID,
		Title:            r.Title,
		DueOn:            models.Date(r.DueOn),
		RemindDaysBefore: models.ValuerSlice[int](slices.Clone(r.RemindDaysBefore)),
		RepeatUnit:       models.ReminderRepeatUnit(r.RepeatUnit),
		RepeatInterval:   r.RepeatInterval,
	}
}

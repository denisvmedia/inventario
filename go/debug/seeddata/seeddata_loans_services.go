package seeddata

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// loanSpec is the dense per-loan row used to build seeded loan
// history. The CommodityName must match a commodityCatalogue entry
// with Count=1 (the #1554 invariant blocks loans on bundle rows).
type loanSpec struct {
	CommodityName   string
	BorrowerName    string
	BorrowerContact string
	BorrowerNote    string
	LentDaysAgo     int
	DueInDays       int // 0 = open-ended, positive = future, negative = overdue. Ignored when ReturnedDaysAgo > 0.
	ReturnedDaysAgo int // 0 = still open
}

// serviceSpec is the dense per-service row. Same Count=1 invariant
// applies via the shared OpenHoldingChecker in the service layer; we
// respect it here even though we bypass that layer by inserting
// directly via the registry.
type serviceSpec struct {
	CommodityName   string
	ProviderName    string
	ProviderContact string
	Reason          string
	SentDaysAgo     int
	ExpectedInDays  int // 0 = open-ended estimate
	ReturnedDaysAgo int // 0 = still open
	CostAmount      float64
	CostCurrency    string
}

// seedLoanCatalogue gives the Lent tab and the per-item Lend history
// realistic content: 3 active + 2 overdue + 2 returned. Same handful
// of plausible names so the audit timeline reads like a real shared
// circle.
var seedLoanCatalogue = []loanSpec{
	// Active
	{CommodityName: "Camera Lens", BorrowerName: "Sarah Klein", BorrowerContact: "sarah@example.org", BorrowerNote: "Promised to bring it back after the conference.", LentDaysAgo: 14, DueInDays: 14},
	{CommodityName: "Game Console", BorrowerName: "Ben (upstairs)", BorrowerContact: "+420 777 222 111", BorrowerNote: "Long weekend session.", LentDaysAgo: 2, DueInDays: 7},
	{CommodityName: "Power Drill", BorrowerName: "Marie Doutrelant", BorrowerContact: "marie.d@example.org", BorrowerNote: "Bookshelf renovation — open-ended.", LentDaysAgo: 20, DueInDays: 0},

	// Overdue
	{CommodityName: "Vinyl Player", BorrowerName: "Tom Holub", BorrowerContact: "tom@example.org", BorrowerNote: "Said he'd return after his birthday party.", LentDaysAgo: 60, DueInDays: -20},
	{CommodityName: "Skis", BorrowerName: "Anna Skala", BorrowerContact: "anna@example.org", BorrowerNote: "Borrowed for the season — should have come back.", LentDaysAgo: 120, DueInDays: -30},

	// Returned
	{CommodityName: "Bicycle", BorrowerName: "Mike Schwarz", BorrowerContact: "mike@example.org", BorrowerNote: "Used during his bike-fit assessment.", LentDaysAgo: 90, ReturnedDaysAgo: 60},
	{CommodityName: "Stand Mixer", BorrowerName: "Lucy Brand", BorrowerContact: "lucy@example.org", BorrowerNote: "Wedding cake project.", LentDaysAgo: 60, ReturnedDaysAgo: 45},
}

// seedServiceCatalogue covers the In Service tab: 2 active + 2
// completed. Workshops are deliberately generic so the demo doesn't
// brand-name any real businesses.
var seedServiceCatalogue = []serviceSpec{
	// Active
	{CommodityName: "Vacuum Cleaner", ProviderName: "Authorized Repair Co.", ProviderContact: "service@authrepair.example", Reason: "Battery pack replacement under warranty.", SentDaysAgo: 10, ExpectedInDays: 14},
	{CommodityName: "Bicycle", ProviderName: "Local Bike Shop", ProviderContact: "+420 555 123 456", Reason: "Brake lever swap, full tune-up.", SentDaysAgo: 2, ExpectedInDays: 5},

	// Completed
	{CommodityName: "Coffee Machine", ProviderName: "Coffee Pro Servis", ProviderContact: "info@coffeepro.example", Reason: "Annual descaling + group head clean.", SentDaysAgo: 90, ReturnedDaysAgo: 80, CostAmount: 1200, CostCurrency: "CZK"},
	{CommodityName: "Smart TV", ProviderName: "Display Specialists", ProviderContact: "support@display.example", Reason: "Backlight strip replacement.", SentDaysAgo: 200, ReturnedDaysAgo: 180, CostAmount: 320, CostCurrency: "EUR"},
}

// seedLoansAndServices writes the loan + service catalogues into the
// per-commodity history. Skips rows that reference unknown
// commodities or bundle commodities (Count > 1) — the row would fail
// validation at the service layer's invariant check.
func seedLoansAndServices(ctx context.Context, set *registry.Set, user *models.User, group *models.LocationGroup, inv *seedInventoryResult) error {
	for _, l := range seedLoanCatalogue {
		commodity, ok := inv.commoditiesByName[l.CommodityName]
		if !ok {
			return fmt.Errorf("loan references unknown commodity %q", l.CommodityName)
		}
		if commodity.Count > 1 {
			return fmt.Errorf("loan on bundle commodity %q would violate #1554 invariant", l.CommodityName)
		}

		now := time.Now()
		lent := models.Date(now.AddDate(0, 0, -l.LentDaysAgo).Format("2006-01-02"))
		loan := models.CommodityLoan{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID:        user.TenantID,
				GroupID:         group.ID,
				CreatedByUserID: user.ID,
			},
			CommodityID:     commodity.ID,
			BorrowerName:    l.BorrowerName,
			BorrowerContact: l.BorrowerContact,
			BorrowerNote:    l.BorrowerNote,
			LentAt:          lent,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if l.DueInDays != 0 && l.ReturnedDaysAgo == 0 {
			loan.DueBackAt = daysFromToday(l.DueInDays)
		}
		if l.ReturnedDaysAgo > 0 {
			loan.ReturnedAt = daysFromToday(-l.ReturnedDaysAgo)
		}
		if _, err := set.CommodityLoanRegistry.Create(ctx, loan); err != nil {
			return fmt.Errorf("create loan for %q: %w", l.CommodityName, err)
		}
	}

	for _, s := range seedServiceCatalogue {
		commodity, ok := inv.commoditiesByName[s.CommodityName]
		if !ok {
			return fmt.Errorf("service references unknown commodity %q", s.CommodityName)
		}
		if commodity.Count > 1 {
			return fmt.Errorf("service on bundle commodity %q would violate #1554 invariant", s.CommodityName)
		}

		now := time.Now()
		sent := models.Date(now.AddDate(0, 0, -s.SentDaysAgo).Format("2006-01-02"))
		svc := models.CommodityService{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID:        user.TenantID,
				GroupID:         group.ID,
				CreatedByUserID: user.ID,
			},
			CommodityID:     commodity.ID,
			ProviderName:    s.ProviderName,
			ProviderContact: s.ProviderContact,
			Reason:          s.Reason,
			SentAt:          sent,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if s.ExpectedInDays != 0 && s.ReturnedDaysAgo == 0 {
			svc.ExpectedReturnAt = daysFromToday(s.ExpectedInDays)
		}
		if s.ReturnedDaysAgo > 0 {
			svc.ReturnedAt = daysFromToday(-s.ReturnedDaysAgo)
		}
		if s.CostAmount != 0 {
			svc.CostAmount = decimal.NewFromFloat(s.CostAmount)
			svc.CostCurrency = s.CostCurrency
		}
		if _, err := set.CommodityServiceRegistry.Create(ctx, svc); err != nil {
			return fmt.Errorf("create service for %q: %w", s.CommodityName, err)
		}
	}

	return nil
}

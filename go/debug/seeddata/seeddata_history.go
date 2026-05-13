package seeddata

import (
	"context"
	"fmt"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// seedHistory writes one completed export + one completed restore so
// the Backup & Restore page (#1534) isn't a blank list, plus a small
// activity feed of CommodityEvents tied to plausible commodity state
// changes and a handful of AuditLog rows the upcoming profile
// activity tab (#1653) can render.
//
// Takes both a userCtx (group-scoped, for the exports/events tables
// that live under RLS) and a serviceCtx (tenant-scoped, for the
// AuditLog table that isn't group-scoped). The orchestrator already
// has both contexts available.
func seedHistory(userCtx, serviceCtx context.Context, userSet *registry.Set, serviceSet *registry.Set, user *models.User, group *models.LocationGroup, inv *seedInventoryResult) error {
	if err := seedExportsAndRestores(userCtx, userSet, user, group); err != nil {
		return err
	}
	if err := seedCommodityEvents(userCtx, userSet, user, group, inv); err != nil {
		return err
	}
	if err := seedAuditLog(serviceCtx, serviceSet, user); err != nil {
		return err
	}
	return nil
}

// seedExportsAndRestores seeds the Backup & Restore page (#1534) with
// one historical export + one historical restore. The seeded export
// uses Status=Failed (with an explanatory ErrorMessage) rather than
// Completed because Completed would advertise a downloadable artifact
// the seed never actually wrote into the blob bucket — clicking
// Download on a fake "completed" export would 404 against the bucket
// streamer. Status=Failed keeps the list non-empty AND aligns with
// what the UI promises (no download for failed runs). The dry-run
// restore stays Status=Completed because RestoreOperation has no
// downloadable artifact — its visible surface is just statistics.
func seedExportsAndRestores(ctx context.Context, set *registry.Set, user *models.User, group *models.LocationGroup) error {
	createdAt := nowPTimestamp(time.Now().AddDate(0, 0, -45))
	completedAt := nowPTimestamp(time.Now().AddDate(0, 0, -45).Add(2 * time.Minute))

	exportErr := "Seed fixture — no real export artifact was produced. " +
		"Trigger a fresh export from this page to generate a downloadable XML bundle."
	export := models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        user.TenantID,
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		Type:            models.ExportTypeFullDatabase,
		Status:          models.ExportStatusFailed,
		IncludeFileData: true,
		Description:     "Pre-renovation snapshot",
		ErrorMessage:    exportErr,
		CreatedDate:     createdAt,
		CompletedDate:   completedAt,
		LocationCount:   3,
		AreaCount:       10,
		CommodityCount:  35,
	}
	createdExport, err := set.ExportRegistry.Create(ctx, export)
	if err != nil {
		return fmt.Errorf("create seed export: %w", err)
	}

	restoreCreated := nowPTimestamp(time.Now().AddDate(0, 0, -40))
	restoreStarted := nowPTimestamp(time.Now().AddDate(0, 0, -40).Add(1 * time.Minute))
	restoreCompleted := nowPTimestamp(time.Now().AddDate(0, 0, -40).Add(3 * time.Minute))
	restore := models.RestoreOperation{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        user.TenantID,
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		ExportID:       createdExport.ID,
		Description:    "Restored from pre-renovation snapshot (dry run)",
		Status:         models.RestoreStatusCompleted,
		Options:        models.RestoreOptions{Strategy: "merge_add", IncludeFileData: false, DryRun: true},
		CreatedDate:    restoreCreated,
		StartedDate:    restoreStarted,
		CompletedDate:  restoreCompleted,
		LocationCount:  3,
		AreaCount:      10,
		CommodityCount: 35,
		ImageCount:     35,
		InvoiceCount:   8,
		ManualCount:    4,
		FileCount:      4,
	}
	if _, err := set.RestoreOperationRegistry.Create(ctx, restore); err != nil {
		return fmt.Errorf("create seed restore: %w", err)
	}
	return nil
}

// nowPTimestamp builds a *Timestamp pointer in RFC3339 format.
func nowPTimestamp(t time.Time) *models.Timestamp {
	ts := models.Timestamp(t.UTC().Format(time.RFC3339))
	return &ts
}

// commodityEventSeed describes the per-commodity events the activity
// feed should carry. Each entry maps a commodity name to the kind +
// occurred-at offset (positive = past).
type commodityEventSeed struct {
	CommodityName string
	Kind          models.CommodityEventKind
	DaysAgo       int
	Note          string
	After         models.CommodityEventPayload
}

// seedEventCatalogue lights up the per-commodity timeline rail.
var seedEventCatalogue = []commodityEventSeed{
	{CommodityName: "Smart TV", Kind: models.CommodityEventKindCreated, DaysAgo: 540},
	{CommodityName: "Smart TV", Kind: models.CommodityEventKindSentForService, DaysAgo: 200, Note: "Backlight strip replacement.", After: models.CommodityEventPayload{"provider_name": "Display Specialists"}},
	{CommodityName: "Smart TV", Kind: models.CommodityEventKindBackFromService, DaysAgo: 180, After: models.CommodityEventPayload{"returned_at": time.Now().AddDate(0, 0, -180).Format("2006-01-02")}},
	{CommodityName: "Smart TV", Kind: models.CommodityEventKindUpdated, DaysAgo: 30, Note: "Updated warranty notes."},

	{CommodityName: "Coffee Machine", Kind: models.CommodityEventKindCreated, DaysAgo: 110},
	{CommodityName: "Coffee Machine", Kind: models.CommodityEventKindSentForService, DaysAgo: 90, After: models.CommodityEventPayload{"provider_name": "Coffee Pro Servis", "reason": "Annual descaling"}},
	{CommodityName: "Coffee Machine", Kind: models.CommodityEventKindBackFromService, DaysAgo: 80},

	{CommodityName: "Bicycle", Kind: models.CommodityEventKindCreated, DaysAgo: 730},
	{CommodityName: "Bicycle", Kind: models.CommodityEventKindLentOut, DaysAgo: 90, After: models.CommodityEventPayload{"borrower_name": "Mike Schwarz"}},
	{CommodityName: "Bicycle", Kind: models.CommodityEventKindReturned, DaysAgo: 60},
	{CommodityName: "Bicycle", Kind: models.CommodityEventKindSentForService, DaysAgo: 2, After: models.CommodityEventPayload{"provider_name": "Local Bike Shop"}},

	{CommodityName: "Power Drill", Kind: models.CommodityEventKindCreated, DaysAgo: 90},
	{CommodityName: "Power Drill", Kind: models.CommodityEventKindLentOut, DaysAgo: 20, After: models.CommodityEventPayload{"borrower_name": "Marie Doutrelant"}},

	{CommodityName: "Refrigerator", Kind: models.CommodityEventKindCreated, DaysAgo: 700},
	{CommodityName: "Refrigerator", Kind: models.CommodityEventKindUpdated, DaysAgo: 95, Note: "Updated serial number after warranty claim."},

	{CommodityName: "Game Console", Kind: models.CommodityEventKindCreated, DaysAgo: 220},
	{CommodityName: "Game Console", Kind: models.CommodityEventKindLentOut, DaysAgo: 2, After: models.CommodityEventPayload{"borrower_name": "Ben (upstairs)"}},

	{CommodityName: "Camera Lens", Kind: models.CommodityEventKindCreated, DaysAgo: 120},
	{CommodityName: "Camera Lens", Kind: models.CommodityEventKindLentOut, DaysAgo: 14, After: models.CommodityEventPayload{"borrower_name": "Sarah Klein"}},
}

// seedCommodityEvents materialises the event-rail content for the
// detail page's history pane and the dashboard activity feed.
func seedCommodityEvents(ctx context.Context, set *registry.Set, user *models.User, group *models.LocationGroup, inv *seedInventoryResult) error {
	now := time.Now()
	for _, ev := range seedEventCatalogue {
		commodity, ok := inv.commoditiesByName[ev.CommodityName]
		if !ok {
			return fmt.Errorf("event references unknown commodity %q", ev.CommodityName)
		}
		occurredAt := now.AddDate(0, 0, -ev.DaysAgo)
		event := models.CommodityEvent{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID:        user.TenantID,
				GroupID:         group.ID,
				CreatedByUserID: user.ID,
			},
			CommodityID: commodity.ID,
			Kind:        ev.Kind,
			OccurredAt:  occurredAt,
			Note:        ev.Note,
			After:       ev.After,
		}
		if _, err := set.CommodityEventRegistry.Create(ctx, event); err != nil {
			return fmt.Errorf("create event for %q: %w", ev.CommodityName, err)
		}
	}
	return nil
}

// auditSeed is the bundled AuditLog row dataset — security-style
// events spaced out over the past few weeks so the upcoming profile
// activity tab (#1653) and any security audit views render with
// realistic content. EntityType is hard-coded to "user" inside
// seedAuditLog because every seeded action is a user-scoped event;
// add the column here when the catalogue grows entries that aren't.
type auditSeed struct {
	Action     string
	DaysAgo    int
	IPAddress  string
	UserAgent  string
	Success    bool
	ErrMessage string
}

var seedAuditCatalogue = []auditSeed{
	{Action: "user.login", DaysAgo: 1, IPAddress: "10.0.0.42", UserAgent: "Mozilla/5.0 (Macintosh; Apple WebKit/600)", Success: true},
	{Action: "user.login", DaysAgo: 2, IPAddress: "10.0.0.42", UserAgent: "Mozilla/5.0 (Macintosh; Apple WebKit/600)", Success: true},
	{Action: "user.login_failed", DaysAgo: 3, IPAddress: "203.0.113.42", UserAgent: "curl/8.4.0", Success: false, ErrMessage: "invalid credentials"},
	{Action: "user.password_changed", DaysAgo: 7, IPAddress: "10.0.0.42", UserAgent: "Mozilla/5.0 (Macintosh; Apple WebKit/600)", Success: true},
	{Action: "user.login", DaysAgo: 10, IPAddress: "10.0.0.42", UserAgent: "Mozilla/5.0 (Macintosh; Apple WebKit/600)", Success: true},
	{Action: "user.session_revoked", DaysAgo: 15, IPAddress: "10.0.0.42", UserAgent: "Mozilla/5.0 (Macintosh; Apple WebKit/600)", Success: true},
	{Action: "user.login", DaysAgo: 20, IPAddress: "10.0.0.42", UserAgent: "Mozilla/5.0 (Macintosh; Apple WebKit/600)", Success: true},
	{Action: "user.email_verification_sent", DaysAgo: 28, IPAddress: "10.0.0.42", UserAgent: "Mozilla/5.0 (Macintosh; Apple WebKit/600)", Success: true},
}

// seedAuditLog writes the bundled audit catalogue. Uses serviceCtx
// because the audit_logs table doesn't carry a group_id column.
func seedAuditLog(ctx context.Context, set *registry.Set, user *models.User) error {
	now := time.Now()
	for _, a := range seedAuditCatalogue {
		entityType := "user"
		entry := models.AuditLog{
			Timestamp:  now.AddDate(0, 0, -a.DaysAgo),
			UserID:     &user.ID,
			TenantID:   &user.TenantID,
			Action:     a.Action,
			EntityType: &entityType,
			EntityID:   &user.ID,
			IPAddress:  a.IPAddress,
			UserAgent:  a.UserAgent,
			Success:    a.Success,
		}
		if a.ErrMessage != "" {
			msg := a.ErrMessage
			entry.ErrorMessage = &msg
		}
		if _, err := set.AuditLogRegistry.Create(ctx, entry); err != nil {
			return fmt.Errorf("create audit log %s: %w", a.Action, err)
		}
	}
	return nil
}

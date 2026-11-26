package registry

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/go-extras/errx"

	"go.5x5.cz/inventario/models"
)

var (
	// ErrUserTenantMoveBlocked is returned when a user cannot move to another
	// tenant because they are still a member of a group or still own or
	// authored rows in a tenant-scoped table (#2179). The concrete error is a
	// *UserTenantMoveBlockedError, which lists what blocks the move.
	ErrUserTenantMoveBlocked = errx.NewSentinel("the user still has groups or content in their tenant")

	// ErrUserTenantChanged is returned when a write names a tenant for a user
	// that is not the tenant the users row holds: either a full-row update
	// that would change users.tenant_id (only UserTenantMover may do that), or
	// an update built from a copy read before the user moved.
	ErrUserTenantChanged = errx.NewSentinel("the user's tenant is not the one the write expects")

	// ErrUserChangedDuringMove is returned when the user's profile photo is not
	// the one the caller read before a move, because a new one was uploaded or
	// the old one removed meanwhile. The move would lose or strand that photo,
	// so it is refused; reading the user again and retrying is safe.
	ErrUserChangedDuringMove = errx.NewSentinel("the user changed while being moved; read them again and retry")

	// ErrTenantNotActive is returned when a user would move into a tenant
	// that is suspended or inactive.
	ErrTenantNotActive = errx.NewSentinel("the target tenant is not active")
)

// OwnedContentKind names a kind of row that ties a user to their tenant: a
// group membership, or a row the user created in a tenant-scoped table. The
// value is the name of the table the row lives in.
type OwnedContentKind string

// The kinds a tenant move counts, in the order UserOwnedContent lists them.
const (
	OwnedGroupMemberships       OwnedContentKind = "group_memberships"
	OwnedLocationGroups         OwnedContentKind = "location_groups"
	OwnedGroupInvites           OwnedContentKind = "group_invites"
	OwnedCommodities            OwnedContentKind = "commodities"
	OwnedFiles                  OwnedContentKind = "files"
	OwnedFileLinks              OwnedContentKind = "file_links"
	OwnedLocations              OwnedContentKind = "locations"
	OwnedAreas                  OwnedContentKind = "areas"
	OwnedTags                   OwnedContentKind = "tags"
	OwnedSellers                OwnedContentKind = "sellers"
	OwnedExports                OwnedContentKind = "exports"
	OwnedCommodityEvents        OwnedContentKind = "commodity_events"
	OwnedCommodityLoans         OwnedContentKind = "commodity_loans"
	OwnedCommodityServices      OwnedContentKind = "commodity_services"
	OwnedCommoditySupplyLinks   OwnedContentKind = "commodity_supply_links"
	OwnedMaintenanceSchedules   OwnedContentKind = "maintenance_schedules"
	OwnedMaintenanceReminders   OwnedContentKind = "maintenance_reminders"
	OwnedCommodityReminders     OwnedContentKind = "commodity_reminders"
	OwnedCommodityReminderSends OwnedContentKind = "commodity_reminder_sends"
	OwnedWarrantyReminders      OwnedContentKind = "warranty_reminders"
	OwnedStorageQuotaReminders  OwnedContentKind = "storage_quota_reminders"
	OwnedCommodityDrafts        OwnedContentKind = "commodity_drafts"
	OwnedIntakeSessions         OwnedContentKind = "intake_sessions"
	OwnedIntakeSources          OwnedContentKind = "intake_sources"
	OwnedIntakeCandidates       OwnedContentKind = "intake_candidates"
	OwnedIntakeDocuments        OwnedContentKind = "intake_documents"
	OwnedItemTypes              OwnedContentKind = "item_types"
	OwnedCustomFieldDefinitions OwnedContentKind = "custom_field_definitions"
	OwnedItemTypeCustomFields   OwnedContentKind = "item_type_custom_fields"
	OwnedSavedViews             OwnedContentKind = "saved_views"
	OwnedCurrencyMigrations     OwnedContentKind = "currency_migrations"
	OwnedCurrencyMigrationAudit OwnedContentKind = "currency_migration_audit_rows"
	OwnedRestoreOperations      OwnedContentKind = "restore_operations"
	OwnedRestoreSteps           OwnedContentKind = "restore_steps"
)

// ownedContentLabel is how a kind reads in a message: the singular and plural
// noun, or, for memberships, the whole phrase.
type ownedContentLabel struct {
	kind     OwnedContentKind
	singular string
	plural   string
}

// ownedContentLabels lists every kind in display order.
var ownedContentLabels = []ownedContentLabel{
	{OwnedGroupMemberships, "group", "groups"},
	{OwnedLocationGroups, "group created", "groups created"},
	{OwnedGroupInvites, "group invite", "group invites"},
	{OwnedCommodities, "item", "items"},
	{OwnedFiles, "file", "files"},
	{OwnedFileLinks, "file attachment", "file attachments"},
	{OwnedLocations, "location", "locations"},
	{OwnedAreas, "area", "areas"},
	{OwnedTags, "tag", "tags"},
	{OwnedSellers, "seller", "sellers"},
	{OwnedExports, "export", "exports"},
	{OwnedCommodityEvents, "item history entry", "item history entries"},
	{OwnedCommodityLoans, "loan", "loans"},
	{OwnedCommodityServices, "service record", "service records"},
	{OwnedCommoditySupplyLinks, "supply link", "supply links"},
	{OwnedMaintenanceSchedules, "maintenance schedule", "maintenance schedules"},
	{OwnedMaintenanceReminders, "maintenance reminder", "maintenance reminders"},
	{OwnedCommodityReminders, "item reminder", "item reminders"},
	{OwnedCommodityReminderSends, "sent item reminder", "sent item reminders"},
	{OwnedWarrantyReminders, "warranty reminder", "warranty reminders"},
	{OwnedStorageQuotaReminders, "storage quota reminder", "storage quota reminders"},
	{OwnedCommodityDrafts, "draft", "drafts"},
	{OwnedIntakeSessions, "intake session", "intake sessions"},
	{OwnedIntakeSources, "intake source", "intake sources"},
	{OwnedIntakeCandidates, "intake candidate", "intake candidates"},
	{OwnedIntakeDocuments, "intake document", "intake documents"},
	{OwnedItemTypes, "item type", "item types"},
	{OwnedCustomFieldDefinitions, "custom field", "custom fields"},
	{OwnedItemTypeCustomFields, "item type field", "item type fields"},
	{OwnedSavedViews, "saved view", "saved views"},
	{OwnedCurrencyMigrations, "currency change", "currency changes"},
	{OwnedCurrencyMigrationAudit, "currency change record", "currency change records"},
	{OwnedRestoreOperations, "restore", "restores"},
	{OwnedRestoreSteps, "restore step", "restore steps"},
}

// OwnedContentKinds returns every kind a tenant move counts, in display order.
func OwnedContentKinds() []OwnedContentKind {
	kinds := make([]OwnedContentKind, 0, len(ownedContentLabels))
	for _, l := range ownedContentLabels {
		kinds = append(kinds, l.kind)
	}
	return kinds
}

// OwnedContentCount is the number of rows of one kind.
type OwnedContentCount struct {
	Kind  OwnedContentKind `json:"kind"`
	Count int              `json:"count"`
}

// UserOwnedContent is what still ties a user to their tenant: one entry per
// kind with a non-zero count, in display order. An empty value means the user
// has no group memberships and created nothing in any tenant-scoped table.
type UserOwnedContent []OwnedContentCount

// NewUserOwnedContent builds a UserOwnedContent from per-kind counts. Zero
// counts are dropped; kinds outside OwnedContentKinds are kept at the end, in
// name order, so a backend that counts a new kind is never silent about it.
func NewUserOwnedContent(counts map[OwnedContentKind]int) UserOwnedContent {
	out := UserOwnedContent{}
	known := make(map[OwnedContentKind]bool, len(ownedContentLabels))
	for _, l := range ownedContentLabels {
		known[l.kind] = true
		if n := counts[l.kind]; n > 0 {
			out = append(out, OwnedContentCount{Kind: l.kind, Count: n})
		}
	}
	var unknown []OwnedContentKind
	for kind, n := range counts {
		if !known[kind] && n > 0 {
			unknown = append(unknown, kind)
		}
	}
	slices.Sort(unknown)
	for _, kind := range unknown {
		out = append(out, OwnedContentCount{Kind: kind, Count: counts[kind]})
	}
	return out
}

// IsEmpty reports whether nothing ties the user to their tenant.
func (c UserOwnedContent) IsEmpty() bool {
	return len(c) == 0
}

// Count returns the count of one kind, zero when it is not listed.
func (c UserOwnedContent) Count(kind OwnedContentKind) int {
	for _, e := range c {
		if e.Kind == kind {
			return e.Count
		}
	}
	return 0
}

// String reads the counts as a list, for example
// "member of 2 groups, 14 items, 3 files".
func (c UserOwnedContent) String() string {
	if c.IsEmpty() {
		return "nothing"
	}
	parts := make([]string, 0, len(c))
	for _, e := range c {
		parts = append(parts, describeOwnedContent(e))
	}
	return strings.Join(parts, ", ")
}

func describeOwnedContent(e OwnedContentCount) string {
	singular, plural := string(e.Kind), string(e.Kind)
	for _, l := range ownedContentLabels {
		if l.kind == e.Kind {
			singular, plural = l.singular, l.plural
			break
		}
	}
	noun := plural
	if e.Count == 1 {
		noun = singular
	}
	if e.Kind == OwnedGroupMemberships {
		return fmt.Sprintf("member of %d %s", e.Count, noun)
	}
	return fmt.Sprintf("%d %s", e.Count, noun)
}

// UserTenantMoveBlockedError is returned by UserTenantMover when the user
// still has group memberships or content. It matches ErrUserTenantMoveBlocked.
type UserTenantMoveBlockedError struct {
	Owned UserOwnedContent
}

func (e *UserTenantMoveBlockedError) Error() string {
	return "the user cannot move to another tenant: " + e.Owned.String() +
		" (a user moves only once they belong to no group and own no content)"
}

func (*UserTenantMoveBlockedError) Is(target error) bool {
	return target == ErrUserTenantMoveBlocked
}

// UserTenantMove asks UserTenantMover to move one user.
type UserTenantMove struct {
	UserID string
	// FromTenantID is the tenant the caller read the user in. The move is
	// refused with ErrUserTenantChanged when the users row holds another one.
	FromTenantID string
	ToTenantID   string
	// FromAvatarPath is the avatar_path the caller read, the photo it copied.
	// The move is refused with ErrUserChangedDuringMove when the users row
	// holds another one.
	FromAvatarPath *string
	// AvatarPath is stored as the user's avatar_path with the move: the key
	// of the profile photo copied into the new tenant's storage, or nil when
	// the user has no photo.
	AvatarPath *string
}

// PhotoUnchanged reports whether current, the avatar_path the users row holds,
// is the photo the caller read. A missing and an empty path are the same.
func (m UserTenantMove) PhotoUnchanged(current *string) bool {
	return derefPath(current) == derefPath(m.FromAvatarPath)
}

func derefPath(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// UserTenantMover moves a user to another tenant (#2179). A move is allowed
// only for a user who has no group memberships and owns or authored nothing in
// any tenant-scoped table, so nothing the user made is left behind in a
// tenant they can no longer reach.
type UserTenantMover interface {
	// CountOwnedContent counts the user's group memberships and the rows
	// they created in every tenant-scoped table, in any tenant. It changes
	// nothing.
	CountOwnedContent(ctx context.Context, userID string) (UserOwnedContent, error)

	// MoveUserToTenant moves the user in one transaction that holds a lock on
	// the users row, so no membership or content referencing the user can
	// be written between the check and the move. It counts again under the
	// lock and returns a *UserTenantMoveBlockedError, changing nothing, when
	// the count is not empty.
	//
	// Otherwise it deletes the user's sessions (refresh tokens), API tokens,
	// pending one-time tokens (email verification, password reset, magic
	// link) and transient per-user rows bound to the old tenant, moves the
	// per-user rows that belong to the person rather than the tenant
	// (settings, MFA, sign-in identities, login history, scan audit, digest
	// sends) and rewrites the users row with the new tenant, no default
	// group and move.AvatarPath. The password hash stays.
	//
	// It returns registry.ErrNotFound when the user or the target tenant does
	// not exist, ErrTenantNotActive when the target tenant is not active,
	// ErrUserTenantChanged when the user is no longer in move.FromTenantID,
	// ErrUserChangedDuringMove when the photo is no longer
	// move.FromAvatarPath, and ErrEmailAlreadyExists when the target tenant
	// has another user with the same email.
	MoveUserToTenant(ctx context.Context, move UserTenantMove) (*models.User, error)
}

package export

import (
	"encoding/json"

	"go.5x5.cz/inventario/internal/inb"
	"go.5x5.cz/inventario/models"
)

// This file defines the documents of a personal data export (#2177): the
// archive a user downloads with a copy of their own data (GDPR Art. 20). It
// is a gzip-compressed tar like the inner payload of a group backup, without
// the signed `.inb` wrapper, and it is never restorable.
//
// Layout:
//
//	manifest.json                                   — PersonalManifest, first member
//	profile.json                                    — PersonalProfile
//	profile/avatar<ext>                             — the user's avatar, when they have one
//	groups/<group-uuid>/group.json                  — PersonalGroupDocument
//	groups/<group-uuid>/files/<file-uuid>/<name>    — bytes of a file the user uploaded
//
// Every id is the entity's immutable UUID, except the user ids, which are the
// users' ids as the API shows them. Another member of a group appears only as
// such an id, never by name or email.

// PersonalDataExportKind is the manifest `kind` of a personal data export.
// The restore path refuses an archive whose manifest carries it.
const PersonalDataExportKind = inb.PersonalDataExportKind

// PersonalFormatVersion is the manifest `formatVersion` of a personal data
// export. A reader of 1.x reads any 1.y archive: minor versions only add
// optional members and fields.
const PersonalFormatVersion = "1.0"

// PersonalNotice is the manifest's statement of what the archive is.
const PersonalNotice = "This archive is a copy of your personal data in Inventario: your profile, " +
	"your settings, your group memberships, and what you created in each group. " +
	"It is not a group backup and cannot be restored."

// Member names of the personal data export.
const (
	PersonalManifestName = "manifest.json"
	PersonalProfileName  = "profile.json"
	personalAvatarPrefix = "profile/avatar"
	personalGroupsPrefix = "groups/"
)

// PersonalManifest is the manifest.json of a personal data export.
type PersonalManifest struct {
	Kind          string `json:"kind"`
	FormatVersion string `json:"formatVersion"`
	ExportID      string `json:"exportId"`
	// SubjectUserID is the id of the user whose data the archive holds.
	SubjectUserID string                  `json:"subjectUserId"`
	CreatedAt     string                  `json:"createdAt"`
	Format        string                  `json:"format"`
	Compression   string                  `json:"compression"`
	Restorable    bool                    `json:"restorable"`
	Notice        string                  `json:"notice"`
	ProfileFile   string                  `json:"profileFile"`
	Groups        []PersonalManifestGroup `json:"groups"`
	Statistics    PersonalStatistics      `json:"statistics"`
}

// PersonalManifestGroup names a group and the member holding what the user
// created in it.
type PersonalManifestGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	File string `json:"file"`
}

// PersonalStatistics counts what the archive holds.
type PersonalStatistics struct {
	GroupCount               int   `json:"groupCount"`
	ItemCount                int   `json:"itemCount"`
	FileCount                int   `json:"fileCount"`
	TotalFileSize            int64 `json:"totalFileSize"`
	LoanCount                int   `json:"loanCount"`
	MaintenanceScheduleCount int   `json:"maintenanceScheduleCount"`
	ServiceRecordCount       int   `json:"serviceRecordCount"`
	ReminderCount            int   `json:"reminderCount"`
	SupplyLinkCount          int   `json:"supplyLinkCount"`
	DraftCount               int   `json:"draftCount"`
	SavedViewCount           int   `json:"savedViewCount"`
	HistoryEntryCount        int   `json:"historyEntryCount"`
}

// PersonalProfile is the profile.json of a personal data export.
type PersonalProfile struct {
	User                    PersonalUser                     `json:"user"`
	Settings                models.SettingsObject            `json:"settings"`
	Memberships             []PersonalMembership             `json:"memberships"`
	NotificationPreferences []PersonalNotificationPreference `json:"notificationPreferences"`
}

// PersonalUser is the user's account.
type PersonalUser struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	IsActive    bool   `json:"isActive"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	LastLoginAt string `json:"lastLoginAt,omitempty"`
	// DefaultGroupID is the UUID of the group the user opens first.
	DefaultGroupID string `json:"defaultGroupId,omitempty"`
	// AvatarFile is the member holding the avatar image.
	AvatarFile string `json:"avatarFile,omitempty"`
}

// PersonalMembership is one group the user belongs to.
type PersonalMembership struct {
	GroupID   string `json:"groupId"`
	GroupName string `json:"groupName"`
	Role      string `json:"role"`
	JoinedAt  string `json:"joinedAt"`
}

// PersonalNotificationPreference is a notification category the user turned
// on or off for one group.
type PersonalNotificationPreference struct {
	GroupID  string `json:"groupId"`
	Category string `json:"category"`
	Enabled  bool   `json:"enabled"`
}

// PersonalGroupDocument holds what the user created in one group. Every list
// is present, empty when there is nothing.
type PersonalGroupDocument struct {
	Group                PersonalGroup                 `json:"group"`
	Items                []PersonalItem                `json:"items"`
	Files                []PersonalFile                `json:"files"`
	Loans                []PersonalLoan                `json:"loans"`
	MaintenanceSchedules []PersonalMaintenanceSchedule `json:"maintenanceSchedules"`
	ServiceRecords       []PersonalServiceRecord       `json:"serviceRecords"`
	Reminders            []PersonalReminder            `json:"reminders"`
	SupplyLinks          []PersonalSupplyLink          `json:"supplyLinks"`
	Drafts               []PersonalDraft               `json:"drafts"`
	SavedViews           []PersonalSavedView           `json:"savedViews"`
	SavedViewPlacements  []PersonalSavedViewPlacement  `json:"savedViewPlacements"`
	History              []PersonalHistoryEntry        `json:"history"`
}

// PersonalGroup names the group a document belongs to.
type PersonalGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// PersonalItem is an item the user created, also one in the group's Trash.
// Its location, area and seller are given by name.
type PersonalItem struct {
	ID                     string                `json:"id"`
	Name                   string                `json:"name"`
	ShortName              string                `json:"shortName,omitempty"`
	Brand                  string                `json:"brand,omitempty"`
	Model                  string                `json:"model,omitempty"`
	Type                   string                `json:"type,omitempty"`
	Location               string                `json:"location,omitempty"`
	Area                   string                `json:"area,omitempty"`
	Count                  int                   `json:"count"`
	Status                 string                `json:"status,omitempty"`
	OriginalPrice          string                `json:"originalPrice,omitempty"`
	OriginalPriceCurrency  string                `json:"originalPriceCurrency,omitempty"`
	ConvertedOriginalPrice string                `json:"convertedOriginalPrice,omitempty"`
	CurrentPrice           string                `json:"currentPrice,omitempty"`
	SalePrice              string                `json:"salePrice,omitempty"`
	AcquisitionPrice       string                `json:"acquisitionPrice,omitempty"`
	AcquisitionCurrency    string                `json:"acquisitionCurrency,omitempty"`
	Seller                 string                `json:"seller,omitempty"`
	SerialNumber           string                `json:"serialNumber,omitempty"`
	ExtraSerialNumbers     []string              `json:"extraSerialNumbers,omitempty"`
	PartNumbers            []string              `json:"partNumbers,omitempty"`
	Tags                   []string              `json:"tags,omitempty"`
	PurchaseDate           string                `json:"purchaseDate,omitempty"`
	RegisteredDate         string                `json:"registeredDate,omitempty"`
	LastModifiedDate       string                `json:"lastModifiedDate,omitempty"`
	WarrantyExpiresAt      string                `json:"warrantyExpiresAt,omitempty"`
	WarrantyNotes          string                `json:"warrantyNotes,omitempty"`
	StatusDate             string                `json:"statusDate,omitempty"`
	StatusNote             string                `json:"statusNote,omitempty"`
	URLs                   []string              `json:"urls,omitempty"`
	Comments               string                `json:"comments,omitempty"`
	Draft                  bool                  `json:"draft"`
	AssetNumber            int                   `json:"assetNumber,omitempty"`
	CustomFields           []PersonalCustomValue `json:"customFields,omitempty"`
	// InTrash marks an item in the group's Trash.
	InTrash bool `json:"inTrash,omitempty"`
}

// PersonalCustomValue is the value of a custom field, by the field's name.
// Value is in the form the API writes it, except that a link to another item
// holds that item's UUID. Label is the chosen option of a select field.
type PersonalCustomValue struct {
	Name  string          `json:"name"`
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
	Label string          `json:"label,omitempty"`
}

// PersonalFile is a file the user uploaded. Path is the member holding its
// bytes; it is empty, and Missing set, when the bytes are no longer in
// storage. InTrash marks a file in the group's Trash.
type PersonalFile struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Title       string             `json:"title,omitempty"`
	Description string             `json:"description,omitempty"`
	MimeType    string             `json:"mimeType,omitempty"`
	Type        string             `json:"type,omitempty"`
	Category    string             `json:"category,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	SizeBytes   int64              `json:"sizeBytes"`
	Path        string             `json:"path,omitempty"`
	Missing     bool               `json:"missing,omitempty"`
	InTrash     bool               `json:"inTrash,omitempty"`
	CreatedAt   string             `json:"createdAt"`
	UpdatedAt   string             `json:"updatedAt"`
	Links       []PersonalFileLink `json:"links"`
}

// PersonalFileLink is where a file is attached: an item, a location, an area
// or a draft, by UUID, with the item's, location's or area's name.
type PersonalFileLink struct {
	EntityType string `json:"entityType"`
	EntityID   string `json:"entityId"`
	EntityName string `json:"entityName,omitempty"`
	Meta       string `json:"meta,omitempty"`
}

// PersonalItemRef names the item a record belongs to. The item may be one
// another member created.
type PersonalItemRef struct {
	ItemID   string `json:"itemId,omitempty"`
	ItemName string `json:"itemName,omitempty"`
}

// PersonalLoan is a loan the user recorded.
type PersonalLoan struct {
	PersonalItemRef
	BorrowerName    string `json:"borrowerName"`
	BorrowerContact string `json:"borrowerContact,omitempty"`
	BorrowerNote    string `json:"borrowerNote,omitempty"`
	LentAt          string `json:"lentAt"`
	DueBackAt       string `json:"dueBackAt,omitempty"`
	ReturnedAt      string `json:"returnedAt,omitempty"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// PersonalMaintenanceSchedule is a maintenance schedule the user created.
type PersonalMaintenanceSchedule struct {
	PersonalItemRef
	Title        string `json:"title"`
	IntervalDays int    `json:"intervalDays"`
	NextDueAt    string `json:"nextDueAt"`
	LastDoneAt   string `json:"lastDoneAt,omitempty"`
	Notes        string `json:"notes,omitempty"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// PersonalServiceRecord is a service visit the user recorded.
type PersonalServiceRecord struct {
	PersonalItemRef
	ProviderName     string `json:"providerName"`
	ProviderContact  string `json:"providerContact,omitempty"`
	Reason           string `json:"reason,omitempty"`
	SentAt           string `json:"sentAt"`
	ExpectedReturnAt string `json:"expectedReturnAt,omitempty"`
	ReturnedAt       string `json:"returnedAt,omitempty"`
	CostAmount       string `json:"costAmount,omitempty"`
	CostCurrency     string `json:"costCurrency,omitempty"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

// PersonalReminder is a reminder the user set on an item.
type PersonalReminder struct {
	PersonalItemRef
	Title            string `json:"title"`
	DueOn            string `json:"dueOn"`
	RemindDaysBefore []int  `json:"remindDaysBefore,omitempty"`
	RepeatUnit       string `json:"repeatUnit,omitempty"`
	RepeatInterval   int    `json:"repeatInterval,omitempty"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

// PersonalSupplyLink is a link to buy supplies the user added to an item.
type PersonalSupplyLink struct {
	PersonalItemRef
	Label     string `json:"label"`
	URL       string `json:"url"`
	Notes     string `json:"notes,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// PersonalDraft is an open draft of a new item the user started. Values are
// the form values as stored, without the ids of the area and the seller and
// without the custom field values, which Location, Area, Seller and
// CustomFields give by name instead.
type PersonalDraft struct {
	ID           string                `json:"id"`
	Values       map[string]any        `json:"values"`
	Location     string                `json:"location,omitempty"`
	Area         string                `json:"area,omitempty"`
	Seller       string                `json:"seller,omitempty"`
	CustomFields []PersonalCustomValue `json:"customFields,omitempty"`
	CreatedAt    string                `json:"createdAt"`
	UpdatedAt    string                `json:"updatedAt"`
}

// PersonalSavedView is a saved view the user created.
type PersonalSavedView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Icon         string `json:"icon,omitempty"`
	Entity       string `json:"entity"`
	Query        string `json:"query"`
	QueryVersion int    `json:"queryVersion"`
	Layout       string `json:"layout"`
	Shared       bool   `json:"shared"`
	SortOrder    int    `json:"sortOrder"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// PersonalSavedViewPlacement is where the user shows a saved view, theirs or
// one another member shared.
type PersonalSavedViewPlacement struct {
	ViewID          string `json:"viewId"`
	ShowInSidebar   bool   `json:"showInSidebar"`
	ShowOnDashboard bool   `json:"showOnDashboard"`
}

// PersonalHistoryEntry is an entry of an item's history the user made.
// Changes holds the fields the entry changed, each with the value it set (null
// for a value it removed): what the user wrote. The values the fields had
// before, and the fields the entry recorded without changing them, are left
// out, since other members may have typed them.
type PersonalHistoryEntry struct {
	PersonalItemRef
	Kind       string         `json:"kind"`
	OccurredAt string         `json:"occurredAt"`
	Changes    map[string]any `json:"changes,omitempty"`
	Note       string         `json:"note,omitempty"`
}

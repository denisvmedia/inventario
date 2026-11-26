package export

import (
	"archive/tar"
	"cmp"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/blobkeys"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// trashPageSize is the page size of the Trash listings a personal data export
// reads.
const trashPageSize = 500

// personalBuilder collects one user's data across the groups they belong to
// and writes it as a personal data export. Like the group backup it plans
// everything first, metadata only, so the manifest with its statistics is the
// first member, and then streams the file bytes from blob storage.
//
// A row belongs to the user when they created it (created_by_user_id), or,
// for rows that carry no author, when it is theirs (user_id). Rows other
// members created are left out, also on the user's own items.
type personalBuilder struct {
	fs     *registry.FactorySet
	bucket *blob.Bucket
	user   *models.User

	stats   PersonalStatistics
	profile PersonalProfile
	avatar  *plannedBlob
	groups  []plannedPersonalGroup
}

// plannedBlob is a file whose bytes follow the documents.
type plannedBlob struct {
	member string
	key    string
	size   int64
}

// plannedPersonalGroup is the document of one group and the file bytes that
// follow it.
type plannedPersonalGroup struct {
	entry PersonalManifestGroup
	doc   PersonalGroupDocument
	files []plannedBlob
}

// writePersonalDataExport writes the personal data export of user into w as
// gzip(tar) and returns what it holds.
func writePersonalDataExport(ctx context.Context, fs *registry.FactorySet, bucket *blob.Bucket, user *models.User, exportID string, createdAt time.Time, w io.Writer) (PersonalStatistics, error) {
	b := &personalBuilder{fs: fs, bucket: bucket, user: user}
	if err := b.plan(appctx.WithUser(ctx, user)); err != nil {
		return PersonalStatistics{}, err
	}

	gzw, err := gzip.NewWriterLevel(w, gzipLevel)
	if err != nil {
		return PersonalStatistics{}, errxtrace.Wrap("failed to create gzip writer", err)
	}
	tw := tar.NewWriter(gzw)

	manifest := PersonalManifest{
		Kind:          PersonalDataExportKind,
		FormatVersion: PersonalFormatVersion,
		ExportID:      exportID,
		SubjectUserID: user.ID,
		CreatedAt:     createdAt.UTC().Format(time.RFC3339),
		Format:        "json",
		Compression:   "tar.gz",
		Restorable:    false,
		Notice:        PersonalNotice,
		ProfileFile:   PersonalProfileName,
		Groups:        make([]PersonalManifestGroup, 0, len(b.groups)),
		Statistics:    b.stats,
	}
	for _, g := range b.groups {
		manifest.Groups = append(manifest.Groups, g.entry)
	}
	if err := writePersonalJSON(tw, PersonalManifestName, manifest); err != nil {
		return PersonalStatistics{}, err
	}
	if err := writePersonalJSON(tw, PersonalProfileName, b.profile); err != nil {
		return PersonalStatistics{}, err
	}
	if b.avatar != nil {
		if err := b.writeBlob(ctx, tw, *b.avatar); err != nil {
			return PersonalStatistics{}, err
		}
	}
	for _, g := range b.groups {
		if err := writePersonalJSON(tw, g.entry.File, g.doc); err != nil {
			return PersonalStatistics{}, err
		}
		for _, f := range g.files {
			if err := b.writeBlob(ctx, tw, f); err != nil {
				return PersonalStatistics{}, err
			}
		}
	}

	if err := tw.Close(); err != nil {
		return PersonalStatistics{}, errxtrace.Wrap("failed to close tar", err)
	}
	if err := gzw.Close(); err != nil {
		return PersonalStatistics{}, errxtrace.Wrap("failed to close gzip writer", err)
	}
	return b.stats, nil
}

func writePersonalJSON(tw *tar.Writer, name string, v any) error {
	data, err := encodeJSONMember(name, v)
	if err != nil {
		return err
	}
	return writeTarMember(tw, name, data)
}

func (b *personalBuilder) writeBlob(ctx context.Context, tw *tar.Writer, f plannedBlob) error {
	n, err := streamTarMember(ctx, tw, b.bucket, f.member, f.key, f.size)
	if err != nil {
		return err
	}
	if n != f.size {
		return errxtrace.Classify(errx.NewSentinel("file changed while it was archived"), errx.Attrs("member", f.member, "expected", f.size, "written", n))
	}
	return nil
}

// blobSize returns the size of the blob at key, and false when it is not in
// storage. Any other failure is an error: an archive that quietly lacks a file
// the user has is worse than one that failed.
func (b *personalBuilder) blobSize(ctx context.Context, key string) (int64, bool, error) {
	if key == "" {
		return 0, false, nil
	}
	attrs, err := b.bucket.Attributes(ctx, key)
	switch {
	case err == nil:
		return attrs.Size, true, nil
	case gcerrors.Code(err) == gcerrors.NotFound:
		return 0, false, nil
	default:
		return 0, false, errxtrace.Wrap("failed to read blob attributes", err, errx.Attrs("blob_key", key))
	}
}

func (b *personalBuilder) plan(ctx context.Context) error {
	user := b.user
	b.profile.User = PersonalUser{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		IsActive:  user.IsActive,
		CreatedAt: formatTime(user.CreatedAt),
		UpdatedAt: formatTime(user.UpdatedAt),
	}
	if user.LastLoginAt != nil {
		b.profile.User.LastLoginAt = formatTime(*user.LastLoginAt)
	}
	if user.AvatarPath != nil && *user.AvatarPath != "" {
		size, present, err := b.blobSize(ctx, *user.AvatarPath)
		if err != nil {
			return err
		}
		if present {
			member := personalAvatarPrefix + strings.ToLower(path.Ext(*user.AvatarPath))
			b.avatar = &plannedBlob{member: member, key: *user.AvatarPath, size: size}
			b.profile.User.AvatarFile = member
		}
	}

	settingsReg, err := b.fs.SettingsRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create settings registry", err)
	}
	settings, err := settingsReg.Get(ctx)
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		return errxtrace.Wrap("failed to read settings", err)
	}
	b.profile.Settings = settings

	memberships, err := b.fs.GroupMembershipRegistry.ListByUser(ctx, user.TenantID, user.ID)
	if err != nil {
		return errxtrace.Wrap("failed to list group memberships", err)
	}
	slices.SortFunc(memberships, func(x, y *models.GroupMembership) int {
		return cmp.Or(x.JoinedAt.Compare(y.JoinedAt), cmp.Compare(x.GroupID, y.GroupID))
	})

	b.profile.Memberships = make([]PersonalMembership, 0, len(memberships))
	b.profile.NotificationPreferences = []PersonalNotificationPreference{}
	for _, m := range memberships {
		group, err := b.fs.LocationGroupRegistry.Get(ctx, m.GroupID)
		if err != nil {
			return errxtrace.Wrap("failed to get group", err, errx.Attrs("group_id", m.GroupID))
		}
		b.profile.Memberships = append(b.profile.Memberships, PersonalMembership{
			GroupID:   group.UUID,
			GroupName: group.Name,
			Role:      string(m.Role),
			JoinedAt:  formatTime(m.JoinedAt),
		})
		if user.DefaultGroupID != nil && *user.DefaultGroupID == group.ID {
			b.profile.User.DefaultGroupID = group.UUID
		}
		if err := b.planNotificationPrefs(ctx, group); err != nil {
			return err
		}

		planned, err := b.planGroup(appctx.WithGroup(ctx, group), group, m)
		if err != nil {
			return errxtrace.Wrap("failed to collect the user's data in a group", err, errx.Attrs("group_id", group.ID))
		}
		b.groups = append(b.groups, planned)
	}
	b.stats.GroupCount = len(b.groups)
	return nil
}

func (b *personalBuilder) planNotificationPrefs(ctx context.Context, group *models.LocationGroup) error {
	if b.fs.GroupNotificationPrefRegistry == nil {
		return nil
	}
	prefs, err := b.fs.GroupNotificationPrefRegistry.ListByUserGroup(ctx, b.user.TenantID, group.ID, b.user.ID)
	if err != nil {
		return errxtrace.Wrap("failed to list notification preferences", err)
	}
	slices.SortFunc(prefs, func(x, y *models.GroupNotificationPref) int { return cmp.Compare(x.Category, y.Category) })
	for _, p := range prefs {
		b.profile.NotificationPreferences = append(b.profile.NotificationPreferences, PersonalNotificationPreference{
			GroupID:  group.UUID,
			Category: p.Category,
			Enabled:  p.Enabled,
		})
	}
	return nil
}

// groupLookups resolves the ids a group's rows hold to the names and UUIDs
// the archive carries.
type groupLookups struct {
	locations  map[string]*models.Location
	areas      map[string]*models.Area
	sellerName map[string]string
	typeName   map[models.CommodityType]string
	tagLabel   map[models.TagKind]map[string]string
	fields     map[string]*models.CustomFieldDefinition
	items      map[string]*models.Commodity
	drafts     map[string]*models.CommodityDraft
	// live and trashed are the group's items, sorted by name.
	live, trashed []*models.Commodity
}

func (l *groupLookups) itemRef(id string) PersonalItemRef {
	item := l.items[id]
	if item == nil {
		return PersonalItemRef{}
	}
	return PersonalItemRef{ItemID: item.UUID, ItemName: item.Name}
}

// place returns the names of the location and the area of areaID.
func (l *groupLookups) place(areaID *string) (location, area string) {
	if areaID == nil {
		return "", ""
	}
	a := l.areas[*areaID]
	if a == nil {
		return "", ""
	}
	if loc := l.locations[a.LocationID]; loc != nil {
		return loc.Name, a.Name
	}
	return "", a.Name
}

func (b *personalBuilder) planGroup(ctx context.Context, group *models.LocationGroup, m *models.GroupMembership) (plannedPersonalGroup, error) {
	planned := plannedPersonalGroup{
		entry: PersonalManifestGroup{
			ID:   group.UUID,
			Name: group.Name,
			File: personalGroupsPrefix + group.UUID + "/group.json",
		},
		doc: PersonalGroupDocument{
			Group:                PersonalGroup{ID: group.UUID, Name: group.Name, Role: string(m.Role)},
			Items:                []PersonalItem{},
			Files:                []PersonalFile{},
			Loans:                []PersonalLoan{},
			MaintenanceSchedules: []PersonalMaintenanceSchedule{},
			ServiceRecords:       []PersonalServiceRecord{},
			Reminders:            []PersonalReminder{},
			SupplyLinks:          []PersonalSupplyLink{},
			Drafts:               []PersonalDraft{},
			SavedViews:           []PersonalSavedView{},
			SavedViewPlacements:  []PersonalSavedViewPlacement{},
			History:              []PersonalHistoryEntry{},
		},
	}

	lookups, err := b.loadGroupLookups(ctx)
	if err != nil {
		return planned, err
	}

	for _, item := range lookups.live {
		if item.CreatedByUserID == b.user.ID {
			planned.doc.Items = append(planned.doc.Items, lookups.personalItem(item))
		}
	}
	for _, item := range lookups.trashed {
		if item.CreatedByUserID == b.user.ID {
			pi := lookups.personalItem(item)
			pi.InTrash = true
			planned.doc.Items = append(planned.doc.Items, pi)
		}
	}
	b.stats.ItemCount += len(planned.doc.Items)

	if err := b.planFiles(ctx, lookups, &planned); err != nil {
		return planned, err
	}

	steps := []func(context.Context, *groupLookups, *PersonalGroupDocument) error{
		b.planLoans,
		b.planMaintenanceSchedules,
		b.planServiceRecords,
		b.planReminders,
		b.planSupplyLinks,
		b.planDrafts,
		b.planSavedViews,
		b.planHistory,
	}
	for _, step := range steps {
		if err := step(ctx, lookups, &planned.doc); err != nil {
			return planned, err
		}
	}
	return planned, nil
}

// personalItem is com as the archive holds it. An item in the Trash has no
// area; the area it was deleted from stands in.
func (l *groupLookups) personalItem(com *models.Commodity) PersonalItem {
	areaID := com.AreaID
	if areaID == nil {
		areaID = com.DeletedFromAreaID
	}
	location, area := l.place(areaID)
	item := PersonalItem{
		ID:                     com.UUID,
		Name:                   com.Name,
		ShortName:              com.ShortName,
		Brand:                  com.Brand,
		Model:                  com.Model,
		Type:                   l.typeOf(com.Type),
		Location:               location,
		Area:                   area,
		Count:                  com.Count,
		Status:                 string(com.Status),
		OriginalPrice:          ptrDecimalString(com.OriginalPrice),
		OriginalPriceCurrency:  string(com.OriginalPriceCurrency),
		ConvertedOriginalPrice: ptrDecimalString(com.ConvertedOriginalPrice),
		CurrentPrice:           ptrDecimalString(com.CurrentPrice),
		SalePrice:              ptrDecimalString(com.SalePrice),
		AcquisitionPrice:       ptrDecimalString(com.AcquisitionPrice),
		SerialNumber:           com.SerialNumber,
		ExtraSerialNumbers:     []string(com.ExtraSerialNumbers),
		PartNumbers:            []string(com.PartNumbers),
		Tags:                   l.tagsOf(models.TagKindCommodity, com.Tags),
		PurchaseDate:           dateString(com.PurchaseDate),
		RegisteredDate:         dateString(com.RegisteredDate),
		LastModifiedDate:       dateString(com.LastModifiedDate),
		WarrantyExpiresAt:      dateString(com.WarrantyExpiresAt),
		WarrantyNotes:          com.WarrantyNotes,
		StatusDate:             dateString(com.StatusDate),
		StatusNote:             com.StatusNote,
		URLs:                   urlStrings(com.URLs),
		Comments:               com.Comments,
		Draft:                  com.Draft,
		CustomFields:           l.customValues(map[string]json.RawMessage(com.CustomFields)),
	}
	if com.AcquisitionCurrency != nil {
		item.AcquisitionCurrency = string(*com.AcquisitionCurrency)
	}
	if com.AssetNumber != nil {
		item.AssetNumber = *com.AssetNumber
	}
	if com.SellerID != nil {
		item.Seller = l.sellerName[*com.SellerID]
	}
	return item
}

// customValues returns the values of fields the group has, by field name. A
// select value gets its option's label, and an item link the linked item's
// UUID; a link to an item that is gone is left out.
func (l *groupLookups) customValues(values map[string]json.RawMessage) []PersonalCustomValue {
	if len(values) == 0 {
		return nil
	}
	out := make([]PersonalCustomValue, 0, len(values))
	for id, raw := range values {
		d := l.fields[id]
		if d == nil || models.IsNullJSON(raw) {
			continue
		}
		v := PersonalCustomValue{Name: d.Name, Type: string(d.Type), Value: raw}
		switch d.Type {
		case models.CustomFieldTypeSelect:
			var option string
			if json.Unmarshal(raw, &option) == nil {
				for _, o := range d.Options {
					if o.ID == option {
						v.Label = o.Label
					}
				}
			}
		case models.CustomFieldTypeItemLink:
			var target string
			if json.Unmarshal(raw, &target) != nil || l.items[target] == nil {
				continue
			}
			encoded, err := json.Marshal(l.items[target].UUID)
			if err != nil {
				continue
			}
			v.Value = encoded
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	slices.SortFunc(out, func(x, y PersonalCustomValue) int { return cmp.Compare(x.Name, y.Name) })
	return out
}

func (b *personalBuilder) loadGroupLookups(ctx context.Context) (*groupLookups, error) {
	lookups := &groupLookups{
		locations:  map[string]*models.Location{},
		areas:      map[string]*models.Area{},
		sellerName: map[string]string{},
		typeName:   map[models.CommodityType]string{},
		tagLabel:   map[models.TagKind]map[string]string{},
		fields:     map[string]*models.CustomFieldDefinition{},
		items:      map[string]*models.Commodity{},
		drafts:     map[string]*models.CommodityDraft{},
	}

	if err := b.loadStructure(ctx, lookups); err != nil {
		return nil, err
	}

	comReg, err := b.fs.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity registry", err)
	}
	items, err := comReg.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list items", err)
	}
	trashed, err := listAllPages(func(offset int) ([]*models.Commodity, int, error) {
		return comReg.ListTrashed(ctx, offset, trashPageSize)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list items in Trash", err)
	}
	for _, item := range items {
		lookups.items[item.ID] = item
	}
	for _, item := range trashed {
		lookups.items[item.ID] = item
	}
	sortItems(items)
	sortItems(trashed)
	lookups.live, lookups.trashed = items, trashed
	if err := b.loadTypeNames(ctx, lookups); err != nil {
		return nil, err
	}

	if b.fs.CommodityDraftRegistryFactory != nil {
		reg, err := b.fs.CommodityDraftRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to create draft registry", err)
		}
		drafts, err := reg.List(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to list drafts", err)
		}
		for _, d := range drafts {
			lookups.drafts[d.ID] = d
		}
	}
	return lookups, nil
}

// loadTypeNames resolves the names of the item types the group's items have.
// It reads the types one by one, since listing them seeds the built-in ones
// into a group that has none.
func (b *personalBuilder) loadTypeNames(ctx context.Context, lookups *groupLookups) error {
	if b.fs.ItemTypeRegistryFactory == nil {
		return nil
	}
	reg, err := b.fs.ItemTypeRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create item type registry", err)
	}
	for _, item := range lookups.items {
		if item.Type == "" {
			continue
		}
		if _, done := lookups.typeName[item.Type]; done {
			continue
		}
		t, err := reg.GetBySlug(ctx, item.Type)
		switch {
		case err == nil:
			lookups.typeName[item.Type] = t.DisplayName()
		case errors.Is(err, registry.ErrNotFound):
			// A group whose types were never seeded still has the built-in
			// ones under their default names.
			lookups.typeName[item.Type] = (&models.ItemType{Slug: item.Type}).DisplayName()
		default:
			return errxtrace.Wrap("failed to get item type", err)
		}
	}
	return nil
}

// typeOf returns the item type slug as the archive holds it.
func (l *groupLookups) typeOf(slug models.CommodityType) *PersonalName {
	if slug == "" {
		return nil
	}
	name := l.typeName[slug]
	if name == "" {
		name = string(slug)
	}
	return &PersonalName{Name: name, Slug: string(slug)}
}

// tagsOf returns tag slugs of kind as the archive holds them.
func (l *groupLookups) tagsOf(kind models.TagKind, slugs []string) []PersonalName {
	if len(slugs) == 0 {
		return nil
	}
	out := make([]PersonalName, 0, len(slugs))
	for _, slug := range slugs {
		name := l.tagLabel[kind][slug]
		if name == "" {
			name = slug
		}
		out = append(out, PersonalName{Name: name, Slug: slug})
	}
	return out
}

// loadStructure loads the group's locations, areas, sellers, tags and custom
// fields, which the archive gives by name.
func (b *personalBuilder) loadStructure(ctx context.Context, lookups *groupLookups) error {
	locReg, err := b.fs.LocationRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create location registry", err)
	}
	locations, err := locReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list locations", err)
	}
	for _, loc := range locations {
		lookups.locations[loc.ID] = loc
	}

	areaReg, err := b.fs.AreaRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create area registry", err)
	}
	areas, err := areaReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list areas", err)
	}
	for _, a := range areas {
		lookups.areas[a.ID] = a
	}

	if b.fs.SellerRegistryFactory != nil {
		reg, err := b.fs.SellerRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create seller registry", err)
		}
		sellers, err := reg.List(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to list sellers", err)
		}
		for _, s := range sellers {
			lookups.sellerName[s.ID] = s.Name
		}
	}

	if b.fs.TagRegistryFactory != nil {
		reg, err := b.fs.TagRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create tag registry", err)
		}
		tags, err := reg.List(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to list tags", err)
		}
		for _, t := range tags {
			if lookups.tagLabel[t.Kind] == nil {
				lookups.tagLabel[t.Kind] = map[string]string{}
			}
			lookups.tagLabel[t.Kind][t.Slug] = t.Label
		}
	}

	if b.fs.CustomFieldRegistryFactory != nil {
		reg, err := b.fs.CustomFieldRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create custom field registry", err)
		}
		fields, err := reg.List(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to list custom fields", err)
		}
		for _, d := range fields {
			lookups.fields[d.ID] = d
		}
	}

	return nil
}

func sortItems(items []*models.Commodity) {
	slices.SortFunc(items, func(x, y *models.Commodity) int {
		return cmp.Or(cmp.Compare(x.Name, y.Name), cmp.Compare(x.UUID, y.UUID))
	})
}

// listAllPages reads a paged listing to its end.
func listAllPages[T any](page func(offset int) ([]T, int, error)) ([]T, error) {
	var out []T
	for {
		rows, total, err := page(len(out))
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
		if len(rows) == 0 || len(out) >= total {
			return out, nil
		}
	}
}

func (b *personalBuilder) planFiles(ctx context.Context, l *groupLookups, planned *plannedPersonalGroup) error {
	fileReg, err := b.fs.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create file registry", err)
	}
	files, err := fileReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list files", err)
	}
	trashed, err := listAllPages(func(offset int) ([]*models.FileEntity, int, error) {
		return fileReg.ListTrashed(ctx, offset, trashPageSize)
	})
	if err != nil {
		return errxtrace.Wrap("failed to list files in Trash", err)
	}

	inTrash := make(map[string]bool, len(trashed))
	for _, f := range trashed {
		inTrash[f.ID] = true
	}
	all := append(slices.Clone(files), trashed...)
	slices.SortFunc(all, func(x, y *models.FileEntity) int {
		return cmp.Or(x.CreatedAt.Compare(y.CreatedAt), cmp.Compare(x.UUID, y.UUID))
	})

	for _, file := range all {
		// A group backup is a copy of everybody's data, not the user's.
		if file == nil || file.CreatedByUserID != b.user.ID || file.IsExportOwned() {
			continue
		}
		pf := PersonalFile{
			ID:          file.UUID,
			Title:       file.Title,
			Description: file.Description,
			Type:        string(file.Type),
			Category:    string(file.Category),
			Tags:        l.tagsOf(models.TagKindFile, file.Tags),
			CreatedAt:   formatTime(file.CreatedAt),
			UpdatedAt:   formatTime(file.UpdatedAt),
			InTrash:     inTrash[file.ID],
			Links:       l.fileLinks(file.Links),
		}
		var key string
		if file.File != nil {
			pf.Name = fileMemberName(file)
			pf.MimeType = file.MIMEType
			key = file.OriginalPath
		}
		size, present, err := b.blobSize(ctx, key)
		if err != nil {
			return err
		}
		if present {
			pf.SizeBytes = size
			pf.Path = blobkeys.SanitizeArchivePath(fmt.Sprintf("%s%s/files/%s/%s", personalGroupsPrefix, planned.entry.ID, file.UUID, pf.Name))
			planned.files = append(planned.files, plannedBlob{member: pf.Path, key: key, size: size})
			b.stats.FileCount++
			b.stats.TotalFileSize += size
		} else {
			pf.Missing = true
		}
		planned.doc.Files = append(planned.doc.Files, pf)
	}
	return nil
}

// fileLinks returns the links to the entities the archive can name. A link
// to an intake source is left out: intake is a work queue, and the file is
// listed either way.
func (l *groupLookups) fileLinks(links []models.FileLink) []PersonalFileLink {
	out := []PersonalFileLink{}
	for _, link := range links {
		pl := PersonalFileLink{EntityType: link.EntityType, Meta: link.Meta}
		switch link.EntityType {
		case models.FileLinkCommodity:
			ref := l.itemRef(link.EntityID)
			if ref.ItemID == "" {
				continue
			}
			pl.EntityID, pl.EntityName = ref.ItemID, ref.ItemName
		case models.FileLinkArea:
			a := l.areas[link.EntityID]
			if a == nil {
				continue
			}
			pl.EntityID, pl.EntityName = a.UUID, a.Name
		case models.FileLinkLocation:
			loc := l.locations[link.EntityID]
			if loc == nil {
				continue
			}
			pl.EntityID, pl.EntityName = loc.UUID, loc.Name
		case models.FileLinkDraft:
			d := l.drafts[link.EntityID]
			if d == nil {
				continue
			}
			pl.EntityID = d.UUID
		default:
			continue
		}
		out = append(out, pl)
	}
	return out
}

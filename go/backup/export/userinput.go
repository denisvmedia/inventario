package export

import (
	"context"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// exportTypeLabel returns a short human-readable label for the export type,
// matching the labels used on the FE export-row chip. Falls back to the raw
// enum string for unknown values.
func exportTypeLabel(t models.ExportType) string {
	switch t {
	case models.ExportTypeFullDatabase:
		return "Full database"
	case models.ExportTypeSelectedItems:
		return "Selected items"
	case models.ExportTypeLocations:
		return "Locations"
	case models.ExportTypeAreas:
		return "Areas"
	case models.ExportTypeCommodities:
		return "Items"
	case models.ExportTypeImported:
		return "Imported"
	}
	return string(t)
}

// defaultExportDescription builds the synthesised description used when the
// user submits a blank description. The wire format intentionally matches
// the literal in the issue ("Backup · {Type label} · {Created at UTC}") so
// the list row stays meaningful without forcing the user to type one. The
// trailing " UTC" suffix is load-bearing — without it the timestamp reads
// as local time, which is ambiguous for users in non-UTC timezones (and
// inconsistent with the actual persisted value, which is always UTC).
func defaultExportDescription(e *models.Export) string {
	created := "—"
	if e.CreatedDate != nil {
		created = e.CreatedDate.ToTime().UTC().Format("2006-01-02 15:04") + " UTC"
	}
	return "Backup · " + exportTypeLabel(e.Type) + " · " + created
}

// CreateExportFromUserInput creates a new export record from user input.
// The export record is created with status "pending" and is ready for processing.
// It will be processed by the ExportWorker in the background.
func CreateExportFromUserInput(ctx context.Context, registrySet *registry.Set, input *models.Export) (models.Export, error) {
	// Normalise whitespace-only descriptions to "" BEFORE validation so the
	// length(0, 500) cap doesn't reject a 500+ char blob of spaces — the
	// service's intent is "treat blank as missing and synthesise a default",
	// and a 422 for unprintable whitespace would surprise the user. We mutate
	// the caller's pointer in place because the validator and the subsequent
	// NewExportFromUserInput copy both read from the same struct.
	if input != nil && strings.TrimSpace(input.Description) == "" {
		input.Description = ""
	}

	// Validate the export
	if err := input.ValidateWithContext(ctx); err != nil {
		return models.Export{}, errxtrace.Wrap("failed to validate export", err)
	}

	export := models.NewExportFromUserInput(input)

	// Synthesise a default description when the user leaves it blank, so the
	// list row never renders as an empty line. Done after NewExportFromUserInput
	// stamps CreatedDate, so the date in the synthesised string matches the
	// row's persisted timestamp.
	if export.Description == "" {
		export.Description = defaultExportDescription(&export)
	}

	// Extract tenant and user from context
	tenantID, userID, err := ExtractTenantUserFromContext(ctx)
	if err != nil {
		return models.Export{}, errxtrace.Wrap("failed to extract tenant/user context", err)
	}

	if export.TenantID == "" {
		export.TenantID = tenantID
	}
	if export.CreatedByUserID == "" {
		export.CreatedByUserID = userID
	}

	// Enrich selected items with names from the database
	if export.Type == models.ExportTypeSelectedItems && len(export.SelectedItems) > 0 {
		// Ensure we have user context for enriching selected items
		if err := enrichSelectedItemsWithNames(ctx, registrySet, &export); err != nil {
			return models.Export{}, errxtrace.Wrap("failed to enrich selected items with names", err)
		}
	}

	exportReg := registrySet.ExportRegistry

	// Create the export
	created, err := exportReg.Create(ctx, export)
	if err != nil {
		return models.Export{}, errxtrace.Wrap("failed to create export", err)
	}

	return *created, nil
}

func enrichSelectedItemsWithNames(ctx context.Context, registrySet *registry.Set, export *models.Export) error {
	locReg := registrySet.LocationRegistry
	areaReg := registrySet.AreaRegistry
	comReg := registrySet.CommodityRegistry

	for i, item := range export.SelectedItems {
		var name string
		var locationID, areaID string

		switch item.Type {
		case models.ExportSelectedItemTypeLocation:
			location, getErr := locReg.Get(ctx, item.ID)
			if getErr != nil {
				// If item doesn't exist, use a fallback name
				name = "[Deleted Location " + item.ID + "]"
			} else {
				name = location.Name
			}
		case models.ExportSelectedItemTypeArea:
			area, getErr := areaReg.Get(ctx, item.ID)
			if getErr != nil {
				// If item doesn't exist, use a fallback name
				name = "[Deleted Area " + item.ID + "]"
			} else {
				name = area.Name
				locationID = area.LocationID // Store the relationship
			}
		case models.ExportSelectedItemTypeCommodity:
			commodity, getErr := comReg.Get(ctx, item.ID)
			if getErr != nil {
				// If item doesn't exist, use a fallback name
				name = "[Deleted Commodity " + item.ID + "]"
			} else {
				name = commodity.Name
				areaID = commodity.AreaID // Store the relationship
			}
		default:
			name = "[Unknown Item " + item.ID + "]"
		}

		// Update the item with the name and relationships
		export.SelectedItems[i].Name = name
		export.SelectedItems[i].LocationID = locationID
		export.SelectedItems[i].AreaID = areaID
	}

	return nil
}

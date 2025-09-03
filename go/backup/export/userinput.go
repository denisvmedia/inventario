package export

import (
	"context"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// CreateExportFromUserInput creates a new export record from user input.
// The export record is created with status "pending" and is ready for processing.
// It will be processed by the ExportWorker in the background.
func CreateExportFromUserInput(ctx context.Context, registrySet *registry.Set, input *models.Export) (models.Export, error) {
	// Validate the export
	if err := input.ValidateWithContext(ctx); err != nil {
		return models.Export{}, errkit.Wrap(err, "failed to validate export")
	}

	export := models.NewExportFromUserInput(input)

	// Extract tenant and user from context
	tenantID, userID, err := ExtractTenantUserFromContext(ctx)
	if err != nil {
		return models.Export{}, errkit.Wrap(err, "failed to extract tenant/user context")
	}

	if export.TenantID == "" {
		export.TenantID = tenantID
	}
	if export.UserID == "" {
		export.UserID = userID
	}

	// Enrich selected items with names from the database
	if export.Type == models.ExportTypeSelectedItems && len(export.SelectedItems) > 0 {
		// Ensure we have user context for enriching selected items
		if err := enrichSelectedItemsWithNames(ctx, registrySet, &export); err != nil {
			return models.Export{}, errkit.Wrap(err, "failed to enrich selected items with names")
		}
	}

	exportReg := registrySet.ExportRegistry

	// Create the export
	created, err := exportReg.Create(ctx, export)
	if err != nil {
		return models.Export{}, errkit.Wrap(err, "failed to create export")
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

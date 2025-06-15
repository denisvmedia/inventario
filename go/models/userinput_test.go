package models_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestSanitizeUserInput(t *testing.T) {
	c := qt.New(t)

	c.Run("sets zero values for fields with userinput false tag", func(c *qt.C) {
		// Create an export with some values set
		export := &models.Export{
			Type:            models.ExportTypeFullDatabase,
			Status:          models.ExportStatusCompleted, // should be reset to zero
			IncludeFileData: true,
			FilePath:        "/some/path",  // should be reset to zero
			ErrorMessage:    "some error",  // should be reset to zero
			Description:     "test export", // should NOT be reset (no userinput tag)
			Imported:        true,          // should be reset to zero
			FileSize:        12345,         // should be reset to zero
			LocationCount:   10,            // should be reset to zero
			AreaCount:       5,             // should be reset to zero
			CommodityCount:  100,           // should be reset to zero
			ImageCount:      20,            // should be reset to zero
			InvoiceCount:    3,             // should be reset to zero
			ManualCount:     7,             // should be reset to zero
			BinaryDataSize:  54321,         // should be reset to zero
		}
		export.SetID("test-id") // should be reset to zero

		// Apply the function
		models.SanitizeUserInput(export)

		// Check that userinput:"false" fields are reset to zero
		c.Assert(export.GetID(), qt.Equals, "")
		c.Assert(export.Status, qt.Equals, models.ExportStatus(""))
		c.Assert(export.FilePath, qt.Equals, "")
		c.Assert(export.ErrorMessage, qt.Equals, "")
		c.Assert(export.Imported, qt.Equals, false)
		c.Assert(export.FileSize, qt.Equals, int64(0))
		c.Assert(export.LocationCount, qt.Equals, 0)
		c.Assert(export.AreaCount, qt.Equals, 0)
		c.Assert(export.CommodityCount, qt.Equals, 0)
		c.Assert(export.ImageCount, qt.Equals, 0)
		c.Assert(export.InvoiceCount, qt.Equals, 0)
		c.Assert(export.ManualCount, qt.Equals, 0)
		c.Assert(export.BinaryDataSize, qt.Equals, int64(0))

		// Check that fields without userinput:"false" tag are NOT reset
		c.Assert(export.Type, qt.Equals, models.ExportTypeFullDatabase)
		c.Assert(export.IncludeFileData, qt.Equals, true)
		c.Assert(export.Description, qt.Equals, "test export")
	})

	c.Run("handles nil pointer gracefully", func(c *qt.C) {
		var export *models.Export
		// Should panic
		c.Assert(func() { models.SanitizeUserInput(export) }, qt.PanicMatches, "entity must be a non-nil pointer to struct")
	})

	c.Run("handles non-struct pointer gracefully", func(c *qt.C) {
		str := "test"
		// Should panic
		c.Assert(func() { models.SanitizeUserInput(&str) }, qt.PanicMatches, "entity must be a non-nil pointer to struct")
	})

	c.Run("handles embedded structs", func(c *qt.C) {
		// Test with Area which embeds EntityID
		area := &models.Area{
			Name:       "Test Area",
			LocationID: "location-123",
		}
		area.SetID("area-id") // This should be reset to zero

		models.SanitizeUserInput(area)

		// Check that the embedded EntityID.ID field is reset
		c.Assert(area.GetID(), qt.Equals, "")
		// Check that other fields are not affected
		c.Assert(area.Name, qt.Equals, "Test Area")
		c.Assert(area.LocationID, qt.Equals, "location-123")
	})

	c.Run("handles timestamp fields correctly", func(c *qt.C) {
		// Test with Export that has PTimestamp fields with userinput:"false"
		export := &models.Export{
			Type:        models.ExportTypeFullDatabase,
			Description: "Test export",
		}
		export.SetID("export-id")
		export.CreatedDate = models.PNow()
		export.CompletedDate = models.PNow()
		export.DeletedAt = models.PNow()

		models.SanitizeUserInput(export)

		// Check that timestamp fields with userinput:"false" are reset
		c.Assert(export.GetID(), qt.Equals, "")
		c.Assert(export.CreatedDate, qt.IsNil)
		c.Assert(export.CompletedDate, qt.IsNil)
		c.Assert(export.DeletedAt, qt.IsNil)
		// Check that fields without userinput:"false" tag are NOT reset
		c.Assert(export.Type, qt.Equals, models.ExportTypeFullDatabase)
		c.Assert(export.Description, qt.Equals, "Test export")
	})

	c.Run("NewExportFromUserInput uses SanitizeUserInput", func(c *qt.C) {
		// Create an export with user input and system-generated fields
		userExport := &models.Export{
			Type:            models.ExportTypeFullDatabase,
			Status:          models.ExportStatusCompleted, // should be overridden
			IncludeFileData: true,
			Description:     "User provided description",
			FilePath:        "/malicious/path", // should be reset to zero
			ErrorMessage:    "fake error",      // should be reset to zero
			Imported:        true,              // should be overridden
			FileSize:        999,               // should be reset to zero
			LocationCount:   888,               // should be reset to zero
		}
		userExport.SetID("user-provided-id")     // should be reset to zero
		userExport.CreatedDate = models.PNow()   // should be overridden
		userExport.CompletedDate = models.PNow() // should be reset to zero

		// Process the user input
		result := models.NewExportFromUserInput(userExport)

		// Check that userinput:"false" fields are reset to zero
		c.Assert(result.GetID(), qt.Equals, "")
		c.Assert(result.FilePath, qt.Equals, "")
		c.Assert(result.ErrorMessage, qt.Equals, "")
		c.Assert(result.FileSize, qt.Equals, int64(0))
		c.Assert(result.LocationCount, qt.Equals, 0)
		c.Assert(result.CompletedDate, qt.IsNil)

		// Check that system-set values are correct
		c.Assert(result.Status, qt.Equals, models.ExportStatusPending)
		c.Assert(result.Imported, qt.Equals, false)
		c.Assert(result.CreatedDate, qt.IsNotNil)

		// Check that user-provided fields without userinput:"false" tag are preserved
		c.Assert(result.Type, qt.Equals, models.ExportTypeFullDatabase)
		c.Assert(result.IncludeFileData, qt.Equals, true)
		c.Assert(result.Description, qt.Equals, "User provided description")
	})

	c.Run("handles child structs and pointers to child structs", func(c *qt.C) {
		// Define test structs with child structs
		type ChildStruct struct {
			ID          string `userinput:"false"`
			Name        string
			SystemField int `userinput:"false"`
		}

		type ChildStructPtr struct {
			ID          string `userinput:"false"`
			Description string
			Count       int `userinput:"false"`
		}

		type ParentStruct struct {
			models.EntityID                 // embedded struct
			Title           string          // regular field
			Child           ChildStruct     // child struct
			ChildPtr        *ChildStructPtr // pointer to child struct
			NilChildPtr     *ChildStructPtr // nil pointer to child struct
			UserField       string          // user field
			SystemField     string          `userinput:"false"` // system field
		}

		// Create test data
		childPtr := &ChildStructPtr{
			ID:          "child-ptr-id",
			Description: "Child pointer description",
			Count:       42,
		}

		parent := &ParentStruct{
			Title: "Parent Title",
			Child: ChildStruct{
				ID:          "child-id",
				Name:        "Child Name",
				SystemField: 123,
			},
			ChildPtr:    childPtr,
			NilChildPtr: nil,
			UserField:   "User provided",
			SystemField: "System provided",
		}
		parent.SetID("parent-id")

		// Apply sanitization
		models.SanitizeUserInput(parent)

		// Check that embedded struct ID is reset
		c.Assert(parent.GetID(), qt.Equals, "")

		// Check that parent system field is reset
		c.Assert(parent.SystemField, qt.Equals, "")

		// Check that parent user fields are preserved
		c.Assert(parent.Title, qt.Equals, "Parent Title")
		c.Assert(parent.UserField, qt.Equals, "User provided")

		// Check that child struct system fields are reset
		c.Assert(parent.Child.ID, qt.Equals, "")
		c.Assert(parent.Child.SystemField, qt.Equals, 0)

		// Check that child struct user fields are preserved
		c.Assert(parent.Child.Name, qt.Equals, "Child Name")

		// Check that child pointer struct system fields are reset
		c.Assert(parent.ChildPtr.ID, qt.Equals, "")
		c.Assert(parent.ChildPtr.Count, qt.Equals, 0)

		// Check that child pointer struct user fields are preserved
		c.Assert(parent.ChildPtr.Description, qt.Equals, "Child pointer description")

		// Check that nil pointer is still nil
		c.Assert(parent.NilChildPtr, qt.IsNil)
	})

	c.Run("handles deeply nested structures", func(c *qt.C) {
		// Define deeply nested test structures
		type DeepChild struct {
			ID       string `userinput:"false"`
			Name     string
			Internal int `userinput:"false"`
		}

		type MiddleChild struct {
			models.EntityID            // embedded struct with userinput:"false" ID
			Title           string     // user field
			Deep            DeepChild  // child struct
			DeepPtr         *DeepChild // pointer to child struct
			SystemVal       string     `userinput:"false"` // system field
		}

		type TopLevel struct {
			models.EntityID                   // embedded struct
			Name            string            // user field
			Middle          MiddleChild       // child struct
			MiddlePtr       *MiddleChild      // pointer to child struct
			Timestamp       models.PTimestamp `userinput:"false"` // system timestamp
		}

		// Create deeply nested test data
		deepChild := &DeepChild{
			ID:       "deep-id",
			Name:     "Deep Name",
			Internal: 999,
		}

		middleChild := &MiddleChild{
			Title: "Middle Title",
			Deep: DeepChild{
				ID:       "deep-child-id",
				Name:     "Deep Child Name",
				Internal: 888,
			},
			DeepPtr:   deepChild,
			SystemVal: "System Value",
		}
		middleChild.SetID("middle-id")

		topLevel := &TopLevel{
			Name:      "Top Name",
			Middle:    *middleChild,
			MiddlePtr: middleChild,
			Timestamp: models.PNow(),
		}
		topLevel.SetID("top-id")

		// Apply sanitization
		models.SanitizeUserInput(topLevel)

		// Check top level
		c.Assert(topLevel.GetID(), qt.Equals, "")
		c.Assert(topLevel.Name, qt.Equals, "Top Name")
		c.Assert(topLevel.Timestamp, qt.IsNil)

		// Check middle level (struct)
		c.Assert(topLevel.Middle.GetID(), qt.Equals, "")
		c.Assert(topLevel.Middle.Title, qt.Equals, "Middle Title")
		c.Assert(topLevel.Middle.SystemVal, qt.Equals, "")

		// Check deep level in middle struct
		c.Assert(topLevel.Middle.Deep.ID, qt.Equals, "")
		c.Assert(topLevel.Middle.Deep.Name, qt.Equals, "Deep Child Name")
		c.Assert(topLevel.Middle.Deep.Internal, qt.Equals, 0)

		// Check deep level pointer in middle struct
		c.Assert(topLevel.Middle.DeepPtr.ID, qt.Equals, "")
		c.Assert(topLevel.Middle.DeepPtr.Name, qt.Equals, "Deep Name")
		c.Assert(topLevel.Middle.DeepPtr.Internal, qt.Equals, 0)

		// Check middle level (pointer)
		c.Assert(topLevel.MiddlePtr.GetID(), qt.Equals, "")
		c.Assert(topLevel.MiddlePtr.Title, qt.Equals, "Middle Title")
		c.Assert(topLevel.MiddlePtr.SystemVal, qt.Equals, "")

		// Check that the same deep pointer was processed (should be the same instance)
		c.Assert(topLevel.MiddlePtr.DeepPtr, qt.Equals, deepChild)
		c.Assert(topLevel.MiddlePtr.DeepPtr.ID, qt.Equals, "")
		c.Assert(topLevel.MiddlePtr.DeepPtr.Name, qt.Equals, "Deep Name")
		c.Assert(topLevel.MiddlePtr.DeepPtr.Internal, qt.Equals, 0)
	})
}

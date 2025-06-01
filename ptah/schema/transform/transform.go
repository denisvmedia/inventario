package transform

import (
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
)

// ProcessEmbeddedFields processes embedded fields and generates corresponding schema fields based on embedding modes.
//
// This function is the core processor for handling embedded struct fields in Go structs, transforming them
// into appropriate database schema fields according to the specified embedding mode. It supports four
// distinct modes of embedding that provide different approaches to handling complex data structures
// in relational databases.
//
// # Parameters
//
//   - embeddedFields: Collection of embedded field definitions to process
//   - allFields: Complete collection of schema fields from all parsed structs
//   - structName: Name of the target struct to process embedded fields for
//
// # Embedding Modes
//
// The function supports four embedding modes, each serving different architectural patterns:
//
// 1. **"inline"**: Expands embedded struct fields as individual table columns
// 2. **"json"**: Serializes the entire embedded struct into a single JSON/JSONB column
// 3. **"relation"**: Creates a foreign key relationship to another table
// 4. **"skip"**: Completely ignores the embedded field during schema generation
//
// # Use Cases
//
//   - **Code Reuse**: Share common field patterns (timestamps, audit info) across multiple tables
//   - **Data Modeling**: Handle complex nested data structures in relational databases
//   - **Performance Optimization**: Choose between normalized (inline/relation) vs denormalized (json) approaches
//   - **Legacy Integration**: Skip problematic embedded fields during migration
//
// # Examples
//
// ## Inline Mode - Field Expansion
//
// Expands embedded struct fields as individual columns, optionally with prefixes:
//
//	// Embedded type definition
//	type Timestamps struct {
//		CreatedAt time.Time // -> created_at TIMESTAMP
//		UpdatedAt time.Time // -> updated_at TIMESTAMP
//	}
//
//	// Usage without prefix
//	embeddedField := types.EmbeddedField{
//		StructName:       "User",
//		Mode:             "inline",
//		EmbeddedTypeName: "Timestamps",
//	}
//	// Results in: created_at, updated_at columns
//
//	// Usage with prefix
//	embeddedField := types.EmbeddedField{
//		StructName:       "User",
//		Mode:             "inline",
//		Prefix:           "audit_",
//		EmbeddedTypeName: "AuditInfo",
//	}
//	// Results in: audit_created_by, audit_reason columns
//
// ## JSON Mode - Serialized Storage
//
// Stores the entire embedded struct as a single JSON/JSONB column:
//
//	embeddedField := types.EmbeddedField{
//		StructName:       "User",
//		Mode:             "json",
//		Name:             "metadata",      // Column name
//		Type:             "JSONB",         // Column type
//		EmbeddedTypeName: "UserMetadata",
//		Comment:          "User metadata in JSON format",
//	}
//	// Results in: metadata JSONB COMMENT 'User metadata in JSON format'
//
//	// With defaults (auto-generated name and type)
//	embeddedField := types.EmbeddedField{
//		StructName:       "User",
//		Mode:             "json",
//		EmbeddedTypeName: "Meta",
//	}
//	// Results in: meta_data JSONB
//
// ## Relation Mode - Foreign Key Relationships
//
// Creates foreign key fields linking to other tables:
//
//	embeddedField := types.EmbeddedField{
//		StructName:       "Post",
//		Mode:             "relation",
//		Field:            "user_id",       // FK field name
//		Ref:              "users(id)",     // Reference table(column)
//		EmbeddedTypeName: "User",
//		Comment:          "Post author",
//	}
//	// Results in: user_id INTEGER REFERENCES users(id) COMMENT 'Post author'
//
//	// UUID-based foreign key
//	embeddedField := types.EmbeddedField{
//		StructName:       "Order",
//		Mode:             "relation",
//		Field:            "customer_uuid",
//		Ref:              "customers(uuid)",
//		EmbeddedTypeName: "Customer",
//		Nullable:         true,
//	}
//	// Results in: customer_uuid VARCHAR(36) REFERENCES customers(uuid)
//
// ## Skip Mode - Ignored Fields
//
//	embeddedField := types.EmbeddedField{
//		StructName:       "User",
//		Mode:             "skip",
//		EmbeddedTypeName: "InternalData",
//	}
//	// Results in: no database columns generated
//
// # Field Filtering
//
// The function automatically filters embedded fields to process only those belonging to the
// specified structName. This allows processing multiple structs with a single call while
// maintaining proper field isolation.
//
// # Type Inference
//
// For relation mode, the function performs intelligent type inference:
//   - Default: INTEGER (for numeric primary keys)
//   - VARCHAR(36): When reference contains "VARCHAR" or "TEXT" (assumes UUID)
//   - Automatic foreign key naming: "fk_{struct}_{field}" pattern
//
// # Error Handling
//
// The function is designed to be resilient:
//   - Missing required fields in relation mode are silently skipped
//   - Invalid modes default to inline behavior
//   - Empty or malformed configurations are handled gracefully
//
// # Return Value
//
// Returns a slice of types.Field representing the generated database fields.
// Each field is fully configured with appropriate types, constraints, and metadata
// ready for further processing by schema converters.
func ProcessEmbeddedFields(embeddedFields []goschema.EmbeddedField, allFields []goschema.Field, structName string) []goschema.Field {
	var generatedFields []goschema.Field

	// Process each embedded field definition
	for _, embedded := range embeddedFields {
		// Filter: only process embedded fields for the target struct
		if embedded.StructName != structName {
			continue
		}

		switch embedded.Mode {
		case "inline":
			// INLINE MODE: Expand embedded struct fields as individual table columns
			//
			// This mode takes all fields from the embedded type and adds them as separate
			// columns to the target table. Optionally applies a prefix to avoid naming conflicts.
			//
			// Example: Timestamps struct with CreatedAt, UpdatedAt becomes:
			//   - Without prefix: created_at, updated_at
			//   - With "audit_" prefix: audit_created_at, audit_updated_at
			for _, field := range allFields {
				if field.StructName == embedded.EmbeddedTypeName {
					// Clone the field and reassign to target struct
					newField := field
					newField.StructName = structName

					// Apply prefix to column name if specified
					if embedded.Prefix != "" {
						newField.Name = embedded.Prefix + field.Name
					}

					generatedFields = append(generatedFields, newField)
				}
			}

		case "json":
			// JSON MODE: Serialize embedded struct into a single JSON/JSONB column
			//
			// This mode stores the entire embedded struct as a JSON document in a single
			// database column. Useful for semi-structured data that doesn't need to be
			// queried at the field level but should be stored atomically.
			//
			// Column naming: Uses embedded.Name if specified, otherwise generates
			// "{embedded_type_name}_data" (e.g., "Meta" -> "meta_data")
			//
			// Type selection: Uses embedded.Type if specified, otherwise defaults to "JSONB"
			// for PostgreSQL compatibility. Platform overrides can specify alternatives.
			columnName := embedded.Name
			if columnName == "" {
				// Auto-generate column name: "Meta" -> "meta_data"
				columnName = strings.ToLower(embedded.EmbeddedTypeName) + "_data"
			}

			columnType := embedded.Type
			if columnType == "" {
				columnType = "JSONB" // Default to PostgreSQL JSONB for best performance
			}

			// Create the JSON column field
			generatedFields = append(generatedFields, goschema.Field{
				StructName: structName,
				FieldName:  embedded.EmbeddedTypeName,
				Name:       columnName,
				Type:       columnType,
				Nullable:   embedded.Nullable,
				Comment:    embedded.Comment,
				Overrides:  embedded.Overrides, // Platform-specific type overrides (JSON vs JSONB vs TEXT)
			})

		case "relation":
			// RELATION MODE: Create a foreign key field linking to another table
			//
			// This mode establishes a relational link between the current table and another
			// table by creating a foreign key column. The embedded struct represents the
			// related entity, but only the foreign key is stored in the current table.
			//
			// Required fields: embedded.Field (FK column name) and embedded.Ref (target table/column)
			// Type inference: Analyzes the reference to determine appropriate column type
			if embedded.Field == "" || embedded.Ref == "" {
				// Skip incomplete relation definitions - both field name and reference are required
				continue
			}

			// Intelligent type inference based on reference pattern
			refType := "INTEGER" // Default assumption: numeric primary key
			if strings.Contains(embedded.Ref, "VARCHAR") || strings.Contains(embedded.Ref, "TEXT") {
				// Reference suggests string-based key (likely UUID)
				refType = "VARCHAR(36)" // Standard UUID length
			}

			// Generate automatic foreign key constraint name following convention
			foreignKeyName := "fk_" + strings.ToLower(structName) + "_" + strings.ToLower(embedded.Field)

			// Create the foreign key field
			generatedFields = append(generatedFields, goschema.Field{
				StructName:     structName,
				FieldName:      embedded.EmbeddedTypeName,
				Name:           embedded.Field,    // e.g., "user_id"
				Type:           refType,           // INTEGER or VARCHAR(36)
				Nullable:       embedded.Nullable, // Can the relationship be optional?
				Foreign:        embedded.Ref,      // e.g., "users(id)"
				ForeignKeyName: foreignKeyName,    // e.g., "fk_posts_user_id"
				Comment:        embedded.Comment,  // Documentation for the relationship
			})

		case "skip":
			// SKIP MODE: Completely ignore this embedded field
			//
			// This mode is useful for embedded fields that should not be represented
			// in the database schema at all. Common use cases include:
			//   - Runtime-only data structures
			//   - Computed fields that don't need persistence
			//   - Legacy embedded fields during migration
			//   - Development/debugging structures
			continue

		default:
			// DEFAULT MODE: Fall back to inline behavior for unrecognized modes
			//
			// When an embedded field has an empty or unrecognized mode, the function
			// defaults to inline behavior (expanding fields as individual columns).
			// This provides backward compatibility and graceful degradation.
			//
			// Note: No prefix is applied in default mode, unlike explicit "inline" mode.
			for _, field := range allFields {
				if field.StructName == embedded.EmbeddedTypeName {
					// Clone field and reassign to target struct (no prefix applied)
					newField := field
					newField.StructName = structName
					generatedFields = append(generatedFields, newField)
				}
			}
		}
	}

	return generatedFields
}

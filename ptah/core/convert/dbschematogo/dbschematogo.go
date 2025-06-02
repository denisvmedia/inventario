package dbschematogo

import (
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	dbschematypes "github.com/denisvmedia/inventario/ptah/dbschema/types"
)

// ConvertDBSchemaToGoSchema converts a database schema to goschema format
// This is needed for down migrations where we use the current DB state as the target
func ConvertDBSchemaToGoSchema(dbSchema *dbschematypes.DBSchema) *goschema.Database {
	database := &goschema.Database{
		Tables:       make([]goschema.Table, 0),
		Fields:       make([]goschema.Field, 0),
		Indexes:      make([]goschema.Index, 0),
		Enums:        make([]goschema.Enum, 0),
		Dependencies: make(map[string][]string),
	}

	// Convert enums
	for _, dbEnum := range dbSchema.Enums {
		database.Enums = append(database.Enums, goschema.Enum{
			Name:   dbEnum.Name,
			Values: dbEnum.Values,
		})
	}

	// Convert tables and their columns
	for _, dbTable := range dbSchema.Tables {
		// Generate struct name from table name (simple conversion)
		structName := generateStructName(dbTable.Name)

		table := goschema.Table{
			StructName: structName,
			Name:       dbTable.Name,
			Comment:    dbTable.Comment,
		}
		database.Tables = append(database.Tables, table)

		// Convert columns to fields
		for _, dbColumn := range dbTable.Columns {
			field := goschema.Field{
				StructName: structName,
				FieldName:  generateFieldName(dbColumn.Name),
				Name:       dbColumn.Name,
				Type:       dbColumn.DataType,
				Nullable:   dbColumn.IsNullable == "YES",
				Primary:    dbColumn.IsPrimaryKey,
				AutoInc:    dbColumn.IsAutoIncrement,
				Unique:     dbColumn.IsUnique,
			}

			// Set default value if present
			if dbColumn.ColumnDefault != nil {
				field.Default = *dbColumn.ColumnDefault
			}

			database.Fields = append(database.Fields, field)
		}
	}

	// Convert indexes
	for _, dbIndex := range dbSchema.Indexes {
		// Skip primary key indexes as they're handled by primary key fields
		if dbIndex.IsPrimary {
			continue
		}

		index := goschema.Index{
			StructName: generateStructName(dbIndex.TableName),
			Name:       dbIndex.Name,
			Fields:     dbIndex.Columns,
			Unique:     dbIndex.IsUnique,
		}
		database.Indexes = append(database.Indexes, index)
	}

	return database
}

// generateStructName converts a table name to a Go struct name
func generateStructName(tableName string) string {
	// Simple conversion: remove underscores and capitalize
	parts := strings.Split(tableName, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// generateFieldName converts a column name to a Go field name
func generateFieldName(columnName string) string {
	// Simple conversion: remove underscores and capitalize
	parts := strings.Split(columnName, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

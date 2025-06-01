package goschema

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/goschema/internal/parseutils"
)

func ParseFile(filename string) ([]EmbeddedField, []Field, []Index, []Table, []Enum) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		slog.Error("Failed to parse file", "error", err)
		panic("Failed to parse file")
	}

	var embeddedFields []EmbeddedField
	var schemaFields []Field
	var schemaIndexes []Index
	var tableDirectives []Table
	globalEnumsMap := make(map[string]Enum)

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structName := typeSpec.Name.Name
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			if genDecl.Doc != nil {
				for _, comment := range genDecl.Doc.List {
					if strings.HasPrefix(comment.Text, "//migrator:schema:table") {
						kv := parseutils.ParseKeyValueComment(comment.Text)
						tableDirectives = append(tableDirectives, Table{
							StructName: structName,
							Name:       kv["name"],
							Engine:     kv["engine"],
							Comment:    kv["comment"],
							PrimaryKey: strings.Split(kv["primary_key"], ","),
							Checks:     strings.Split(kv["checks"], ","),
							CustomSQL:  kv["custom"],
							Overrides:  parseutils.ParsePlatformSpecific(kv),
						})
					}
				}
			}
			for _, field := range structType.Fields.List {
				if field.Doc == nil {
					continue
				}
				for _, comment := range field.Doc.List {
					if strings.HasPrefix(comment.Text, "//migrator:schema:field") {
						kv := parseutils.ParseKeyValueComment(comment.Text)
						for _, name := range field.Names {
							enumRaw := kv["enum"]
							var enum []string
							if enumRaw != "" {
								enum = strings.Split(enumRaw, ",")
								for i := range enum {
									enum[i] = strings.TrimSpace(enum[i])
								}
							}

							// Determine the field type - if it's ENUM with enum values, use the generated enum name
							fieldType := kv["type"]
							if len(enumRaw) > 0 && kv["type"] == "ENUM" {
								enumName := "enum_" + strings.ToLower(structName) + "_" + strings.ToLower(name.Name)
								globalEnumsMap[enumName] = Enum{
									Name:   enumName,
									Values: enum,
								}
								// Update the field type to use the generated enum name
								fieldType = enumName
							}

							schemaFields = append(schemaFields, Field{
								StructName:     structName,
								FieldName:      name.Name,
								Name:           kv["name"],
								Type:           fieldType,
								Nullable:       kv["not_null"] != "true",
								Primary:        kv["primary"] == "true",
								AutoInc:        kv["auto_increment"] == "true",
								Unique:         kv["unique"] == "true",
								UniqueExpr:     kv["unique_expr"],
								Default:        kv["default"],
								DefaultExpr:    kv["default_expr"],
								Foreign:        kv["foreign"],
								ForeignKeyName: kv["foreign_key_name"],
								Enum:           enum,
								Check:          kv["check"],
								Comment:        kv["comment"],
								Overrides:      parseutils.ParsePlatformSpecific(kv),
							})
						}
					} else if strings.HasPrefix(comment.Text, "//migrator:embedded") {
						kv := parseutils.ParseKeyValueComment(comment.Text)
						// Handle embedded fields - get the field type name
						var fieldTypeName string
						if field.Type != nil {
							if ident, ok := field.Type.(*ast.Ident); ok {
								fieldTypeName = ident.Name
							}
						}

						embeddedFields = append(embeddedFields, EmbeddedField{
							StructName:       structName,
							Mode:             kv["mode"],
							Prefix:           kv["prefix"],
							Name:             kv["name"],
							Type:             kv["type"],
							Nullable:         kv["nullable"] == "true",
							Index:            kv["index"] == "true",
							Field:            kv["field"],
							Ref:              kv["ref"],
							OnDelete:         kv["on_delete"],
							OnUpdate:         kv["on_update"],
							Comment:          kv["comment"],
							EmbeddedTypeName: fieldTypeName,
							Overrides:        parseutils.ParsePlatformSpecific(kv),
						})
					} else if strings.HasPrefix(comment.Text, "//migrator:schema:index") {
						kv := parseutils.ParseKeyValueComment(comment.Text)
						fields := strings.Split(kv["fields"], ",")
						for i := range fields {
							fields[i] = strings.TrimSpace(fields[i])
						}
						schemaIndexes = append(schemaIndexes, Index{
							StructName: structName,
							Name:       kv["name"],
							Fields:     fields,
							Unique:     kv["unique"] == "true",
							Comment:    kv["comment"],
						})
					}
				}
			}
		}
	}

	enums := make([]Enum, 0, len(globalEnumsMap))
	keys := make([]string, 0, len(globalEnumsMap))
	for k := range globalEnumsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		enums = append(enums, globalEnumsMap[k])
	}

	return embeddedFields, schemaFields, schemaIndexes, tableDirectives, enums
}

// ParseFileWithDependencies parses a Go file and automatically discovers and parses
// related files in the same directory to resolve embedded type references
func ParseFileWithDependencies(filename string) ([]EmbeddedField, []Field, []Index, []Table, []Enum) {
	// Parse the main file
	embeddedFields, fields, indexes, tables, enums := ParseFile(filename)

	// Get the directory of the main file
	dir := filepath.Dir(filename)

	// Parse all other .go files in the same directory to find embedded type definitions
	pattern := filepath.Join(dir, "*.go")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		slog.Warn("Failed to find related files", "error", err)
		return embeddedFields, fields, indexes, tables, enums
	}

	// Collect embedded type names that we need to resolve
	embeddedTypeNames := make(map[string]bool)
	for _, embedded := range embeddedFields {
		embeddedTypeNames[embedded.EmbeddedTypeName] = true
	}

	// Parse each related file to collect embedded type definitions
	for _, match := range matches {
		if match == filename {
			continue // Skip the main file as it's already parsed
		}

		// Parse the related file
		_, relatedFields, _, _, _ := ParseFile(match)

		// Only add fields from embedded types that we actually need
		for _, field := range relatedFields {
			if embeddedTypeNames[field.StructName] {
				fields = append(fields, field)
			}
		}
	}

	return embeddedFields, fields, indexes, tables, enums
}

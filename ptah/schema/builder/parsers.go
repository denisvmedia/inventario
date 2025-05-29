package builder

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/denisvmedia/inventario/ptah/schema/meta"
)

func parseKeyValueComment(comment string) map[string]string {
	result := make(map[string]string)

	// First, handle key=value pairs (quoted and unquoted)
	r := regexp.MustCompile(`(\w+(?:\.\w+)*)=(?:"([^"]*)"|([^\s]+))`)
	matches := r.FindAllStringSubmatch(comment, -1)
	for _, match := range matches {
		key := match[1]
		// match[2] is the quoted value (if quoted), match[3] is the unquoted value
		if match[2] != "" {
			result[key] = match[2] // Use quoted value
		} else {
			result[key] = match[3] // Use unquoted value
		}
	}

	// Then, handle standalone boolean attributes (no =value)
	// Remove all key=value pairs from the comment first
	cleanComment := r.ReplaceAllString(comment, "")

	// Find standalone words that could be boolean flags
	boolRegex := regexp.MustCompile(`\b(\w+(?:\.\w+)*)\b`)
	boolMatches := boolRegex.FindAllStringSubmatch(cleanComment, -1)

	// Known boolean attributes that can be standalone
	booleanAttrs := map[string]bool{
		"not_null": true, "nullable": true, "primary": true, "unique": true,
		"auto_increment": true, "index": true, "autoincrement": true,
	}

	for _, match := range boolMatches {
		attr := match[1]
		// Skip directive names and other non-boolean words
		if attr == "migrator" || attr == "schema" || attr == "field" ||
			attr == "table" || attr == "embed" || attr == "embedded" {
			continue
		}
		// Only treat as boolean if it's a known boolean attribute or follows boolean naming pattern
		if booleanAttrs[attr] || strings.HasSuffix(attr, "_null") ||
			strings.HasPrefix(attr, "is_") || strings.HasPrefix(attr, "has_") {
			// Only set if not already set by key=value parsing
			if _, exists := result[attr]; !exists {
				result[attr] = "true"
			}
		}
	}

	return result
}

func parsePlatformSpecific(kv map[string]string) map[string]map[string]string {
	out := make(map[string]map[string]string)
	for k, v := range kv {
		// Only use platform. prefix, dropping override. completely
		if strings.HasPrefix(k, "platform.") {
			parts := strings.SplitN(k, ".", 3)

			if len(parts) == 3 {
				db := parts[1]
				key := parts[2]
				if _, ok := out[db]; !ok {
					out[db] = make(map[string]string)
				}
				out[db][key] = v
			}
		}

		// Move engine and comment to platform-specific attributes
		if k == "engine" {
			for _, dialect := range []string{"mysql", "mariadb"} {
				if _, ok := out[dialect]; !ok {
					out[dialect] = make(map[string]string)
				}
				out[dialect]["engine"] = v
			}
		}

		if k == "comment" {
			for _, dialect := range []string{"mysql", "mariadb"} {
				if _, ok := out[dialect]; !ok {
					out[dialect] = make(map[string]string)
				}
				out[dialect]["comment"] = v
			}
		}
	}
	return out
}

func ParseFile(filename string) ([]meta.EmbeddedField, []meta.SchemaField, []meta.SchemaIndex, []meta.TableDirective, []meta.GlobalEnum) {
	fset := token.NewFileSet()
	fmt.Println(os.Getwd())
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		slog.Error("Failed to parse file", "error", err)
		panic("Failed to parse file")
	}

	embeddedFields := []meta.EmbeddedField{}
	schemaFields := []meta.SchemaField{}
	schemaIndexes := []meta.SchemaIndex{}
	tableDirectives := []meta.TableDirective{}
	globalEnumsMap := map[string]meta.GlobalEnum{}

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
						kv := parseKeyValueComment(comment.Text)
						tableDirectives = append(tableDirectives, meta.TableDirective{
							StructName: structName,
							Name:       kv["name"],
							Engine:     kv["engine"],
							Comment:    kv["comment"],
							PrimaryKey: strings.Split(kv["primary_key"], ","),
							Checks:     strings.Split(kv["checks"], ","),
							CustomSQL:  kv["custom"],
							Overrides:  parsePlatformSpecific(kv),
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
						kv := parseKeyValueComment(comment.Text)
						for _, name := range field.Names {
							enumRaw := kv["enum"]
							enum := strings.Split(enumRaw, ",")
							for i := range enum {
								enum[i] = strings.TrimSpace(enum[i])
							}

							// Determine the field type - if it's ENUM with enum values, use the generated enum name
							fieldType := kv["type"]
							if len(enumRaw) > 0 && kv["type"] == "ENUM" {
								enumName := "enum_" + strings.ToLower(structName) + "_" + strings.ToLower(name.Name)
								globalEnumsMap[enumName] = meta.GlobalEnum{
									Name:   enumName,
									Values: enum,
								}
								// Update the field type to use the generated enum name
								fieldType = enumName
							}

							schemaFields = append(schemaFields, meta.SchemaField{
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
								DefaultFn:      kv["default_fn"],
								Foreign:        kv["foreign"],
								ForeignKeyName: kv["foreign_key_name"],
								Enum:           enum,
								Check:          kv["check"],
								Comment:        kv["comment"],
								Overrides:      parsePlatformSpecific(kv),
							})
						}
					} else if strings.HasPrefix(comment.Text, "//migrator:embedded") {
						kv := parseKeyValueComment(comment.Text)
						// Handle embedded fields - get the field type name
						var fieldTypeName string
						if field.Type != nil {
							if ident, ok := field.Type.(*ast.Ident); ok {
								fieldTypeName = ident.Name
							}
						}

						embeddedFields = append(embeddedFields, meta.EmbeddedField{
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
							Overrides:        parsePlatformSpecific(kv),
						})
					} else if strings.HasPrefix(comment.Text, "//migrator:schema:index") {
						kv := parseKeyValueComment(comment.Text)
						fields := strings.Split(kv["fields"], ",")
						for i := range fields {
							fields[i] = strings.TrimSpace(fields[i])
						}
						schemaIndexes = append(schemaIndexes, meta.SchemaIndex{
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

	enums := make([]meta.GlobalEnum, 0, len(globalEnumsMap))
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
func ParseFileWithDependencies(filename string) ([]meta.EmbeddedField, []meta.SchemaField, []meta.SchemaIndex, []meta.TableDirective, []meta.GlobalEnum) {
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

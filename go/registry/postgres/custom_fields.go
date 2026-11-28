package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// customFieldNameKeyIndex is the unique index on (group_id, name_key). A
// unique violation on it is a duplicate field name, reported as
// registry.ErrAlreadyExists.
const customFieldNameKeyIndex = "idx_custom_field_definitions_group_name_key"

// customFieldLockKey is the first half of the transaction-scoped advisory
// lock Create takes so the count of a group's fields and the insert happen
// as one step; the second half is the group ID.
const customFieldLockKey = "custom_fields"

// CustomFieldRegistryFactory creates CustomFieldRegistry instances with
// proper context (#2803).
type CustomFieldRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// CustomFieldRegistry is the postgres-backed group-scoped registry of custom
// field definitions and their item type templates.
type CustomFieldRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.CustomFieldRegistry        = (*CustomFieldRegistry)(nil)
	_ registry.CustomFieldRegistryFactory = (*CustomFieldRegistryFactory)(nil)
)

func NewCustomFieldRegistry(dbx *sqlx.DB) *CustomFieldRegistryFactory {
	return NewCustomFieldRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewCustomFieldRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *CustomFieldRegistryFactory {
	return &CustomFieldRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *CustomFieldRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CustomFieldRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CustomFieldRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CustomFieldRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &CustomFieldRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
	}, nil
}

func (f *CustomFieldRegistryFactory) CreateServiceRegistry() registry.CustomFieldRegistry {
	return &CustomFieldRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *CustomFieldRegistry) newSQLRegistry() *store.RLSGroupRepository[models.CustomFieldDefinition, *models.CustomFieldDefinition] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.CustomFieldDefinition](r.dbx, r.tableNames.CustomFieldDefinitions())
	}
	return store.NewGroupAwareSQLRegistry[models.CustomFieldDefinition](
		r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.CustomFieldDefinitions())
}

func (r *CustomFieldRegistry) Get(ctx context.Context, id string) (*models.CustomFieldDefinition, error) {
	return r.getBy(ctx, "id", id)
}

func (r *CustomFieldRegistry) GetByName(ctx context.Context, name string) (*models.CustomFieldDefinition, error) {
	return r.getBy(ctx, "name_key", models.CustomFieldNameKey(name))
}

// readOneSnapshot makes every later statement of tx read the same snapshot.
// A field and its templates are two reads, and under the default READ
// COMMITTED an update committed between them would pair the old options
// with the new defaults. It must run before the first query of tx.
func readOneSnapshot(ctx context.Context, tx *sqlx.Tx) error {
	if _, err := tx.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ"); err != nil {
		return errxtrace.Wrap("failed to set the read snapshot", err)
	}
	return nil
}

// getBy reads one field by a column the caller names in code.
func (r *CustomFieldRegistry) getBy(ctx context.Context, column, value string) (*models.CustomFieldDefinition, error) {
	var d models.CustomFieldDefinition
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := readOneSnapshot(ctx, tx); err != nil {
			return err
		}
		query := fmt.Sprintf(`SELECT * FROM %s WHERE %s = $1 LIMIT 1`, r.tableNames.CustomFieldDefinitions(), column)
		if err := scanOne(ctx, tx, &d, query, value); err != nil {
			return err
		}
		return r.loadTemplatesInTx(ctx, tx, &d)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to get custom field", err, errx.Attrs(column, value))
	}
	return &d, nil
}

// List returns the group's fields ordered by name key, then ID. The service
// registry lists every group's fields.
func (r *CustomFieldRegistry) List(ctx context.Context) ([]*models.CustomFieldDefinition, error) {
	var defs []*models.CustomFieldDefinition
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := readOneSnapshot(ctx, tx); err != nil {
			return err
		}
		var err error
		query := fmt.Sprintf(`SELECT * FROM %s ORDER BY name_key, id`, r.tableNames.CustomFieldDefinitions())
		if defs, err = scanAll[models.CustomFieldDefinition](ctx, tx, query); err != nil {
			return errxtrace.Wrap("failed to list custom fields", err)
		}
		return r.loadTemplatesInTx(ctx, tx, defs...)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list custom fields", err)
	}
	return defs, nil
}

func (r *CustomFieldRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count custom fields", err)
	}
	return cnt, nil
}

// Create inserts a field into the registry's group together with its
// template entries.
func (r *CustomFieldRegistry) Create(ctx context.Context, def models.CustomFieldDefinition) (*models.CustomFieldDefinition, error) {
	def.Normalize()
	if err := def.ValidateFields(ctx); err != nil {
		return nil, errxtrace.Wrap("invalid custom field", err)
	}
	// Postgres keeps microseconds, so the returned field matches what a
	// later read gives.
	now := time.Now().Truncate(time.Microsecond)
	def.CreatedAt = now
	def.UpdatedAt = now
	templates := def.Templates
	created, err := r.newSQLRegistry().CreateWithHooks(ctx, def, func(ctx context.Context, tx *sqlx.Tx, row *models.CustomFieldDefinition) error {
		if err := lockGroupCustomFields(ctx, tx, row.GroupID); err != nil {
			return err
		}
		var count int
		query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE group_id = $1`, r.tableNames.CustomFieldDefinitions())
		if err := tx.GetContext(ctx, &count, query, row.GroupID); err != nil {
			return errxtrace.Wrap("failed to count custom fields", err)
		}
		if count >= models.MaxCustomFieldsPerGroup {
			return errxtrace.Classify(registry.ErrResourceLimitExceeded, errx.Attrs("limit", models.MaxCustomFieldsPerGroup))
		}
		return lockTemplateItemTypesInTx(ctx, tx, r.tableNames, row.GroupID, templates)
	}, func(ctx context.Context, tx *sqlx.Tx, row *models.CustomFieldDefinition) error {
		if err := r.insertTemplatesInTx(ctx, tx, row, templates); err != nil {
			return err
		}
		return r.loadTemplatesInTx(ctx, tx, row)
	})
	if err != nil {
		if isUniqueViolationOn(err, customFieldNameKeyIndex) {
			return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("name", def.Name))
		}
		return nil, errxtrace.Wrap("failed to create custom field", err)
	}
	return &created, nil
}

// Update writes the field's name and options, and its template entries when
// def.Templates is not nil. The type, the set entry and the creation time
// stay as stored.
// The values and template defaults that name a dropped option are deleted
// in the same transaction.
func (r *CustomFieldRegistry) Update(ctx context.Context, def models.CustomFieldDefinition) (*models.CustomFieldDefinition, error) {
	var updated models.CustomFieldDefinition
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var stored models.CustomFieldDefinition
		query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 FOR UPDATE`, r.tableNames.CustomFieldDefinitions())
		if err := scanOne(ctx, tx, &stored, query, def.ID); err != nil {
			return errxtrace.Wrap("failed to lock custom field", err, errx.Attrs("custom_field_id", def.ID))
		}
		def.TenantGroupAwareEntityID = stored.TenantGroupAwareEntityID
		def.Type = stored.Type
		def.SetEntry = stored.SetEntry
		def.CreatedAt = stored.CreatedAt
		def.UpdatedAt = time.Now().Truncate(time.Microsecond)
		def.Normalize()
		if err := def.ValidateFields(ctx); err != nil {
			return errxtrace.Wrap("invalid custom field", err)
		}
		if def.Templates != nil {
			if err := lockTemplateItemTypesInTx(ctx, tx, r.tableNames, def.GroupID, def.Templates); err != nil {
				return err
			}
		}
		update := fmt.Sprintf(`UPDATE %s SET name = $2, name_key = $3, options = $4, updated_at = $5 WHERE id = $1`,
			r.tableNames.CustomFieldDefinitions())
		if _, err := tx.ExecContext(ctx, update, def.ID, def.Name, def.NameKey, def.Options, def.UpdatedAt); err != nil {
			return errxtrace.Wrap("failed to update custom field", err)
		}
		if err := r.dropRemovedOptionsInTx(ctx, tx, &stored, &def); err != nil {
			return err
		}
		if def.Templates != nil {
			del := fmt.Sprintf(`DELETE FROM %s WHERE field_id = $1`, r.tableNames.ItemTypeCustomFields())
			if _, err := tx.ExecContext(ctx, del, def.ID); err != nil {
				return errxtrace.Wrap("failed to replace custom field templates", err)
			}
			if err := r.insertTemplatesInTx(ctx, tx, &def, def.Templates); err != nil {
				return err
			}
		}
		updated = def
		return r.loadTemplatesInTx(ctx, tx, &updated)
	})
	if err != nil {
		if isUniqueViolationOn(err, customFieldNameKeyIndex) {
			return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("name", def.Name))
		}
		return nil, errxtrace.Wrap("failed to update custom field", err)
	}
	return &updated, nil
}

// AddTemplates adds the missing template entries of the field. It locks the
// field's row and then the item types, in the order Update takes them, so a
// concurrent Update or AddTemplates of the field waits and then sees these
// entries.
func (r *CustomFieldRegistry) AddTemplates(ctx context.Context, id string, itemTypeIDs []string) (int, error) {
	added := 0
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var stored models.CustomFieldDefinition
		query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 FOR UPDATE`, r.tableNames.CustomFieldDefinitions())
		if err := scanOne(ctx, tx, &stored, query, id); err != nil {
			return errxtrace.Wrap("failed to lock custom field", err, errx.Attrs("custom_field_id", id))
		}
		if err := r.loadTemplatesInTx(ctx, tx, &stored); err != nil {
			return err
		}
		var missing []models.CustomFieldTemplate
		for _, typeID := range itemTypeIDs {
			onTemplate := slices.ContainsFunc(stored.Templates, func(t models.CustomFieldTemplate) bool { return t.ItemTypeID == typeID })
			listed := slices.ContainsFunc(missing, func(t models.CustomFieldTemplate) bool { return t.ItemTypeID == typeID })
			if !onTemplate && !listed {
				missing = append(missing, models.CustomFieldTemplate{ItemTypeID: typeID})
			}
		}
		if err := lockTemplateItemTypesInTx(ctx, tx, r.tableNames, stored.GroupID, missing); err != nil {
			return err
		}
		if len(missing) == 0 {
			return nil
		}
		if err := r.insertTemplatesInTx(ctx, tx, &stored, missing); err != nil {
			return err
		}
		touch := fmt.Sprintf(`UPDATE %s SET updated_at = $2 WHERE id = $1`, r.tableNames.CustomFieldDefinitions())
		if _, err := tx.ExecContext(ctx, touch, id, time.Now().Truncate(time.Microsecond)); err != nil {
			return errxtrace.Wrap("failed to update custom field", err)
		}
		added = len(missing)
		return nil
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to add custom field templates", err)
	}
	return added, nil
}

// dropRemovedOptionsInTx deletes the values and clears the template
// defaults that name an option stored had and def no longer has.
func (r *CustomFieldRegistry) dropRemovedOptionsInTx(ctx context.Context, tx *sqlx.Tx, stored, def *models.CustomFieldDefinition) error {
	keep := def.OptionIDs()
	var removed []string
	for _, id := range stored.OptionIDs() {
		if !slices.Contains(keep, id) {
			removed = append(removed, id)
		}
	}
	if len(removed) == 0 {
		return nil
	}
	query := fmt.Sprintf(`DELETE FROM %s WHERE field_id = $1 AND value_select = ANY($2)`, r.tableNames.CustomFieldValues())
	if _, err := tx.ExecContext(ctx, query, def.ID, removed); err != nil {
		return errxtrace.Wrap("failed to delete values of removed options", err)
	}
	query = fmt.Sprintf(`UPDATE %s SET default_value = NULL WHERE field_id = $1 AND default_value #>> '{}' = ANY($2)`,
		r.tableNames.ItemTypeCustomFields())
	if _, err := tx.ExecContext(ctx, query, def.ID, removed); err != nil {
		return errxtrace.Wrap("failed to clear defaults of removed options", err)
	}
	return nil
}

// Delete deletes a field that no item and no open draft has a value of.
func (r *CustomFieldRegistry) Delete(ctx context.Context, id string) error {
	_, err := r.deleteField(ctx, id, 0)
	return err
}

// DeleteWithValues deletes the field, its values and its template entries;
// the foreign keys cascade. The drafts lose their value of the field.
func (r *CustomFieldRegistry) DeleteWithValues(ctx context.Context, id string, expected int) (int, error) {
	return r.deleteField(ctx, id, expected)
}

// deleteField deletes the field and returns how many items and open drafts
// had a value of it. When another number than expected has one, it fails
// with ErrCustomFieldInUse and deletes nothing. The rows of the open drafts
// that have an entry of the field are locked first, as a promotion locks its
// draft before the fields of the item; then the field's row is locked FOR
// UPDATE before the values are counted, so an item write that sets a value
// of the field either commits first and is counted, or waits and then finds
// the field gone.
func (r *CustomFieldRegistry) deleteField(ctx context.Context, id string, expected int) (int, error) {
	used := 0
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var stored models.CustomFieldDefinition
		query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1`, r.tableNames.CustomFieldDefinitions())
		if err := scanOne(ctx, tx, &stored, query, id); err != nil {
			return errxtrace.Wrap("failed to get custom field", err, errx.Attrs("custom_field_id", id))
		}
		drafts := r.tableNames.CommodityDrafts()
		if err := lockDraftsWithFieldInTx(ctx, tx, drafts, stored.GroupID, id); err != nil {
			return err
		}
		query = fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 FOR UPDATE`, r.tableNames.CustomFieldDefinitions())
		if err := scanOne(ctx, tx, &stored, query, id); err != nil {
			return errxtrace.Wrap("failed to lock custom field", err, errx.Attrs("custom_field_id", id))
		}
		items := 0
		count := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE field_id = $1`, r.tableNames.CustomFieldValues())
		if err := tx.GetContext(ctx, &items, count, id); err != nil {
			return errxtrace.Wrap("failed to count custom field values", err)
		}
		draftValues, err := countDraftValuesInTx(ctx, tx, drafts, stored.GroupID, id)
		if err != nil {
			return err
		}
		if err := registry.CheckCustomFieldValueCount(id, items, draftValues, expected); err != nil {
			return err
		}
		if err := dropDraftFieldInTx(ctx, tx, drafts, stored.GroupID, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, r.tableNames.CustomFieldDefinitions()), id); err != nil {
			return errxtrace.Wrap("failed to delete custom field", err)
		}
		used = items + draftValues
		return nil
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to delete custom field", err)
	}
	return used, nil
}

// Usage counts, per field, the items with a value, apart for live and
// trashed items, and per option of a select field, and the open drafts with
// a value.
func (r *CustomFieldRegistry) Usage(ctx context.Context) (map[string]registry.CustomFieldUsage, error) {
	usage := map[string]registry.CustomFieldUsage{}
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT v.field_id, COUNT(*) FILTER (WHERE c.%s), COUNT(*) FILTER (WHERE c.%s)
			 FROM %s v JOIN %s c ON c.id = v.commodity_id GROUP BY v.field_id`,
			liveRow, trashedRow, r.tableNames.CustomFieldValues(), r.tableNames.Commodities())
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return errxtrace.Wrap("failed to count custom field values", err)
		}
		defer rows.Close()
		for rows.Next() {
			var (
				id string
				u  registry.CustomFieldUsage
			)
			if err := rows.Scan(&id, &u.Live, &u.Trashed); err != nil {
				return errxtrace.Wrap("failed to scan custom field usage", err)
			}
			usage[id] = u
		}
		if err := rows.Err(); err != nil {
			return err
		}
		drafts, err := countDraftValuesPerFieldInTx(ctx, tx, r.tableNames.CommodityDrafts())
		if err != nil {
			return err
		}
		for id, n := range drafts {
			u := usage[id]
			u.Drafts = n
			usage[id] = u
		}
		return r.countOptionsInTx(ctx, tx, usage)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count custom field usage", err)
	}
	return usage, nil
}

func (r *CustomFieldRegistry) countOptionsInTx(ctx context.Context, tx *sqlx.Tx, usage map[string]registry.CustomFieldUsage) error {
	query := fmt.Sprintf(
		`SELECT field_id, value_select, COUNT(*) FROM %s WHERE value_select IS NOT NULL GROUP BY field_id, value_select`,
		r.tableNames.CustomFieldValues())
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return errxtrace.Wrap("failed to count custom field options", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id, option string
			n          int
		)
		if err := rows.Scan(&id, &option, &n); err != nil {
			return errxtrace.Wrap("failed to scan custom field option count", err)
		}
		u := usage[id]
		if u.Options == nil {
			u.Options = map[string]int{}
		}
		u.Options[option] = n
		usage[id] = u
	}
	return rows.Err()
}

func lockGroupCustomFields(ctx context.Context, tx *sqlx.Tx, groupID string) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`, customFieldLockKey, groupID)
	if err != nil {
		return errxtrace.Wrap("failed to acquire custom field lock", err, errx.Attrs("group_id", groupID))
	}
	return nil
}

// lockTemplateItemTypesInTx checks that every template entry names an item
// type of the group and locks those types FOR SHARE, before any template
// row is touched. A type delete locks the type FOR UPDATE and then
// cascades to its template rows, so taking the types first keeps the two
// from waiting on each other in a cycle.
func lockTemplateItemTypesInTx(ctx context.Context, tx *sqlx.Tx, tables store.TableNames, groupID string, templates []models.CustomFieldTemplate) error {
	if len(templates) == 0 {
		return nil
	}
	ids := make([]string, 0, len(templates))
	for _, t := range templates {
		ids = append(ids, t.ItemTypeID)
	}
	slices.Sort(ids)
	var found []string
	query := fmt.Sprintf(`SELECT id FROM %s WHERE group_id = $1 AND id = ANY($2) ORDER BY id FOR SHARE`, tables.ItemTypes())
	if err := tx.SelectContext(ctx, &found, query, groupID, ids); err != nil {
		return errxtrace.Wrap("failed to lock template item types", err)
	}
	for _, id := range ids {
		if !slices.Contains(found, id) {
			return errxtrace.Classify(registry.ErrInvalidInput, errx.Attrs("field_name", "templates", "item_type_id", id))
		}
	}
	return nil
}

func (r *CustomFieldRegistry) insertTemplatesInTx(
	ctx context.Context, tx *sqlx.Tx, def *models.CustomFieldDefinition, templates []models.CustomFieldTemplate,
) error {
	// The entries carry the field's author, whoever edits the template:
	// they are part of the field.
	now := time.Now().Truncate(time.Microsecond)
	txReg := store.NewTxRegistry[models.ItemTypeCustomField](tx, r.tableNames.ItemTypeCustomFields())
	for _, t := range templates {
		row := models.ItemTypeCustomField{
			ItemTypeID:   t.ItemTypeID,
			FieldID:      def.ID,
			DefaultValue: t.DefaultValue,
			CreatedAt:    now,
		}
		row.ID = uuid.New().String()
		row.UUID = uuid.New().String()
		row.TenantID = def.TenantID
		row.GroupID = def.GroupID
		row.CreatedByUserID = def.CreatedByUserID
		if err := txReg.Insert(ctx, row); err != nil {
			return errxtrace.Wrap("failed to insert custom field template", err, errx.Attrs("item_type_id", t.ItemTypeID))
		}
	}
	return nil
}

// loadTemplatesInTx sets Templates on each field to its stored entries,
// ordered by item type ID, in one query. A field without entries gets an
// empty slice.
func (r *CustomFieldRegistry) loadTemplatesInTx(ctx context.Context, tx *sqlx.Tx, defs ...*models.CustomFieldDefinition) error {
	if len(defs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(defs))
	for _, d := range defs {
		ids = append(ids, d.ID)
		d.Templates = []models.CustomFieldTemplate{}
	}
	query := fmt.Sprintf(`SELECT * FROM %s WHERE field_id = ANY($1) ORDER BY field_id, item_type_id`, r.tableNames.ItemTypeCustomFields())
	rows, err := scanAll[models.ItemTypeCustomField](ctx, tx, query, ids)
	if err != nil {
		return errxtrace.Wrap("failed to read custom field templates", err)
	}
	byField := make(map[string]*models.CustomFieldDefinition, len(defs))
	for _, d := range defs {
		byField[d.ID] = d
	}
	for _, row := range rows {
		if d := byField[row.FieldID]; d != nil {
			d.Templates = append(d.Templates, models.CustomFieldTemplate{ItemTypeID: row.ItemTypeID, DefaultValue: row.DefaultValue})
		}
	}
	return nil
}

// isUniqueViolationOn reports whether err is a unique violation (SQLSTATE
// 23505) on the named index.
func isUniqueViolationOn(err error, index string) bool {
	type sqlStater interface{ SQLState() string }
	type constrainter interface{ ConstraintName() string }
	var s sqlStater
	if !errors.As(err, &s) || s.SQLState() != "23505" {
		return false
	}
	var c constrainter
	if errors.As(err, &c) {
		return c.ConstraintName() == index
	}
	return strings.Contains(err.Error(), index)
}

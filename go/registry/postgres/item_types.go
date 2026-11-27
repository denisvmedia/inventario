package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// itemTypeNameKeyIndex is the unique index on (group_id, name_key). A unique
// violation on it is a duplicate item type name, reported as
// registry.ErrAlreadyExists.
const itemTypeNameKeyIndex = "idx_item_types_group_name_key"

// itemTypeLockKey is the first half of the transaction-scoped advisory lock
// that serializes the writes of one group's item types; the second half is
// the group id. Seeding, Create and DeleteReassigning take it first, so the
// "group has no types" check and the seed, the slug choice and the insert,
// and the "last type" check and the delete each happen as one step.
const itemTypeLockKey = "item_types"

// ItemTypeRegistryFactory creates ItemTypeRegistry instances with proper
// context (#2802).
type ItemTypeRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// ItemTypeRegistry is the postgres-backed group-scoped registry of item
// types.
type ItemTypeRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.ItemTypeRegistry        = (*ItemTypeRegistry)(nil)
	_ registry.ItemTypeRegistryFactory = (*ItemTypeRegistryFactory)(nil)
)

func NewItemTypeRegistry(dbx *sqlx.DB) *ItemTypeRegistryFactory {
	return NewItemTypeRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewItemTypeRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *ItemTypeRegistryFactory {
	return &ItemTypeRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *ItemTypeRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.ItemTypeRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *ItemTypeRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.ItemTypeRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &ItemTypeRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
	}, nil
}

func (f *ItemTypeRegistryFactory) CreateServiceRegistry() registry.ItemTypeRegistry {
	return &ItemTypeRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *ItemTypeRegistry) newSQLRegistry() *store.RLSGroupRepository[models.ItemType, *models.ItemType] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.ItemType](r.dbx, r.tableNames.ItemTypes())
	}
	return store.NewGroupAwareSQLRegistry[models.ItemType](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.ItemTypes())
}

func (r *ItemTypeRegistry) Get(ctx context.Context, id string) (*models.ItemType, error) {
	var t models.ItemType
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &t); err != nil {
		return nil, errxtrace.Wrap("failed to get item type", err)
	}
	return &t, nil
}

func (r *ItemTypeRegistry) GetBySlug(ctx context.Context, slug models.CommodityType) (*models.ItemType, error) {
	var t models.ItemType
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("slug", string(slug)), &t); err != nil {
		return nil, errxtrace.Wrap("failed to get item type by slug", err)
	}
	return &t, nil
}

func (r *ItemTypeRegistry) GetByName(ctx context.Context, name string) (*models.ItemType, error) {
	var t models.ItemType
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("name_key", models.ItemTypeNameKey(name)), &t); err != nil {
		return nil, errxtrace.Wrap("failed to get item type by name", err)
	}
	return &t, nil
}

// List returns the group's types sorted by registry.SortItemTypes, seeding
// the built-in types first when the group has none. The service registry
// lists every group's types and seeds nothing.
func (r *ItemTypeRegistry) List(ctx context.Context) ([]*models.ItemType, error) {
	var types []*models.ItemType
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if !r.service {
			if _, err := ensureBuiltinItemTypesInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID); err != nil {
				return err
			}
		}
		rows, err := tx.QueryxContext(ctx, fmt.Sprintf(`SELECT * FROM %s`, r.tableNames.ItemTypes()))
		if err != nil {
			return errxtrace.Wrap("failed to list item types", err)
		}
		defer rows.Close()
		for rows.Next() {
			var t models.ItemType
			if err := rows.StructScan(&t); err != nil {
				return errxtrace.Wrap("failed to scan item type", err)
			}
			types = append(types, &t)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list item types", err)
	}
	registry.SortItemTypes(types)
	return types, nil
}

func (r *ItemTypeRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count item types", err)
	}
	return cnt, nil
}

// Create inserts a type into the registry's group. It seeds the built-in
// types first when the group has none, so a group never holds only the
// types its members added.
func (r *ItemTypeRegistry) Create(ctx context.Context, itemType models.ItemType) (*models.ItemType, error) {
	itemType.Normalize()
	now := time.Now()
	itemType.CreatedAt = now
	itemType.UpdatedAt = now
	created, err := r.newSQLRegistry().CreateWithPrepare(ctx, itemType, func(ctx context.Context, tx *sqlx.Tx, row *models.ItemType) error {
		if err := lockGroupItemTypes(ctx, tx, row.GroupID); err != nil {
			return err
		}
		if _, err := ensureBuiltinItemTypesInTx(ctx, tx, r.tableNames, row.TenantID, row.GroupID); err != nil {
			return err
		}
		var count int
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE group_id = $1`, r.tableNames.ItemTypes())
		if err := tx.GetContext(ctx, &count, countQuery, row.GroupID); err != nil {
			return errxtrace.Wrap("failed to count item types", err)
		}
		if count >= models.MaxItemTypesPerGroup {
			return errxtrace.Classify(registry.ErrItemTypeLimit, errx.Attrs("group_id", row.GroupID, "limit", models.MaxItemTypesPerGroup))
		}
		return r.assignSlugInTx(ctx, tx, row)
	})
	if err != nil {
		if isItemTypeNameUniqueViolation(err) {
			return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("name", itemType.Name))
		}
		return nil, errxtrace.Wrap("failed to create item type", err)
	}
	return &created, nil
}

// assignSlugInTx sets row.Slug under the group lock Create took. A slug the caller set is kept when it is free and fails with
// ErrAlreadyExists otherwise; without one, the slug is made from the name
// and suffixed with -2, -3, ... until it is free and is not a built-in slug.
func (r *ItemTypeRegistry) assignSlugInTx(ctx context.Context, tx *sqlx.Tx, row *models.ItemType) error {
	taken := func(slug models.CommodityType) (bool, error) {
		var exists bool
		query := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE group_id = $1 AND slug = $2)`, r.tableNames.ItemTypes())
		if err := tx.GetContext(ctx, &exists, query, row.GroupID, string(slug)); err != nil {
			return false, errxtrace.Wrap("failed to check item type slug", err)
		}
		return exists, nil
	}
	if row.Slug != "" {
		isTaken, err := taken(row.Slug)
		if err != nil {
			return err
		}
		if isTaken {
			return errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("slug", string(row.Slug)))
		}
		return nil
	}
	return registry.ChooseItemTypeSlug(row, taken)
}

// Update writes the type's name and icon. The slug, the set entry and the
// creation time stay as stored.
func (r *ItemTypeRegistry) Update(ctx context.Context, itemType models.ItemType) (*models.ItemType, error) {
	existing, err := r.Get(ctx, itemType.ID)
	if err != nil {
		return nil, err
	}
	itemType.Slug = existing.Slug
	itemType.SetEntry = existing.SetEntry
	itemType.Normalize()
	itemType.CreatedAt = existing.CreatedAt
	itemType.UpdatedAt = time.Now()
	if err := r.newSQLRegistry().Update(ctx, itemType, nil); err != nil {
		if isItemTypeNameUniqueViolation(err) {
			return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("name", itemType.Name))
		}
		return nil, errxtrace.Wrap("failed to update item type", err)
	}
	return r.Get(ctx, itemType.ID)
}

func (r *ItemTypeRegistry) Delete(ctx context.Context, id string) error {
	_, err := r.DeleteReassigning(ctx, id, "")
	return err
}

// DeleteReassigning deletes a type, moving its items to reassignTo first.
//
// Lock order: the group lock, then the deleted type's row (FOR UPDATE) and
// the target's (FOR SHARE), then the group's currency-migration lock, then
// the items' rows. An item write locks the
// item's row and then, when it changes the type, the new type's row FOR
// SHARE; it never waits for the type it is leaving, so the two orders cannot
// deadlock. A write that names the deleted type waits for this transaction
// and then finds the type gone.
func (r *ItemTypeRegistry) DeleteReassigning(ctx context.Context, id, reassignTo string) (int, error) {
	types := r.tableNames.ItemTypes()
	moved := 0
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		src, err := lockItemTypeForDelete(ctx, tx, types, id)
		if err != nil {
			return err
		}
		target, err := lockReassignTarget(ctx, tx, types, src, reassignTo)
		if err != nil {
			return err
		}
		if moved, err = moveItemsOfType(ctx, tx, r.tableNames.Commodities(), src, target); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, types), id); err != nil {
			return errxtrace.Wrap("failed to delete item type", err)
		}
		return nil
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to delete item type", err)
	}
	return moved, nil
}

// lockItemTypeForDelete takes the group lock and the type's row FOR UPDATE,
// and refuses to delete the group's last type.
func lockItemTypeForDelete(ctx context.Context, tx *sqlx.Tx, types store.TableName, id string) (*models.ItemType, error) {
	var src models.ItemType
	if err := scanOne(ctx, tx, &src, fmt.Sprintf(`SELECT * FROM %s WHERE id = $1`, types), id); err != nil {
		return nil, errxtrace.Wrap("failed to get item type", err, errx.Attrs("item_type_id", id))
	}
	if err := lockGroupItemTypes(ctx, tx, src.GroupID); err != nil {
		return nil, err
	}
	if err := scanOne(ctx, tx, &src, fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 FOR UPDATE`, types), id); err != nil {
		return nil, errxtrace.Wrap("failed to lock item type", err, errx.Attrs("item_type_id", id))
	}
	var count int
	if err := tx.GetContext(ctx, &count, fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE group_id = $1`, types), src.GroupID); err != nil {
		return nil, errxtrace.Wrap("failed to count item types", err)
	}
	if count <= 1 {
		return nil, errxtrace.Classify(registry.ErrLastItemType, errx.Attrs("item_type_id", id))
	}
	return &src, nil
}

// lockReassignTarget locks the type the items move to FOR SHARE. It returns
// nil for an empty reassignTo, and ErrInvalidInput for one that is not
// another type of src's group.
func lockReassignTarget(
	ctx context.Context, tx *sqlx.Tx, types store.TableName, src *models.ItemType, reassignTo string,
) (*models.ItemType, error) {
	if reassignTo == "" {
		return nil, nil
	}
	var t models.ItemType
	query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 AND group_id = $2 FOR SHARE`, types)
	err := scanOne(ctx, tx, &t, query, reassignTo, src.GroupID)
	if errors.Is(err, registry.ErrNotFound) || t.ID == src.ID {
		return nil, errxtrace.Classify(registry.ErrInvalidInput, errx.Attrs("reassign_to", reassignTo))
	}
	if err != nil {
		return nil, errxtrace.Wrap("failed to lock target item type", err, errx.Attrs("reassign_to", reassignTo))
	}
	return &t, nil
}

// moveItemsOfType gives the items of src's group that have src, those in
// Trash included, the type target, and returns how many it moved. With
// items and no target it fails with ErrItemTypeInUse.
func moveItemsOfType(
	ctx context.Context, tx *sqlx.Tx, commodities store.TableName, src, target *models.ItemType,
) (int, error) {
	var used int
	usedQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE group_id = $1 AND type = $2`, commodities)
	if err := tx.GetContext(ctx, &used, usedQuery, src.GroupID, string(src.Slug)); err != nil {
		return 0, errxtrace.Wrap("failed to count items of the type", err)
	}
	if used == 0 {
		return 0, nil
	}
	if target == nil {
		return 0, errxtrace.Classify(registry.ErrItemTypeInUse, errx.Attrs("item_type_id", src.ID, "items", used))
	}
	// A running currency migration updates the group's items one by one
	// under its lock; moving them all at once in another order waits for it.
	if err := lockGroupCurrencyMigration(ctx, tx, src.GroupID); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET type = $1, revision = revision + 1 WHERE group_id = $2 AND type = $3`, commodities),
		string(target.Slug), src.GroupID, string(src.Slug))
	if err != nil {
		return 0, errxtrace.Wrap("failed to move items to another type", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, errxtrace.Wrap("failed to count moved items", err)
	}
	return int(n), nil
}

func (r *ItemTypeRegistry) ItemCounts(ctx context.Context) (map[models.CommodityType]registry.ItemTypeUsage, error) {
	counts := map[models.CommodityType]registry.ItemTypeUsage{}
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT type, COUNT(*) FILTER (WHERE %s), COUNT(*) FILTER (WHERE %s) FROM %s GROUP BY type`,
			liveRow, trashedRow, r.tableNames.Commodities())
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return errxtrace.Wrap("failed to count items per type", err)
		}
		defer rows.Close()
		for rows.Next() {
			var (
				slug  string
				usage registry.ItemTypeUsage
			)
			if err := rows.Scan(&slug, &usage.Live, &usage.Trashed); err != nil {
				return errxtrace.Wrap("failed to scan item count", err)
			}
			counts[models.CommodityType(slug)] = usage
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count items per type", err)
	}
	return counts, nil
}

func (r *ItemTypeRegistry) EnsureBuiltins(ctx context.Context) (int, error) {
	if r.service {
		return 0, errxtrace.Classify(store.ErrGroupIDRequired)
	}
	created := 0
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var err error
		created, err = ensureBuiltinItemTypesInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID)
		return err
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to seed item types", err)
	}
	return created, nil
}

// BackfillBuiltins seeds every group that has no item types, each in its own
// background-worker transaction. A group seeded meanwhile by a request is
// found to have types under the group lock and left alone.
func (r *ItemTypeRegistry) BackfillBuiltins(ctx context.Context) (int, error) {
	type groupRow struct {
		ID       string `db:"id"`
		TenantID string `db:"tenant_id"`
	}
	var groups []groupRow
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT g.id, g.tenant_id FROM %s g
			WHERE NOT EXISTS (SELECT 1 FROM %s t WHERE t.group_id = g.id)
			ORDER BY g.id`, r.tableNames.LocationGroups(), r.tableNames.ItemTypes())
		return tx.SelectContext(ctx, &groups, query)
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to list groups without item types", err)
	}

	total := 0
	for _, g := range groups {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		created := 0
		err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
			var err error
			created, err = ensureBuiltinItemTypesInTx(ctx, tx, r.tableNames, g.TenantID, g.ID)
			return err
		})
		if err != nil {
			return total, errxtrace.Wrap("failed to seed item types", err, errx.Attrs("group_id", g.ID))
		}
		total += created
	}
	return total, nil
}

func lockGroupItemTypes(ctx context.Context, tx *sqlx.Tx, groupID string) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`, itemTypeLockKey, groupID)
	if err != nil {
		return errxtrace.Wrap("failed to acquire item type lock", err, errx.Attrs("group_id", groupID))
	}
	return nil
}

// ensureBuiltinItemTypesInTx inserts the built-in item types when the group
// has none and returns how many it inserted. A group that has types never
// loses the last one, so the first check needs no lock; only a group that
// looks unseeded takes the group's item type lock, held until the
// transaction ends, and checks again under it. The seeded rows are
// attributed to the group's creator, who owns the group row already, so
// seeding never makes another member the author of content they did not add.
func ensureBuiltinItemTypesInTx(ctx context.Context, tx *sqlx.Tx, tables store.TableNames, tenantID, groupID string) (int, error) {
	if groupID == "" {
		return 0, errxtrace.Classify(store.ErrGroupIDRequired)
	}
	seeded := func() (bool, error) {
		var exists bool
		query := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE group_id = $1)`, tables.ItemTypes())
		if err := tx.GetContext(ctx, &exists, query, groupID); err != nil {
			return false, errxtrace.Wrap("failed to check item types", err, errx.Attrs("group_id", groupID))
		}
		return exists, nil
	}
	if ok, err := seeded(); err != nil || ok {
		return 0, err
	}
	if err := lockGroupItemTypes(ctx, tx, groupID); err != nil {
		return 0, err
	}
	if ok, err := seeded(); err != nil || ok {
		return 0, err
	}

	var creator string
	query := fmt.Sprintf(`SELECT created_by FROM %s WHERE id = $1`, tables.LocationGroups())
	if err := tx.GetContext(ctx, &creator, query, groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "LocationGroup", "group_id", groupID))
		}
		return 0, errxtrace.Wrap("failed to read group creator", err, errx.Attrs("group_id", groupID))
	}

	now := time.Now()
	txReg := store.NewTxRegistry[models.ItemType](tx, tables.ItemTypes())
	for _, b := range models.BuiltinItemTypes {
		row := models.NewBuiltinItemType(b)
		row.ID = uuid.New().String()
		row.UUID = uuid.New().String()
		row.TenantID = tenantID
		row.GroupID = groupID
		row.CreatedByUserID = creator
		row.CreatedAt = now
		row.UpdatedAt = now
		if err := txReg.Insert(ctx, row); err != nil {
			return 0, errxtrace.Wrap("failed to seed item type", err, errx.Attrs("group_id", groupID, "slug", string(b.Slug)))
		}
	}
	return len(models.BuiltinItemTypes), nil
}

// checkCommodityTypeInTx verifies, inside an item write's transaction, that
// slug names an item type of the group, and locks that type's row FOR SHARE
// so a concurrent delete waits for the write. A built-in slug in a group
// that has no types yet seeds them first. An empty slug names no type and is
// left to the model's validation, which every API write runs.
func checkCommodityTypeInTx(ctx context.Context, tx *sqlx.Tx, tables store.TableNames, tenantID, groupID string, slug models.CommodityType) error {
	if groupID == "" || slug == "" {
		return nil
	}
	query := fmt.Sprintf(`SELECT id FROM %s WHERE group_id = $1 AND slug = $2 FOR SHARE`, tables.ItemTypes())
	found := func() (bool, error) {
		var id string
		err := tx.GetContext(ctx, &id, query, groupID, string(slug))
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, errxtrace.Wrap("failed to look up item type", err, errx.Attrs("type", string(slug)))
		}
		return true, nil
	}
	ok, err := found()
	if err != nil || ok {
		return err
	}
	if slug.IsBuiltin() {
		// Seeded here or by a transaction that committed while this one
		// waited for the group lock: either way the row may exist now.
		if _, err := ensureBuiltinItemTypesInTx(ctx, tx, tables, tenantID, groupID); err != nil {
			return err
		}
		if ok, err = found(); err != nil || ok {
			return err
		}
	}
	return errxtrace.Classify(registry.ErrUnknownItemType, errx.Attrs("type", string(slug)))
}

// isItemTypeNameUniqueViolation reports whether err is a unique violation
// (SQLSTATE 23505) on itemTypeNameKeyIndex. A unique violation on any other
// index is not a duplicate name and is returned as it is.
func isItemTypeNameUniqueViolation(err error) bool {
	type sqlStater interface{ SQLState() string }
	type constrainter interface{ ConstraintName() string }
	var s sqlStater
	if !errors.As(err, &s) || s.SQLState() != "23505" {
		return false
	}
	var c constrainter
	if errors.As(err, &c) {
		return c.ConstraintName() == itemTypeNameKeyIndex
	}
	return strings.Contains(err.Error(), itemTypeNameKeyIndex)
}

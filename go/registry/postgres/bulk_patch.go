package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// errPatchUnchanged ends the transaction of a patch whose change changed
// nothing, so a tag row it created for the change is rolled back with it.
var errPatchUnchanged = errors.New("the patch changed nothing")

// Patch writes the bulk-editable columns of a live item under its row lock.
// The tags it may add are locked first and the item after them, the order
// every writer of an item's tags follows (see ensureTagRowsInTx).
func (r *CommodityRegistry) Patch(ctx context.Context, id string, addTags []string, change func(models.Commodity) (models.Commodity, bool, error)) (before, after *models.Commodity, err error) {
	table := r.tableNames.Commodities()
	var locked, stored models.Commodity
	var changeErr error
	reg := r.newSQLRegistry()
	err = reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := ensureTagRowsInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID, r.createdByUserID, models.TagKindCommodity, addTags); err != nil {
			return err
		}
		if err := lockLiveRow(ctx, tx, table, "commodity", id); err != nil {
			return err
		}
		if err := scanOne(ctx, tx, &locked, fmt.Sprintf(`SELECT * FROM %s WHERE id = $1`, table), id); err != nil {
			return err
		}
		if err := loadCustomFieldValuesInTx(ctx, tx, r.tableNames, &locked); err != nil {
			return err
		}
		input := locked
		input.Tags = slices.Clone(locked.Tags)
		input.CustomFields = locked.CustomFields.Clone()
		next, changed, err := change(input)
		if err != nil {
			changeErr = err
			return err
		}
		if !changed {
			return errPatchUnchanged
		}
		patched, err := registry.PatchedCommodity(locked, next, addTags)
		if err != nil {
			return err
		}
		if err := r.checkPatchTargets(ctx, tx, locked, patched); err != nil {
			return err
		}
		query := fmt.Sprintf(`
			UPDATE %s SET tags = $2, type = $3, status = $4, status_date = $5, status_note = $6, sale_price = $7, area_id = $8
			WHERE id = $1 RETURNING *`, table)
		if err := scanOne(ctx, tx, &stored, query, id,
			patched.Tags, patched.Type, patched.Status, patched.StatusDate, patched.StatusNote, patched.SalePrice, patched.AreaID); err != nil {
			return err
		}
		stored.CustomFields = locked.CustomFields.Clone()
		return nil
	})
	switch {
	case changeErr != nil:
		return nil, nil, changeErr
	case errors.Is(err, errPatchUnchanged):
		return &locked, nil, nil
	case err != nil:
		return nil, nil, errxtrace.Wrap("failed to patch commodity", err, errx.Attrs("commodity_id", id))
	}
	return &locked, &stored, nil
}

// checkPatchTargets checks the type and the area a patch gives an item, in
// the patch's transaction. As in Update, the new type's row is locked after
// the item's.
func (r *CommodityRegistry) checkPatchTargets(ctx context.Context, tx *sqlx.Tx, locked, patched models.Commodity) error {
	if patched.Type != locked.Type {
		if err := checkCommodityTypeInTx(ctx, tx, r.tableNames, locked.TenantID, locked.GroupID, patched.Type); err != nil {
			return err
		}
	}
	if patched.AreaID != nil && (locked.AreaID == nil || *locked.AreaID != *patched.AreaID) {
		if _, err := getAreaInTx(ctx, tx, r.tableNames, *patched.AreaID); err != nil {
			return err
		}
	}
	return nil
}

// PatchTags writes the tags of a live file under its row lock, after the
// tags it may add, in the order Patch takes them.
func (r *FileRegistry) PatchTags(ctx context.Context, id string, addTags []string, change func(models.FileEntity) (models.FileEntity, bool, error)) (before, after *models.FileEntity, err error) {
	table := r.tableNames.Files()
	read := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1`, table)
	var locked, stored *models.FileEntity
	var changeErr error
	reg := r.newSQLRegistry()
	err = reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := ensureTagRowsInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID, r.createdByUserID, models.TagKindFile, addTags); err != nil {
			return err
		}
		if err := lockLiveRow(ctx, tx, table, "file", id); err != nil {
			return err
		}
		var err error
		if locked, err = r.queryOneFile(ctx, tx, read, id); err != nil {
			return err
		}
		input := *locked
		input.Tags = slices.Clone(locked.Tags)
		input.Links = slices.Clone(locked.Links)
		next, changed, err := change(input)
		if err != nil {
			changeErr = err
			return err
		}
		if !changed {
			return errPatchUnchanged
		}
		patched, err := registry.PatchedFileTags(*locked, next, addTags)
		if err != nil {
			return err
		}
		write := fmt.Sprintf(`UPDATE %s SET tags = $2, updated_at = $3 WHERE id = $1`, table)
		if _, err := tx.ExecContext(ctx, write, id, patched.Tags, time.Now()); err != nil {
			return errxtrace.Wrap("failed to write file tags", err)
		}
		stored, err = r.queryOneFile(ctx, tx, read, id)
		return err
	})
	switch {
	case changeErr != nil:
		return nil, nil, changeErr
	case errors.Is(err, errPatchUnchanged):
		return locked, nil, nil
	case err != nil:
		return nil, nil, errxtrace.Wrap("failed to patch file tags", err, errx.Attrs("file_id", id))
	}
	return locked, stored, nil
}

// queryOneFile reads the one file query selects, with its links.
func (r *FileRegistry) queryOneFile(ctx context.Context, tx *sqlx.Tx, query string, id string) (*models.FileEntity, error) {
	files, err := r.queryFiles(ctx, tx, query, id)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", id))
	}
	return files[0], nil
}

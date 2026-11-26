package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	sqlb "github.com/huandu/go-sqlbuilder"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// This file reads and writes the custom field values of items (#2803),
// rows of commodity_custom_field_values that the commodity registry keeps
// as models.Commodity.CustomFields. Every function runs on the transaction
// of the item read or write it belongs to.

// customFieldValueColumns are the value columns in the order the queries
// below list them.
const customFieldValueColumns = "value_text, value_long_text, value_number, value_date, value_bool, value_select, value_money, value_currency, value_item_id"

// customFieldValueRow is one stored value with the type of its field.
type customFieldValueRow struct {
	CommodityID string                 `db:"commodity_id"`
	FieldID     string                 `db:"field_id"`
	Type        models.CustomFieldType `db:"type"`
	models.CustomFieldData
}

// loadCustomFieldValuesInTx sets CustomFields on each item to the values
// stored for it, in one query. An item without values gets nil.
func loadCustomFieldValuesInTx(ctx context.Context, tx *sqlx.Tx, tables store.TableNames, items ...*models.Commodity) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, c := range items {
		if c != nil {
			ids = append(ids, c.ID)
			c.CustomFields = nil
		}
	}
	rows, err := readCustomFieldValuesInTx(ctx, tx, tables, ids)
	if err != nil {
		return err
	}
	byItem := make(map[string]models.CustomFieldValues, len(items))
	for _, row := range rows {
		raw := row.JSON(row.Type)
		if raw == nil {
			continue
		}
		if byItem[row.CommodityID] == nil {
			byItem[row.CommodityID] = models.CustomFieldValues{}
		}
		byItem[row.CommodityID][row.FieldID] = raw
	}
	for _, c := range items {
		if c != nil {
			c.CustomFields = byItem[c.ID]
		}
	}
	return nil
}

func readCustomFieldValuesInTx(ctx context.Context, tx *sqlx.Tx, tables store.TableNames, commodityIDs []string) ([]*customFieldValueRow, error) {
	query := fmt.Sprintf(`SELECT v.commodity_id, v.field_id, d.type, %s
		FROM %s v JOIN %s d ON d.id = v.field_id
		WHERE v.commodity_id = ANY($1)`,
		prefixColumns("v.", customFieldValueColumns), tables.CustomFieldValues(), tables.CustomFieldDefinitions())
	rows, err := scanAll[customFieldValueRow](ctx, tx, query, commodityIDs)
	if err != nil {
		return nil, errxtrace.Wrap("failed to read custom field values", err)
	}
	return rows, nil
}

func prefixColumns(prefix, columns string) string {
	parts := strings.Split(columns, ", ")
	for i, p := range parts {
		parts[i] = prefix + p
	}
	return strings.Join(parts, ", ")
}

// writeCustomFieldValuesInTx applies the custom field values of an item
// write to row, which is already stored, and then sets row.CustomFields to
// every value the item has. See models.CustomFieldValues for what a nil
// map, a missing key and a null mean.
//
// Each value is checked against its field, which is locked FOR SHARE so a
// concurrent delete of the field, or of one of its options, waits for this
// write and then removes what it wrote. A link must name another item of
// the group, and a live one unless the item already had that link. All
// refused values come back together as a *registry.CustomFieldValueError.
func writeCustomFieldValuesInTx(ctx context.Context, tx *sqlx.Tx, tables store.TableNames, row *models.Commodity, input models.CustomFieldValues) error {
	if len(input) > 0 {
		if err := applyCustomFieldValuesInTx(ctx, tx, tables, row, input); err != nil {
			return err
		}
	}
	return loadCustomFieldValuesInTx(ctx, tx, tables, row)
}

func applyCustomFieldValuesInTx(ctx context.Context, tx *sqlx.Tx, tables store.TableNames, row *models.Commodity, input models.CustomFieldValues) error {
	defs, err := lockCustomFieldsInTx(ctx, tx, tables, row.GroupID, slices.Sorted(maps.Keys(input)))
	if err != nil {
		return err
	}
	stored := models.CustomFieldValues{}
	rows, err := readCustomFieldValuesInTx(ctx, tx, tables, []string{row.ID})
	if err != nil {
		return err
	}
	for _, r := range rows {
		stored[r.FieldID] = r.JSON(r.Type)
	}
	plan, err := registry.PlanCustomFieldWrite(row.ID, defs, stored, input, func(id string) (registry.LinkedItemState, error) {
		return linkedItemStateInTx(ctx, tx, tables, row.GroupID, id)
	})
	if err != nil {
		return err
	}

	if len(plan.Deletes) > 0 {
		query := fmt.Sprintf(`DELETE FROM %s WHERE commodity_id = $1 AND field_id = ANY($2)`, tables.CustomFieldValues())
		if _, err := tx.ExecContext(ctx, query, row.ID, plan.Deletes); err != nil {
			return errxtrace.Wrap("failed to delete custom field values", err)
		}
	}
	now := time.Now().UTC()
	for _, id := range slices.Sorted(maps.Keys(plan.Sets)) {
		if err := upsertCustomFieldValueInTx(ctx, tx, tables, row, id, plan.Sets[id], now); err != nil {
			return err
		}
	}
	return nil
}

// lockCustomFieldsInTx returns the fields of the group among ids, keyed by
// ID, locked FOR SHARE.
func lockCustomFieldsInTx(ctx context.Context, tx *sqlx.Tx, tables store.TableNames, groupID string, ids []string) (map[string]*models.CustomFieldDefinition, error) {
	query := fmt.Sprintf(`SELECT * FROM %s WHERE group_id = $1 AND id = ANY($2) ORDER BY id FOR SHARE`, tables.CustomFieldDefinitions())
	rows, err := scanAll[models.CustomFieldDefinition](ctx, tx, query, groupID, ids)
	if err != nil {
		return nil, errxtrace.Wrap("failed to lock custom fields", err)
	}
	out := make(map[string]*models.CustomFieldDefinition, len(rows))
	for _, d := range rows {
		out[d.ID] = d
	}
	return out, nil
}

// linkedItemStateInTx looks up the item a new link names in the group and
// locks it FOR KEY SHARE, the lock the foreign key takes anyway, so a purge
// of the item waits for this write and then deletes the value with it.
func linkedItemStateInTx(ctx context.Context, tx *sqlx.Tx, tables store.TableNames, groupID, id string) (registry.LinkedItemState, error) {
	var trashed bool
	query := fmt.Sprintf(`SELECT deleted_at IS NOT NULL FROM %s WHERE id = $1 AND group_id = $2 FOR KEY SHARE`, tables.Commodities())
	err := tx.GetContext(ctx, &trashed, query, id, groupID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return registry.LinkedItemState{}, nil
	case err != nil:
		return registry.LinkedItemState{}, errxtrace.Wrap("failed to look up linked item", err, errx.Attrs("item_id", id))
	}
	return registry.LinkedItemState{Exists: true, Trashed: trashed}, nil
}

func upsertCustomFieldValueInTx(ctx context.Context, tx *sqlx.Tx, tables store.TableNames, row *models.Commodity, fieldID string, data models.CustomFieldData, now time.Time) error {
	cols := strings.Split(customFieldValueColumns, ", ")
	sets := make([]string, 0, len(cols)+1)
	for _, col := range cols {
		sets = append(sets, col+" = EXCLUDED."+col)
	}
	sets = append(sets, "updated_at = EXCLUDED.updated_at")
	query := fmt.Sprintf(`INSERT INTO %s (id, tenant_id, group_id, commodity_id, field_id, %s, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (commodity_id, field_id) DO UPDATE SET %s`,
		tables.CustomFieldValues(), customFieldValueColumns, strings.Join(sets, ", "))
	var date any
	if data.Date != nil {
		date = string(*data.Date)
	}
	var currency any
	if data.Currency != nil {
		currency = string(*data.Currency)
	}
	_, err := tx.ExecContext(ctx, query,
		row.TenantID, row.GroupID, row.ID, fieldID,
		data.Text, data.LongText, data.Number, date, data.Bool, data.Select, data.Money, currency, data.ItemID,
		now)
	if err != nil {
		return errxtrace.Wrap("failed to write custom field value", err, errx.Attrs("field_id", fieldID))
	}
	return nil
}

// customFieldFilterExpr states registry.CustomFieldFilter.Matches in SQL:
// an EXISTS, or for unset a NOT EXISTS, over the item's value for the
// field. The (field_id, value_x) index of the compared column serves it.
func customFieldFilterExpr(cond *sqlb.Cond, f registry.CustomFieldFilter, commoditiesTable, valuesTable string) string {
	if f.Type == "" {
		return "FALSE"
	}
	preds := []string{"v.commodity_id = " + commoditiesTable + ".id", "v.field_id = " + cond.Var(f.FieldID)}
	exists := func() string {
		return fmt.Sprintf("EXISTS (SELECT 1 FROM %s v WHERE %s)", valuesTable, strings.Join(preds, " AND "))
	}
	switch f.Op {
	case registry.CustomFieldOpSet:
		return exists()
	case registry.CustomFieldOpUnset:
		return "NOT " + exists()
	}
	pred := customFieldValuePredicate(cond, f)
	if pred == "" {
		return "FALSE"
	}
	preds = append(preds, pred)
	return exists()
}

func customFieldValuePredicate(cond *sqlb.Cond, f registry.CustomFieldFilter) string {
	compare := func(column, operand string) string {
		switch f.Op {
		case registry.CustomFieldOpEq:
			return column + " = " + operand
		case registry.CustomFieldOpGte:
			return column + " >= " + operand
		case registry.CustomFieldOpLte:
			return column + " <= " + operand
		}
		return ""
	}
	switch f.Type {
	case models.CustomFieldTypeText, models.CustomFieldTypeURL, models.CustomFieldTypeLongText:
		column := "v.value_text"
		if f.Type == models.CustomFieldTypeLongText {
			column = "v.value_long_text"
		}
		switch f.Op {
		case registry.CustomFieldOpContains:
			return column + " ILIKE " + cond.Var(containsPattern(f.Text))
		case registry.CustomFieldOpEq:
			return "LOWER(" + column + ") = LOWER(" + cond.Var(f.Text) + ")"
		}
	case models.CustomFieldTypeNumber:
		return compare("v.value_number", cond.Var(f.Number.String())+"::numeric")
	case models.CustomFieldTypeMoney:
		return compare("v.value_money", cond.Var(f.Number.String())+"::numeric")
	case models.CustomFieldTypeDate:
		return compare("v.value_date", cond.Var(f.Text)+"::date")
	case models.CustomFieldTypeBoolean:
		if f.Op == registry.CustomFieldOpEq {
			return "v.value_bool = " + cond.Var(f.Bool)
		}
	case models.CustomFieldTypeSelect:
		if (f.Op == registry.CustomFieldOpEq || f.Op == registry.CustomFieldOpIn) && len(f.OptionIDs) > 0 {
			return "v.value_select = ANY(" + cond.Var(f.OptionIDs) + "::text[])"
		}
	case models.CustomFieldTypeItemLink:
		if f.Op == registry.CustomFieldOpEq {
			return "v.value_item_id = " + cond.Var(f.Text)
		}
	}
	return ""
}

// customFieldSearchExprs are the alternatives a search term gets from
// custom field values, see registry.CustomFieldSearch.
func customFieldSearchExprs(cond *sqlb.Cond, s registry.CustomFieldSearch, term, commoditiesTable, valuesTable string) []string {
	var out []string
	if len(s.TextFields) > 0 {
		ids := slices.Sorted(maps.Keys(s.TextFields))
		pattern := cond.Var(containsPattern(term))
		out = append(out, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM %s v WHERE v.commodity_id = %s.id AND v.field_id = ANY(%s::text[]) AND (v.value_text ILIKE %s OR v.value_long_text ILIKE %s))",
			valuesTable, commoditiesTable, cond.Var(ids), pattern, pattern))
	}
	if refs := s.Options[term]; len(refs) > 0 {
		pairs := make([]string, 0, len(refs))
		for _, ref := range refs {
			pairs = append(pairs, "(v.field_id = "+cond.Var(ref.FieldID)+" AND v.value_select = "+cond.Var(ref.OptionID)+")")
		}
		out = append(out, fmt.Sprintf("EXISTS (SELECT 1 FROM %s v WHERE v.commodity_id = %s.id AND (%s))",
			valuesTable, commoditiesTable, strings.Join(pairs, " OR ")))
	}
	return out
}

// customFieldSortExpr is the ORDER BY expression of a
// registry.CustomFieldSort: the item's value for the field, NULL when it has
// none.
func customFieldSortExpr(sb *sqlb.SelectBuilder, s registry.CustomFieldSort, commoditiesTable, valuesTable string) string {
	var expr string
	switch s.Type {
	case models.CustomFieldTypeText, models.CustomFieldTypeURL:
		expr = "LOWER(v.value_text)"
	case models.CustomFieldTypeNumber:
		expr = "v.value_number"
	case models.CustomFieldTypeMoney:
		expr = "v.value_money"
	case models.CustomFieldTypeDate:
		expr = "v.value_date"
	case models.CustomFieldTypeBoolean:
		expr = "v.value_bool"
	case models.CustomFieldTypeSelect:
		// An option the field no longer has sorts after every known one.
		expr = "COALESCE(array_position(" + sb.Var(s.OptionOrder) + "::text[], v.value_select), " + strconv.Itoa(len(s.OptionOrder)+1) + ")"
	default:
		return ""
	}
	return fmt.Sprintf("(SELECT %s FROM %s v WHERE v.commodity_id = %s.id AND v.field_id = %s)",
		expr, valuesTable, commoditiesTable, sb.Var(s.FieldID))
}

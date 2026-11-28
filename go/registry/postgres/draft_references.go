package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// The open drafts of a group name item types and custom fields in their
// form values: the type's slug under "type" and the field values by field
// ID under "custom_fields". The item type and custom field registries count
// those drafts as users and rewrite them when a type or field is deleted.
//
// A promotion locks its draft's row and then the rows of the type and the
// fields the item has. A delete therefore locks the drafts it rewrites
// before it locks the type's or the field's row, so the two never wait for
// each other in a cycle.

// draftTypeSQL is the item type slug a draft's values name, NULL when the
// "type" value is missing or not a text.
const draftTypeSQL = `CASE WHEN jsonb_typeof(form_values->'type') = 'string' THEN form_values->>'type' END`

// draftCustomFieldsSQL is a draft's custom_fields object, an empty object
// when the values hold none or something else.
const draftCustomFieldsSQL = `CASE WHEN jsonb_typeof(form_values->'custom_fields') = 'object' THEN form_values->'custom_fields' ELSE '{}'::jsonb END`

// draftScalarHasValueSQL is models.DraftCustomFieldHasValue for a scalar
// JSON value v.
func draftScalarHasValueSQL(v string) string {
	return fmt.Sprintf(`CASE jsonb_typeof(%[1]s)
		WHEN 'string' THEN btrim(%[1]s #>> '{}', E'%[2]s') <> ''
		WHEN 'number' THEN true
		WHEN 'boolean' THEN true
		ELSE false END`, v, sqlBlank)
}

// sqlBlank is models.DraftBlank written for a PostgreSQL escape string.
const sqlBlank = ` \t\n\r`

// draftCustomFieldHasValueSQL is models.DraftCustomFieldHasValue for the
// custom_fields entry e (a jsonb expression).
func draftCustomFieldHasValueSQL(e string) string {
	return fmt.Sprintf(`CASE
		WHEN jsonb_typeof(%[1]s) = 'object' THEN CASE
			WHEN %[1]s->'value' IS NOT NULL THEN %[2]s
			WHEN %[1]s->'amount' IS NOT NULL THEN jsonb_typeof(%[1]s->'amount') <> 'null'
			ELSE false END
		ELSE %[3]s END`, e, draftScalarHasValueSQL(e+"->'value'"), draftScalarHasValueSQL(e))
}

func openDraftStatus() string {
	return string(models.CommodityDraftOpen)
}

// lockDraftsOfTypeInTx locks the rows of the group's open drafts that name
// slug, until tx ends.
func lockDraftsOfTypeInTx(ctx context.Context, tx *sqlx.Tx, drafts store.TableName, groupID string, slug models.CommodityType) error {
	query := fmt.Sprintf(`SELECT id FROM %s WHERE group_id = $1 AND status = $2 AND %s = $3 ORDER BY id FOR UPDATE`,
		drafts, draftTypeSQL)
	var ids []string
	if err := tx.SelectContext(ctx, &ids, query, groupID, openDraftStatus(), string(slug)); err != nil {
		return errxtrace.Wrap("failed to lock the drafts of the item type", err, errx.Attrs("type", string(slug)))
	}
	return nil
}

// countDraftsOfTypeInTx counts the group's open drafts that name slug.
func countDraftsOfTypeInTx(ctx context.Context, tx *sqlx.Tx, drafts store.TableName, groupID string, slug models.CommodityType) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE group_id = $1 AND status = $2 AND %s = $3`, drafts, draftTypeSQL)
	var n int
	if err := tx.GetContext(ctx, &n, query, groupID, openDraftStatus(), string(slug)); err != nil {
		return 0, errxtrace.Wrap("failed to count the drafts of the item type", err, errx.Attrs("type", string(slug)))
	}
	return n, nil
}

// moveDraftsOfTypeInTx gives the group's open drafts that name from the type
// to instead, each with a new revision.
func moveDraftsOfTypeInTx(ctx context.Context, tx *sqlx.Tx, drafts store.TableName, groupID string, from, to models.CommodityType) error {
	query := fmt.Sprintf(`
		UPDATE %s SET form_values = jsonb_set(form_values, '{type}', to_jsonb($1::text)),
			revision = revision + 1, updated_at = $2
		WHERE group_id = $3 AND status = $4 AND %s = $5`, drafts, draftTypeSQL)
	if _, err := tx.ExecContext(ctx, query, string(to), time.Now(), groupID, openDraftStatus(), string(from)); err != nil {
		return errxtrace.Wrap("failed to move the drafts to another item type", err, errx.Attrs("type", string(from)))
	}
	return nil
}

// countDraftTypesInTx counts the open drafts per item type slug they name,
// in the drafts RLS lets tx see.
func countDraftTypesInTx(ctx context.Context, tx *sqlx.Tx, drafts store.TableName) (map[models.CommodityType]int, error) {
	query := fmt.Sprintf(`SELECT %[1]s, COUNT(*) FROM %[2]s WHERE status = $1 AND %[1]s IS NOT NULL GROUP BY 1`,
		draftTypeSQL, drafts)
	rows, err := tx.QueryContext(ctx, query, openDraftStatus())
	if err != nil {
		return nil, errxtrace.Wrap("failed to count drafts per item type", err)
	}
	defer rows.Close()
	counts := map[models.CommodityType]int{}
	for rows.Next() {
		var (
			slug string
			n    int
		)
		if err := rows.Scan(&slug, &n); err != nil {
			return nil, errxtrace.Wrap("failed to scan draft count", err)
		}
		counts[models.CommodityType(slug)] = n
	}
	return counts, rows.Err()
}

// lockDraftsWithFieldInTx locks the rows of the group's open drafts whose
// custom_fields have an entry for fieldID, value or not, until tx ends. It
// locks the drafts with an empty entry too, although dropDraftFieldInTx
// leaves them as they are: a draft that gets a value meanwhile waits for the
// delete, so the count and the rewrite see the same drafts, and the rewrite
// locks no draft row after the field's row.
func lockDraftsWithFieldInTx(ctx context.Context, tx *sqlx.Tx, drafts store.TableName, groupID, fieldID string) error {
	query := fmt.Sprintf(`SELECT id FROM %s WHERE group_id = $1 AND status = $2 AND (%s)->$3::text IS NOT NULL ORDER BY id FOR UPDATE`,
		drafts, draftCustomFieldsSQL)
	var ids []string
	if err := tx.SelectContext(ctx, &ids, query, groupID, openDraftStatus(), fieldID); err != nil {
		return errxtrace.Wrap("failed to lock the drafts of the custom field", err, errx.Attrs("custom_field_id", fieldID))
	}
	return nil
}

// countDraftValuesInTx counts the group's open drafts that hold a value of
// fieldID.
func countDraftValuesInTx(ctx context.Context, tx *sqlx.Tx, drafts store.TableName, groupID, fieldID string) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE group_id = $1 AND status = $2 AND %s`,
		drafts, draftCustomFieldHasValueSQL("(("+draftCustomFieldsSQL+")->$3::text)"))
	var n int
	if err := tx.GetContext(ctx, &n, query, groupID, openDraftStatus(), fieldID); err != nil {
		return 0, errxtrace.Wrap("failed to count the draft values of the custom field", err, errx.Attrs("custom_field_id", fieldID))
	}
	return n, nil
}

// dropDraftFieldInTx takes fieldID's entry out of the custom_fields of the
// group's open drafts that hold a value of it, each with a new revision. A
// draft whose entry is empty keeps it and its revision: the item form keeps
// an empty entry for every field it shows, and leaves a field it does not
// know out of the item it saves.
func dropDraftFieldInTx(ctx context.Context, tx *sqlx.Tx, drafts store.TableName, groupID, fieldID string) error {
	query := fmt.Sprintf(`
		UPDATE %s SET form_values = form_values #- ARRAY['custom_fields', $1::text],
			revision = revision + 1, updated_at = $2
		WHERE group_id = $3 AND status = $4 AND %s`, drafts,
		draftCustomFieldHasValueSQL("(("+draftCustomFieldsSQL+")->$1::text)"))
	if _, err := tx.ExecContext(ctx, query, fieldID, time.Now(), groupID, openDraftStatus()); err != nil {
		return errxtrace.Wrap("failed to remove the custom field from the drafts", err, errx.Attrs("custom_field_id", fieldID))
	}
	return nil
}

// countDraftValuesPerFieldInTx counts, per field ID, the open drafts that
// hold a value of the field, in the drafts RLS lets tx see.
func countDraftValuesPerFieldInTx(ctx context.Context, tx *sqlx.Tx, drafts store.TableName) (map[string]int, error) {
	query := fmt.Sprintf(`SELECT e.key, COUNT(*) FROM %s d, jsonb_each(%s) e
		WHERE d.status = $1 AND %s GROUP BY e.key`,
		drafts, "CASE WHEN jsonb_typeof(d.form_values->'custom_fields') = 'object' THEN d.form_values->'custom_fields' ELSE '{}'::jsonb END",
		draftCustomFieldHasValueSQL("e.value"))
	rows, err := tx.QueryContext(ctx, query, openDraftStatus())
	if err != nil {
		return nil, errxtrace.Wrap("failed to count draft values per custom field", err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var (
			id string
			n  int
		)
		if err := rows.Scan(&id, &n); err != nil {
			return nil, errxtrace.Wrap("failed to scan draft value count", err)
		}
		counts[id] = n
	}
	return counts, rows.Err()
}

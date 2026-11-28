package postgres

import (
	"context"
	"fmt"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
)

// removeDepartedMemberRows deletes, in the transaction that deletes the
// membership, the rows that were personal to the member in the group
// (#2892). Nobody in the group can see or remove them once the member is
// gone, and a member invited back would find them as they were:
//
//   - the member's own placements of saved views, and their personal views
//     (the placements of those go with them by ON DELETE CASCADE). Shared
//     views stay, other members use them;
//   - the member's notification overrides for the group;
//   - the item reminders the member set (their send records cascade); they
//     only ever email their author;
//   - the drafts the member started and the file links to them. The files
//     stay in the group as standalone files, as on account deletion. A draft
//     made for a document another system delivered stays: it belongs to the
//     group's inbox, and the document record names it;
//   - the member's Add item sessions, with their sources, candidates and the
//     file links of the sources;
//   - the invites the member created for the group that nobody used yet;
//   - and the API tokens the member limited to the group are revoked, as they
//     no longer reach anything.
//
// Everything else the member created in the group (items, files, places,
// tags, loans, history, ...) is the group's data and stays.
func (r *GroupMembershipRegistry) removeDepartedMemberRows(ctx context.Context, tx *sqlx.Tx, tenantID, groupID, userID string) error {
	t := r.tableNames
	drafts := fmt.Sprintf(
		`SELECT id FROM %s WHERE tenant_id = $1 AND group_id = $2 AND created_by_user_id = $3 AND client_key NOT LIKE $4`,
		t.CommodityDrafts())
	sources := fmt.Sprintf(
		`SELECT id FROM %s WHERE tenant_id = $1 AND group_id = $2 AND created_by_user_id = $3`,
		t.IntakeSources())
	delivered := models.IntakeDeliveredDraftKeyPrefix + "%"
	steps := []struct {
		what  string
		query string
		args  []any
	}{
		{"saved view placements", fmt.Sprintf(
			`DELETE FROM %s WHERE tenant_id = $1 AND group_id = $2 AND user_id = $3`, t.SavedViewPlacements()), nil},
		{"personal saved views", fmt.Sprintf(
			`DELETE FROM %s WHERE tenant_id = $1 AND group_id = $2 AND created_by_user_id = $3 AND NOT shared`, t.SavedViews()), nil},
		{"notification overrides", fmt.Sprintf(
			`DELETE FROM %s WHERE tenant_id = $1 AND group_id = $2 AND user_id = $3`, t.GroupNotificationPrefs()), nil},
		{"item reminders", fmt.Sprintf(
			`DELETE FROM %s WHERE tenant_id = $1 AND group_id = $2 AND created_by_user_id = $3`, t.CommodityReminders()), nil},
		{"links to drafts", fmt.Sprintf(
			`DELETE FROM %s WHERE tenant_id = $1 AND entity_type = $5 AND entity_id IN (%s)`, t.FileLinks(), drafts),
			[]any{delivered, models.FileLinkDraft}},
		{"drafts", fmt.Sprintf(
			`DELETE FROM %s WHERE tenant_id = $1 AND group_id = $2 AND created_by_user_id = $3 AND client_key NOT LIKE $4`,
			t.CommodityDrafts()), []any{delivered}},
		{"links to intake sources", fmt.Sprintf(
			`DELETE FROM %s WHERE tenant_id = $1 AND entity_type = $4 AND entity_id IN (%s)`, t.FileLinks(), sources),
			[]any{models.FileLinkIntake}},
		{"intake sessions", fmt.Sprintf(
			`DELETE FROM %s WHERE tenant_id = $1 AND group_id = $2 AND created_by_user_id = $3`, t.IntakeSessions()), nil},
		{"unused invites", fmt.Sprintf(
			`DELETE FROM %s WHERE tenant_id = $1 AND group_id = $2 AND created_by = $3 AND used_by IS NULL`, t.GroupInvites()), nil},
		{"group API tokens", fmt.Sprintf(
			`UPDATE %s SET revoked_at = CURRENT_TIMESTAMP WHERE tenant_id = $1 AND group_id = $2 AND user_id = $3 AND revoked_at IS NULL`,
			t.APITokens()), nil},
	}
	for _, step := range steps {
		args := append([]any{tenantID, groupID, userID}, step.args...)
		if _, err := tx.ExecContext(ctx, step.query, args...); err != nil {
			return errxtrace.Wrap("failed to remove the departed member's "+step.what, err,
				errx.Attrs("tenant_id", tenantID, "group_id", groupID, "user_id", userID))
		}
	}
	return nil
}

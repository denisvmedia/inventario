package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// IntakeRegistryFactory creates IntakeRegistry instances with proper context
// (#2810).
type IntakeRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// IntakeRegistry is the postgres-backed group-scoped registry of intake
// sessions, their sources and their detected products.
type IntakeRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.IntakeRegistry        = (*IntakeRegistry)(nil)
	_ registry.IntakeRegistryFactory = (*IntakeRegistryFactory)(nil)
)

func NewIntakeRegistry(dbx *sqlx.DB) *IntakeRegistryFactory {
	return NewIntakeRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewIntakeRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *IntakeRegistryFactory {
	return &IntakeRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *IntakeRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.IntakeRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *IntakeRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.IntakeRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &IntakeRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
	}, nil
}

func (f *IntakeRegistryFactory) CreateServiceRegistry() registry.IntakeRegistry {
	return &IntakeRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *IntakeRegistry) newSQLRegistry() *store.RLSGroupRepository[models.IntakeSession, *models.IntakeSession] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.IntakeSession](r.dbx, r.tableNames.IntakeSessions())
	}
	return store.NewGroupAwareSQLRegistry[models.IntakeSession](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.IntakeSessions())
}

// do runs fn in a transaction with the registry's RLS context: the group of a
// user registry, or the background-worker role of the service registry.
func (r *IntakeRegistry) do(ctx context.Context, fn func(context.Context, *sqlx.Tx) error) error {
	return r.newSQLRegistry().Do(ctx, fn)
}

func (r *IntakeRegistry) requireGroup() error {
	if r.service {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	if r.tenantID == "" {
		return store.ErrTenantIDRequired
	}
	if r.groupID == "" {
		return store.ErrGroupIDRequired
	}
	if r.createdByUserID == "" {
		return store.ErrCreatedByUserIDRequired
	}
	return nil
}

func intakeNotFound(entityType, id string) error {
	return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", entityType, "entity_id", id))
}

func (r *IntakeRegistry) Get(ctx context.Context, id string) (*models.IntakeSession, error) {
	var session models.IntakeSession
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &session); err != nil {
		return nil, errxtrace.Wrap("failed to get intake session", err)
	}
	return &session, nil
}

func (r *IntakeRegistry) GetSessionByClientKey(ctx context.Context, clientKey string) (*models.IntakeSession, error) {
	var session models.IntakeSession
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("client_key", clientKey), &session); err != nil {
		return nil, errxtrace.Wrap("failed to get intake session by client key", err)
	}
	return &session, nil
}

func (r *IntakeRegistry) List(ctx context.Context) ([]*models.IntakeSession, error) {
	var sessions []*models.IntakeSession
	for session, err := range r.newSQLRegistry().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list intake sessions", err)
		}
		sessions = append(sessions, new(session))
	}
	return sessions, nil
}

func (r *IntakeRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count intake sessions", err)
	}
	return cnt, nil
}

// Create stores a session without sources. CreateSession is the form the
// service uses; Create exists for the Registry contract.
func (r *IntakeRegistry) Create(ctx context.Context, session models.IntakeSession) (*models.IntakeSession, error) {
	stored, _, err := r.CreateSession(ctx, session, nil)
	return stored, err
}

// Update writes the session's client key and origin. The session carries no
// other state.
func (r *IntakeRegistry) Update(ctx context.Context, session models.IntakeSession) (*models.IntakeSession, error) {
	session.UpdatedAt = time.Now().UTC()
	if err := r.newSQLRegistry().Update(ctx, session, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update intake session", err)
	}
	return &session, nil
}

// Delete deletes the session. Its sources and candidates go through the ON
// DELETE CASCADE foreign keys; the file links of the sources are polymorphic
// and are deleted here, in the same transaction.
func (r *IntakeRegistry) Delete(ctx context.Context, id string) error {
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		links := fmt.Sprintf(`
			DELETE FROM %s
			WHERE entity_type = $1 AND entity_id IN (SELECT id FROM %s WHERE session_id = $2)`,
			r.tableNames.FileLinks(), r.tableNames.IntakeSources())
		if _, err := tx.ExecContext(ctx, links, models.FileLinkIntake, id); err != nil {
			return errxtrace.Wrap("failed to delete intake file links", err)
		}
		query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, r.tableNames.IntakeSessions())
		deleted, err := execAffected(ctx, tx, query, id)
		if err != nil {
			return err
		}
		if !deleted {
			return intakeNotFound("intake_session", id)
		}
		return nil
	})
	if err != nil {
		return errxtrace.Wrap("failed to delete intake session", err)
	}
	return nil
}

// CreateSession stores the session, its sources and their file links in one
// transaction. The session insert uses ON CONFLICT DO NOTHING on the
// (group_id, client_key) index, so two concurrent creates with one key store
// one session and both callers get it.
func (r *IntakeRegistry) CreateSession(ctx context.Context, session models.IntakeSession, sources []registry.IntakeSourceInput) (*models.IntakeSession, bool, error) {
	if err := r.requireGroup(); err != nil {
		return nil, false, err
	}
	now := time.Now().UTC()
	session.ID = generateID()
	session.UUID = generateID()
	session.TenantID = r.tenantID
	session.GroupID = r.groupID
	session.CreatedByUserID = r.createdByUserID
	session.CreatedAt = now
	session.UpdatedAt = now
	if session.Origin == "" {
		session.Origin = models.IntakeOriginUpload
	}

	var (
		stored  models.IntakeSession
		created bool
	)
	files := &FileRegistry{tableNames: r.tableNames, createdByUserID: r.createdByUserID}
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		insert := fmt.Sprintf(`
			INSERT INTO %s (id, uuid, tenant_id, group_id, created_by_user_id, client_key, origin, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (group_id, client_key) DO NOTHING
			RETURNING *`, r.tableNames.IntakeSessions())
		err := tx.QueryRowxContext(ctx, insert,
			session.ID, session.UUID, session.TenantID, session.GroupID, session.CreatedByUserID,
			session.ClientKey, string(session.Origin), session.CreatedAt, session.UpdatedAt,
		).StructScan(&stored)
		if errors.Is(err, sql.ErrNoRows) {
			existing := fmt.Sprintf(`SELECT * FROM %s WHERE group_id = $1 AND client_key = $2`, r.tableNames.IntakeSessions())
			if err := tx.QueryRowxContext(ctx, existing, r.groupID, session.ClientKey).StructScan(&stored); err != nil {
				return errxtrace.Wrap("failed to read existing intake session", err)
			}
			return nil
		}
		if err != nil {
			return errxtrace.Wrap("failed to insert intake session", err)
		}
		created = true

		for i, input := range sources {
			if err := r.insertSource(ctx, tx, files, stored.ID, i, input, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to create intake session", err)
	}
	return &stored, created, nil
}

func (r *IntakeRegistry) ListSessions(ctx context.Context, offset, limit int) ([]*models.IntakeSession, int, error) {
	var (
		sessions []*models.IntakeSession
		total    int
	)
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		count := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, r.tableNames.IntakeSessions())
		if err := tx.GetContext(ctx, &total, count); err != nil {
			return errxtrace.Wrap("failed to count intake sessions", err)
		}
		query := fmt.Sprintf(`SELECT * FROM %s ORDER BY created_at DESC, id DESC OFFSET $1 LIMIT $2`, r.tableNames.IntakeSessions())
		return selectInto(ctx, tx, &sessions, query, max(offset, 0), max(limit, 0))
	})
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list intake sessions", err)
	}
	return sessions, total, nil
}

// selectInto runs a query in tx and appends a copy of every row to out.
func selectInto[T any](ctx context.Context, tx *sqlx.Tx, out *[]*T, query string, args ...any) error {
	rows, err := tx.QueryxContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var row T
		if err := rows.StructScan(&row); err != nil {
			return err
		}
		*out = append(*out, &row)
	}
	return rows.Err()
}

func (r *IntakeRegistry) ListSources(ctx context.Context, sessionIDs ...string) ([]*models.IntakeSource, error) {
	sources := []*models.IntakeSource{}
	if len(sessionIDs) == 0 {
		return sources, nil
	}
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE session_id = ANY($1) ORDER BY session_id, sort_order, id`, r.tableNames.IntakeSources())
		return selectInto(ctx, tx, &sources, query, sessionIDs)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list intake sources", err)
	}
	return sources, nil
}

func (r *IntakeRegistry) getSource(ctx context.Context, field, value string) (*models.IntakeSource, error) {
	var source models.IntakeSource
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		return store.NewTxRegistry[models.IntakeSource](tx, r.tableNames.IntakeSources()).ScanOneByField(ctx, store.Pair(field, value), &source)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to get intake source", err)
	}
	return &source, nil
}

func (r *IntakeRegistry) GetSource(ctx context.Context, id string) (*models.IntakeSource, error) {
	return r.getSource(ctx, "id", id)
}

func (r *IntakeRegistry) GetSourceByExternalID(ctx context.Context, system, instance, documentID string) (*models.IntakeSource, error) {
	if documentID == "" {
		return nil, intakeNotFound("intake_source", documentID)
	}
	var source models.IntakeSource
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT * FROM %s
			WHERE external_system = $1 AND external_instance = $2 AND external_document_id = $3`, r.tableNames.IntakeSources())
		err := tx.QueryRowxContext(ctx, query, system, instance, documentID).StructScan(&source)
		if errors.Is(err, sql.ErrNoRows) {
			return intakeNotFound("intake_source", documentID)
		}
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to get intake source by external id", err)
	}
	return &source, nil
}

func (r *IntakeRegistry) ListCandidates(ctx context.Context, sessionIDs ...string) ([]*models.IntakeCandidate, error) {
	candidates := []*models.IntakeCandidate{}
	if len(sessionIDs) == 0 {
		return candidates, nil
	}
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			SELECT c.* FROM %s c
			JOIN %s s ON s.id = c.source_id
			WHERE c.session_id = ANY($1)
			ORDER BY c.session_id, s.sort_order, c.sort_order, c.id`,
			r.tableNames.IntakeCandidates(), r.tableNames.IntakeSources())
		return selectInto(ctx, tx, &candidates, query, sessionIDs)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list intake candidates", err)
	}
	return candidates, nil
}

func (r *IntakeRegistry) GetCandidate(ctx context.Context, id string) (*models.IntakeCandidate, error) {
	var candidate models.IntakeCandidate
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		return store.NewTxRegistry[models.IntakeCandidate](tx, r.tableNames.IntakeCandidates()).ScanOneByField(ctx, store.Pair("id", id), &candidate)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to get intake candidate", err)
	}
	return &candidate, nil
}

// lockCandidate reads the candidate and holds its row lock until tx ends.
func (r *IntakeRegistry) lockCandidate(ctx context.Context, tx *sqlx.Tx, id string) (*models.IntakeCandidate, error) {
	var candidate models.IntakeCandidate
	query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 FOR UPDATE`, r.tableNames.IntakeCandidates())
	if err := tx.QueryRowxContext(ctx, query, id).StructScan(&candidate); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, intakeNotFound("intake_candidate", id)
		}
		return nil, errxtrace.Wrap("failed to read intake candidate", err)
	}
	return &candidate, nil
}

func (r *IntakeRegistry) ReviewCandidate(ctx context.Context, id string, review registry.IntakeCandidateReview) (*models.IntakeCandidate, error) {
	var updated models.IntakeCandidate
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		candidate, err := r.lockCandidate(ctx, tx, id)
		if err != nil {
			return err
		}
		if candidate.Outcome == models.IntakeCandidateCreated {
			return errxtrace.Classify(registry.ErrIntakeCandidateFinished, errx.Attrs("entity_id", id))
		}
		if review.Selected != nil {
			candidate.Selected = *review.Selected
			if candidate.Selected && candidate.Outcome == models.IntakeCandidateSkipped {
				candidate.Outcome = models.IntakeCandidatePending
			}
		}
		if review.AcceptedFields != nil {
			candidate.AcceptedFields = models.ValuerSlice[string](append([]string{}, *review.AcceptedFields...))
		}
		if review.Quantity != nil {
			candidate.Quantity = *review.Quantity
		}
		if candidate.AcceptedFields == nil {
			candidate.AcceptedFields = models.ValuerSlice[string]{}
		}
		update := fmt.Sprintf(`
			UPDATE %s SET selected = $1, accepted_fields = $2, quantity = $3, outcome = $4, updated_at = $5
			WHERE id = $6
			RETURNING *`, r.tableNames.IntakeCandidates())
		return tx.QueryRowxContext(ctx, update,
			candidate.Selected, candidate.AcceptedFields, candidate.Quantity, string(candidate.Outcome), time.Now().UTC(), id,
		).StructScan(&updated)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to review intake candidate", err)
	}
	return &updated, nil
}

func (r *IntakeRegistry) SetCandidateOutcome(ctx context.Context, id string, outcome models.IntakeCandidateOutcome, draftID *string, errorCode string) (*models.IntakeCandidate, error) {
	var storedDraftID *string
	storedError := ""
	switch outcome {
	case models.IntakeCandidateCreated:
		storedDraftID = draftID
	case models.IntakeCandidateFailed:
		storedError = errorCode
	}
	var updated models.IntakeCandidate
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		update := fmt.Sprintf(`
			UPDATE %s SET outcome = $1, draft_id = $2, error_code = $3, updated_at = $4
			WHERE id = $5
			RETURNING *`, r.tableNames.IntakeCandidates())
		err := tx.QueryRowxContext(ctx, update, string(outcome), storedDraftID, storedError, time.Now().UTC(), id).StructScan(&updated)
		if errors.Is(err, sql.ErrNoRows) {
			return intakeNotFound("intake_candidate", id)
		}
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to record intake candidate outcome", err)
	}
	return &updated, nil
}

func (r *IntakeRegistry) execCount(ctx context.Context, query string, args ...any) (int, error) {
	var n int64
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
		n, err = res.RowsAffected()
		return err
	})
	return int(n), err
}

func (r *IntakeRegistry) CancelSession(ctx context.Context, sessionID string) (int, error) {
	query := fmt.Sprintf(`
		UPDATE %s SET status = $1, wait_reason = '', updated_at = $2
		WHERE session_id = $3 AND status IN ($4, $5)`, r.tableNames.IntakeSources())
	n, err := r.execCount(ctx, query, string(models.IntakeSourceCanceled), time.Now().UTC(), sessionID,
		string(models.IntakeSourcePending), string(models.IntakeSourceScanning))
	if err != nil {
		return 0, errxtrace.Wrap("failed to cancel intake session", err)
	}
	return n, nil
}

func (r *IntakeRegistry) ResumeSession(ctx context.Context, sessionID string, now time.Time) (int, error) {
	query := fmt.Sprintf(`
		UPDATE %s SET status = $1, attempts = 0, next_attempt_at = $2, claimed_at = NULL,
			wait_reason = '', error_code = '', updated_at = $2
		WHERE session_id = $3 AND status IN ($4, $5)`, r.tableNames.IntakeSources())
	n, err := r.execCount(ctx, query, string(models.IntakeSourcePending), now.UTC(), sessionID,
		string(models.IntakeSourceCanceled), string(models.IntakeSourceFailed))
	if err != nil {
		return 0, errxtrace.Wrap("failed to resume intake session", err)
	}
	return n, nil
}

// lockSource reads the source and holds its row lock until tx ends.
func (r *IntakeRegistry) lockSource(ctx context.Context, tx *sqlx.Tx, id string) (*models.IntakeSource, error) {
	var source models.IntakeSource
	query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 FOR UPDATE`, r.tableNames.IntakeSources())
	if err := tx.QueryRowxContext(ctx, query, id).StructScan(&source); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, intakeNotFound("intake_source", id)
		}
		return nil, errxtrace.Wrap("failed to read intake source", err)
	}
	return &source, nil
}

func (r *IntakeRegistry) RescanSource(ctx context.Context, id string, now time.Time) (*models.IntakeSource, error) {
	var updated models.IntakeSource
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		source, err := r.lockSource(ctx, tx, id)
		if err != nil {
			return err
		}
		if source.Status.IsQueued() {
			return errxtrace.Classify(registry.ErrIntakeSourceBusy, errx.Attrs("entity_id", id))
		}
		var hasDrafts bool
		exists := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE source_id = $1 AND outcome = $2)`, r.tableNames.IntakeCandidates())
		if err := tx.QueryRowxContext(ctx, exists, id, string(models.IntakeCandidateCreated)).Scan(&hasDrafts); err != nil {
			return errxtrace.Wrap("failed to check intake candidates", err)
		}
		if hasDrafts {
			return errxtrace.Classify(registry.ErrIntakeSourceHasDrafts, errx.Attrs("entity_id", id))
		}
		del := fmt.Sprintf(`DELETE FROM %s WHERE source_id = $1`, r.tableNames.IntakeCandidates())
		if _, err := tx.ExecContext(ctx, del, id); err != nil {
			return errxtrace.Wrap("failed to delete intake candidates", err)
		}
		update := fmt.Sprintf(`
			UPDATE %s SET status = $1, attempts = 0, next_attempt_at = $2, claimed_at = NULL,
				wait_reason = '', error_code = '', warnings = '[]'::jsonb, scanned_at = NULL, updated_at = $2
			WHERE id = $3
			RETURNING *`, r.tableNames.IntakeSources())
		return tx.QueryRowxContext(ctx, update, string(models.IntakeSourcePending), now.UTC(), id).StructScan(&updated)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to rescan intake source", err)
	}
	return &updated, nil
}

func (r *IntakeRegistry) CountQueued(ctx context.Context) (int, error) {
	var n int
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE status IN ($1, $2)`, r.tableNames.IntakeSources())
		return tx.GetContext(ctx, &n, query, string(models.IntakeSourcePending), string(models.IntakeSourceScanning))
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to count queued intake sources", err)
	}
	return n, nil
}

func (r *IntakeRegistry) SweepStuckSources(ctx context.Context, claimedBefore, now time.Time) (int, error) {
	query := fmt.Sprintf(`
		UPDATE %s SET status = $1, next_attempt_at = $2, claimed_at = NULL, updated_at = $2
		WHERE status = $3 AND (claimed_at IS NULL OR claimed_at < $4)`, r.tableNames.IntakeSources())
	n, err := r.execCount(ctx, query, string(models.IntakeSourcePending), now.UTC(),
		string(models.IntakeSourceScanning), claimedBefore.UTC())
	if err != nil {
		return 0, errxtrace.Wrap("failed to sweep stuck intake sources", err)
	}
	return n, nil
}

// ClaimNextSource takes the due source with SKIP LOCKED, so concurrent
// workers pass over a row another one is claiming instead of waiting for it.
func (r *IntakeRegistry) ClaimNextSource(ctx context.Context, now time.Time) (*models.IntakeSource, error) {
	var (
		claimed models.IntakeSource
		found   bool
	)
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`
			UPDATE %[1]s SET status = $1, attempts = attempts + 1, claimed_at = $2, updated_at = $2
			WHERE id = (
				SELECT id FROM %[1]s
				WHERE status = $3 AND next_attempt_at <= $2
				ORDER BY next_attempt_at, created_at, sort_order, id
				FOR UPDATE SKIP LOCKED
				LIMIT 1
			)
			RETURNING *`, r.tableNames.IntakeSources())
		err := tx.QueryRowxContext(ctx, query, string(models.IntakeSourceScanning), now.UTC(), string(models.IntakeSourcePending)).StructScan(&claimed)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to claim intake source", err)
	}
	if !found {
		return nil, intakeNotFound("intake_source", "")
	}
	return &claimed, nil
}

// lockSourceInScan locks the source and checks that it is still the scan the
// worker claimed: scanning, or canceled after the claim, with the same
// attempt.
func (r *IntakeRegistry) lockSourceInScan(ctx context.Context, tx *sqlx.Tx, id string, attempt int) (*models.IntakeSource, error) {
	source, err := r.lockSource(ctx, tx, id)
	if errors.Is(err, registry.ErrNotFound) {
		return nil, errxtrace.Classify(registry.ErrIntakeSourceChanged, errx.Attrs("entity_id", id))
	}
	if err != nil {
		return nil, err
	}
	if source.Attempts == attempt {
		switch {
		case source.Status == models.IntakeSourceScanning:
			return source, nil
		case source.Status == models.IntakeSourceCanceled && source.ClaimedAt != nil:
			return source, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrIntakeSourceChanged, errx.Attrs("entity_id", id))
}

// lockScanning is lockSourceInScan for an outcome a canceled scan does not
// take.
func (r *IntakeRegistry) lockScanning(ctx context.Context, tx *sqlx.Tx, id string, attempt int) error {
	source, err := r.lockSourceInScan(ctx, tx, id, attempt)
	if err != nil {
		return err
	}
	if source.Status != models.IntakeSourceScanning {
		return errxtrace.Classify(registry.ErrIntakeSourceChanged, errx.Attrs("entity_id", id))
	}
	return nil
}

func (r *IntakeRegistry) StoreScan(ctx context.Context, sourceID string, attempt int, scan registry.IntakeScan, now time.Time) error {
	now = now.UTC()
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		source, err := r.lockSourceInScan(ctx, tx, sourceID, attempt)
		if err != nil {
			return err
		}
		del := fmt.Sprintf(`DELETE FROM %s WHERE source_id = $1`, r.tableNames.IntakeCandidates())
		if _, err := tx.ExecContext(ctx, del, sourceID); err != nil {
			return errxtrace.Wrap("failed to delete intake candidates", err)
		}
		insert := store.NewTxRegistry[models.IntakeCandidate](tx, r.tableNames.IntakeCandidates())
		for i, candidate := range scan.Candidates {
			candidate.TenantGroupAwareEntityID = models.TenantGroupAwareEntityID{
				EntityID:        models.EntityID{ID: generateID(), UUID: generateID()},
				TenantID:        source.TenantID,
				GroupID:         source.GroupID,
				CreatedByUserID: source.CreatedByUserID,
			}
			candidate.SessionID = source.SessionID
			candidate.SourceID = source.ID
			candidate.SortOrder = i
			if candidate.Fields == nil {
				candidate.Fields = models.IntakeFields{}
			}
			if candidate.AcceptedFields == nil {
				candidate.AcceptedFields = models.ValuerSlice[string]{}
			}
			if candidate.Quantity < 1 {
				candidate.Quantity = 1
			}
			candidate.Outcome = models.IntakeCandidatePending
			candidate.DraftID = nil
			candidate.ErrorCode = ""
			candidate.CreatedAt = now
			candidate.UpdatedAt = now
			if err := insert.Insert(ctx, candidate); err != nil {
				return errxtrace.Wrap("failed to insert intake candidate", err)
			}
		}
		warnings := models.ValuerSlice[models.IntakeWarning](scan.Warnings)
		if warnings == nil {
			warnings = models.ValuerSlice[models.IntakeWarning]{}
		}
		update := fmt.Sprintf(`
			UPDATE %s SET status = $1, warnings = $2, scanned_at = $3, wait_reason = '', error_code = '', updated_at = $3
			WHERE id = $4`, r.tableNames.IntakeSources())
		_, err = tx.ExecContext(ctx, update, string(models.IntakeSourceScanned), warnings, now, sourceID)
		return err
	})
	if err != nil {
		return errxtrace.Wrap("failed to store intake scan", err)
	}
	return nil
}

func (r *IntakeRegistry) RequeueSource(ctx context.Context, sourceID string, attempt int, next time.Time, waitReason string) error {
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := r.lockScanning(ctx, tx, sourceID, attempt); err != nil {
			return err
		}
		refund := 0
		if waitReason == models.IntakeWaitRateLimited {
			refund = 1
		}
		update := fmt.Sprintf(`
			UPDATE %s SET status = $1, next_attempt_at = $2, claimed_at = NULL, wait_reason = $3,
				attempts = GREATEST(attempts - $4, 0), updated_at = $5
			WHERE id = $6`, r.tableNames.IntakeSources())
		_, err := tx.ExecContext(ctx, update, string(models.IntakeSourcePending), next.UTC(), waitReason, refund, time.Now().UTC(), sourceID)
		return err
	})
	if err != nil {
		return errxtrace.Wrap("failed to requeue intake source", err)
	}
	return nil
}

func (r *IntakeRegistry) FailSource(ctx context.Context, sourceID string, attempt int, errorCode string) error {
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := r.lockScanning(ctx, tx, sourceID, attempt); err != nil {
			return err
		}
		update := fmt.Sprintf(`
			UPDATE %s SET status = $1, claimed_at = NULL, wait_reason = '', error_code = $2, updated_at = $3
			WHERE id = $4`, r.tableNames.IntakeSources())
		_, err := tx.ExecContext(ctx, update, string(models.IntakeSourceFailed), errorCode, time.Now().UTC(), sourceID)
		return err
	})
	if err != nil {
		return errxtrace.Wrap("failed to mark intake source failed", err)
	}
	return nil
}

// insertSource inserts one source of a new session and links its files to
// it. A file in Trash counts as not found; a source whose external identity
// the group already has is ErrAlreadyExists.
func (r *IntakeRegistry) insertSource(ctx context.Context, tx *sqlx.Tx, files *FileRegistry, sessionID string, sortOrder int, input registry.IntakeSourceInput, now time.Time) error {
	source := models.IntakeSource{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID:        models.EntityID{ID: generateID(), UUID: generateID()},
			TenantID:        r.tenantID,
			GroupID:         r.groupID,
			CreatedByUserID: r.createdByUserID,
		},
		SessionID:          sessionID,
		SortOrder:          sortOrder,
		OneItem:            input.OneItem,
		Status:             models.IntakeSourcePending,
		NextAttemptAt:      now,
		Warnings:           models.ValuerSlice[models.IntakeWarning]{},
		ExternalSystem:     input.ExternalSystem,
		ExternalInstance:   input.ExternalInstance,
		ExternalDocumentID: input.ExternalDocumentID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := store.NewTxRegistry[models.IntakeSource](tx, r.tableNames.IntakeSources()).Insert(ctx, source); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {
			return errxtrace.Classify(registry.ErrAlreadyExists,
				errx.Attrs("entity_type", "intake_source", "external_document_id", input.ExternalDocumentID))
		}
		return errxtrace.Wrap("failed to insert intake source", err)
	}
	for _, fileID := range input.FileIDs {
		file, err := files.lockFileForShare(ctx, tx, fileID)
		if err != nil {
			return err
		}
		if file.IsTrashed() {
			return intakeNotFound("file", fileID)
		}
		link := models.FileLink{
			FileID:     fileID,
			EntityType: models.FileLinkIntake,
			EntityID:   source.ID,
			Meta:       models.DefaultFileLinkMeta(models.FileLinkIntake, file.Category),
			CreatedAt:  now,
		}
		if _, _, err := files.insertLink(ctx, tx, file, link); err != nil {
			return err
		}
	}
	return nil
}

package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/currency"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/services"
)

// Currency-migration apiserver surface — issue #202 / #1551 PR 2/4.
//
// Four endpoints under /api/v1/g/{groupSlug}/currency-migrations:
//   - POST /preview     — dry-run, issues a 10-minute HMAC-signed preview_token
//   - POST /            — start; verifies token, opens a pending migration row
//   - GET  /            — paginated history
//   - GET  /{id}        — poll one
//
// Plus the requireGroupNotMigrating middleware that guards commodity
// write routes by 423 while a migration is pending|running.
//
// The whole surface is gated by params.FeatureCurrencyMigration;
// when off, the routes are not mounted at all so the schema and
// registries shipped in PR 1 stay inert (#202 §8).

// JSON:API error code constants. The FE branches on these strings
// (issue #202 §4.6) so they are part of the public API surface.
const (
	codeCurrencyMigrationSameCurrency      = "currency_migration.same_currency"
	codeCurrencyMigrationFromMismatch      = "currency_migration.from_mismatch"
	codeCurrencyMigrationRateInvalid       = "currency_migration.rate_invalid"
	codeCurrencyMigrationTokenInvalid      = "currency_migration.token_invalid"
	codeCurrencyMigrationPreviewExpired    = "currency_migration.preview_expired"
	codeCurrencyMigrationStateChanged      = "currency_migration.state_changed"
	codeCurrencyMigrationInProgress        = "currency_migration.migration_in_progress"
	codeCurrencyMigrationRestoreInProgress = "currency_migration.restore_in_progress"
	codeCurrencyMigrationDailyCapReached   = "currency_migration.daily_cap_reached"
	codeCurrencyMigrationLocked            = "currency_migration.locked"
)

// Constants from #202 §4 — code-resident, not config-driven (the
// design issue calls these out as "constants in code, not exposed as
// settings"). PR 3 mirrors currencyMigrationStuckThreshold for the
// recovery sweep.
const (
	currencyMigrationPreviewTTL = 10 * time.Minute
	currencyMigrationDailyCap   = 2
	previewMaxDiffEntries       = 100 // keep response sizes bounded
)

// currencyMigrationsAPI is the handler set for the four endpoints.
// Group context (slug → group) is provided by GroupSlugResolverMiddleware
// upstream; group-admin authorization is enforced by requireGroupAdmin
// at mount time.
//
// The featureEnabled flag mirrors Params.FeatureCurrencyMigration. The
// routes are always mounted (so swagger stays in sync with the chi
// router); each handler short-circuits with 404 when the flag is off,
// keeping the surface inert in production until the operator flips it.
type currencyMigrationsAPI struct {
	groupService   *services.GroupService
	auditService   services.AuditLogger
	featureEnabled bool
}

// CurrencyMigrations is the route-builder for the four endpoints. The
// caller mounts it under /g/{groupSlug}/currency-migrations after the
// group-aware middleware chain has populated the registry set + group
// context.
//
// All routes are admin-only. The route group itself enforces this via
// requireGroupAdmin so individual handlers don't need to repeat the
// check.
func CurrencyMigrations(params Params, groupService *services.GroupService, auditService services.AuditLogger) func(r chi.Router) {
	api := &currencyMigrationsAPI{
		groupService:   groupService,
		auditService:   auditService,
		featureEnabled: params.FeatureCurrencyMigration,
	}

	return func(r chi.Router) {
		// Feature gate runs first so non-admins also see 404 (rather
		// than 403) when the flag is off — pretending the route does
		// not exist at all matches the §8 "inert" promise.
		r.Use(api.featureGate)
		// Whole subtree is admin-only. Non-admin members get 403 from
		// requireGroupAdmin before any handler runs.
		r.Use(requireGroupAdmin(groupService))

		r.Post("/preview", api.preview)
		r.Post("/", api.start)
		r.Get("/", api.list)
		r.Get("/{id}", api.get)
	}
}

// featureGate is the always-mounted, per-handler kill-switch for the
// currency-migration surface (#202 §8). When FeatureCurrencyMigration is
// false the middleware returns plain 404 — the routes act as if they
// were never registered. Mounted before requireGroupAdmin so non-admin
// callers also see 404 in flag-off state, hiding even the existence of
// the surface.
func (api *currencyMigrationsAPI) featureGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !api.featureEnabled {
			_ = notFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// preview runs the conversion in-memory across every commodity in the
// group, returns projected totals + per-row diffs + a 10-minute
// HMAC-signed preview_token. No DB writes; no daily-quota slot
// consumed. Allowed during an in-flight migration (read-only).
//
// Order of validation: payload → from!=to (422) → rate (422) →
// commodity read → token. Same-currency and rate failures are caught
// before any group-scoped read — keeps the cost of bad inputs flat.
//
// @Summary Preview a currency migration
// @Description Dry-run the conversion, returning projected totals + per-row diffs + a signed preview_token.
// @Tags currency-migrations
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param request body jsonapi.CurrencyMigrationPreviewRequest true "Preview request"
// @Success 200 {object} jsonapi.CurrencyMigrationPreviewResponse "OK"
// @Failure 403 {object} jsonapi.Errors "Forbidden — non-admin"
// @Failure 422 {object} jsonapi.Errors "Validation error"
// @Router /g/{groupSlug}/currency-migrations/preview [post].
func (api *currencyMigrationsAPI) preview(w http.ResponseWriter, r *http.Request) {
	rs := RegistrySetFromContext(r.Context())
	if rs == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	user := GetUserFromRequest(r)
	if user == nil {
		unauthorizedError(w, r, nil)
		return
	}

	var input jsonapi.CurrencyMigrationPreviewRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes

	if attrs.FromCurrency != group.GroupCurrency {
		_ = codedUnprocessableEntityError(w, r,
			fmt.Errorf("from_currency must equal group's current currency (%s)", group.GroupCurrency),
			codeCurrencyMigrationFromMismatch)
		return
	}
	if attrs.FromCurrency == attrs.ToCurrency {
		_ = codedUnprocessableEntityError(w, r, currency.ErrSameCurrency, codeCurrencyMigrationSameCurrency)
		return
	}
	if err := currency.ValidateRate(attrs.ExchangeRate); err != nil {
		_ = codedUnprocessableEntityError(w, r, err, codeCurrencyMigrationRateInvalid)
		return
	}

	if rs.CurrencyMigrationRegistry == nil {
		_ = internalServerError(w, r, errors.New("currency migration registry not wired"))
		return
	}

	commodities, err := rs.CommodityRegistry.ListByGroup(r.Context(), user.TenantID, group.ID)
	if err != nil {
		_ = internalServerError(w, r, err)
		return
	}

	body, err := buildPreviewBody(commodities, attrs.FromCurrency, attrs.ToCurrency, attrs.ExchangeRate)
	if err != nil {
		_ = internalServerError(w, r, err)
		return
	}

	expiresAt := time.Now().UTC().Add(currencyMigrationPreviewTTL)
	token, err := rs.CurrencyMigrationRegistry.IssuePreviewToken(registry.PreviewTokenInputs{
		GroupID:      group.ID,
		FromCurrency: string(attrs.FromCurrency),
		ToCurrency:   string(attrs.ToCurrency),
		Rate:         canonicalRateString(attrs.ExchangeRate),
		StateHash:    body.StateHash,
		ExpiresAt:    expiresAt,
	})
	if err != nil {
		_ = internalServerError(w, r, err)
		return
	}

	body.PreviewToken = token
	body.PreviewExpiresAt = expiresAt
	body.PreviewExpiresInSec = int(currencyMigrationPreviewTTL / time.Second)

	if renderErr := render.Render(w, r, jsonapi.NewCurrencyMigrationPreviewResponse(body)); renderErr != nil {
		_ = internalServerError(w, r, renderErr)
		return
	}
}

// start verifies the preview token, runs the cross-op + daily-cap
// checks, and inserts a pending currency_migrations row. The worker
// (PR 3) picks the row up; this handler does NOT touch commodities.
//
// Order of checks (per #202 §4.6):
//  1. payload + same-currency / rate validation → 422
//  2. token signature → 422 token_invalid
//  3. token expiry → 409 preview_expired
//  4. token bindings (from/to/rate match the body) → 409 state_changed
//  5. live state hash recomputed → 409 state_changed
//  6. in-flight migration → 409 migration_in_progress
//  7. in-flight restore in this group → 409 restore_in_progress
//  8. daily cap → 429 daily_cap_reached
//  9. INSERT pending (partial unique index → 409 migration_in_progress)
//  10. set group lock + write start audit (best-effort; the registry's
//     Create unique-violation in 9 is the canonical race-loser).
//
// @Summary Start a currency migration
// @Description Verify preview token, perform cross-op + daily-cap checks, insert a pending migration row.
// @Tags currency-migrations
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param request body jsonapi.CurrencyMigrationStartRequest true "Start request"
// @Success 201 {object} jsonapi.CurrencyMigrationResponse "Created"
// @Failure 403 {object} jsonapi.Errors "Forbidden — non-admin"
// @Failure 409 {object} jsonapi.Errors "Token expired / state changed / migration in progress / restore in progress"
// @Failure 422 {object} jsonapi.Errors "Validation / token invalid"
// @Failure 429 {object} jsonapi.Errors "Daily cap reached"
// @Router /g/{groupSlug}/currency-migrations [post].
func (api *currencyMigrationsAPI) start(w http.ResponseWriter, r *http.Request) {
	rs := RegistrySetFromContext(r.Context())
	if rs == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	user := GetUserFromRequest(r)
	if user == nil {
		unauthorizedError(w, r, nil)
		return
	}

	if rs.CurrencyMigrationRegistry == nil {
		_ = internalServerError(w, r, errors.New("currency migration registry not wired"))
		return
	}

	var input jsonapi.CurrencyMigrationStartRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes

	if !validateStartAttributes(w, r, attrs, group) {
		return
	}
	now := time.Now().UTC()
	tokenInputs, ok := verifyStartToken(w, r, rs, attrs, now)
	if !ok {
		return
	}
	if !checkTokenBindingsAndState(w, r, rs, group, user, attrs, tokenInputs) {
		return
	}
	if !checkInFlightAndCap(w, r, rs, group, now) {
		return
	}

	// Persist pending row. The registry maps a partial unique-index
	// violation on (group_id) WHERE status IN ('pending', 'running')
	// to ErrMigrationInFlight — that's the canonical race-loser
	// response.
	expiresAt := tokenInputs.ExpiresAt
	previewToken := attrs.PreviewToken
	op := models.CurrencyMigration{
		FromCurrency:     attrs.FromCurrency,
		ToCurrency:       attrs.ToCurrency,
		ExchangeRate:     attrs.ExchangeRate,
		PreviewToken:     &previewToken,
		PreviewExpiresAt: &expiresAt,
	}
	created, err := rs.CurrencyMigrationRegistry.Create(r.Context(), op)
	switch {
	case errors.Is(err, registry.ErrMigrationInFlight):
		_ = codedConflictError(w, r, err, codeCurrencyMigrationInProgress, nil)
		return
	case err != nil:
		_ = renderEntityError(w, r, err)
		return
	}

	// 10. Set the group lock + write start audit. The lock is a
	//     best-effort UPDATE — if it fails (RLS, FK timing) the
	//     middleware/lock UX still reads InFlightForGroup as the source
	//     of truth, so the row's status alone is what matters.
	if err := setLocationGroupCurrencyMigrationID(r.Context(), rs, group.ID, &created.ID); err != nil {
		// Don't fail the request — the migration row is the canonical
		// state. Surface as a warning via the audit logger and continue.
		api.logAuditCurrencyMigration(r, user, group, "currency_migration.start", false, fmt.Sprintf("group lock update failed: %v", err))
	} else {
		api.logAuditCurrencyMigration(r, user, group, "currency_migration.start", true, "")
	}

	if renderErr := render.Render(w, r, jsonapi.NewCurrencyMigrationResponse(created).WithStatusCode(http.StatusCreated)); renderErr != nil {
		_ = internalServerError(w, r, renderErr)
		return
	}
}

// list returns the group's migration history newest-first. No
// pagination cursor in this PR — the daily cap (2/day) keeps the row
// count bounded. PR 4 may add cursor-based pagination if needed.
//
// @Summary List currency migrations for a group
// @Description Returns the group's full currency-migration history newest-first.
// @Tags currency-migrations
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} jsonapi.CurrencyMigrationsResponse "OK"
// @Failure 403 {object} jsonapi.Errors "Forbidden — non-admin"
// @Router /g/{groupSlug}/currency-migrations [get].
func (api *currencyMigrationsAPI) list(w http.ResponseWriter, r *http.Request) {
	rs := RegistrySetFromContext(r.Context())
	if rs == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	migrations, err := rs.CurrencyMigrationRegistry.List(r.Context())
	if err != nil {
		_ = internalServerError(w, r, err)
		return
	}

	// Newest-first ordering — the registry's underlying SCAN is
	// insertion-order dependent across backends, so sort here.
	sort.SliceStable(migrations, func(i, j int) bool {
		return migrations[i].CreatedAt.After(migrations[j].CreatedAt)
	})

	if renderErr := render.Render(w, r, jsonapi.NewCurrencyMigrationsResponse(migrations)); renderErr != nil {
		_ = internalServerError(w, r, renderErr)
		return
	}
}

// get returns a single migration row. The FE polls this endpoint
// while a migration is non-terminal so the lock UX can flip back to
// "ready" the moment TX2 commits or the recovery sweep flips the row
// to failed.
//
// @Summary Get a currency migration
// @Description Returns one currency-migration row by id.
// @Tags currency-migrations
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param id path string true "Currency migration ID"
// @Success 200 {object} jsonapi.CurrencyMigrationResponse "OK"
// @Failure 403 {object} jsonapi.Errors "Forbidden — non-admin"
// @Failure 404 {object} jsonapi.Errors "Not found"
// @Router /g/{groupSlug}/currency-migrations/{id} [get].
func (api *currencyMigrationsAPI) get(w http.ResponseWriter, r *http.Request) {
	rs := RegistrySetFromContext(r.Context())
	if rs == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	op, err := rs.CurrencyMigrationRegistry.Get(r.Context(), id)
	if err != nil {
		_ = renderEntityError(w, r, err)
		return
	}

	if renderErr := render.Render(w, r, jsonapi.NewCurrencyMigrationResponse(op)); renderErr != nil {
		_ = internalServerError(w, r, renderErr)
		return
	}
}

// GroupMigrationLockOptions parameterises requireGroupNotMigrating.
//
// FeatureEnabled mirrors Params.FeatureCurrencyMigration; when false
// the middleware is a no-op so the codepath stays dead until the
// operator flips the flag on (#202 §8). Wrapping the toggle in an
// option struct (rather than a bare bool flag-parameter) lets the
// middleware grow more knobs (per-route opt-out, custom code, etc.)
// without churning callers and silences revive's flag-parameter rule.
type GroupMigrationLockOptions struct {
	FeatureEnabled bool
}

// requireGroupNotMigrating is the lock middleware applied to commodity
// write paths (and the restore-create endpoint). On POST/PATCH/PUT/DELETE
// it queries CurrencyMigrationRegistry.InFlightForGroup; if a pending
// or running row exists, it returns 423 Locked with code
// currency_migration.locked and meta {migration_id, status}.
//
// GETs pass through unmodified — reads are not blocked (#202 §3.2).
func requireGroupNotMigrating(opts GroupMigrationLockOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if !opts.FeatureEnabled {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
				// fall through to the lock check
			default:
				next.ServeHTTP(w, r)
				return
			}

			rs := RegistrySetFromContext(r.Context())
			group := groupFromContext(r.Context())
			if rs == nil || group == nil {
				next.ServeHTTP(w, r)
				return
			}

			inFlight, err := rs.CurrencyMigrationRegistry.InFlightForGroup(r.Context(), group.ID)
			if err != nil {
				_ = internalServerError(w, r, err)
				return
			}
			if inFlight != nil {
				_ = lockedError(w, r,
					errors.New("group is locked while a currency migration is in progress"),
					codeCurrencyMigrationLocked,
					map[string]any{
						"migration_id": inFlight.ID,
						"status":       string(inFlight.Status),
					},
				)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// validateStartAttributes runs the from-mismatch / same-currency / rate
// guards on the request body. Returns false (and writes the response)
// when the body is invalid; true to continue.
func validateStartAttributes(w http.ResponseWriter, r *http.Request, attrs *jsonapi.CurrencyMigrationStartAttributes, group *models.LocationGroup) bool {
	if attrs.FromCurrency != group.GroupCurrency {
		_ = codedUnprocessableEntityError(w, r,
			fmt.Errorf("from_currency must equal group's current currency (%s)", group.GroupCurrency),
			codeCurrencyMigrationFromMismatch)
		return false
	}
	if attrs.FromCurrency == attrs.ToCurrency {
		_ = codedUnprocessableEntityError(w, r, currency.ErrSameCurrency, codeCurrencyMigrationSameCurrency)
		return false
	}
	if err := currency.ValidateRate(attrs.ExchangeRate); err != nil {
		_ = codedUnprocessableEntityError(w, r, err, codeCurrencyMigrationRateInvalid)
		return false
	}
	return true
}

// verifyStartToken handles steps 2+3: HMAC signature check (→ 422
// token_invalid) and expiry check (→ 409 preview_expired).
func verifyStartToken(w http.ResponseWriter, r *http.Request, rs *registry.Set, attrs *jsonapi.CurrencyMigrationStartAttributes, now time.Time) (registry.PreviewTokenInputs, bool) {
	tokenInputs, err := rs.CurrencyMigrationRegistry.VerifyPreviewToken(attrs.PreviewToken, now)
	switch {
	case errors.Is(err, registry.ErrPreviewTokenInvalid):
		_ = codedUnprocessableEntityError(w, r, err, codeCurrencyMigrationTokenInvalid)
		return registry.PreviewTokenInputs{}, false
	case errors.Is(err, registry.ErrPreviewTokenExpired):
		_ = codedConflictError(w, r, err, codeCurrencyMigrationPreviewExpired, nil)
		return registry.PreviewTokenInputs{}, false
	case err != nil:
		_ = internalServerError(w, r, err)
		return registry.PreviewTokenInputs{}, false
	}
	return tokenInputs, true
}

// checkTokenBindingsAndState handles steps 4+5: token bindings (group/from/to/rate
// match the body) and live state-hash drift detection. Both fail with
// 409 currency_migration.state_changed.
func checkTokenBindingsAndState(w http.ResponseWriter, r *http.Request, rs *registry.Set, group *models.LocationGroup, user *models.User, attrs *jsonapi.CurrencyMigrationStartAttributes, tokenInputs registry.PreviewTokenInputs) bool {
	if tokenInputs.GroupID != group.ID ||
		tokenInputs.FromCurrency != string(attrs.FromCurrency) ||
		tokenInputs.ToCurrency != string(attrs.ToCurrency) ||
		tokenInputs.Rate != canonicalRateString(attrs.ExchangeRate) {
		_ = codedConflictError(w, r, errors.New("preview token bindings do not match request"), codeCurrencyMigrationStateChanged, nil)
		return false
	}

	commodities, err := rs.CommodityRegistry.ListByGroup(r.Context(), user.TenantID, group.ID)
	if err != nil {
		_ = internalServerError(w, r, err)
		return false
	}
	currentHash := postgres.HashGroupState(len(commodities), sumCurrentString(commodities))
	if currentHash != tokenInputs.StateHash {
		_ = codedConflictError(w, r, errors.New("group state changed since preview"), codeCurrencyMigrationStateChanged, nil)
		return false
	}
	return true
}

// checkInFlightAndCap handles steps 6+7+8: existing migration (→ 409
// migration_in_progress), in-flight restore on the group (→ 409
// restore_in_progress), and the daily cap (→ 429 daily_cap_reached
// with retry_after_seconds meta).
func checkInFlightAndCap(w http.ResponseWriter, r *http.Request, rs *registry.Set, group *models.LocationGroup, now time.Time) bool {
	existing, err := rs.CurrencyMigrationRegistry.InFlightForGroup(r.Context(), group.ID)
	if err != nil {
		_ = internalServerError(w, r, err)
		return false
	}
	if existing != nil {
		_ = codedConflictError(w, r, errors.New("currency migration already in progress"), codeCurrencyMigrationInProgress, map[string]any{
			"migration_id": existing.ID,
			"status":       string(existing.Status),
		})
		return false
	}

	hasInFlightRestore, err := groupHasInFlightRestore(r.Context(), rs)
	if err != nil {
		_ = internalServerError(w, r, err)
		return false
	}
	if hasInFlightRestore {
		_ = codedConflictError(w, r, errors.New("a restore operation is in progress for this group"), codeCurrencyMigrationRestoreInProgress, nil)
		return false
	}

	completed, err := rs.CurrencyMigrationRegistry.CompletedTodayForGroup(r.Context(), group.ID, now)
	if err != nil {
		_ = internalServerError(w, r, err)
		return false
	}
	if completed >= currencyMigrationDailyCap {
		retryAfter := nextUTCMidnight(now).Sub(now)
		_ = codedTooManyRequestsError(w, r, errors.New("daily cap of currency migrations reached for this group"), codeCurrencyMigrationDailyCapReached, map[string]any{
			"retry_after_seconds": int(retryAfter.Seconds()),
		})
		return false
	}
	return true
}

// buildPreviewBody walks the commodities, applies the pure
// ApplyConversion to each, computes the totals, and assembles the
// JSON:API attributes. PreviewToken / PreviewExpiresAt / PreviewExpiresInSec
// are filled in by the caller after the registry signs the token.
func buildPreviewBody(commodities []*models.Commodity, from, to models.Currency, rate decimal.Decimal) (jsonapi.CurrencyMigrationPreviewBody, error) {
	body := jsonapi.CurrencyMigrationPreviewBody{
		FromCurrency:   from,
		ToCurrency:     to,
		ExchangeRate:   rate,
		CommodityCount: len(commodities),
		Diffs:          make([]jsonapi.CurrencyMigrationPreviewDiff, 0, len(commodities)),
	}

	allDiffs := make([]jsonapi.CurrencyMigrationPreviewDiff, 0, len(commodities))
	totalCurrentBefore := decimal.Zero
	totalCurrentAfter := decimal.Zero
	fillsCount := 0

	for _, c := range commodities {
		if c == nil {
			continue
		}
		result := currency.ApplyConversion(*c, from, to, rate)
		totalCurrentBefore = totalCurrentBefore.Add(result.Before.CurrentPrice)
		totalCurrentAfter = totalCurrentAfter.Add(result.After.CurrentPrice)
		if result.FillAcquisition {
			fillsCount++
		}
		allDiffs = append(allDiffs, jsonapi.CurrencyMigrationPreviewDiff{
			CommodityID:            c.GetID(),
			CommodityName:          c.Name,
			CurrentPriceBefore:     result.Before.CurrentPrice,
			CurrentPriceAfter:      result.After.CurrentPrice,
			OriginalPriceBefore:    result.Before.OriginalPrice,
			OriginalPriceAfter:     result.After.OriginalPrice,
			OriginalCurrencyBefore: result.Before.OriginalPriceCurrency,
			OriginalCurrencyAfter:  result.After.OriginalPriceCurrency,
		})
	}

	// Sort by absolute current-price delta descending so the response's
	// truncated diff list is the most informative for the FE preview
	// table. Stable so equal-delta rows keep insertion order.
	sort.SliceStable(allDiffs, func(i, j int) bool {
		di := allDiffs[i].CurrentPriceAfter.Sub(allDiffs[i].CurrentPriceBefore).Abs()
		dj := allDiffs[j].CurrentPriceAfter.Sub(allDiffs[j].CurrentPriceBefore).Abs()
		return di.GreaterThan(dj)
	})

	if len(allDiffs) > previewMaxDiffEntries {
		allDiffs = allDiffs[:previewMaxDiffEntries]
	}
	body.Diffs = allDiffs
	body.TotalCurrentBefore = totalCurrentBefore
	body.TotalCurrentAfter = totalCurrentAfter
	body.AcquisitionFills = fillsCount
	body.StateHash = postgres.HashGroupState(len(commodities), totalCurrentBefore.String())

	return body, nil
}

// sumCurrentString returns the canonical decimal string of the sum of
// CurrentPrice across all commodities. The state hash is derived from
// (count, this string) — both must be deterministic across replicas.
func sumCurrentString(commodities []*models.Commodity) string {
	sum := decimal.Zero
	for _, c := range commodities {
		if c == nil {
			continue
		}
		sum = sum.Add(c.CurrentPrice)
	}
	return sum.String()
}

// canonicalRateString returns the rate's String form that goes into
// the preview token's payload. Using Decimal.String (no scientific
// notation) ensures the FE can round-trip the rate via JSON without
// losing precision and makes the comparison on commit unambiguous.
func canonicalRateString(d decimal.Decimal) string {
	return d.String()
}

// nextUTCMidnight returns the next UTC midnight strictly after `now`.
// Used to compute meta.retry_after_seconds for the daily-cap 429.
func nextUTCMidnight(now time.Time) time.Time {
	u := now.UTC()
	return time.Date(u.Year(), u.Month(), u.Day()+1, 0, 0, 0, 0, time.UTC)
}

// groupHasInFlightRestore reports whether the current group has a
// restore_operation row in pending or running status. The user-aware
// RestoreOperationRegistry is RLS-scoped to the current group, so a
// plain List() returns the right slice.
func groupHasInFlightRestore(ctx context.Context, rs *registry.Set) (bool, error) {
	ops, err := rs.RestoreOperationRegistry.List(ctx)
	if err != nil {
		return false, err
	}
	for _, op := range ops {
		if op == nil {
			continue
		}
		if op.Status == models.RestoreStatusPending || op.Status == models.RestoreStatusRunning {
			return true, nil
		}
	}
	return false, nil
}

// setLocationGroupCurrencyMigrationID writes the in-flight migration
// id on the group row so the FE's lock UX can drive off
// LocationGroup.currency_migration_id without re-querying the
// migration registry. The recovery sweep clears the column when it
// flips a stuck running row to failed; PR 3's worker clears it on
// successful TX2 commit.
//
// Done via the LocationGroupRegistry update flow so the row's RLS
// context is consistent. Not transactional with the migration row —
// see start handler comments — but the migration row itself is the
// source of truth (the lock middleware reads InFlightForGroup, not
// this column).
func setLocationGroupCurrencyMigrationID(ctx context.Context, rs *registry.Set, groupID string, migrationID *string) error {
	if rs.LocationGroupRegistry == nil {
		return errors.New("location group registry unavailable")
	}
	group, err := rs.LocationGroupRegistry.Get(ctx, groupID)
	if err != nil {
		return err
	}
	group.CurrencyMigrationID = migrationID
	if _, err := rs.LocationGroupRegistry.Update(ctx, *group); err != nil {
		return err
	}
	return nil
}

// logAuditCurrencyMigration writes one row to audit_logs marking the
// start (or failure-to-start) of a migration. Best-effort — failures
// are logged in the AuditService and never propagate.
func (api *currencyMigrationsAPI) logAuditCurrencyMigration(r *http.Request, user *models.User, group *models.LocationGroup, action string, success bool, errMsg string) {
	if api.auditService == nil {
		return
	}
	userID := user.ID
	tenantID := user.TenantID
	var em *string
	if errMsg != "" {
		em = &errMsg
	}
	api.auditService.LogAuth(r.Context(), action, &userID, &tenantID, success, r, em)
	_ = group // group context lives in the entity_id field of a future audit-log expansion; today's AuditLogger signature is auth-shaped
}

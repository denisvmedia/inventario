package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

const commodityLoanCtxKey ctxValueKey = "commodity_loan"

func loanFromContext(ctx context.Context) *models.CommodityLoan {
	loan, ok := ctx.Value(commodityLoanCtxKey).(*models.CommodityLoan)
	if !ok {
		return nil
	}
	return loan
}

// loanCtx loads the loan referenced by the {loanID} URL param into the
// request context so the handler functions are short. Mirrors tagCtx.
func loanCtx() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			regSet := RegistrySetFromContext(r.Context())
			if regSet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}
			loanID := chi.URLParam(r, "loanID")
			loan, err := regSet.CommodityLoanRegistry.Get(r.Context(), loanID)
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			// Defence-in-depth: when this handler is mounted under a
			// /commodities/{commodityID}/loans/{loanID} prefix, the loan
			// must belong to that commodity. Mismatch surfaces as 404
			// (don't leak the existence of the cross-commodity loan).
			if commodityID := chi.URLParam(r, "commodityID"); commodityID != "" && loan.CommodityID != commodityID {
				renderEntityError(w, r, registry.ErrNotFound)
				return
			}
			ctx := context.WithValue(r.Context(), commodityLoanCtxKey, loan)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type commodityLoansAPI struct {
	factorySet  *registry.FactorySet
	loanService *services.CommodityLoanService
}

// listCommodityLoans returns all loans (open + closed) for the
// commodity in the URL path, most-recent-first. Drives the per-item
// Lend tab.
//
// @Summary List loans for a commodity
// @Description All loans (open + closed) for the commodity in the URL, most-recent-first.
// @Tags commodity_loans
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Success 200 {object} jsonapi.CommodityLoansResponse "OK"
// @Router /g/{groupSlug}/commodities/{commodityID}/loans [get].
func (api *commodityLoansAPI) listCommodityLoans(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	commodityID := chi.URLParam(r, "commodityID")
	loans, err := regSet.CommodityLoanRegistry.ListByCommodity(r.Context(), commodityID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCommodityLoansResponse(loans, len(loans))); err != nil {
		internalServerError(w, r, err)
	}
}

// createCommodityLoan opens a new loan for the commodity in the URL.
// Returns 409 when an open loan already exists.
//
// @Summary Start a loan
// @Description Open a new loan for the commodity. Returns 409 if one is already open.
// @Tags commodity_loans
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param loan body jsonapi.CommodityLoanRequest true "Loan attributes"
// @Success 201 {object} jsonapi.CommodityLoanResponse "Loan created"
// @Failure 409 {object} jsonapi.Errors "Commodity already has an open loan"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities/{commodityID}/loans [post].
func (api *commodityLoansAPI) createCommodityLoan(w http.ResponseWriter, r *http.Request) {
	commodityID := chi.URLParam(r, "commodityID")

	var input jsonapi.CommodityLoanRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	loan := models.CommodityLoan{
		CommodityID:     commodityID,
		BorrowerName:    input.Data.Attributes.BorrowerName,
		BorrowerContact: input.Data.Attributes.BorrowerContact,
		BorrowerNote:    input.Data.Attributes.BorrowerNote,
		LentAt:          input.Data.Attributes.LentAt,
		DueBackAt:       input.Data.Attributes.DueBackAt,
	}

	created, existing, crossHolding, err := api.loanService.StartLoan(r.Context(), loan)
	if err != nil {
		if errors.Is(err, services.ErrLoanAlreadyOpen) {
			conflictError(w, r,
				err,
				fmt.Errorf("commodity already has an open loan (loan_id=%s)", existing.ID),
			)
			return
		}
		if errors.Is(err, services.ErrCommodityAlreadyOut) && crossHolding != nil {
			conflictError(w, r,
				err,
				fmt.Errorf("commodity is already out (kind=%s, id=%s, party=%s)", crossHolding.Kind, crossHolding.ID, crossHolding.PartyName),
			)
			return
		}
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewCommodityLoanResponse(created).WithStatusCode(http.StatusCreated)); err != nil {
		internalServerError(w, r, err)
	}
}

// updateCommodityLoan patches a loan's mutable fields.
//
// @Summary Update a loan
// @Description Patch borrower name/contact/note and due_back_at. Sending due_back_at as JSON null clears it (open-ended loan); omitting the key leaves it unchanged.
// @Tags commodity_loans
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param loanID path string true "Loan ID"
// @Param loan body jsonapi.CommodityLoanUpdateRequest true "Loan patch payload"
// @Success 200 {object} jsonapi.CommodityLoanResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Loan not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities/{commodityID}/loans/{loanID} [patch].
func (api *commodityLoansAPI) updateCommodityLoan(w http.ResponseWriter, r *http.Request) {
	loan := loanFromContext(r.Context())
	if loan == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.CommodityLoanUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// PATCH semantics:
	//   - non-nil pointer / non-nil PDate → set to this value;
	//   - nil + clear flag false → leave unchanged;
	//   - explicit JSON null on `due_back_at` → clear the column
	//     (issue #1513). The presence-aware UnmarshalJSON on the
	//     request data flips ClearDueBackAt for that case.
	updated, err := api.loanService.UpdateLoan(r.Context(), loan.ID, services.LoanUpdate{
		BorrowerName:    input.Data.Attributes.BorrowerName,
		BorrowerContact: input.Data.Attributes.BorrowerContact,
		BorrowerNote:    input.Data.Attributes.BorrowerNote,
		DueBackAt:       input.Data.Attributes.DueBackAt,
		ClearDueBackAt:  input.Data.Attributes.ClearDueBackAt,
	})
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewCommodityLoanResponse(updated)); err != nil {
		internalServerError(w, r, err)
	}
}

// returnCommodityLoan closes out a loan. The body is optional; an
// empty body means "today, server clock".
//
// @Summary Mark a loan as returned
// @Description Close a loan. Defaults returned_at to today. 409 if already returned.
// @Tags commodity_loans
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param loanID path string true "Loan ID"
// @Param payload body jsonapi.CommodityLoanReturnRequest false "Optional explicit returned_at"
// @Success 200 {object} jsonapi.CommodityLoanResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Loan not found"
// @Failure 409 {object} jsonapi.Errors "Loan already returned"
// @Router /g/{groupSlug}/commodities/{commodityID}/loans/{loanID}/return [post].
func (api *commodityLoansAPI) returnCommodityLoan(w http.ResponseWriter, r *http.Request) {
	loan := loanFromContext(r.Context())
	if loan == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.CommodityLoanReturnRequest
	// Empty body is allowed — the BE defaults to "today, server clock".
	// Probe the body via r.Body (chunked requests have ContentLength
	// == -1, so a `> 0` gate would silently drop a client-supplied
	// returned_at). io.EOF / "no data" decodes are tolerated by
	// CommodityLoanReturnRequest.Bind, which short-circuits when
	// Data is nil.
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if err := render.Bind(r, &input); err != nil {
			unprocessableEntityError(w, r, err)
			return
		}
	}
	var returnedAt models.PDate
	if input.Data != nil {
		returnedAt = input.Data.Attributes.ReturnedAt
	}

	updated, err := api.loanService.MarkReturned(r.Context(), loan.ID, returnedAt)
	if err != nil {
		if errors.Is(err, services.ErrLoanAlreadyReturned) {
			conflictError(w, r, err, err)
			return
		}
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewCommodityLoanResponse(updated)); err != nil {
		internalServerError(w, r, err)
	}
}

// deleteCommodityLoan permanently removes a loan row. Used to undo a
// mistaken Lend (the FE shows a "delete" affordance only on rows the
// user just created, to keep the audit trail clean for the rest).
//
// @Summary Delete a loan
// @Description Hard-delete a loan row.
// @Tags commodity_loans
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param loanID path string true "Loan ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.Errors "Loan not found"
// @Router /g/{groupSlug}/commodities/{commodityID}/loans/{loanID} [delete].
func (api *commodityLoansAPI) deleteCommodityLoan(w http.ResponseWriter, r *http.Request) {
	loan := loanFromContext(r.Context())
	if loan == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	if err := regSet.CommodityLoanRegistry.Delete(r.Context(), loan.ID); err != nil {
		renderEntityError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listGroupLoans returns the group-wide loan list. Drives the
// dedicated /lent page.
//
// @Summary List group-wide loans
// @Description List loans across the current group with optional state filter.
// @Tags commodity_loans
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param state query string false "Filter by state" Enums(all,open,overdue,returned) default(all)
// @Param page query int false "Page number (1-based)" default(1)
// @Param per_page query int false "Items per page" default(50)
// @Success 200 {object} jsonapi.CommodityLoanListResponse "OK"
// @Router /g/{groupSlug}/loans [get].
func (api *commodityLoansAPI) listGroupLoans(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))
	offset := (page - 1) * perPage

	state := registry.LoanState(q.Get("state"))
	if !state.IsValid() {
		state = registry.LoanStateAll
	}

	loans, total, err := regSet.CommodityLoanRegistry.ListPaginated(r.Context(), offset, perPage, registry.LoanListOptions{
		State: state,
	})
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Batch-fetch the parent commodities for every loan on the page in a
	// single round-trip (issue #1512). Pre-#1512 the loop above did one
	// Get per unique CommodityID; bounded by per_page (≤50) but still up
	// to 50 round-trips per page request. The id slice is deduped so
	// repeated commodity references on the page collapse to a single
	// IN-list entry, and the GetMany contract drops missing /
	// RLS-hidden rows silently — the FE renders those rows without the
	// commodity ref (rather than 500ing) on a cascaded-away commodity.
	uniqueIDs := uniqueCommodityIDsForLoans(loans)
	commoditiesByID := make(map[string]*models.Commodity, len(uniqueIDs))
	if len(uniqueIDs) > 0 {
		fetched, cerr := regSet.CommodityRegistry.GetMany(r.Context(), uniqueIDs)
		if cerr != nil {
			renderEntityError(w, r, cerr)
			return
		}
		for _, c := range fetched {
			commoditiesByID[c.ID] = c
		}
	}

	setPaginationHeaders(w, page, perPage, total)
	if err := render.Render(w, r, jsonapi.NewCommodityLoanListResponse(loans, total, commoditiesByID)); err != nil {
		internalServerError(w, r, err)
	}
}

// uniqueCommodityIDsForLoans collects each loan's CommodityID once,
// preserving first-seen order. The order itself is incidental for the
// downstream IN-list query (GetMany result order is unspecified anyway),
// but a deterministic walk makes the function easy to reason about in
// tests and gives stable inputs to the SQL planner across requests with
// the same shape. Empty / blank ids are dropped — a malformed loan row
// shouldn't widen the WHERE IN list with a useless "" entry.
func uniqueCommodityIDsForLoans(loans []*models.CommodityLoan) []string {
	if len(loans) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(loans))
	ids := make([]string, 0, len(loans))
	for _, l := range loans {
		if l == nil || l.CommodityID == "" {
			continue
		}
		if _, ok := seen[l.CommodityID]; ok {
			continue
		}
		seen[l.CommodityID] = struct{}{}
		ids = append(ids, l.CommodityID)
	}
	return ids
}

// getGroupLoanCounts returns per-commodity open-loan counts for a list
// of commodity ids. Backs the list-page "lent out" badge in a single
// round-trip.
//
// @Summary Get open-loan counts by commodity
// @Description Map of commodity_id → open-loan count for a list of commodities.
// @Tags commodity_loans
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodity_id query []string true "Repeatable commodity IDs to look up" collectionFormat(multi)
// @Success 200 {object} jsonapi.CommodityLoanCountsResponse "OK"
// @Router /g/{groupSlug}/loans/counts [get].
func (api *commodityLoansAPI) getGroupLoanCounts(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	ids := r.URL.Query()["commodity_id"]
	counts, err := regSet.CommodityLoanRegistry.CountOpenByCommodity(r.Context(), ids)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCommodityLoanCountsResponse(counts)); err != nil {
		internalServerError(w, r, err)
	}
}

// CommodityLoans returns the chi sub-router mounted under the per-
// commodity prefix `/commodities/{commodityID}/loans`. All routes
// require the commodity context already loaded by the parent
// commodities router.
func CommodityLoans(params Params) func(r chi.Router) {
	api := &commodityLoansAPI{
		factorySet:  params.FactorySet,
		loanService: services.NewCommodityLoanService(params.FactorySet),
	}
	return func(r chi.Router) {
		r.Get("/", api.listCommodityLoans)
		r.Post("/", api.createCommodityLoan)
		r.Route("/{loanID}", func(r chi.Router) {
			r.Use(loanCtx())
			r.Patch("/", api.updateCommodityLoan)
			r.Delete("/", api.deleteCommodityLoan)
			r.Post("/return", api.returnCommodityLoan)
		})
	}
}

// GroupLoans returns the chi sub-router for the group-wide /loans
// surface — list + bulk counts.
func GroupLoans(params Params) func(r chi.Router) {
	api := &commodityLoansAPI{
		factorySet:  params.FactorySet,
		loanService: services.NewCommodityLoanService(params.FactorySet),
	}
	return func(r chi.Router) {
		r.Get("/", api.listGroupLoans)
		r.Get("/counts", api.getGroupLoanCounts)
	}
}

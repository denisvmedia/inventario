package apiserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

const sellerCtxKey ctxValueKey = "seller"

// sellerNameTakenCode is the JSON:API error code of a 409 on a seller
// create or rename whose name another seller of the group already has.
const sellerNameTakenCode = "seller.name_taken"

func sellerFromContext(ctx context.Context) *models.Seller {
	seller, ok := ctx.Value(sellerCtxKey).(*models.Seller)
	if !ok {
		return nil
	}
	return seller
}

func sellerCtx() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			regSet := RegistrySetFromContext(r.Context())
			if regSet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}
			seller, err := regSet.SellerRegistry.Get(r.Context(), chi.URLParam(r, "sellerID"))
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), sellerCtxKey, seller)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type sellersAPI struct{}

// renderSellerWriteError maps a seller write failure: field errors to 422,
// a duplicate name to 409, and the rest through renderEntityError.
func renderSellerWriteError(w http.ResponseWriter, r *http.Request, err error) {
	if _, ok := errors.AsType[validation.Errors](err); ok {
		unprocessableEntityError(w, r, err)
		return
	}
	if errors.Is(err, registry.ErrAlreadyExists) {
		codedConflictError(w, r, errors.New("a seller with this name already exists"), sellerNameTakenCode, nil)
		return
	}
	renderEntityError(w, r, err)
}

// listSellers lists the group's sellers with their item counts.
// @Summary List sellers
// @Description The sellers of the group sorted by name, each with the number of items bought from it (drafts and inactive items included, items in Trash left out).
// @Tags sellers
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param q query string false "Keep sellers whose name contains this text, ignoring case"
// @Success 200 {object} jsonapi.SellersResponse "OK"
// @Router /g/{groupSlug}/sellers [get].
func (*sellersAPI) listSellers(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	sellers, err := regSet.SellerRegistry.List(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	sellers = registry.FilterSellers(sellers, r.URL.Query().Get("q"))
	counts, err := regSet.SellerRegistry.ItemCounts(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewSellersResponse(sellers, counts)); err != nil {
		internalServerError(w, r, err)
	}
}

// getSeller returns one seller with its item count.
// @Summary Get a seller
// @Tags sellers
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param sellerID path string true "Seller ID"
// @Success 200 {object} jsonapi.SellerResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Seller not found"
// @Router /g/{groupSlug}/sellers/{sellerID} [get].
func (*sellersAPI) getSeller(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	seller := sellerFromContext(r.Context())
	regSet := RegistrySetFromContext(r.Context())
	if seller == nil || regSet == nil {
		http.Error(w, "Seller not found in context", http.StatusInternalServerError)
		return
	}
	counts, err := regSet.SellerRegistry.ItemCounts(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewSellerResponse(seller, counts[seller.ID])); err != nil {
		internalServerError(w, r, err)
	}
}

// createSeller adds a seller to the group.
// @Summary Create a seller
// @Description The name is required and unique in the group, ignoring case and runs of spaces. The website, when set, is an absolute http(s) URL.
// @Tags sellers
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param seller body jsonapi.SellerRequest true "Seller object"
// @Success 201 {object} jsonapi.SellerResponse "Seller created"
// @Failure 409 {object} jsonapi.Errors "Another seller of the group has this name"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/sellers [post].
func (*sellersAPI) createSeller(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	var input jsonapi.SellerRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes
	seller := models.Seller{Name: attrs.Name, Website: attrs.Website, Contact: attrs.Contact, Note: attrs.Note}
	seller.Normalize()
	if err := seller.ValidateFields(r.Context()); err != nil {
		renderSellerWriteError(w, r, err)
		return
	}
	created, err := regSet.SellerRegistry.Create(r.Context(), seller)
	if err != nil {
		renderSellerWriteError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewSellerResponse(created, 0).WithStatusCode(http.StatusCreated)); err != nil {
		internalServerError(w, r, err)
	}
}

// updateSeller changes a seller's fields.
// @Summary Update a seller
// @Description Attributes left out keep their value. The merged seller is validated as on create.
// @Tags sellers
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param sellerID path string true "Seller ID"
// @Param seller body jsonapi.SellerUpdateRequest true "Seller patch"
// @Success 200 {object} jsonapi.SellerResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Seller not found"
// @Failure 409 {object} jsonapi.Errors "Another seller of the group has this name"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/sellers/{sellerID} [patch].
func (*sellersAPI) updateSeller(w http.ResponseWriter, r *http.Request) {
	seller := sellerFromContext(r.Context())
	regSet := RegistrySetFromContext(r.Context())
	if seller == nil || regSet == nil {
		http.Error(w, "Seller not found in context", http.StatusInternalServerError)
		return
	}
	var input jsonapi.SellerUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	if input.Data.ID != "" && input.Data.ID != seller.ID {
		unprocessableEntityError(w, r, errors.New("ID in URL does not match ID in request body"))
		return
	}
	merged := *seller
	input.Data.Attributes.Apply(&merged)
	merged.Normalize()
	if err := merged.ValidateFields(r.Context()); err != nil {
		renderSellerWriteError(w, r, err)
		return
	}
	updated, err := regSet.SellerRegistry.Update(r.Context(), merged)
	if err != nil {
		renderSellerWriteError(w, r, err)
		return
	}
	counts, err := regSet.SellerRegistry.ItemCounts(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewSellerResponse(updated, counts[updated.ID])); err != nil {
		internalServerError(w, r, err)
	}
}

// deleteSeller removes a seller. Its items stay and lose the seller.
// @Summary Delete a seller
// @Description Deletes the seller. The items bought from it are kept and no longer name a seller.
// @Tags sellers
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param sellerID path string true "Seller ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Seller not found"
// @Router /g/{groupSlug}/sellers/{sellerID} [delete].
func (*sellersAPI) deleteSeller(w http.ResponseWriter, r *http.Request) {
	seller := sellerFromContext(r.Context())
	regSet := RegistrySetFromContext(r.Context())
	if seller == nil || regSet == nil {
		http.Error(w, "Seller not found in context", http.StatusInternalServerError)
		return
	}
	if err := regSet.SellerRegistry.Delete(r.Context(), seller.ID); err != nil {
		renderEntityError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validateCommoditySeller checks that the seller an item names, when it
// names one, belongs to the group. An unknown seller is a field error on
// seller_id rather than a 404, since the item itself was found. The error
// sits at data.attributes.seller_id, where the request validation puts its
// field errors, so a client reads both the same way.
func validateCommoditySeller(ctx context.Context, sellers registry.SellerRegistry, sellerID *string) error {
	if sellerID == nil || *sellerID == "" {
		return nil
	}
	if _, err := sellers.Get(ctx, *sellerID); err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			return validation.Errors{"data": validation.Errors{"attributes": validation.Errors{
				"seller_id": errors.New("no seller with this ID in the group"),
			}}}
		}
		return err
	}
	return nil
}

// Sellers returns the chi sub-router for /sellers (#2804).
func Sellers() func(r chi.Router) {
	api := &sellersAPI{}
	return func(r chi.Router) {
		r.Get("/", api.listSellers)
		r.Post("/", api.createSeller)
		r.Route("/{sellerID}", func(r chi.Router) {
			r.Use(sellerCtx())
			r.Get("/", api.getSeller)
			r.Patch("/", api.updateSeller)
			r.Delete("/", api.deleteSeller)
		})
	}
}

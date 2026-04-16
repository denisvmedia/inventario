package apiserver

import (
	"context"
	"errors"
	"mime"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver/internal/downloadutils"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services"
)

type locationsAPI struct {
	uploadLocation     string
	fileService        *services.FileService
	fileSigningService *services.FileSigningService
}

// generateSignedURLsForFiles generates signed URLs for a list of files.
func (api *locationsAPI) generateSignedURLsForFiles(ctx context.Context, files []*models.FileEntity) map[string]jsonapi.URLData {
	signedUrls := make(map[string]jsonapi.URLData)
	user := appctx.UserFromContext(ctx)
	if user == nil {
		return signedUrls
	}

	for _, file := range files {
		originalURL, thumbnails, err := api.fileSigningService.GenerateSignedURLsWithThumbnails(file, user.ID)
		if err != nil {
			continue
		}
		signedUrls[file.ID] = jsonapi.URLData{
			URL:        originalURL,
			Thumbnails: thumbnails,
		}
	}

	return signedUrls
}

// listLocations lists all locations with pagination.
// @Summary List locations
// @Description get locations
// @Tags locations
// @Accept json-api
// @Produce json-api
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 50, max 100)"
// @Success 200 {object} jsonapi.LocationsResponse "OK"
// @Router /locations [get].
func (api *locationsAPI) listLocations(w http.ResponseWriter, r *http.Request) {
	// Get user-aware registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	locReg := registrySet.LocationRegistry

	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))
	offset := (page - 1) * perPage

	locations, total, err := locReg.ListPaginated(r.Context(), offset, perPage)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	setPaginationHeaders(w, page, perPage, total)

	if err := render.Render(w, r, jsonapi.NewLocationsResponse(locations, total, page, perPage)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getLocation gets a location by ID.
// @Summary Get a location
// @Description get location by ID
// @Tags locations
// @Accept  json-api
// @Produce  json-api
// @Param id path string true "Location ID"
// @Success 200 {object} jsonapi.LocationResponse "OK"
// @Router /locations/{id} [get].
func (api *locationsAPI) getLocation(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	// Get user-aware registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	locReg := registrySet.LocationRegistry

	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	areas, err := locReg.GetAreas(r.Context(), location.ID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	respLocation := &jsonapi.Location{
		Location: location,
		Areas:    areas,
	}

	if err := render.Render(w, r, jsonapi.NewLocationResponse(respLocation)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// Create a new location
// @Summary Create a new location
// @Description add by location data
// @Tags locations
// @Accept json-api
// @Produce json-api
// @Param location body jsonapi.LocationRequest true "Location object"
// @Success 201 {object} jsonapi.LocationResponse "Location created"
// @Failure 404 {object} jsonapi.Errors "Location not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /locations [post].
func (api *locationsAPI) createLocation(w http.ResponseWriter, r *http.Request) {
	// Get user-aware registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	var input jsonapi.LocationRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Extract user from authenticated request context
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "User context required", http.StatusInternalServerError)
		return
	}

	location := *input.Data.Attributes
	if location.TenantID == "" {
		location.TenantID = user.TenantID
	}

	// Use WithUser to ensure proper user context and validation
	ctx := appctx.WithUser(r.Context(), user)
	locationReg := registrySet.LocationRegistry
	createdLocation, err := locationReg.Create(ctx, location)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	areas, err := locationReg.GetAreas(ctx, createdLocation.ID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	respLocation := &jsonapi.Location{
		Location: createdLocation,
		Areas:    areas,
	}

	resp := jsonapi.NewLocationResponse(respLocation).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteLocation deletes a location by ID.
// @Summary Delete a location
// @Description Delete by location ID
// @Tags locations
// @Accept  json-api
// @Produce  json-api
// @Param id path string true "Location ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Location not found"
// @Router /locations/{id} [delete].
func (api *locationsAPI) deleteLocation(w http.ResponseWriter, r *http.Request) {
	// Get user-aware registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	// Use WithCurrentUser to ensure proper user context and validation
	ctx := r.Context()
	locationReg := registrySet.LocationRegistry
	err := locationReg.Delete(ctx, location.ID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// updateLocation updates a location.
// @Summary Update a location
// @Description Update by location data
// @Tags locations
// @Accept json-api
// @Produce json-api
// @Param id path string true "Location ID"
// @Param location body jsonapi.LocationRequest true "Location object"
// @Success 200 {object} jsonapi.LocationResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Location not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /locations/{id} [put].
func (api *locationsAPI) updateLocation(w http.ResponseWriter, r *http.Request) {
	// Get user-aware registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.LocationRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	if location.ID != input.Data.ID {
		unprocessableEntityError(w, r, nil)
		return
	}

	// Preserve tenant_id and user_id from the existing location
	// This ensures the foreign key constraints are satisfied during updates
	updateData := *input.Data.Attributes
	if updateData.TenantID == "" {
		updateData.TenantID = location.TenantID
	}

	// Use WithCurrentUser to ensure proper user context and validation
	ctx := r.Context()
	locationReg := registrySet.LocationRegistry
	newLocation, err := locationReg.Update(ctx, updateData)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	areas, err := locationReg.GetAreas(r.Context(), location.ID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	respLocation := &jsonapi.Location{
		Location: newLocation,
		Areas:    areas,
	}

	resp := jsonapi.NewLocationResponse(respLocation).WithStatusCode(http.StatusOK)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// listLocationImages returns all images linked to the given location.
func (api *locationsAPI) listLocationImages(w http.ResponseWriter, r *http.Request) {
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		internalServerError(w, r, errors.New("registry set not found in context"))
		return
	}

	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	files, err := registrySet.FileRegistry.ListByLinkedEntityAndMeta(r.Context(), "location", location.ID, "images")
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	signedUrls := api.generateSignedURLsForFiles(r.Context(), files)
	resp := jsonapi.NewFilesResponseWithSignedUrls(files, len(files), signedUrls)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// listLocationFiles returns all files linked to the given location.
func (api *locationsAPI) listLocationFiles(w http.ResponseWriter, r *http.Request) {
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		internalServerError(w, r, errors.New("registry set not found in context"))
		return
	}

	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	files, err := registrySet.FileRegistry.ListByLinkedEntityAndMeta(r.Context(), "location", location.ID, "files")
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	signedUrls := api.generateSignedURLsForFiles(r.Context(), files)
	resp := jsonapi.NewFilesResponseWithSignedUrls(files, len(files), signedUrls)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// deleteLocationImage deletes an image associated with the given location.
func (api *locationsAPI) deleteLocationImage(w http.ResponseWriter, r *http.Request) {
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		internalServerError(w, r, errors.New("registry set not found in context"))
		return
	}

	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	imageID := chi.URLParam(r, "imageID")
	file, err := registrySet.FileRegistry.Get(r.Context(), imageID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if file.LinkedEntityID != location.ID || file.LinkedEntityType != "location" || file.LinkedEntityMeta != "images" {
		notFound(w, r)
		return
	}

	if err := api.fileService.DeleteFileWithPhysical(r.Context(), file.ID); err != nil {
		renderEntityError(w, r, err)
		return
	}

	render.NoContent(w, r)
}

// deleteLocationFile deletes a file associated with the given location.
func (api *locationsAPI) deleteLocationFile(w http.ResponseWriter, r *http.Request) {
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		internalServerError(w, r, errors.New("registry set not found in context"))
		return
	}

	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	fileID := chi.URLParam(r, "fileID")
	file, err := registrySet.FileRegistry.Get(r.Context(), fileID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if file.LinkedEntityID != location.ID || file.LinkedEntityType != "location" || file.LinkedEntityMeta != "files" {
		notFound(w, r)
		return
	}

	if err := api.fileService.DeleteFileWithPhysical(r.Context(), file.ID); err != nil {
		renderEntityError(w, r, err)
		return
	}

	render.NoContent(w, r)
}

// getLocationImageData downloads the raw image data for the given location.
func (api *locationsAPI) getLocationImageData(w http.ResponseWriter, r *http.Request) {
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		internalServerError(w, r, errors.New("registry set not found in context"))
		return
	}

	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	imageID := chi.URLParam(r, "imageID")
	file, err := registrySet.FileRegistry.Get(r.Context(), imageID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if file.LinkedEntityID != location.ID || file.LinkedEntityType != "location" || file.LinkedEntityMeta != "images" {
		notFound(w, r)
		return
	}

	api.streamFile(w, r, file)
}

// getLocationFileData downloads the raw file data for the given location.
func (api *locationsAPI) getLocationFileData(w http.ResponseWriter, r *http.Request) {
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		internalServerError(w, r, errors.New("registry set not found in context"))
		return
	}

	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	fileID := chi.URLParam(r, "fileID")
	file, err := registrySet.FileRegistry.Get(r.Context(), fileID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if file.LinkedEntityID != location.ID || file.LinkedEntityType != "location" || file.LinkedEntityMeta != "files" {
		notFound(w, r)
		return
	}

	api.streamFile(w, r, file)
}

// streamFile streams the content of a FileEntity to the response.
func (api *locationsAPI) streamFile(w http.ResponseWriter, r *http.Request, file *models.FileEntity) {
	// Use OriginalPath for blob storage lookup; Path+Ext is only the user-visible download filename.
	storagePath := file.OriginalPath

	attrs, err := downloadutils.GetFileAttributes(r.Context(), api.uploadLocation, storagePath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	reader, err := api.getDownloadFile(r.Context(), storagePath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	defer reader.Close()

	contentType := mime.TypeByExtension(file.Ext)
	downloadutils.SetStreamingHeaders(w, contentType, attrs.Size, file.Path+file.Ext)
	if err := downloadutils.CopyFileInChunks(w, reader); err != nil {
		internalServerError(w, r, err)
	}
}

// getDownloadFile opens and returns a reader for the file stored at originalPath.
func (api *locationsAPI) getDownloadFile(ctx context.Context, originalPath string) (*blob.Reader, error) {
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	return b.NewReader(context.Background(), originalPath, nil)
}

// Locations returns a Chi router function that registers all location-related routes.
func Locations(params Params) func(r chi.Router) {
	api := &locationsAPI{
		uploadLocation:     params.UploadLocation,
		fileService:        services.NewFileService(params.FactorySet, params.UploadLocation),
		fileSigningService: services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration),
	}
	return func(r chi.Router) {
		r.With(paginate).Get("/", api.listLocations) // GET /locations
		r.Route("/{locationID}", func(r chi.Router) {
			r.Use(locationCtx(nil))           // locationCtx will get registry from context
			r.Get("/", api.getLocation)       // GET /locations/123
			r.Put("/", api.updateLocation)    // PUT /locations/123
			r.Delete("/", api.deleteLocation) // DELETE /locations/123

			r.Get("/images", api.listLocationImages)                                       // GET /locations/123/images
			r.Get("/files", api.listLocationFiles)                                         // GET /locations/123/files
			r.Delete("/images/{imageID}", api.deleteLocationImage)                         // DELETE /locations/123/images/456
			r.Delete("/files/{fileID}", api.deleteLocationFile)                            // DELETE /locations/123/files/456
			r.Get("/images/{imageID}{imageExt:[.][a-zA-Z0-9]+}", api.getLocationImageData) // GET /locations/123/images/456.png
			r.Get("/files/{fileID}{fileExt:[.][a-zA-Z0-9]+}", api.getLocationFileData)     // GET /locations/123/files/456.pdf
		})
		r.Post("/", api.createLocation) // POST /locations
	}
}

package apiserver

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/restore"
)

type exportRestoresAPI struct {
	registrySet    *registry.Set
	uploadLocation string
}

// listExportRestores lists all restore operations for an export.
// @Summary List export restore operations
// @Description get restore operations for an export
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param id path string true "Export ID"
// @Success 200 {object} jsonapi.RestoreOperationsResponse "OK"
// @Router /exports/{id}/restores [get].
func (api *exportRestoresAPI) listExportRestores(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "id")
	if exportID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Verify export exists
	_, err := api.registrySet.ExportRegistry.Get(r.Context(), exportID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	restoreOperations, err := api.registrySet.RestoreOperationRegistry.ListByExport(r.Context(), exportID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewRestoreOperationsResponse(restoreOperations)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getExportRestore returns a specific restore operation for an export.
// @Summary Get export restore operation
// @Description get restore operation by ID for an export
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param id path string true "Export ID"
// @Param restoreId path string true "Restore Operation ID"
// @Success 200 {object} jsonapi.RestoreOperationResponse "OK"
// @Failure 404 {object} jsonapi.ErrorResponse "Not Found"
// @Router /exports/{id}/restores/{restoreId} [get].
func (api *exportRestoresAPI) getExportRestore(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "id")
	restoreID := chi.URLParam(r, "restoreId")

	if exportID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	if restoreID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Verify export exists
	_, err := api.registrySet.ExportRegistry.Get(r.Context(), exportID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	restoreOperation, err := api.registrySet.RestoreOperationRegistry.Get(r.Context(), restoreID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify the restore operation belongs to this export
	if restoreOperation.ExportID != exportID {
		notFound(w, r)
		return
	}

	// Load steps for this restore operation
	steps, err := api.registrySet.RestoreStepRegistry.ListByRestoreOperation(r.Context(), restoreID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	// Convert steps to the format expected by the model
	restoreOperation.Steps = make([]models.RestoreStep, len(steps))
	for i, step := range steps {
		restoreOperation.Steps[i] = *step
	}

	if err := render.Render(w, r, jsonapi.NewRestoreOperationResponse(restoreOperation)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// createExportRestore creates a new restore operation for an export.
// @Summary Create export restore operation
// @Description create a new restore operation for an export
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param id path string true "Export ID"
// @Param request body jsonapi.RestoreOperationCreateRequest true "Restore operation data"
// @Success 201 {object} jsonapi.RestoreOperationResponse "Created"
// @Failure 400 {object} jsonapi.ErrorResponse "Bad Request"
// @Failure 404 {object} jsonapi.ErrorResponse "Not Found"
// @Router /exports/{id}/restores [post].
func (api *exportRestoresAPI) createExportRestore(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "id")
	if exportID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Verify export exists and is completed
	export, err := api.registrySet.ExportRegistry.Get(r.Context(), exportID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if export.Status != models.ExportStatusCompleted {
		badRequest(w, r, ErrInvalidContentType)
		return
	}

	data := &jsonapi.RestoreOperationCreateRequest{}
	if err := render.Bind(r, data); err != nil {
		renderEntityError(w, r, err)
		return
	}

	restoreOperation := *data.Data.Attributes
	restoreOperation.ExportID = exportID

	// Set created date (we do not accept it from the client)
	restoreOperation.CreatedDate = models.PNow()

	createdRestoreOperation, err := api.registrySet.RestoreOperationRegistry.Create(r.Context(), restoreOperation)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Start restore processing in background with proper context
	// Use a background context to ensure the restore continues even if the request is cancelled
	go func() {
		// Create a new context for the background operation
		bgCtx := context.Background()
		api.processRestore(bgCtx, createdRestoreOperation.ID, export.FilePath, restoreOperation.Options)
	}()

	// Return immediately with the created restore operation
	w.WriteHeader(http.StatusCreated)
	if err := render.Render(w, r, jsonapi.NewRestoreOperationResponse(createdRestoreOperation)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteExportRestore deletes a restore operation for an export.
// @Summary Delete export restore operation
// @Description delete a restore operation for an export
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param id path string true "Export ID"
// @Param restoreId path string true "Restore Operation ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.ErrorResponse "Not Found"
// @Router /exports/{id}/restores/{restoreId} [delete].
func (api *exportRestoresAPI) deleteExportRestore(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "id")
	restoreID := chi.URLParam(r, "restoreId")

	if exportID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	if restoreID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Verify export exists
	_, err := api.registrySet.ExportRegistry.Get(r.Context(), exportID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify the restore operation exists and belongs to this export
	restoreOperation, err := api.registrySet.RestoreOperationRegistry.Get(r.Context(), restoreID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if restoreOperation.ExportID != exportID {
		notFound(w, r)
		return
	}

	// Don't allow deletion of running restore operations
	if restoreOperation.Status == models.RestoreStatusRunning {
		badRequest(w, r, ErrInvalidContentType)
		return
	}

	err = api.registrySet.RestoreOperationRegistry.Delete(r.Context(), restoreID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// processRestore processes a restore operation in the background with detailed logging
func (api *exportRestoresAPI) processRestore(ctx context.Context, restoreOperationID, exportFilePath string, options models.RestoreOptions) {
	// Get the restore operation
	restoreOperation, err := api.registrySet.RestoreOperationRegistry.Get(ctx, restoreOperationID)
	if err != nil {
		api.markRestoreFailed(ctx, restoreOperationID, fmt.Sprintf("failed to get restore operation: %v", err))
		return
	}

	// Update status to running
	restoreOperation.Status = models.RestoreStatusRunning
	restoreOperation.StartedDate = models.PNow()
	_, err = api.registrySet.RestoreOperationRegistry.Update(ctx, *restoreOperation)
	if err != nil {
		api.markRestoreFailed(ctx, restoreOperationID, fmt.Sprintf("failed to update restore status: %v", err))
		return
	}

	// Create initial restore steps
	api.createRestoreStep(ctx, restoreOperationID, "Initializing restore", models.RestoreStepResultInProgress, "")

	// Open the export file
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		api.markRestoreFailed(ctx, restoreOperationID, fmt.Sprintf("failed to open blob bucket: %v", err))
		return
	}
	defer b.Close()

	reader, err := b.NewReader(ctx, exportFilePath, nil)
	if err != nil {
		api.markRestoreFailed(ctx, restoreOperationID, fmt.Sprintf("failed to open export file: %v", err))
		return
	}
	defer reader.Close()

	// Update step to processing
	api.updateRestoreStep(ctx, restoreOperationID, "Initializing restore", models.RestoreStepResultSuccess, "")
	api.createRestoreStep(ctx, restoreOperationID, "Reading XML file", models.RestoreStepResultInProgress, "")

	// Convert options to restore service format
	restoreOptions := restore.RestoreOptions{
		Strategy:        restore.RestoreStrategy(options.Strategy),
		IncludeFileData: options.IncludeFileData,
		DryRun:         options.DryRun,
		BackupExisting: options.BackupExisting,
	}

	// Update step to processing
	api.updateRestoreStep(ctx, restoreOperationID, "Reading XML file", models.RestoreStepResultSuccess, "")

	// Create restore service with detailed logging callback
	service := restore.NewRestoreService(api.registrySet, api.uploadLocation)

	// Process with detailed logging
	stats, err := api.processRestoreWithDetailedLogging(ctx, restoreOperationID, service, reader, restoreOptions)
	if err != nil {
		api.markRestoreFailed(ctx, restoreOperationID, fmt.Sprintf("restore failed: %v", err))
		return
	}

	// Create final step
	api.createRestoreStep(ctx, restoreOperationID, "Finalizing restore", models.RestoreStepResultSuccess, "")

	// Update restore operation with final statistics and status
	restoreOperation.Status = models.RestoreStatusCompleted
	restoreOperation.CompletedDate = models.PNow()
	restoreOperation.LocationCount = stats.LocationCount
	restoreOperation.AreaCount = stats.AreaCount
	restoreOperation.CommodityCount = stats.CommodityCount
	restoreOperation.ImageCount = stats.ImageCount
	restoreOperation.InvoiceCount = stats.InvoiceCount
	restoreOperation.ManualCount = stats.ManualCount
	restoreOperation.BinaryDataSize = stats.BinaryDataSize
	restoreOperation.ErrorCount = stats.ErrorCount

	_, err = api.registrySet.RestoreOperationRegistry.Update(ctx, *restoreOperation)
	if err != nil {
		// Log error but don't fail the restore since it actually succeeded
		fmt.Printf("Failed to update restore operation with final stats: %v\n", err)
	}
}

// markRestoreFailed marks a restore operation as failed with an error message
func (api *exportRestoresAPI) markRestoreFailed(ctx context.Context, restoreOperationID, errorMessage string) {
	restoreOperation, err := api.registrySet.RestoreOperationRegistry.Get(ctx, restoreOperationID)
	if err != nil {
		fmt.Printf("Failed to get restore operation for error marking: %v\n", err)
		return
	}

	restoreOperation.Status = models.RestoreStatusFailed
	restoreOperation.CompletedDate = models.PNow()
	restoreOperation.ErrorMessage = errorMessage

	_, err = api.registrySet.RestoreOperationRegistry.Update(ctx, *restoreOperation)
	if err != nil {
		fmt.Printf("Failed to update restore operation with error: %v\n", err)
	}

	// Create error step
	api.createRestoreStep(ctx, restoreOperationID, "Restore failed", models.RestoreStepResultError, errorMessage)
}

// createRestoreStep creates a new restore step
func (api *exportRestoresAPI) createRestoreStep(ctx context.Context, restoreOperationID, name string, result models.RestoreStepResult, reason string) {
	step := models.RestoreStep{
		RestoreOperationID: restoreOperationID,
		Name:              name,
		Result:            result,
		Reason:            reason,
		CreatedDate:       models.PNow(),
		UpdatedDate:       models.PNow(),
	}

	_, err := api.registrySet.RestoreStepRegistry.Create(ctx, step)
	if err != nil {
		fmt.Printf("Failed to create restore step: %v\n", err)
	}
}

// updateRestoreStep updates an existing restore step by name
func (api *exportRestoresAPI) updateRestoreStep(ctx context.Context, restoreOperationID, name string, result models.RestoreStepResult, reason string) {
	// Get all steps for this restore operation
	steps, err := api.registrySet.RestoreStepRegistry.ListByRestoreOperation(ctx, restoreOperationID)
	if err != nil {
		fmt.Printf("Failed to list restore steps for update: %v\n", err)
		return
	}

	// Find the step with the matching name
	for _, step := range steps {
		if step.Name == name {
			step.Result = result
			step.Reason = reason
			step.UpdatedDate = models.PNow()

			_, err := api.registrySet.RestoreStepRegistry.Update(ctx, *step)
			if err != nil {
				fmt.Printf("Failed to update restore step: %v\n", err)
			}
			return
		}
	}

	// If step not found, create it
	api.createRestoreStep(ctx, restoreOperationID, name, result, reason)
}

// processRestoreWithDetailedLogging processes the restore with detailed step-by-step logging
func (api *exportRestoresAPI) processRestoreWithDetailedLogging(ctx context.Context, restoreOperationID string, service *restore.RestoreService, reader io.Reader, options restore.RestoreOptions) (*restore.RestoreStats, error) {
	// Create a custom restore processor that logs each item
	processor := &DetailedRestoreProcessor{
		api:                 api,
		restoreOperationID:  restoreOperationID,
		service:            service,
		options:            options,
	}

	return processor.ProcessWithLogging(ctx, reader)
}

// DetailedRestoreProcessor handles restore processing with detailed logging
type DetailedRestoreProcessor struct {
	api                *exportRestoresAPI
	restoreOperationID string
	service           *restore.RestoreService
	options           restore.RestoreOptions
}

// ProcessWithLogging processes the restore with detailed step logging
func (p *DetailedRestoreProcessor) ProcessWithLogging(ctx context.Context, reader io.Reader) (*restore.RestoreStats, error) {
	// Create step for loading existing data
	p.api.createRestoreStep(ctx, p.restoreOperationID, "Loading existing data", models.RestoreStepResultInProgress, "")

	// Create a custom restore service with logging callbacks
	loggedService := &LoggedRestoreService{
		service:            p.service,
		processor:          p,
		ctx:               ctx,
	}

	stats, err := loggedService.RestoreFromXMLWithLogging(ctx, reader, p.options)
	if err != nil {
		p.api.updateRestoreStep(ctx, p.restoreOperationID, "Loading existing data", models.RestoreStepResultError, err.Error())
		return stats, err
	}

	p.api.updateRestoreStep(ctx, p.restoreOperationID, "Loading existing data", models.RestoreStepResultSuccess, "")

	return stats, nil
}

// LoggedRestoreService wraps the restore service to provide detailed logging
type LoggedRestoreService struct {
	service   *restore.RestoreService
	processor *DetailedRestoreProcessor
	ctx       context.Context
}

// RestoreFromXMLWithLogging processes XML with detailed item-by-item logging
func (l *LoggedRestoreService) RestoreFromXMLWithLogging(ctx context.Context, xmlReader io.Reader, options restore.RestoreOptions) (*restore.RestoreStats, error) {
	// Read the entire XML content first so we can process it with detailed logging
	xmlContent, err := io.ReadAll(xmlReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read XML content: %v", err)
	}

	// Parse XML to analyze items for logging purposes
	l.processor.api.createRestoreStep(ctx, l.processor.restoreOperationID, "Analyzing XML content", models.RestoreStepResultInProgress, "")

	// Parse the XML structure to extract individual items for logging
	exportData, err := l.parseExportXML(xmlContent)
	if err != nil {
		l.processor.api.updateRestoreStep(ctx, l.processor.restoreOperationID, "Analyzing XML content", models.RestoreStepResultError, err.Error())
		return nil, err
	}

	l.processor.api.updateRestoreStep(ctx, l.processor.restoreOperationID, "Analyzing XML content", models.RestoreStepResultSuccess,
		fmt.Sprintf("Found %d locations, %d areas, %d commodities", len(exportData.Locations), len(exportData.Areas), len(exportData.Commodities)))

	// Create detailed logging steps for what will be processed
	l.createDetailedLoggingSteps(ctx, exportData, options)

	// Now use the original restore service to do the actual data processing
	l.processor.api.createRestoreStep(ctx, l.processor.restoreOperationID, "Processing data with full restore service", models.RestoreStepResultInProgress, "")

	reader := strings.NewReader(string(xmlContent))
	stats, err := l.service.RestoreFromXML(ctx, reader, options)
	if err != nil {
		l.processor.api.updateRestoreStep(ctx, l.processor.restoreOperationID, "Processing data with full restore service", models.RestoreStepResultError, err.Error())
		return stats, err
	}

	l.processor.api.updateRestoreStep(ctx, l.processor.restoreOperationID, "Processing data with full restore service", models.RestoreStepResultSuccess,
		fmt.Sprintf("Completed processing with %d errors", stats.ErrorCount))

	return stats, nil
}

// ExportData holds the parsed XML export data
type ExportData struct {
	XMLName     xml.Name        `xml:"inventory"`
	Locations   []LocationData  `xml:"locations>location"`
	Areas       []AreaData      `xml:"areas>area"`
	Commodities []CommodityData `xml:"commodities>commodity"`
}

// LocationData represents a location in the XML
type LocationData struct {
	ID      string `xml:"id,attr"`
	Name    string `xml:"locationName"`
	Address string `xml:"address"`
}

// AreaData represents an area in the XML
type AreaData struct {
	ID         string `xml:"id,attr"`
	Name       string `xml:"areaName"`
	LocationID string `xml:"locationId"`
}

// CommodityData represents a commodity in the XML
type CommodityData struct {
	ID     string `xml:"id,attr"`
	Name   string `xml:"commodityName"`
	AreaID string `xml:"areaId"`
}

// parseExportXML parses the XML content into structured data
func (l *LoggedRestoreService) parseExportXML(xmlContent []byte) (*ExportData, error) {
	var exportData ExportData
	err := xml.Unmarshal(xmlContent, &exportData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML: %v", err)
	}
	return &exportData, nil
}

// createDetailedLoggingSteps creates detailed logging steps for each item that will be processed
func (l *LoggedRestoreService) createDetailedLoggingSteps(ctx context.Context, exportData *ExportData, options restore.RestoreOptions) {
	// Create detailed steps for locations
	l.processor.api.createRestoreStep(ctx, l.processor.restoreOperationID, "Will process locations", models.RestoreStepResultInProgress, "")

	for _, location := range exportData.Locations {
		action := l.predictAction(ctx, "location", location.ID, options)
		emoji := l.getEmojiForAction(action)
		actionDesc := l.getActionDescription(action, options.DryRun)

		l.processor.api.createRestoreStep(ctx, l.processor.restoreOperationID,
			fmt.Sprintf("%s Location: %s", emoji, location.Name), models.RestoreStepResultSuccess, actionDesc)
	}

	l.processor.api.updateRestoreStep(ctx, l.processor.restoreOperationID, "Will process locations", models.RestoreStepResultSuccess,
		fmt.Sprintf("Will process %d locations", len(exportData.Locations)))

	// Create detailed steps for areas
	l.processor.api.createRestoreStep(ctx, l.processor.restoreOperationID, "Will process areas", models.RestoreStepResultInProgress, "")

	for _, area := range exportData.Areas {
		action := l.predictAction(ctx, "area", area.ID, options)
		emoji := l.getEmojiForAction(action)
		actionDesc := l.getActionDescription(action, options.DryRun)

		l.processor.api.createRestoreStep(ctx, l.processor.restoreOperationID,
			fmt.Sprintf("%s Area: %s", emoji, area.Name), models.RestoreStepResultSuccess, actionDesc)
	}

	l.processor.api.updateRestoreStep(ctx, l.processor.restoreOperationID, "Will process areas", models.RestoreStepResultSuccess,
		fmt.Sprintf("Will process %d areas", len(exportData.Areas)))

	// Create detailed steps for commodities
	l.processor.api.createRestoreStep(ctx, l.processor.restoreOperationID, "Will process commodities", models.RestoreStepResultInProgress, "")

	for _, commodity := range exportData.Commodities {
		action := l.predictAction(ctx, "commodity", commodity.ID, options)
		emoji := l.getEmojiForAction(action)
		actionDesc := l.getActionDescription(action, options.DryRun)

		l.processor.api.createRestoreStep(ctx, l.processor.restoreOperationID,
			fmt.Sprintf("%s Commodity: %s", emoji, commodity.Name), models.RestoreStepResultSuccess, actionDesc)
	}

	l.processor.api.updateRestoreStep(ctx, l.processor.restoreOperationID, "Will process commodities", models.RestoreStepResultSuccess,
		fmt.Sprintf("Will process %d commodities", len(exportData.Commodities)))
}

// predictAction predicts what action will be taken for an item based on strategy and existence
func (l *LoggedRestoreService) predictAction(ctx context.Context, itemType, itemID string, options restore.RestoreOptions) string {
	var exists bool
	var err error

	// Check if item exists
	switch itemType {
	case "location":
		_, err = l.processor.api.registrySet.LocationRegistry.Get(ctx, itemID)
	case "area":
		_, err = l.processor.api.registrySet.AreaRegistry.Get(ctx, itemID)
	case "commodity":
		_, err = l.processor.api.registrySet.CommodityRegistry.Get(ctx, itemID)
	}

	exists = err == nil

	// Predict action based on strategy and existence
	switch options.Strategy {
	case restore.RestoreStrategyFullReplace:
		return "created" // Always create in full replace (database is cleared)
	case restore.RestoreStrategyMergeAdd:
		if exists {
			return "skipped"
		}
		return "created"
	case restore.RestoreStrategyMergeUpdate:
		if exists {
			return "updated"
		}
		return "created"
	default:
		return "processed"
	}
}

// getEmojiForAction returns the appropriate emoji for an action
func (l *LoggedRestoreService) getEmojiForAction(action string) string {
	switch action {
	case "created":
		return "✅"
	case "updated":
		return "🔄"
	case "skipped":
		return "⏭️"
	default:
		return "📝"
	}
}

// getActionDescription returns a description for the action taken
func (l *LoggedRestoreService) getActionDescription(action string, dryRun bool) string {
	if dryRun {
		switch action {
		case "created":
			return "Would be created"
		case "updated":
			return "Would be updated"
		case "skipped":
			return "Would be skipped (already exists)"
		default:
			return "Would be processed"
		}
	} else {
		switch action {
		case "created":
			return "Created new item"
		case "updated":
			return "Updated existing item"
		case "skipped":
			return "Skipped (already exists)"
		default:
			return "Processed"
		}
	}
}







// ExportRestores sets up the export restore API routes.
func ExportRestores(params Params) func(r chi.Router) {
	api := &exportRestoresAPI{
		registrySet:    params.RegistrySet,
		uploadLocation: params.UploadLocation,
	}

	return func(r chi.Router) {
		r.Get("/", api.listExportRestores)
		r.Post("/", api.createExportRestore)
		r.Get("/{restoreId}", api.getExportRestore)
		r.Delete("/{restoreId}", api.deleteExportRestore)
	}
}

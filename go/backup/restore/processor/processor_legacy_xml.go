//go:build legacy_xml_backup

// DEPRECATED — LEGACY XML BACKUP CODE.
// Compiled ONLY under the `legacy_xml_backup` build tag; NOT in the default build.
// Implements the obsolete XML backup format that #534 replaced with the signed
// JSON `.inb` archive. Retained solely to be extracted into a separate repo as an
// XML-streaming proof-of-concept. Do not extend; do not couple new code to it.

package processor

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/shopspring/decimal"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore/security"
	"github.com/denisvmedia/inventario/backup/restore/types"
	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
)

// decodeAndRestore is the legacy XML decode entry point. It streams the XML
// reader and applies each entity through the shared model-level strategy
// handlers.
func (l *RestoreOperationProcessor) decodeAndRestore(ctx context.Context, reader io.Reader, options types.RestoreOptions) (*types.RestoreStats, error) {
	return l.restoreFromXML(ctx, reader, options)
}

// RestoreFromXML is the exported entry point used by the legacy XML test suite
// to drive the restore directly from an XML reader. Production code goes through
// Process; this exists only under the legacy build for the XML-coupled tests.
func (l *RestoreOperationProcessor) RestoreFromXML(ctx context.Context, xmlReader io.Reader, options types.RestoreOptions) (*types.RestoreStats, error) {
	return l.restoreFromXML(ctx, xmlReader, options)
}

// restoreFromXML processes the restore with detailed logging using a streaming
// XML approach.
func (l *RestoreOperationProcessor) restoreFromXML(
	ctx context.Context,
	xmlReader io.Reader,
	options types.RestoreOptions,
) (*types.RestoreStats, error) {
	stats := &types.RestoreStats{}

	prep, err := l.prepareRestore(ctx, options)
	if err != nil {
		return stats, err
	}
	ctx = prep.ctx
	existingEntities := prep.existing
	idMapping := prep.idMapping

	decoder := xml.NewDecoder(xmlReader)
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return stats, errxtrace.Wrap("failed to read XML token", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if err := l.restoreTopLevelElements(ctx, t, decoder, stats, existingEntities, idMapping, options); err != nil {
				return stats, err
			}
		case xml.ProcInst, xml.Directive, xml.Comment, xml.CharData, xml.EndElement:
			continue
		default:
			return stats, errxtrace.ClassifyNew("unexpected token type", errx.Attrs("token_type", fmt.Sprintf("%T", t)))
		}
	}

	return stats, nil
}

func (l *RestoreOperationProcessor) restoreTopLevelElements(
	ctx context.Context,
	t xml.StartElement,
	decoder *xml.Decoder,
	stats *types.RestoreStats,
	existingEntities *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	switch t.Name.Local {
	case "inventory":
		return nil
	case "locations":
		return l.processLocationsWithLogging(ctx, decoder, stats, existingEntities, idMapping, options)
	case "areas":
		return l.processAreasWithLogging(ctx, decoder, stats, existingEntities, idMapping, options)
	case "commodities":
		return l.processCommoditiesWithLogging(ctx, decoder, stats, existingEntities, idMapping, options)
	case "files":
		return l.processFilesWithLogging(ctx, decoder, stats, existingEntities, idMapping, options)
	}
	return nil
}

func (l *RestoreOperationProcessor) processLocationsWithLogging(
	ctx context.Context,
	decoder *xml.Decoder,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errxtrace.Wrap("failed to read locations token", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "location" {
				if err := l.processLocation(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to process location: %v", err))
				}
			}
		case xml.EndElement:
			if t.Name.Local == "locations" {
				return nil
			}
		}
	}
}

func (l *RestoreOperationProcessor) processLocation(
	ctx context.Context,
	decoder *xml.Decoder,
	startElement *xml.StartElement,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	var xmlLocation types.XMLLocation
	if err := decoder.DecodeElement(&xmlLocation, startElement); err != nil {
		return errxtrace.Wrap("failed to decode location", err)
	}

	action := l.predictAction(ctx, "location", xmlLocation.ID, options)
	step := locationStep(xmlLocation.LocationName)
	l.createRestoreStep(ctx, step, models.RestoreStepResultInProgress, l.getActionDescription(action, options))

	location := xmlLocation.ConvertToLocation()
	if err := location.ValidateWithContext(ctx); err != nil {
		l.updateRestoreStep(ctx, step, models.RestoreStepResultError, err.Error())
		return errxtrace.Wrap("invalid location", err, errx.Attrs("location_id", location.ID))
	}

	existingLocation := existing.Locations[xmlLocation.ID]
	if err := l.applyStrategyForLocationModel(ctx, location, existingLocation, xmlLocation.ID, xmlLocation.LocationName, stats, existing, idMapping, options); err != nil {
		l.updateRestoreStep(ctx, step, models.RestoreStepResultError, err.Error())
		return errxtrace.Wrap("failed to apply strategy for location", err)
	}
	l.updateRestoreStep(ctx, step, models.RestoreStepResultSuccess, "Completed")
	return nil
}

func (l *RestoreOperationProcessor) processAreasWithLogging(
	ctx context.Context,
	decoder *xml.Decoder,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errxtrace.Wrap("failed to read areas token", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "area" {
				if err := l.processArea(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to process area: %v", err))
				}
			}
		case xml.EndElement:
			if t.Name.Local == "areas" {
				return nil
			}
		}
	}
}

func (l *RestoreOperationProcessor) processArea(
	ctx context.Context,
	decoder *xml.Decoder,
	startElement *xml.StartElement,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	var xmlArea types.XMLArea
	if err := decoder.DecodeElement(&xmlArea, startElement); err != nil {
		return errxtrace.Wrap("failed to decode area", err)
	}

	step := fmt.Sprintf("Area: %s", xmlArea.AreaName)
	action := l.predictAction(ctx, "area", xmlArea.ID, options)
	l.createRestoreStep(ctx, step, models.RestoreStepResultInProgress, l.getActionDescription(action, options))

	if existing.Locations[xmlArea.LocationID] == nil {
		err := fmt.Errorf("area %s references non-existent location %s", xmlArea.ID, xmlArea.LocationID)
		l.updateRestoreStep(ctx, step, models.RestoreStepResultError, err.Error())
		return err
	}
	actualLocationID := idMapping.Locations[xmlArea.LocationID]
	if actualLocationID == "" {
		err := fmt.Errorf("no ID mapping found for location %s", xmlArea.LocationID)
		l.updateRestoreStep(ctx, step, models.RestoreStepResultError, err.Error())
		return err
	}

	area := xmlArea.ConvertToArea()
	area.LocationID = actualLocationID
	if err := area.ValidateWithContext(ctx); err != nil {
		l.updateRestoreStep(ctx, step, models.RestoreStepResultError, err.Error())
		return errxtrace.Wrap("invalid area", err, errx.Attrs("xml_id", xmlArea.ID))
	}

	existingArea := existing.Areas[xmlArea.ID]
	if err := l.applyStrategyForAreaModel(ctx, area, existingArea, xmlArea.ID, stats, existing, idMapping, options); err != nil {
		l.updateRestoreStep(ctx, step, models.RestoreStepResultError, err.Error())
		return errxtrace.Wrap("failed to apply strategy for area", err)
	}
	l.updateRestoreStep(ctx, step, models.RestoreStepResultSuccess, "Completed")
	return nil
}

func (l *RestoreOperationProcessor) processCommoditiesWithLogging(
	ctx context.Context,
	decoder *xml.Decoder,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errxtrace.Wrap("failed to read commodities token", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "commodity" {
				if err := l.processCommodity(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to process commodity: %v", err))
				}
			}
		case xml.EndElement:
			if t.Name.Local == "commodities" {
				return nil
			}
		}
	}
}

func (l *RestoreOperationProcessor) processCommodity(
	ctx context.Context,
	decoder *xml.Decoder,
	startElement *xml.StartElement,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	var xmlCommodity types.XMLCommodity
	for _, attr := range startElement.Attr {
		if attr.Name.Local == "id" {
			xmlCommodity.ID = attr.Value
			break
		}
	}

	step := fmt.Sprintf("Commodity: %s", xmlCommodity.ID)
	l.createRestoreStep(ctx, step, models.RestoreStepResultInProgress, l.getActionDescription(l.predictAction(ctx, "commodity", xmlCommodity.ID, options), options))

	for {
		tok, err := decoder.Token()
		if err != nil {
			l.updateRestoreStep(ctx, step, models.RestoreStepResultError, err.Error())
			return errxtrace.Wrap("failed to read commodity element token", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := l.collectCommodityData(ctx, step, t, decoder, stats, &xmlCommodity); err != nil {
				l.updateRestoreStep(ctx, step, models.RestoreStepResultError, err.Error())
				return err
			}
		case xml.EndElement:
			if t.Name.Local != "commodity" {
				continue
			}
			if err := l.createOrUpdateCommodity(ctx, &xmlCommodity, stats, existing, idMapping, options); err != nil {
				l.updateRestoreStep(ctx, step, models.RestoreStepResultError, err.Error())
				return err
			}
			l.updateRestoreStep(ctx, step, models.RestoreStepResultSuccess, "Completed")
			return nil
		}
	}
}

//nolint:gocyclo // the XML field switch is inherently wide and legacy
func (l *RestoreOperationProcessor) collectCommodityData(
	ctx context.Context,
	stepName string,
	t xml.StartElement,
	decoder *xml.Decoder,
	stats *types.RestoreStats,
	xmlCommodity *types.XMLCommodity,
) error {
	switch t.Name.Local {
	case "commodityName":
		if err := decoder.DecodeElement(&xmlCommodity.CommodityName, &t); err != nil {
			return errxtrace.Wrap("failed to decode commodity name", err)
		}
		l.updateRestoreStep(ctx, stepName, models.RestoreStepResultInProgress, fmt.Sprintf("Processing %s", xmlCommodity.CommodityName))
	case "shortName":
		return decoder.DecodeElement(&xmlCommodity.ShortName, &t)
	case "areaId":
		return decoder.DecodeElement(&xmlCommodity.AreaID, &t)
	case "type":
		return decoder.DecodeElement(&xmlCommodity.Type, &t)
	case "count":
		return decoder.DecodeElement(&xmlCommodity.Count, &t)
	case "status":
		return decoder.DecodeElement(&xmlCommodity.Status, &t)
	case "originalPrice":
		return decoder.DecodeElement(&xmlCommodity.OriginalPrice, &t)
	case "originalPriceCurrency":
		return decoder.DecodeElement(&xmlCommodity.OriginalCurrency, &t)
	case "convertedOriginalPrice":
		return decoder.DecodeElement(&xmlCommodity.ConvertedOriginalPrice, &t)
	case "currentPrice":
		return decoder.DecodeElement(&xmlCommodity.CurrentPrice, &t)
	case "currentCurrency":
		return decoder.DecodeElement(&xmlCommodity.CurrentCurrency, &t)
	case "serialNumber":
		return decoder.DecodeElement(&xmlCommodity.SerialNumber, &t)
	case "extraSerialNumbers":
		return decoder.DecodeElement(&xmlCommodity.ExtraSerialNumbers, &t)
	case "comments":
		return decoder.DecodeElement(&xmlCommodity.Comments, &t)
	case "draft":
		return decoder.DecodeElement(&xmlCommodity.Draft, &t)
	case "purchaseDate":
		return decoder.DecodeElement(&xmlCommodity.PurchaseDate, &t)
	case "registeredDate":
		return decoder.DecodeElement(&xmlCommodity.RegisteredDate, &t)
	case "lastModifiedDate":
		return decoder.DecodeElement(&xmlCommodity.LastModifiedDate, &t)
	case "partNumbers":
		return decoder.DecodeElement(&xmlCommodity.PartNumbers, &t)
	case "tags":
		return decoder.DecodeElement(&xmlCommodity.Tags, &t)
	case "urls":
		return decoder.DecodeElement(&xmlCommodity.URLs, &t)
	case "images", "invoices", "manuals":
		stats.ErrorCount++
		stats.Errors = append(stats.Errors, fmt.Sprintf(
			"unsupported pre-cutover attachment section <%s> on commodity %s; restore the data via a backup created after PR #1485 instead",
			t.Name.Local, xmlCommodity.ID,
		))
		if err := skipElement(decoder, t.Name.Local); err != nil {
			return errxtrace.Wrap("failed to skip legacy attachment section", err)
		}
	}
	return nil
}

func skipElement(decoder *xml.Decoder, name string) error {
	depth := 1
	for depth > 0 {
		tok, err := decoder.Token()
		if err != nil {
			return errxtrace.Wrap("failed to read token while skipping element", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 && t.Name.Local == name {
				return nil
			}
		}
	}
	return nil
}

func (l *RestoreOperationProcessor) createOrUpdateCommodity(
	ctx context.Context,
	xmlCommodity *types.XMLCommodity,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	if existing.Areas[xmlCommodity.AreaID] == nil {
		return errxtrace.ClassifyNew("commodity references non-existent area", errx.Attrs(
			"original_commodity_id", xmlCommodity.ID,
			"original_area_id", xmlCommodity.AreaID,
		))
	}
	actualAreaID := idMapping.Areas[xmlCommodity.AreaID]
	if actualAreaID == "" {
		return errxtrace.ClassifyNew("no ID mapping found for area", errx.Attrs("original_area_id", xmlCommodity.AreaID))
	}

	commodity, err := xmlCommodity.ConvertToCommodity()
	if err != nil {
		return errxtrace.Wrap("failed to convert commodity", err, errx.Attrs("original_commodity_id", xmlCommodity.ID))
	}
	commodity.AreaID = actualAreaID

	if groupCurrency, gcErr := validationctx.GroupCurrencyFromContext(ctx); gcErr == nil && string(commodity.OriginalPriceCurrency) == groupCurrency {
		commodity.ConvertedOriginalPrice = decimal.Zero
	}
	if err := commodity.ValidateWithContext(ctx); err != nil {
		return errxtrace.Wrap("invalid commodity", err, errx.Attrs("original_commodity_id", xmlCommodity.ID))
	}

	currentUser := appctx.UserFromContext(ctx)
	if currentUser == nil {
		return security.ErrNoUserContext
	}
	if err := l.validateCommodityOwnershipInDB(ctx, xmlCommodity.ID, currentUser, existing, stats); err != nil {
		return err
	}

	existingCommodity := existing.Commodities[xmlCommodity.ID]
	return l.applyStrategyForCommodityModel(ctx, commodity, existingCommodity, xmlCommodity.ID, stats, existing, idMapping, options)
}

func (l *RestoreOperationProcessor) processFilesWithLogging(
	ctx context.Context,
	decoder *xml.Decoder,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errxtrace.Wrap("failed to read files token", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "file" {
				if err := l.processFile(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to process file: %v", err))
				}
			}
		case xml.EndElement:
			if t.Name.Local == "files" {
				return nil
			}
		}
	}
}

func (l *RestoreOperationProcessor) processFile(
	ctx context.Context,
	decoder *xml.Decoder,
	startElement *xml.StartElement,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	xmlFile, blobBytes, err := l.decodeFileElement(ctx, decoder, startElement, existing, idMapping, options)
	if err != nil {
		return errxtrace.Wrap("failed to decode file element", err)
	}
	stats.BinaryDataSize += blobBytes

	originalXMLID := xmlFile.ID
	action := l.predictFileAction(xmlFile.ID, options, existing)
	displayName := xmlFile.Title
	if displayName == "" {
		displayName = xmlFile.Path
	}
	step := fmt.Sprintf("File: %s", displayName)
	l.createRestoreStep(ctx, step, models.RestoreStepResultInProgress, l.getActionDescription(action, options))

	linkedDBID, ok := l.resolveLinkedEntityDBID(xmlFile.LinkedEntityType, xmlFile.LinkedEntityID, idMapping)
	if !ok {
		stats.ErrorCount++
		msg := fmt.Sprintf("file %s references unknown %s %s", originalXMLID, xmlFile.LinkedEntityType, xmlFile.LinkedEntityID)
		stats.Errors = append(stats.Errors, msg)
		l.updateRestoreStep(ctx, step, models.RestoreStepResultError, msg)
		return nil
	}

	fileEntity := xmlFile.ConvertToFileEntity(linkedDBID)
	user := appctx.UserFromContext(ctx)
	if user == nil {
		stats.ErrorCount++
		l.updateRestoreStep(ctx, step, models.RestoreStepResultError, "missing user context")
		return security.ErrNoUserContext
	}
	fileEntity.TenantID = user.TenantID
	fileEntity.CreatedByUserID = user.ID
	if group := appctx.GroupFromContext(ctx); group != nil {
		fileEntity.GroupID = group.ID
	}

	if err := l.applyStrategyForFileModel(ctx, fileEntity, originalXMLID, stats, existing, idMapping, options); err != nil {
		l.updateRestoreStep(ctx, step, models.RestoreStepResultError, err.Error())
		return err
	}
	l.updateRestoreStep(ctx, step, models.RestoreStepResultSuccess, "Completed")
	return nil
}

// decodeFileElement walks the children of a <file> StartElement, decoding each
// metadata sub-element and streaming the <data> chardata directly into a blob
// writer (constant memory). Returns the populated XMLFile and the decoded blob
// byte count.
func (l *RestoreOperationProcessor) decodeFileElement(
	ctx context.Context,
	decoder *xml.Decoder,
	startElement *xml.StartElement,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) (*types.XMLFile, int64, error) {
	var xmlFile types.XMLFile
	for _, attr := range startElement.Attr {
		if attr.Name.Local == "id" {
			xmlFile.ID = attr.Value
			break
		}
	}

	var rawSize int64
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, 0, errxtrace.Wrap("failed to read file element token", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "data" {
				size, err := l.handleDataStart(ctx, decoder, &xmlFile, existing, idMapping, options)
				if err != nil {
					return nil, 0, err
				}
				rawSize = size
				continue
			}
			if err := decodeFileChild(decoder, &xmlFile, &t); err != nil {
				return nil, 0, err
			}
		case xml.EndElement:
			if t.Name.Local == "file" {
				return &xmlFile, rawSize, nil
			}
		}
	}
}

func (l *RestoreOperationProcessor) handleDataStart(
	ctx context.Context,
	decoder *xml.Decoder,
	xmlFile *types.XMLFile,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) (int64, error) {
	if xmlFile.OriginalPath == "" {
		return 0, errors.New("malformed export: <data> element preceded <originalPath>")
	}
	user := appctx.UserFromContext(ctx)
	if user == nil || user.TenantID == "" {
		return 0, errors.New("tenant context is required to restore file data")
	}
	xmlFile.OriginalPath = rewriteImportKey(xmlFile.OriginalPath, user.TenantID)
	size, err := l.handleFileDataElement(ctx, decoder, xmlFile, existing, idMapping, options)
	if err != nil {
		return 0, errxtrace.Wrap("failed to stream file data", err, errx.Attrs("xml_id", xmlFile.ID))
	}
	return size, nil
}

func (l *RestoreOperationProcessor) handleFileDataElement(
	ctx context.Context,
	decoder *xml.Decoder,
	xmlFile *types.XMLFile,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) (int64, error) {
	if shouldWriteFileBlob(xmlFile, existing, idMapping, options) {
		return l.streamDecodeFileData(ctx, decoder, xmlFile.OriginalPath)
	}
	return drainFileDataElement(decoder)
}

func shouldWriteFileBlob(
	xmlFile *types.XMLFile,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) bool {
	if options.DryRun {
		return false
	}
	if existing != nil && existing.Files != nil {
		if _, dupe := existing.Files[xmlFile.ID]; dupe && options.Strategy == types.RestoreStrategyMergeAdd {
			return false
		}
	}
	if xmlFile.LinkedEntityType != "" && xmlFile.LinkedEntityID != "" {
		if !linkedEntityResolves(xmlFile.LinkedEntityType, xmlFile.LinkedEntityID, idMapping) {
			return false
		}
	}
	return true
}

func linkedEntityResolves(linkedType, linkedXMLID string, idMapping *types.IDMapping) bool {
	if idMapping == nil {
		return true
	}
	switch linkedType {
	case "commodity":
		_, ok := idMapping.Commodities[linkedXMLID]
		return ok
	case "location":
		_, ok := idMapping.Locations[linkedXMLID]
		return ok
	case "area":
		_, ok := idMapping.Areas[linkedXMLID]
		return ok
	}
	return true
}

func drainFileDataElement(decoder *xml.Decoder) (int64, error) {
	chardataReader := &xmlChardataReader{decoder: decoder, terminator: "data"}
	base64Reader := base64.NewDecoder(base64.StdEncoding, chardataReader)
	n, err := io.Copy(io.Discard, base64Reader)
	if err != nil {
		return 0, errxtrace.Wrap("failed to drain base64 stream", err)
	}
	if !chardataReader.done {
		return 0, errors.New("malformed export: <data> stream ended before </data>")
	}
	return n, nil
}

func decodeFileChild(decoder *xml.Decoder, xmlFile *types.XMLFile, t *xml.StartElement) error {
	switch t.Name.Local {
	case "linkedEntityType":
		return decoder.DecodeElement(&xmlFile.LinkedEntityType, t)
	case "linkedEntityId":
		return decoder.DecodeElement(&xmlFile.LinkedEntityID, t)
	case "linkedEntityMeta":
		return decoder.DecodeElement(&xmlFile.LinkedEntityMeta, t)
	case "type":
		return decoder.DecodeElement(&xmlFile.Type, t)
	case "category":
		return decoder.DecodeElement(&xmlFile.Category, t)
	case "title":
		return decoder.DecodeElement(&xmlFile.Title, t)
	case "description":
		return decoder.DecodeElement(&xmlFile.Description, t)
	case "tags":
		return decoder.DecodeElement(&xmlFile.Tags, t)
	case "path":
		return decoder.DecodeElement(&xmlFile.Path, t)
	case "originalPath":
		return decoder.DecodeElement(&xmlFile.OriginalPath, t)
	case "extension":
		return decoder.DecodeElement(&xmlFile.Extension, t)
	case "mimeType":
		return decoder.DecodeElement(&xmlFile.MimeType, t)
	case "createdAt":
		return decoder.DecodeElement(&xmlFile.CreatedAt, t)
	case "updatedAt":
		return decoder.DecodeElement(&xmlFile.UpdatedAt, t)
	default:
		var ignored struct{}
		return decoder.DecodeElement(&ignored, t)
	}
}

func (l *RestoreOperationProcessor) streamDecodeFileData(
	ctx context.Context,
	decoder *xml.Decoder,
	blobKey string,
) (int64, error) {
	if l.uploadLocation == "" {
		return drainFileDataElement(decoder)
	}

	chardataReader := &xmlChardataReader{decoder: decoder, terminator: "data"}
	base64Reader := base64.NewDecoder(base64.StdEncoding, chardataReader)

	bucket, err := blob.OpenBucket(ctx, l.uploadLocation)
	if err != nil {
		return 0, errxtrace.Wrap("failed to open blob bucket for streaming write", err)
	}
	defer bucket.Close()

	writer, err := bucket.NewWriter(ctx, blobKey, nil)
	if err != nil {
		return 0, errxtrace.Wrap("failed to create blob writer", err, errx.Attrs("blob_key", blobKey))
	}

	n, copyErr := io.Copy(writer, base64Reader)
	closeErr := writer.Close()
	if copyErr != nil {
		return 0, errxtrace.Wrap("failed to stream blob bytes", copyErr, errx.Attrs("blob_key", blobKey))
	}
	if closeErr != nil {
		return 0, errxtrace.Wrap("failed to close blob writer", closeErr, errx.Attrs("blob_key", blobKey))
	}
	if !chardataReader.done {
		return 0, errors.New("malformed export: <data> stream ended before </data>")
	}
	return n, nil
}

// resolveLinkedEntityDBID maps a linked-entity reference (UUID) to the
// destination DB ID via the IDMapping (legacy XML restore path).
func (l *RestoreOperationProcessor) resolveLinkedEntityDBID(
	linkedEntityType, linkedEntityXMLID string,
	idMapping *types.IDMapping,
) (string, bool) {
	if linkedEntityType == "" || linkedEntityXMLID == "" {
		return "", true
	}
	switch linkedEntityType {
	case "commodity":
		id, ok := idMapping.Commodities[linkedEntityXMLID]
		return id, ok
	case "location":
		id, ok := idMapping.Locations[linkedEntityXMLID]
		return id, ok
	case "area":
		id, ok := idMapping.Areas[linkedEntityXMLID]
		return id, ok
	default:
		return linkedEntityXMLID, true
	}
}

// predictAction predicts the action for an entity based on strategy.
func (l *RestoreOperationProcessor) predictAction(ctx context.Context, entityType, entityID string, options types.RestoreOptions) string {
	switch options.Strategy {
	case types.RestoreStrategyFullReplace:
		return "create"
	case types.RestoreStrategyMergeAdd:
		if l.entityExists(ctx, entityType, entityID) {
			return "skip"
		}
		return "create"
	case types.RestoreStrategyMergeUpdate:
		if l.entityExists(ctx, entityType, entityID) {
			return "update"
		}
		return "create"
	default:
		return "unknown"
	}
}

func (l *RestoreOperationProcessor) entityExists(ctx context.Context, entityType, entityID string) bool {
	switch entityType {
	case "location":
		locReg := l.factorySet.LocationRegistryFactory.CreateServiceRegistry()
		_, err := locReg.Get(ctx, entityID)
		return err == nil
	case "area":
		areaReg := l.factorySet.AreaRegistryFactory.CreateServiceRegistry()
		_, err := areaReg.Get(ctx, entityID)
		return err == nil
	case "commodity":
		comReg := l.factorySet.CommodityRegistryFactory.CreateServiceRegistry()
		_, err := comReg.Get(ctx, entityID)
		return err == nil
	default:
		return false
	}
}

func (l *RestoreOperationProcessor) getActionDescription(action string, options types.RestoreOptions) string {
	prefix := "Will "
	if options.DryRun {
		prefix = "[DRY RUN] Would "
	}
	switch action {
	case "create":
		return prefix + "create new entity"
	case "update":
		return prefix + "update existing entity"
	case "skip":
		return prefix + "skip (already exists)"
	default:
		return prefix + "perform unknown action"
	}
}

func (l *RestoreOperationProcessor) predictFileAction(xmlID string, options types.RestoreOptions, existing *types.ExistingEntities) string {
	switch options.Strategy {
	case types.RestoreStrategyFullReplace:
		return "create"
	case types.RestoreStrategyMergeAdd:
		if _, ok := existing.Files[xmlID]; ok {
			return "skip"
		}
		return "create"
	case types.RestoreStrategyMergeUpdate:
		if _, ok := existing.Files[xmlID]; ok {
			return "update"
		}
		return "create"
	default:
		return "unknown"
	}
}

// rewriteImportKey normalises an OriginalPath into the importing tenant's
// namespace. Legacy flat keys go through RewriteForTenant; already-prefixed
// (foreign-tenant) keys have their prefix stripped and re-prefixed.
func rewriteImportKey(originalPath, tenantID string) string {
	if blobkeys.HasTenantPrefix(originalPath) {
		return blobkeys.RewriteForTenant(stripTenantPrefix(originalPath), tenantID)
	}
	return blobkeys.RewriteForTenant(originalPath, tenantID)
}

// stripTenantPrefix removes a `t/<anything>/` prefix from a blob key.
func stripTenantPrefix(key string) string {
	if !blobkeys.HasTenantPrefix(key) {
		return key
	}
	_, after, found := strings.Cut(key[len(blobkeys.Prefix):], "/")
	if !found {
		return key
	}
	return after
}

// xmlChardataReader exposes a <data> element's chardata as an io.Reader,
// stripping pretty-print whitespace so the base64 decoder sees a contiguous
// stream.
type xmlChardataReader struct {
	decoder    *xml.Decoder
	terminator string
	buf        []byte
	done       bool
}

func (r *xmlChardataReader) Read(p []byte) (int, error) {
	for len(r.buf) == 0 && !r.done {
		tok, err := r.decoder.Token()
		if err != nil {
			return 0, err
		}
		switch t := tok.(type) {
		case xml.CharData:
			for _, b := range t {
				switch b {
				case ' ', '\n', '\r', '\t':
					continue
				}
				r.buf = append(r.buf, b)
			}
		case xml.EndElement:
			if t.Name.Local == r.terminator {
				r.done = true
			}
		}
	}
	if r.done && len(r.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

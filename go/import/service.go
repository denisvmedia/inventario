package importpkg

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"

	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/filekit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ImportService handles XML import operations
type ImportService struct {
	registrySet    *registry.Set
	uploadLocation string
}

// NewImportService creates a new import service
func NewImportService(registrySet *registry.Set, uploadLocation string) *ImportService {
	return &ImportService{
		registrySet:    registrySet,
		uploadLocation: uploadLocation,
	}
}

// ImportFromXML imports data from an XML file using streaming approach
func (s *ImportService) ImportFromXML(ctx context.Context, xmlReader io.Reader) (*ImportStats, error) {
	stats := &ImportStats{}
	
	decoder := xml.NewDecoder(xmlReader)
	
	// Track locations, areas, and commodities for validation
	locationIDs := make(map[string]bool)
	areaIDs := make(map[string]bool)
	
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return stats, errkit.Wrap(err, "failed to read XML token")
		}
		
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "locations":
				if err := s.processLocations(ctx, decoder, &t, locationIDs, stats); err != nil {
					return stats, errkit.Wrap(err, "failed to process locations")
				}
			case "areas":
				if err := s.processAreas(ctx, decoder, &t, areaIDs, stats); err != nil {
					return stats, errkit.Wrap(err, "failed to process areas")
				}
			case "commodities":
				if err := s.processCommodities(ctx, decoder, &t, stats); err != nil {
					return stats, errkit.Wrap(err, "failed to process commodities")
				}
			}
		}
	}
	
	return stats, nil
}

// processLocations processes the locations section
func (s *ImportService) processLocations(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, locationIDs map[string]bool, stats *ImportStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read location token")
		}
		
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "location" {
				var xmlLocation XMLLocation
				if err := decoder.DecodeElement(&xmlLocation, &t); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to decode location: %v", err))
					continue
				}
				
				location := xmlLocation.ConvertToLocation()
				if err := location.ValidateWithContext(ctx); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("invalid location %s: %v", location.ID, err))
					continue
				}
				
				if _, err := s.registrySet.LocationRegistry.Create(ctx, *location); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to create location %s: %v", location.ID, err))
					continue
				}
				
				locationIDs[location.ID] = true
				stats.LocationCount++
			}
		case xml.EndElement:
			if t.Name.Local == startElement.Name.Local {
				return nil
			}
		}
	}
}

// processAreas processes the areas section
func (s *ImportService) processAreas(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, areaIDs map[string]bool, stats *ImportStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read area token")
		}
		
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "area" {
				var xmlArea XMLArea
				if err := decoder.DecodeElement(&xmlArea, &t); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to decode area: %v", err))
					continue
				}
				
				area := xmlArea.ConvertToArea()
				if err := area.ValidateWithContext(ctx); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("invalid area %s: %v", area.ID, err))
					continue
				}
				
				if _, err := s.registrySet.AreaRegistry.Create(ctx, *area); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to create area %s: %v", area.ID, err))
					continue
				}
				
				areaIDs[area.ID] = true
				stats.AreaCount++
			}
		case xml.EndElement:
			if t.Name.Local == startElement.Name.Local {
				return nil
			}
		}
	}
}

// processCommodities processes the commodities section
func (s *ImportService) processCommodities(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, stats *ImportStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read commodity token")
		}
		
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "commodity" {
				if err := s.processCommodity(ctx, decoder, &t, stats); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to process commodity: %v", err))
					continue
				}
			}
		case xml.EndElement:
			if t.Name.Local == startElement.Name.Local {
				return nil
			}
		}
	}
}

// processCommodity processes a single commodity with its files
func (s *ImportService) processCommodity(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, stats *ImportStats) error {
	// First, we need to extract the commodity attributes
	var xmlCommodity XMLCommodity
	
	// Read the commodity element manually to handle nested file sections
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read commodity token")
		}
		
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "images":
				// Process images after commodity is created
				if err := s.processFiles(ctx, decoder, &t, xmlCommodity.ID, "image", stats); err != nil {
					return errkit.Wrap(err, "failed to process images")
				}
			case "invoices":
				// Process invoices after commodity is created
				if err := s.processFiles(ctx, decoder, &t, xmlCommodity.ID, "invoice", stats); err != nil {
					return errkit.Wrap(err, "failed to process invoices")
				}
			case "manuals":
				// Process manuals after commodity is created
				if err := s.processFiles(ctx, decoder, &t, xmlCommodity.ID, "manual", stats); err != nil {
					return errkit.Wrap(err, "failed to process manuals")
				}
			default:
				// Handle other commodity fields
				if err := s.processCommodityField(decoder, &t, &xmlCommodity); err != nil {
					return errkit.Wrap(err, "failed to process commodity field")
				}
			}
		case xml.EndElement:
			if t.Name.Local == startElement.Name.Local {
				// Create the commodity now that we have all its data
				commodity, err := xmlCommodity.ConvertToCommodity()
				if err != nil {
					return errkit.Wrap(err, "failed to convert commodity")
				}
				
				if err := commodity.ValidateWithContext(ctx); err != nil {
					return errkit.Wrap(err, "invalid commodity")
				}
				
				if _, err := s.registrySet.CommodityRegistry.Create(ctx, *commodity); err != nil {
					return errkit.Wrap(err, "failed to create commodity")
				}
				
				stats.CommodityCount++
				return nil
			}
		}
	}
}

// processCommodityField processes individual commodity fields
func (s *ImportService) processCommodityField(decoder *xml.Decoder, startElement *xml.StartElement, commodity *XMLCommodity) error {
	// Extract commodity ID from attributes if this is the commodity element
	if startElement.Name.Local == "commodity" {
		for _, attr := range startElement.Attr {
			if attr.Name.Local == "id" {
				commodity.ID = attr.Value
				break
			}
		}
	}
	
	// Decode the element based on its name
	switch startElement.Name.Local {
	case "areaId":
		return decoder.DecodeElement(&commodity.AreaID, startElement)
	case "name":
		return decoder.DecodeElement(&commodity.Name, startElement)
	case "description":
		return decoder.DecodeElement(&commodity.Description, startElement)
	case "category":
		return decoder.DecodeElement(&commodity.Category, startElement)
	case "status":
		return decoder.DecodeElement(&commodity.Status, startElement)
	case "condition":
		return decoder.DecodeElement(&commodity.Condition, startElement)
	case "brand":
		return decoder.DecodeElement(&commodity.Brand, startElement)
	case "model":
		return decoder.DecodeElement(&commodity.Model, startElement)
	case "serialNumber":
		return decoder.DecodeElement(&commodity.SerialNumber, startElement)
	case "originalPrice":
		var price XMLPrice
		if err := decoder.DecodeElement(&price, startElement); err != nil {
			return err
		}
		commodity.OriginalPrice = &price
	case "currentPrice":
		var price XMLPrice
		if err := decoder.DecodeElement(&price, startElement); err != nil {
			return err
		}
		commodity.CurrentPrice = &price
	case "purchaseDate":
		return decoder.DecodeElement(&commodity.PurchaseDate, startElement)
	case "registeredDate":
		return decoder.DecodeElement(&commodity.RegisteredDate, startElement)
	case "lastModifiedDate":
		return decoder.DecodeElement(&commodity.LastModifiedDate, startElement)
	case "partNumbers":
		var partNumbers XMLPartNumbers
		if err := decoder.DecodeElement(&partNumbers, startElement); err != nil {
			return err
		}
		commodity.PartNumbers = &partNumbers
	case "tags":
		var tags XMLTags
		if err := decoder.DecodeElement(&tags, startElement); err != nil {
			return err
		}
		commodity.Tags = &tags
	case "urls":
		var urls XMLURLs
		if err := decoder.DecodeElement(&urls, startElement); err != nil {
			return err
		}
		commodity.URLs = &urls
	default:
		// Skip unknown elements
		return decoder.Skip()
	}

	return nil
}

// processFiles processes file sections (images, invoices, manuals) with streaming base64 decoding
func (s *ImportService) processFiles(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, commodityID, fileType string, stats *ImportStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read file token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == fileType {
				if err := s.processFile(ctx, decoder, &t, commodityID, fileType, stats); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to process %s file: %v", fileType, err))
					continue
				}
			}
		case xml.EndElement:
			if t.Name.Local == startElement.Name.Local {
				return nil
			}
		}
	}
}

// processFile processes a single file with streaming base64 decoding
func (s *ImportService) processFile(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, commodityID, fileType string, stats *ImportStats) error {
	var xmlFile XMLFile

	// Extract file ID from attributes
	for _, attr := range startElement.Attr {
		if attr.Name.Local == "id" {
			xmlFile.ID = attr.Value
			break
		}
	}

	// Process file elements
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read file element token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "path":
				if err := decoder.DecodeElement(&xmlFile.Path, &t); err != nil {
					return errkit.Wrap(err, "failed to decode path")
				}
			case "originalPath":
				if err := decoder.DecodeElement(&xmlFile.OriginalPath, &t); err != nil {
					return errkit.Wrap(err, "failed to decode originalPath")
				}
			case "extension":
				if err := decoder.DecodeElement(&xmlFile.Extension, &t); err != nil {
					return errkit.Wrap(err, "failed to decode extension")
				}
			case "mimeType":
				if err := decoder.DecodeElement(&xmlFile.MimeType, &t); err != nil {
					return errkit.Wrap(err, "failed to decode mimeType")
				}
			case "data":
				// Stream decode base64 data
				if err := s.processFileData(ctx, decoder, &t, &xmlFile, stats); err != nil {
					return errkit.Wrap(err, "failed to process file data")
				}
			default:
				if err := decoder.Skip(); err != nil {
					return errkit.Wrap(err, "failed to skip unknown element")
				}
			}
		case xml.EndElement:
			if t.Name.Local == startElement.Name.Local {
				// Create the file entity
				return s.createFileEntity(ctx, &xmlFile, commodityID, fileType, stats)
			}
		}
	}
}

// processFileData streams and decodes base64 file data
func (s *ImportService) processFileData(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, xmlFile *XMLFile, stats *ImportStats) error {
	// Generate unique filename for storage
	filename := filekit.UploadFileName(xmlFile.OriginalPath)

	// Open blob storage
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return errkit.Wrap(err, "failed to open blob bucket")
	}
	defer b.Close()

	// Create blob writer
	writer, err := b.NewWriter(ctx, filename, nil)
	if err != nil {
		return errkit.Wrap(err, "failed to create blob writer")
	}
	defer writer.Close()

	// Create base64 decoder
	base64Decoder := base64.NewDecoder(base64.StdEncoding, &xmlCharDataReader{decoder: decoder, startElement: startElement})

	// Stream copy from base64 decoder to blob storage
	bytesWritten, err := io.Copy(writer, base64Decoder)
	if err != nil {
		return errkit.Wrap(err, "failed to copy file data")
	}

	// Update file path and stats
	xmlFile.Path = filename
	stats.BinaryDataSize += bytesWritten

	return nil
}

// xmlCharDataReader reads character data from XML decoder until end element
type xmlCharDataReader struct {
	decoder      *xml.Decoder
	startElement *xml.StartElement
	done         bool
}

func (r *xmlCharDataReader) Read(p []byte) (n int, err error) {
	if r.done {
		return 0, io.EOF
	}

	tok, err := r.decoder.Token()
	if err != nil {
		return 0, err
	}

	switch t := tok.(type) {
	case xml.CharData:
		copy(p, t)
		return len(t), nil
	case xml.EndElement:
		if t.Name.Local == r.startElement.Name.Local {
			r.done = true
			return 0, io.EOF
		}
		return 0, errkit.WithMessage(nil, "unexpected end element")
	default:
		return 0, errkit.WithMessage(nil, "unexpected token type")
	}
}

// createFileEntity creates the appropriate file entity based on type
func (s *ImportService) createFileEntity(ctx context.Context, xmlFile *XMLFile, commodityID, fileType string, stats *ImportStats) error {
	file := xmlFile.ConvertToFile()

	switch fileType {
	case "image":
		image := &models.Image{
			EntityID:    models.EntityID{ID: xmlFile.ID},
			CommodityID: commodityID,
			File:        file,
		}
		if err := image.ValidateWithContext(ctx); err != nil {
			return errkit.Wrap(err, "invalid image")
		}
		if _, err := s.registrySet.ImageRegistry.Create(ctx, *image); err != nil {
			return errkit.Wrap(err, "failed to create image")
		}
		if err := s.registrySet.CommodityRegistry.AddImage(ctx, commodityID, image.ID); err != nil {
			return errkit.Wrap(err, "failed to add image to commodity")
		}
		stats.ImageCount++

	case "invoice":
		invoice := &models.Invoice{
			EntityID:    models.EntityID{ID: xmlFile.ID},
			CommodityID: commodityID,
			File:        file,
		}
		if err := invoice.ValidateWithContext(ctx); err != nil {
			return errkit.Wrap(err, "invalid invoice")
		}
		if _, err := s.registrySet.InvoiceRegistry.Create(ctx, *invoice); err != nil {
			return errkit.Wrap(err, "failed to create invoice")
		}
		if err := s.registrySet.CommodityRegistry.AddInvoice(ctx, commodityID, invoice.ID); err != nil {
			return errkit.Wrap(err, "failed to add invoice to commodity")
		}
		stats.InvoiceCount++

	case "manual":
		manual := &models.Manual{
			EntityID:    models.EntityID{ID: xmlFile.ID},
			CommodityID: commodityID,
			File:        file,
		}
		if err := manual.ValidateWithContext(ctx); err != nil {
			return errkit.Wrap(err, "invalid manual")
		}
		if _, err := s.registrySet.ManualRegistry.Create(ctx, *manual); err != nil {
			return errkit.Wrap(err, "failed to create manual")
		}
		if err := s.registrySet.CommodityRegistry.AddManual(ctx, commodityID, manual.ID); err != nil {
			return errkit.Wrap(err, "failed to add manual to commodity")
		}
		stats.ManualCount++
	}

	return nil
}

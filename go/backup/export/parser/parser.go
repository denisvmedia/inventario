package parser

import (
	"context"
	"encoding/xml"
	"io"

	"github.com/denisvmedia/inventario/backup/export/types"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

// ParseXMLMetadata parses XML file to extract statistics and determine export type
func ParseXMLMetadata(_ctx context.Context, reader io.Reader) (*types.ExportStats, models.ExportType, error) {
	stats := &types.ExportStats{}
	exportType := models.ExportTypeImported // Default to imported type

	decoder := xml.NewDecoder(reader)

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return stats, exportType, errkit.Wrap(err, "failed to read XML token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			err = parseTopLevelToken(t, &exportType, decoder, stats)
			if err != nil {
				return stats, exportType, err
			}
		default:
			// Skip other token types
			continue
		}
	}

	// The following was commented out because detecting the export type from the content is not
	// reliable. It's better to just set it to "imported" and let the user decide.
	//
	//// If export type wasn't specified in XML, try to infer it from content
	//
	// if exportType == models.ExportTypeImported {
	//	if hasLocations && hasAreas && hasCommodities {
	//		exportType = models.ExportTypeFullDatabase
	//	} else if hasLocations && !hasAreas && !hasCommodities {
	//		exportType = models.ExportTypeLocations
	//	} else if hasAreas && !hasLocations && !hasCommodities {
	//		exportType = models.ExportTypeAreas
	//	} else if hasCommodities && !hasLocations && !hasAreas {
	//		exportType = models.ExportTypeCommodities
	//	} else {
	//		exportType = models.ExportTypeSelectedItems
	//	}
	// }

	return stats, exportType, nil
}

func parseTopLevelToken(t xml.StartElement, exportType *models.ExportType, decoder *xml.Decoder, stats *types.ExportStats) error {
	switch t.Name.Local {
	case "inventory":
		// Check export type attribute if present
		for _, attr := range t.Attr {
			if attr.Name.Local != "exportType" {
				continue
			}
			switch attr.Value {
			case "full_database":
				*exportType = models.ExportTypeFullDatabase
			case "selected_items":
				*exportType = models.ExportTypeSelectedItems
			case "locations":
				*exportType = models.ExportTypeLocations
			case "areas":
				*exportType = models.ExportTypeAreas
			case "commodities":
				*exportType = models.ExportTypeCommodities
			}
		}
	case "locations":
		if err := countLocations(decoder, stats); err != nil {
			return errkit.Wrap(err, "failed to count locations")
		}
	case "areas":
		if err := countAreas(decoder, stats); err != nil {
			return errkit.Wrap(err, "failed to count areas")
		}
	case "commodities":
		if err := countCommodities(decoder, stats); err != nil {
			return errkit.Wrap(err, "failed to count commodities")
		}
	default:
		return errkit.Wrap(ErrUnsupportedExportType, "unsupported top-level element", "element", t.Name.Local)
	}

	return nil
}

// countLocations counts location elements in XML
func countLocations(decoder *xml.Decoder, stats *types.ExportStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read location token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "location" {
				stats.LocationCount++
			}
		case xml.EndElement:
			if t.Name.Local == "locations" {
				return nil
			}
		}
	}
}

// countAreas counts area elements in XML
func countAreas(decoder *xml.Decoder, stats *types.ExportStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read area token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "area" {
				stats.AreaCount++
			}
		case xml.EndElement:
			if t.Name.Local == "areas" {
				return nil
			}
		}
	}
}

// countCommodities counts commodity elements and their files in XML
func countCommodities(decoder *xml.Decoder, stats *types.ExportStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read commodity token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "commodity":
				stats.CommodityCount++
			case "images":
				if err := countFiles(decoder, "images", &stats.ImageCount, stats); err != nil {
					return errkit.Wrap(err, "failed to count images")
				}
			case "invoices":
				if err := countFiles(decoder, "invoices", &stats.InvoiceCount, stats); err != nil {
					return errkit.Wrap(err, "failed to count invoices")
				}
			case "manuals":
				if err := countFiles(decoder, "manuals", &stats.ManualCount, stats); err != nil {
					return errkit.Wrap(err, "failed to count manuals")
				}
			}
		case xml.EndElement:
			if t.Name.Local == "commodities" {
				return nil
			}
		}
	}
}

// countFiles counts file elements within a file section and estimates binary data size
func countFiles(decoder *xml.Decoder, sectionName string, counter *int, stats *types.ExportStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read file section token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "file" {
				*counter++
				// Process the file element to detect if it has data
				if err := processFileElement(decoder, stats); err != nil {
					return errkit.Wrap(err, "failed to process file element")
				}
			}
		case xml.EndElement:
			if t.Name.Local == sectionName {
				return nil
			}
		case xml.CharData:
			// Skip character data at this level
			continue
		}
	}
}

// processFileElement processes a file element to detect and estimate binary data size
func processFileElement(decoder *xml.Decoder, stats *types.ExportStats) error {
	depth := 1 // We're inside a file element

	for depth > 0 {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read file element token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "data" {
				// We found a data element - estimate its size efficiently
				if err := estimateDataElementSize(decoder, stats); err != nil {
					return errkit.Wrap(err, "failed to estimate data element size")
				}
				depth-- // estimateDataElementSize consumes the end element
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			// Skip character data at this level
			continue
		}
	}

	return nil
}

// estimateDataElementSize efficiently estimates the size of a data element without loading all content
func estimateDataElementSize(decoder *xml.Decoder, stats *types.ExportStats) error {
	var totalBase64Length int64
	const maxSampleSize = 1024 * 1024 // Sample up to 1MB to estimate size
	var sampledLength int64

	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read data element token")
		}

		switch t := tok.(type) {
		case xml.CharData:
			dataLength := int64(len(t))
			totalBase64Length += dataLength

			// Only sample the first part for efficiency
			if sampledLength < maxSampleSize {
				sampledLength += dataLength
			}
		case xml.EndElement:
			if t.Name.Local == "data" {
				// Convert base64 length to original binary size (approximately 3/4 of base64 length)
				if totalBase64Length > 0 {
					originalSize := (totalBase64Length * 3) / 4
					stats.BinaryDataSize += originalSize
				}
				return nil
			}
		}
	}
}

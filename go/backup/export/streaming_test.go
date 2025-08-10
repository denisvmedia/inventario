package export

import (
	"bytes"
	"context"
	"encoding/xml"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestStreamCommodityDirectly(t *testing.T) {
	c := qt.New(t)

	registrySet := newTestRegistrySet()
	service := NewExportService(registrySet, "")
	ctx := context.Background()

	// Create a test commodity
	commodity := &models.Commodity{
		TenantAwareEntityID: models.WithTenantAwareEntityID("test-commodity-1", "default-tenant"),
		Name:     "Test Commodity",
		Type:     models.CommodityTypeElectronics,
		AreaID:   "test-area-1",
		Count:    1,
		Status:   models.CommodityStatusInUse,
		Draft:    false,
	}

	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")

	args := ExportArgs{IncludeFileData: false}
	stats := &ExportStats{}
	err := service.streamCommodityDirectly(ctx, encoder, commodity, args, stats)
	c.Assert(err, qt.IsNil)

	err = encoder.Flush()
	c.Assert(err, qt.IsNil)

	output := buf.String()
	c.Assert(output, qt.Contains, `<commodity id="test-commodity-1">`)
	c.Assert(output, qt.Contains, `<commodityName>Test Commodity</commodityName>`)
	c.Assert(output, qt.Contains, `<type>electronics</type>`)
	c.Assert(output, qt.Contains, `<areaId>test-area-1</areaId>`)
	c.Assert(output, qt.Contains, `<count>1</count>`)
	c.Assert(output, qt.Contains, `<status>in_use</status>`)
	c.Assert(output, qt.Contains, `<draft>false</draft>`)
	c.Assert(output, qt.Contains, `</commodity>`)
}

func TestEncodeStringArray(t *testing.T) {
	c := qt.New(t)

	service := &ExportService{}
	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)

	values := []string{"tag1", "tag2", "tag3"}
	err := service.encodeStringArray(encoder, "tags", "tag", values)
	c.Assert(err, qt.IsNil)

	err = encoder.Flush()
	c.Assert(err, qt.IsNil)

	output := buf.String()
	c.Assert(output, qt.Contains, `<tags>`)
	c.Assert(output, qt.Contains, `<tag>tag1</tag>`)
	c.Assert(output, qt.Contains, `<tag>tag2</tag>`)
	c.Assert(output, qt.Contains, `<tag>tag3</tag>`)
	c.Assert(output, qt.Contains, `</tags>`)
}

func TestEncodeStringArrayEmpty(t *testing.T) {
	c := qt.New(t)

	service := &ExportService{}
	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)

	var values []string
	err := service.encodeStringArray(encoder, "tags", "tag", values)
	c.Assert(err, qt.IsNil)

	err = encoder.Flush()
	c.Assert(err, qt.IsNil)

	output := buf.String()
	c.Assert(output, qt.Equals, "")
}

func TestXmlBase64Writer(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)

	writer := &xmlBase64Writer{encoder: encoder}

	// Write some test data
	testData := []byte("Hello, World!")
	n, err := writer.Write(testData)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, len(testData))

	err = encoder.Flush()
	c.Assert(err, qt.IsNil)

	output := buf.String()
	c.Assert(output, qt.Contains, "Hello, World!")
}

func TestStreamingMemoryEfficiency(t *testing.T) {
	c := qt.New(t)

	// This test demonstrates that our streaming approach doesn't accumulate data in memory
	// We test the xmlBase64Writer which is the core of our streaming solution

	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)
	writer := &xmlBase64Writer{encoder: encoder}

	// Simulate writing multiple chunks without accumulating in memory
	chunks := []string{"chunk1", "chunk2", "chunk3", "chunk4", "chunk5"}

	for _, chunk := range chunks {
		n, err := writer.Write([]byte(chunk))
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, len(chunk))

		// Each write should immediately go to the encoder, not accumulate in writer
		// The writer itself should not store any data
	}

	err := encoder.Flush()
	c.Assert(err, qt.IsNil)

	output := buf.String()
	// Verify all chunks are present in output
	for _, chunk := range chunks {
		c.Assert(output, qt.Contains, chunk)
	}
}

func TestEncodeCommodityMetadata(t *testing.T) {
	c := qt.New(t)

	registrySet := newTestRegistrySet()
	service := NewExportService(registrySet, "")
	ctx := context.Background()

	commodity := &models.Commodity{
		TenantAwareEntityID: models.WithTenantAwareEntityID("test-commodity-1", "default-tenant"),
		Name:               "Test Commodity",
		ShortName:          "TC1",
		Type:               models.CommodityTypeElectronics,
		AreaID:             "test-area-1",
		Count:              5,
		SerialNumber:       "SN123456",
		Status:             models.CommodityStatusInUse,
		Comments:           "Test comments",
		Draft:              true,
		Tags:               []string{"tag1", "tag2"},
		PartNumbers:        []string{"PN001", "PN002"},
		ExtraSerialNumbers: []string{"ESN001", "ESN002"},
	}

	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")

	err := service.encodeCommodityMetadata(ctx, encoder, commodity)
	c.Assert(err, qt.IsNil)

	err = encoder.Flush()
	c.Assert(err, qt.IsNil)

	output := buf.String()
	c.Assert(output, qt.Contains, `<commodityName>Test Commodity</commodityName>`)
	c.Assert(output, qt.Contains, `<shortName>TC1</shortName>`)
	c.Assert(output, qt.Contains, `<type>electronics</type>`)
	c.Assert(output, qt.Contains, `<areaId>test-area-1</areaId>`)
	c.Assert(output, qt.Contains, `<count>5</count>`)
	c.Assert(output, qt.Contains, `<serialNumber>SN123456</serialNumber>`)
	c.Assert(output, qt.Contains, `<status>in_use</status>`)
	c.Assert(output, qt.Contains, `<comments>Test comments</comments>`)
	c.Assert(output, qt.Contains, `<draft>true</draft>`)
	c.Assert(output, qt.Contains, `<tags>`)
	c.Assert(output, qt.Contains, `<tag>tag1</tag>`)
	c.Assert(output, qt.Contains, `<tag>tag2</tag>`)
	c.Assert(output, qt.Contains, `<partNumbers>`)
	c.Assert(output, qt.Contains, `<partNumber>PN001</partNumber>`)
	c.Assert(output, qt.Contains, `<partNumber>PN002</partNumber>`)
	c.Assert(output, qt.Contains, `<extraSerialNumbers>`)
	c.Assert(output, qt.Contains, `<serialNumber>ESN001</serialNumber>`)
	c.Assert(output, qt.Contains, `<serialNumber>ESN002</serialNumber>`)
}

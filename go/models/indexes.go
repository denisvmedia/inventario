package models

// LocationAreas represents an index of areas for a given location.
// Map key is a LocationID and value is a slice of AreaIDs.
type LocationAreas map[string][]string

// AreaCommodities represents an index of commodities for a given area.
// Map key is an AreaID and value is a slice of CommodityIDs.
type AreaCommodities map[string][]string

// CommodityImages represents an index of images for a given commodity.
// Map key is a CommodityID and value is a slice of ImageIDs.
type CommodityImages map[string][]string

// CommodityManuals represents an index of manuals for a given commodity.
// Map key is a CommodityID and value is a slice of ManualIDs.
type CommodityManuals map[string][]string

// CommodityInvoices represents an index of invoices for a given commodity.
// Map key is a CommodityID and value is a slice of InvoiceIDs.
type CommodityInvoices map[string][]string

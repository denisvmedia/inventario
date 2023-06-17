package models

// LocationAreas represents an index of areas for a given location.
// Map key is a LocationID and value is a slice of AreaIDs.
type LocationAreas map[string][]string

// AreaCommmodities represents an index of commodities for a given area.
// Map key is an AreaID and value is a slice of CommodityIDs.
type AreaCommmodities map[string][]string

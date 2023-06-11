package models

// LocationAreas represents an index of areas for a given location.
// Map key is a LocationID and value is a slice of AreaIDs.
type LocationAreas map[string][]string

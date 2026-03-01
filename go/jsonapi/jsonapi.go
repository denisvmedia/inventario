package jsonapi

func statusCodeDef(origStatus, defaultStatus int) int {
	if origStatus == 0 {
		origStatus = defaultStatus
	}
	return origStatus
}

// computeTotalPages calculates the total number of pages for pagination.
// Returns 0 when total is 0 (no items), 1 when perPage <= 0, otherwise uses ceiling division.
func computeTotalPages(total, perPage int) int {
	if total == 0 {
		return 0
	}
	if perPage <= 0 {
		return 1
	}
	return (total + perPage - 1) / perPage
}

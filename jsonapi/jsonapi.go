package jsonapi

func statusCodeDef(origStatus, defaultStatus int) int {
	if origStatus == 0 {
		origStatus = defaultStatus
	}
	return origStatus
}

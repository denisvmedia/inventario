package models

import (
	"regexp"
)

var (
	reNotEmpty = regexp.MustCompile(`^(\S|\S.*\S)$`)
)

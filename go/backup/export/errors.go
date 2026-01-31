package export

import (
	"github.com/go-extras/errx"
)

var (
	ErrUnsupportedExportType = errx.NewSentinel("unsupported export type")
)

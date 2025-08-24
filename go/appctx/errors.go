package appctx

import (
	"errors"
)

var (
	ErrUserContextRequired = errors.New("user context required")
)

package rules

import (
	"github.com/jellydator/validation"
)

var (
	// ErrIsEmpty is the error that returns when a value is empty.
	ErrIsEmpty = validation.NewError("validation_is_empty", "cannot be blank")
)

var NotEmpty validation.Rule = notEmptyRule{}

type notEmptyRule struct{}

func (n notEmptyRule) Validate(value any) error {
	if IsEmpty(value) {
		return ErrIsEmpty
	}
	return nil
}

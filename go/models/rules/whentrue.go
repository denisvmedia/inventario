package rules

import (
	"errors"

	"github.com/jellydator/validation"
)

type WhenTrueRule struct {
	Value bool
	Rules []validation.Rule
}

var (
	_ validation.Rule = (*WhenTrueRule)(nil)
)

// WhenTrue returns a new WhenTrueRule.
// It applies the given rules when the Value flag is true.
func WhenTrue(draft bool, rules ...validation.Rule) WhenTrueRule {
	return WhenTrueRule{
		Value: draft,
		Rules: rules,
	}
}

// WithRules returns a new WhenTrueRule with the given rules added.
//
// The returned rule owns its slice. `ret := r` copies the slice HEADER, so an
// append with spare capacity would write into the receiver's backing array and
// mutate a rule the caller still holds. The `copy` that used to sit here was a
// no-op — it copied into ret.Rules, which is r.Rules (#2131).
func (r WhenTrueRule) WithRules(rules ...validation.Rule) WhenTrueRule {
	ret := r
	ret.Rules = make([]validation.Rule, 0, len(r.Rules)+len(rules))
	ret.Rules = append(ret.Rules, r.Rules...)
	ret.Rules = append(ret.Rules, rules...)
	return ret
}

// Validate implements the validation.Rule interface.
// It checks the following conditions:
// 1. If the Value flag is true, it applies the given rules.
// 2. If the Value flag is false, it does not apply any rules.
func (r WhenTrueRule) Validate(value any) error {
	if !r.Value {
		return nil
	}

	var errs []error
	for _, rule := range r.Rules {
		errs = append(errs, rule.Validate(value))
	}

	return errors.Join(errs...)
}

package models_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestCurrency_IsValid(t *testing.T) {
	t.Run("valid currency code", func(t *testing.T) {
		c := qt.New(t)

		c.Assert(models.Currency("USD").IsValid(), qt.IsTrue)
		c.Assert(models.Currency("EUR").IsValid(), qt.IsTrue)
		c.Assert(models.Currency("JPY").IsValid(), qt.IsTrue)
	})

	t.Run("invalid currency code", func(t *testing.T) {
		c := qt.New(t)

		c.Assert(models.Currency("").IsValid(), qt.IsFalse)
		c.Assert(models.Currency("FOO").IsValid(), qt.IsFalse)
		c.Assert(models.Currency("US").IsValid(), qt.IsFalse)
		c.Assert(models.Currency("USDD").IsValid(), qt.IsFalse)
	})
}

func TestCurrency_Validate(t *testing.T) {
	t.Run("valid currency code", func(t *testing.T) {
		c := qt.New(t)

		c.Assert(models.Currency("USD").Validate(), qt.IsNil)
		c.Assert(models.Currency("EUR").Validate(), qt.IsNil)
		c.Assert(models.Currency("JPY").Validate(), qt.IsNil)
	})

	t.Run("invalid currency code", func(t *testing.T) {
		c := qt.New(t)

		c.Assert(models.Currency("").Validate(), qt.IsNotNil)
		c.Assert(models.Currency("FOO").Validate(), qt.IsNotNil)
		c.Assert(models.Currency("US").Validate(), qt.IsNotNil)
		c.Assert(models.Currency("USDD").Validate(), qt.IsNotNil)
	})
}

package models_test

import (
	"context"
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
	c := qt.New(t)

	currency := models.Currency("USD")
	err := currency.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorIs, models.ErrMustUseValidateWithContext)
}

func TestCurrency_ValidateWithContext(t *testing.T) {
	t.Run("valid currency code", func(t *testing.T) {
		c := qt.New(t)

		ctx := context.Background()
		c.Assert(models.Currency("USD").ValidateWithContext(ctx), qt.IsNil)
		c.Assert(models.Currency("EUR").ValidateWithContext(ctx), qt.IsNil)
		c.Assert(models.Currency("JPY").ValidateWithContext(ctx), qt.IsNil)
	})

	t.Run("invalid currency code", func(t *testing.T) {
		c := qt.New(t)

		ctx := context.Background()
		c.Assert(models.Currency("").ValidateWithContext(ctx), qt.IsNotNil)
		c.Assert(models.Currency("FOO").ValidateWithContext(ctx), qt.IsNotNil)
		c.Assert(models.Currency("US").ValidateWithContext(ctx), qt.IsNotNil)
		c.Assert(models.Currency("USDD").ValidateWithContext(ctx), qt.IsNotNil)
	})
}

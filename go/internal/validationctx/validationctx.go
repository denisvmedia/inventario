package validationctx

import (
	"context"
)

type ctxValueKey string

const mainCurrencyCtxKey ctxValueKey = "mainCurrency"

func MainCurrencyFromContext(ctx context.Context) (string, error) {
	mainCurrency, ok := ctx.Value(mainCurrencyCtxKey).(string)
	if !ok {
		return "", ErrMainCurrencyNotSet
	}
	return mainCurrency, nil
}

func WithMainCurrency(ctx context.Context, mainCurrency string) context.Context {
	return context.WithValue(ctx, mainCurrencyCtxKey, mainCurrency)
}

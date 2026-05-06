package validationctx

import (
	"context"
)

type ctxValueKey string

const groupCurrencyCtxKey ctxValueKey = "groupCurrency"

func GroupCurrencyFromContext(ctx context.Context) (string, error) {
	groupCurrency, ok := ctx.Value(groupCurrencyCtxKey).(string)
	if !ok {
		return "", ErrGroupCurrencyNotSet
	}
	return groupCurrency, nil
}

func WithGroupCurrency(ctx context.Context, groupCurrency string) context.Context {
	return context.WithValue(ctx, groupCurrencyCtxKey, groupCurrency)
}

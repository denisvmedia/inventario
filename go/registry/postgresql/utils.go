package postgresql

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type txOrPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// generateID generates a new UUID string
func generateID() string {
	return uuid.New().String()
}

type txKey string

const txKeyVal txKey = "tx"

func ContextWithTransaction(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKeyVal, tx)
}

func TransactionFromContext(ctx context.Context) pgx.Tx {
	tx, ok := ctx.Value(txKeyVal).(pgx.Tx)
	if !ok {
		return nil
	}
	return tx
}

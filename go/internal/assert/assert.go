package assert

import (
	"log/slog"
)

func NoError(err error) {
	if err != nil {
		slog.Error("unexpected error", "error", err)
		panic(err)
	}
}

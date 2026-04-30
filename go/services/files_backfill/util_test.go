package files_backfill_test

import (
	"crypto/rand"
	"encoding/hex"
)

func randomHex(n int) string {
	buf := make([]byte, n/2)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

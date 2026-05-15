package secrets

import (
	"crypto/sha256"
	"hash"
)

// sha256NewHash is the hash constructor passed to hkdf.New. Kept in its
// own file so callers can swap it in tests (build-tagged) without
// touching the production crypto code.
func sha256NewHash() hash.Hash {
	return sha256.New()
}

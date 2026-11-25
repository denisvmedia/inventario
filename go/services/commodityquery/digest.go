package commodityquery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/registry"
)

// Digest names a set of item ids: the hex SHA-256 of the ids sorted and
// joined with commas. The order the ids come in does not matter, so two
// queries that select the same items have the same digest, and a set that
// lost one item and gained another does not, even with the same count.
func Digest(ids []string) string {
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, ",")))
	return hex.EncodeToString(sum[:])
}

// SelectionDigest returns the Digest of the items q selects. When q selects
// more than limit items, ok is false and there is no digest: a bulk action
// cannot take that selection anyway.
func SelectionDigest(ctx context.Context, set *registry.Set, q Query, limit int) (digest string, ok bool, err error) {
	opts, err := Resolve(ctx, set, q)
	if err != nil {
		return "", false, err
	}
	ids, err := set.CommodityRegistry.ListIDs(ctx, opts, limit+1)
	if err != nil {
		return "", false, errxtrace.Wrap("failed to list item ids", err)
	}
	if len(ids) > limit {
		return "", false, nil
	}
	return Digest(ids), true, nil
}

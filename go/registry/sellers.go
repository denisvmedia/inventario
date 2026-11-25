package registry

import (
	"slices"
	"strings"

	"go.5x5.cz/inventario/models"
)

// SortSellers orders sellers by name key, then by name as typed, then by ID,
// the order both backends list them in. Sorting in Go rather than in SQL keeps
// the order independent of the database collation.
func SortSellers(sellers []*models.Seller) {
	slices.SortStableFunc(sellers, func(a, b *models.Seller) int {
		if c := strings.Compare(a.NameKey, b.NameKey); c != 0 {
			return c
		}
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
}

// FilterSellers returns the sellers whose name key contains the key of
// query. An empty query returns sellers unchanged.
func FilterSellers(sellers []*models.Seller, query string) []*models.Seller {
	key := models.SellerNameKey(query)
	if key == "" {
		return sellers
	}
	out := make([]*models.Seller, 0, len(sellers))
	for _, s := range sellers {
		if strings.Contains(s.NameKey, key) {
			out = append(out, s)
		}
	}
	return out
}

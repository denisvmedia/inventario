package commodityquery

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// Resolve turns q into the registry options for the group of set: it adds,
// for every search term, the slugs of the item tags whose label contains the
// term, and, for a backend that does not join loans itself
// (registry.NativeLentOutFilterer), the IDs of the items on an open loan.
func Resolve(ctx context.Context, set *registry.Set, q Query) (registry.CommodityListOptions, error) {
	opts := q.ListOptions()

	if set.TagRegistry != nil {
		for _, term := range registry.SplitSearchTerms(q.Text) {
			slugs, err := set.TagRegistry.SlugsByLabel(ctx, models.TagKindCommodity, term)
			if err != nil {
				return registry.CommodityListOptions{}, errxtrace.Wrap("failed to resolve tag labels", err)
			}
			if len(slugs) == 0 {
				continue
			}
			if opts.SearchTagLabels == nil {
				opts.SearchTagLabels = make(map[string][]string)
			}
			opts.SearchTagLabels[term] = slugs
		}
	}

	if q.LentOut != nil && set.CommodityLoanRegistry != nil {
		if _, native := set.CommodityRegistry.(registry.NativeLentOutFilterer); !native {
			ids, err := OpenLoanCommodityIDs(ctx, set.CommodityLoanRegistry)
			if err != nil {
				return registry.CommodityListOptions{}, err
			}
			opts.OpenLoanCommodityIDs = ids
		}
	}
	return opts, nil
}

// Page is one page of the items a Query selects.
type Page struct {
	Commodities []*models.Commodity
	// Total counts every item the query selects, not only this page.
	Total int
	// Matches says, per item ID on this page, which fields the search
	// text was found in. It is nil when the query has no text.
	Matches map[string][]registry.CommoditySearchMatch
}

// List runs q against the group of set and returns the page at offset.
func List(ctx context.Context, set *registry.Set, q Query, offset, limit int) (Page, error) {
	opts, err := Resolve(ctx, set, q)
	if err != nil {
		return Page{}, err
	}
	commodities, total, err := set.CommodityRegistry.ListPaginated(ctx, offset, limit, opts)
	if err != nil {
		return Page{}, errxtrace.Wrap("failed to list commodities", err)
	}
	return Page{
		Commodities: commodities,
		Total:       total,
		Matches:     Explain(commodities, opts),
	}, nil
}

// Explain reports, per commodity, the fields the search text of opts was
// found in. It returns nil when opts has no search text. Every commodity a
// registry returns for opts matches, so each gets at least one entry.
func Explain(commodities []*models.Commodity, opts registry.CommodityListOptions) map[string][]registry.CommoditySearchMatch {
	terms := registry.SplitSearchTerms(opts.Search)
	if len(terms) == 0 {
		return nil
	}
	out := make(map[string][]registry.CommoditySearchMatch, len(commodities))
	for _, c := range commodities {
		if c == nil {
			continue
		}
		if matches, ok := registry.MatchCommoditySearch(c, terms, opts.SearchTagLabels); ok {
			out[c.ID] = matches
		}
	}
	return out
}

// OpenLoanCommodityIDs collects every commodity ID in the current group that
// has at least one open loan. It pages through the loan registry until the
// total is reached, so the filter stays correct past one page of loans.
func OpenLoanCommodityIDs(ctx context.Context, loanReg registry.CommodityLoanRegistry) ([]string, error) {
	const pageSize = 1000
	seen := make(map[string]struct{})
	var ids []string
	offset := 0
	for {
		loans, total, err := loanReg.ListPaginated(ctx, offset, pageSize, registry.LoanListOptions{State: registry.LoanStateOpen})
		if err != nil {
			return nil, errxtrace.Wrap("failed to list open loans", err)
		}
		for _, l := range loans {
			if l == nil {
				continue
			}
			if _, dup := seen[l.CommodityID]; dup {
				continue
			}
			seen[l.CommodityID] = struct{}{}
			ids = append(ids, l.CommodityID)
		}
		offset += len(loans)
		// An empty page stops the loop even if the reported total is
		// higher, so a registry that overstates it cannot spin forever.
		if len(loans) == 0 || offset >= total {
			break
		}
	}
	return ids, nil
}

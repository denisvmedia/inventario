package services

import (
	"context"

	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/commodityquery"
)

// InboxCheckKey names one of the inbox's optional quality checks.
type InboxCheckKey string

const (
	InboxCheckNoLocation      InboxCheckKey = "no_location"
	InboxCheckNoPhoto         InboxCheckKey = "no_photo"
	InboxCheckNoPurchasePrice InboxCheckKey = "no_purchase_price"
	InboxCheckNoPurchaseDate  InboxCheckKey = "no_purchase_date"
	InboxCheckUntagged        InboxCheckKey = "untagged"
)

// InboxCheck is an optional quality check next to the inbox (#2794): the
// items that lack one piece of information. Missing information is not
// unfinished work, so the checks stay out of the inbox and its counts.
//
// Each check is an item query of the contract, and Count is the total the
// items list reports for that query, so a link built from Query opens a list
// that shows exactly Count items.
type InboxCheck struct {
	Key   InboxCheckKey
	Query commodityquery.Query
	Count int
}

// inboxCheckQuery is the query of a check: the items list's default view
// (items in use that are not marked as draft; the inbox lists the drafts)
// narrowed by one condition.
func inboxCheckQuery(narrow func(*commodityquery.Query)) commodityquery.Query {
	q := commodityquery.Default()
	q.IncludeInactive = false
	narrow(&q)
	return q
}

// InboxChecks returns the checks in display order, without counts.
func InboxChecks() []InboxCheck {
	return []InboxCheck{
		{Key: InboxCheckNoLocation, Query: inboxCheckQuery(func(q *commodityquery.Query) { q.Unassigned = true })},
		{Key: InboxCheckNoPhoto, Query: inboxCheckQuery(func(q *commodityquery.Query) { q.NoPhoto = true })},
		{Key: InboxCheckNoPurchasePrice, Query: inboxCheckQuery(func(q *commodityquery.Query) { q.NoPurchasePrice = true })},
		{Key: InboxCheckNoPurchaseDate, Query: inboxCheckQuery(func(q *commodityquery.Query) { q.NoPurchaseDate = true })},
		{Key: InboxCheckUntagged, Query: inboxCheckQuery(func(q *commodityquery.Query) { q.Tags.Untagged = true })},
	}
}

// Checks counts the quality checks in regSet's group, in one transaction,
// through the WHERE clause of the items list.
func (s *InboxService) Checks(ctx context.Context, regSet *registry.Set) ([]InboxCheck, error) {
	checks := InboxChecks()
	qs := make([]commodityquery.Query, len(checks))
	for i, c := range checks {
		qs[i] = c.Query
	}
	counts, err := commodityquery.CountEach(ctx, regSet, qs)
	if err != nil {
		return nil, err
	}
	for i := range checks {
		checks[i].Count = counts[i]
	}
	return checks, nil
}

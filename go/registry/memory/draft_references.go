package memory

import (
	"maps"
	"time"

	"go.5x5.cz/inventario/models"
)

// The open drafts of a group name item types and custom fields in their
// form values: the type's slug under "type" and the field values by field
// ID under "custom_fields". The item type and custom field factories count
// those drafts as users and rewrite them when a type or field is deleted.
//
// A promotion holds the draft factory's writes mutex while it creates the
// item, which read-locks the item type and custom field mutexes. A delete
// therefore takes writes (lockWrites) before its own mutex.

// lockWrites takes the draft factory's writes mutex and returns its unlock.
// A nil factory has nothing to lock.
func (f *CommodityDraftRegistryFactory) lockWrites() func() {
	if f == nil {
		return func() {}
	}
	f.writes.Lock()
	return f.writes.Unlock
}

// eachOpen calls fn with the values of every open draft, of groupID when it
// is not empty.
func (f *CommodityDraftRegistryFactory) eachOpen(groupID string, fn func(models.CommodityDraftValues)) {
	if f == nil {
		return
	}
	f.base.lock.RLock()
	defer f.base.lock.RUnlock()
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		d := pair.Value
		if d.Status == models.CommodityDraftOpen && (groupID == "" || d.GroupID == groupID) {
			fn(d.Values)
		}
	}
}

// countOpen returns how many open drafts of groupID match.
func (f *CommodityDraftRegistryFactory) countOpen(groupID string, match func(models.CommodityDraftValues) bool) int {
	n := 0
	f.eachOpen(groupID, func(v models.CommodityDraftValues) {
		if match(v) {
			n++
		}
	})
	return n
}

// rewriteOpen replaces the values of each open draft of groupID that edit
// changes with what edit returns, and gives that draft a new revision. edit
// gets a copy it may change, and reports whether it did. The caller holds
// writes.
func (f *CommodityDraftRegistryFactory) rewriteOpen(groupID string, edit func(models.CommodityDraftValues) bool) {
	if f == nil {
		return
	}
	f.base.lock.Lock()
	defer f.base.lock.Unlock()
	now := time.Now()
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		d := pair.Value
		if d.Status != models.CommodityDraftOpen || d.GroupID != groupID {
			continue
		}
		values := maps.Clone(d.Values)
		if fields := values.DraftCustomFields(); fields != nil {
			values["custom_fields"] = maps.Clone(fields)
		}
		if !edit(values) {
			continue
		}
		cp := *d
		cp.Values = values
		cp.Revision++
		cp.UpdatedAt = now
		pair.Value = &cp
	}
}

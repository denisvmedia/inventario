package memory

import (
	"context"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// creatorStore adapts a base registry whose rows carry a creator to the
// groupConfiguration hand-over the user purge runs on group rows: tags,
// sellers, item history, loans, service records, supply links, maintenance,
// reminder send records and restores.
type creatorStore[T any, P registry.PIDable[T]] struct {
	base *Registry[T, P]
}

// handOverStore returns the hand-over adapter of base.
func handOverStore[T any, P registry.PIDable[T]](base *Registry[T, P]) groupConfiguration {
	return creatorStore[T, P]{base: base}
}

// groupsCreatedBy returns the groups of tenantID with a row userID created.
func (s creatorStore[T, P]) groupsCreatedBy(tenantID, userID string) []string {
	s.base.lock.RLock()
	defer s.base.lock.RUnlock()
	seen := map[string]bool{}
	var out []string
	for pair := s.base.items.Oldest(); pair != nil; pair = pair.Next() {
		row, ok := any(pair.Value).(models.TenantGroupAware)
		if !ok || row.GetTenantID() != tenantID || row.GetCreatedByUserID() != userID || seen[row.GetGroupID()] {
			continue
		}
		seen[row.GetGroupID()] = true
		out = append(out, row.GetGroupID())
	}
	return out
}

// reattributeCreator gives the rows userID created in tenantID to
// heirs[groupID] and returns how many each group handed over. A row whose
// group has no heir keeps its creator. Each changed row is a copy, so a
// reader holding the stored one sees no change.
func (s creatorStore[T, P]) reattributeCreator(tenantID, userID string, heirs map[string]string) map[string]int {
	s.base.lock.Lock()
	defer s.base.lock.Unlock()
	moved := map[string]int{}
	for pair := s.base.items.Oldest(); pair != nil; pair = pair.Next() {
		row, ok := any(pair.Value).(models.TenantGroupAware)
		if !ok || row.GetTenantID() != tenantID || row.GetCreatedByUserID() != userID {
			continue
		}
		heir := heirs[row.GetGroupID()]
		if heir == "" {
			continue
		}
		cp := P(copyOut((*T)(pair.Value)))
		if changed, ok := any(cp).(models.TenantGroupAware); ok {
			changed.SetCreatedByUserID(heir)
			pair.Value = cp
			moved[row.GetGroupID()]++
		}
	}
	return moved
}

// currencyMigrationAuditStore hands over the audit rows of currency changes,
// which the memory backend keeps in a slice beside the migrations.
type currencyMigrationAuditStore struct {
	f *CurrencyMigrationRegistryFactory
}

func (s currencyMigrationAuditStore) groupsCreatedBy(tenantID, userID string) []string {
	s.f.auditLock.RLock()
	defer s.f.auditLock.RUnlock()
	seen := map[string]bool{}
	var out []string
	for _, row := range *s.f.auditRows {
		if row.TenantID == tenantID && row.CreatedByUserID == userID && !seen[row.GroupID] {
			seen[row.GroupID] = true
			out = append(out, row.GroupID)
		}
	}
	return out
}

func (s currencyMigrationAuditStore) reattributeCreator(tenantID, userID string, heirs map[string]string) map[string]int {
	s.f.auditLock.Lock()
	defer s.f.auditLock.Unlock()
	moved := map[string]int{}
	for i, row := range *s.f.auditRows {
		heir := heirs[row.GroupID]
		if row.TenantID != tenantID || row.CreatedByUserID != userID || heir == "" {
			continue
		}
		cp := *row
		cp.CreatedByUserID = heir
		(*s.f.auditRows)[i] = &cp
		moved[row.GroupID]++
	}
	return moved
}

// groupRowStore is one store of group rows the user purge hands over, named
// by the postgres table it stands for, the name the audit row counts its
// rows under.
type groupRowStore struct {
	table string
	store groupConfiguration
}

// groupRowStores returns the hand-over adapters of the group rows the user
// purge gives to another member. Item types, custom fields and delivered
// documents are wired with their own setters.
func groupRowStores(fs *registry.FactorySet) []groupRowStore {
	var stores []groupRowStore
	add := func(table string, store groupConfiguration) {
		stores = append(stores, groupRowStore{table: table, store: store})
	}
	if f, ok := fs.TagRegistryFactory.(*TagRegistryFactory); ok {
		add("tags", handOverStore(f.base))
	}
	if f, ok := fs.SellerRegistryFactory.(*SellerRegistryFactory); ok {
		add("sellers", handOverStore(f.base))
	}
	if f, ok := fs.CommodityEventRegistryFactory.(*CommodityEventRegistryFactory); ok {
		add("commodity_events", handOverStore(f.base))
	}
	if f, ok := fs.CommodityLoanRegistryFactory.(*CommodityLoanRegistryFactory); ok {
		add("commodity_loans", handOverStore(f.base))
	}
	if f, ok := fs.CommodityServiceRegistryFactory.(*CommodityServiceRegistryFactory); ok {
		add("commodity_services", handOverStore(f.base))
	}
	if f, ok := fs.SupplyLinkRegistryFactory.(*SupplyLinkRegistryFactory); ok {
		add("commodity_supply_links", handOverStore(f.base))
	}
	if f, ok := fs.MaintenanceScheduleRegistryFactory.(*MaintenanceScheduleRegistryFactory); ok {
		add("maintenance_schedules", handOverStore(f.base))
	}
	if r, ok := fs.MaintenanceReminderRegistry.(*MaintenanceReminderRegistry); ok {
		add("maintenance_reminders", handOverStore(r.baseMaintenanceReminderRegistry))
	}
	if r, ok := fs.WarrantyReminderRegistry.(*WarrantyReminderRegistry); ok {
		add("warranty_reminders", handOverStore(r.baseWarrantyReminderRegistry))
	}
	if r, ok := fs.StorageQuotaReminderRegistry.(*StorageQuotaReminderRegistry); ok {
		add("storage_quota_reminders", handOverStore(r.baseStorageQuotaReminderRegistry))
	}
	if f, ok := fs.CurrencyMigrationRegistryFactory.(*CurrencyMigrationRegistryFactory); ok {
		add("currency_migrations", handOverStore(f.baseRegistry))
		add("currency_migration_audit_rows", currencyMigrationAuditStore{f: f})
	}
	if f, ok := fs.RestoreOperationRegistryFactory.(*RestoreOperationRegistryFactory); ok {
		add("restore_operations", handOverStore(f.baseRestoreOperationRegistry))
	}
	if f, ok := fs.RestoreStepRegistryFactory.(*RestoreStepRegistryFactory); ok {
		add("restore_steps", handOverStore(f.baseRestoreStepRegistry))
	}
	return stores
}

// deleteForUser removes the invites userID created in tenantID and the ones
// userID accepted in any tenant, as the postgres purge does. An invite with
// no UsedBy could be accepted again, so the row goes rather than the field.
func (r *GroupInviteRegistry) deleteForUser(_ context.Context, tenantID, userID string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	var victims []string
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		invite := pair.Value
		created := invite.TenantID == tenantID && invite.CreatedBy == userID
		accepted := invite.UsedBy != nil && *invite.UsedBy == userID
		if created || accepted {
			victims = append(victims, pair.Key)
		}
	}
	for _, id := range victims {
		r.items.Delete(id)
	}
}

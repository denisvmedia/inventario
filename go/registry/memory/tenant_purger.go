package memory

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.TenantPurger = (*TenantPurger)(nil)

// TenantPurger is the in-memory counterpart to postgres.TenantPurger. It
// deletes every tenant-scoped row whose TenantID matches the request via each
// registry's service-mode view, in FK-safe order.
//
// Unlike GroupPurger (which took the individual factories it needed) the
// tenant purge touches almost every registry on the FactorySet — group-scoped
// data AND the tenant-only auth/identity tables that GroupPurger never sees —
// so it accepts the whole FactorySet. The orchestration layer wires it last,
// after every fs.* field is populated.
//
// It does NOT delete the tenants row itself — the orchestration layer drops it
// after the dependents are gone, exactly as postgres.TenantPurger leaves the
// tenants DELETE to its caller.
//
// Three tenant-scoped tables are intentionally NOT purged here: settings,
// commodity_scan_audits and storage_quota_reminders. Their registry interfaces
// expose no enumerable per-row List+Delete (SettingsRegistry is Get/Save/Patch;
// CommodityScanAudit is Record/Count/DeleteOlderThan; StorageQuotaReminder is
// HasSent/CreateOnce/DeleteByGroupThreshold), so there is no interface-level
// handle to remove their rows wholesale. The memory GroupPurger omits
// storage_quota_reminders for the same reason. The postgres purger clears all
// three via raw SQL and is the authoritative production path; the memory
// backend is dev/test only, where these tables are negligible. See the
// postgres purger for the full set.
type TenantPurger struct {
	fs *registry.FactorySet
}

// NewTenantPurger wires a TenantPurger to the populated FactorySet. It must be
// called after every fs.* registry field is set.
func NewTenantPurger(fs *registry.FactorySet) *TenantPurger {
	return &TenantPurger{fs: fs}
}

// PurgeTenantDependents walks each tenant-scoped registry in FK-safe order and
// deletes every row whose TenantID matches. Unlike the postgres variant these
// deletes are not in a single transaction — memory mode is only used for tests
// where partial failure is acceptable.
//
// Ordering mirrors postgres.tenantPurgeOrder: the export/restore + thumbnail
// chains and the commodity subtree drop before their parents, currency
// migrations + their audit rows before location_groups, the auth/identity
// tables before users, and finally location_groups then users (every
// tenant-scoped table FKs user_id/created_by -> users NO ACTION, and the group
// data FKs group_id -> location_groups NO ACTION).
func (r *TenantPurger) PurgeTenantDependents(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return errxtrace.Wrap("tenantID required", registry.ErrFieldRequired)
	}

	fs := r.fs

	// Resolve the tenant's location groups once. Two registries
	// (currency-migration audit rows and group_notification_prefs) only expose
	// a per-(tenant, group) delete — no per-tenant handle — so the purge fans
	// those out across this group set. ListGroupIDsForTenant returns every
	// group id (any status) belonging to the tenant.
	groupIDs, err := r.listGroupIDsForTenant(ctx, tenantID)
	if err != nil {
		return errxtrace.Wrap("failed to list tenant groups for purge", err)
	}

	type step struct {
		name string
		run  func() error
	}
	steps := []step{
		// Restore pipeline (deepest children first).
		{"restore_steps", func() error {
			reg := fs.RestoreStepRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.RestoreStep])
		}},
		{"restore_operations", func() error {
			reg := fs.RestoreOperationRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.RestoreOperation])
		}},
		{"exports", func() error {
			reg := fs.ExportRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.Export])
		}},
		// Thumbnail chain (#2117): slots -> jobs -> files. Both job and slot
		// rows carry a tenant_id, so the generic per-tenant path applies (no
		// per-file subquery scoping like the group purger needs).
		{"user_concurrency_slots", func() error {
			reg := fs.UserConcurrencySlotRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.UserConcurrencySlot])
		}},
		{"thumbnail_generation_jobs", func() error {
			reg := fs.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.ThumbnailGenerationJob])
		}},
		{"files", func() error {
			reg := fs.FileRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.FileEntity])
		}},
		{"commodity_events", func() error {
			reg := fs.CommodityEventRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.CommodityEvent])
		}},
		// Currency-migration audit rows before the migrations. The only delete
		// handle is DeleteAuditRowsByGroup(tenant, group), so fan out across the
		// tenant's groups (mirrors the postgres explicit per-tenant DELETE).
		{"currency_migration_audit_rows", func() error {
			reg := fs.CurrencyMigrationRegistryFactory.CreateServiceRegistry()
			for _, groupID := range groupIDs {
				if _, derr := reg.DeleteAuditRowsByGroup(ctx, tenantID, groupID); derr != nil {
					return derr
				}
			}
			return nil
		}},
		// Maintenance reminders before their schedules; the reminder registry
		// is service-mode only, keyed by schedule, so fan out per matching
		// schedule (mirrors the group purger's DeleteBySchedule path).
		{"maintenance_reminders", func() error {
			scheduleReg := fs.MaintenanceScheduleRegistryFactory.CreateServiceRegistry()
			schedules, listErr := scheduleReg.List(ctx)
			if listErr != nil {
				return listErr
			}
			for _, s := range schedules {
				if s == nil || s.GetTenantID() != tenantID {
					continue
				}
				if _, derr := fs.MaintenanceReminderRegistry.DeleteBySchedule(ctx, s.ID); derr != nil {
					return derr
				}
			}
			return nil
		}},
		{"maintenance_schedules", func() error {
			reg := fs.MaintenanceScheduleRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.MaintenanceSchedule])
		}},
		// Commodity sub-resources before commodities.
		{"commodity_supply_links", func() error {
			reg := fs.SupplyLinkRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.SupplyLink])
		}},
		{"commodity_services", func() error {
			reg := fs.CommodityServiceRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.CommodityService])
		}},
		{"commodity_loans", func() error {
			reg := fs.CommodityLoanRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.CommodityLoan])
		}},
		{"currency_migrations", func() error {
			reg := fs.CurrencyMigrationRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.CurrencyMigration])
		}},
		{"commodities", func() error {
			reg := fs.CommodityRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.Commodity])
		}},
		{"areas", func() error {
			reg := fs.AreaRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.Area])
		}},
		{"locations", func() error {
			reg := fs.LocationRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.Location])
		}},
		{"tags", func() error {
			reg := fs.TagRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.Tag])
		}},
		// Per-user/per-group notification overrides. The only delete handle is
		// DeleteByGroup(tenant, group), so fan out across the tenant's groups
		// (mirrors the postgres per-tenant DELETE).
		{"group_notification_prefs", func() error {
			for _, groupID := range groupIDs {
				if _, derr := fs.GroupNotificationPrefRegistry.DeleteByGroup(ctx, tenantID, groupID); derr != nil {
					return derr
				}
			}
			return nil
		}},
		// Audit logs (nullable tenant_id, no FK to tenants); a plain registry.
		// TenantID is *string, so read it through a nil-safe extractor.
		{"audit_logs", func() error {
			return purgeByTenant(ctx, tenantID, fs.AuditLogRegistry.List, fs.AuditLogRegistry.Delete, func(a *models.AuditLog) string {
				if a.TenantID == nil {
					return ""
				}
				return *a.TenantID
			})
		}},
		// Auth/session/identity tables — all FK user_id -> users NO ACTION, so
		// they drop before users. Each embeds Registry[T] (generic List/Delete);
		// the token tables expose a public TenantID field rather than the
		// TenantAware interface, so read it directly.
		{"login_events", func() error {
			return purgeByTenant(ctx, tenantID, fs.LoginEventRegistry.List, fs.LoginEventRegistry.Delete, tenantAware[models.LoginEvent])
		}},
		{"refresh_tokens", func() error {
			return purgeByTenant(ctx, tenantID, fs.RefreshTokenRegistry.List, fs.RefreshTokenRegistry.Delete, tenantAware[models.RefreshToken])
		}},
		{"email_verifications", func() error {
			return purgeByTenant(ctx, tenantID, fs.EmailVerificationRegistry.List, fs.EmailVerificationRegistry.Delete, func(e *models.EmailVerification) string {
				return e.TenantID
			})
		}},
		{"password_resets", func() error {
			return purgeByTenant(ctx, tenantID, fs.PasswordResetRegistry.List, fs.PasswordResetRegistry.Delete, func(p *models.PasswordReset) string {
				return p.TenantID
			})
		}},
		{"magic_link_tokens", func() error {
			return purgeByTenant(ctx, tenantID, fs.MagicLinkTokenRegistry.List, fs.MagicLinkTokenRegistry.Delete, func(m *models.MagicLinkToken) string {
				return m.TenantID
			})
		}},
		{"user_mfa_secrets", func() error {
			return purgeByTenant(ctx, tenantID, fs.UserMFASecretRegistry.List, fs.UserMFASecretRegistry.Delete, tenantAware[models.UserMFASecret])
		}},
		{"user_oauth_identities", func() error {
			return purgeByTenant(ctx, tenantID, fs.OAuthIdentityRegistry.List, fs.OAuthIdentityRegistry.Delete, tenantAware[models.OAuthIdentity])
		}},
		{"operation_slots", func() error {
			reg := fs.OperationSlotRegistryFactory.CreateServiceRegistry()
			return purgeByTenant(ctx, tenantID, reg.List, reg.Delete, tenantAware[models.OperationSlot])
		}},
		// Membership + invite rows: FK both users and location_groups NO ACTION,
		// so they drop before both. GroupMembership embeds Registry[T], so the
		// generic per-tenant path applies.
		{"group_memberships", func() error {
			return purgeByTenant(ctx, tenantID, fs.GroupMembershipRegistry.List, fs.GroupMembershipRegistry.Delete, tenantAware[models.GroupMembership])
		}},
		{"group_invites", func() error {
			return purgeByTenant(ctx, tenantID, fs.GroupInviteRegistry.List, fs.GroupInviteRegistry.Delete, tenantAware[models.GroupInvite])
		}},
		{"group_invites_audit", func() error {
			return purgeByTenant(ctx, tenantID, fs.GroupInviteAuditRegistry.List, fs.GroupInviteAuditRegistry.Delete, tenantAware[models.GroupInviteAudit])
		}},
		// location_groups before users (location_groups.created_by -> users
		// NO ACTION); after every group-scoped row + currency_migrations.
		{"location_groups", func() error {
			return purgeByTenant(ctx, tenantID, fs.LocationGroupRegistry.List, fs.LocationGroupRegistry.Delete, tenantAware[models.LocationGroup])
		}},
		// users last.
		{"users", func() error {
			return purgeByTenant(ctx, tenantID, fs.UserRegistry.List, fs.UserRegistry.Delete, tenantAware[models.User])
		}},
	}

	for _, s := range steps {
		if err := s.run(); err != nil {
			return errxtrace.Wrap("failed to purge "+s.name, err)
		}
	}
	return nil
}

// tenantAware reads the tenant id off any model whose pointer satisfies
// models.TenantAware. It is the default extractor for purgeByTenant; the few
// token models that expose a public TenantID field but no GetTenantID method
// pass a field-reading closure instead.
func tenantAware[T any](item *T) string {
	if ta, ok := any(item).(models.TenantAware); ok {
		return ta.GetTenantID()
	}
	return ""
}

// purgeByTenant lists everything from a service-mode registry view, keeps only
// rows whose extracted tenant id matches, and deletes each. The service-mode
// List returns all rows (user/group/tenant filtering disabled), so filtering
// happens here. tenantOf yields the row's tenant id.
func purgeByTenant[T any](
	ctx context.Context,
	tenantID string,
	list func(context.Context) ([]*T, error),
	del func(context.Context, string) error,
	tenantOf func(*T) string,
) error {
	items, err := list(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item == nil || tenantOf(item) != tenantID {
			continue
		}
		idable, ok := any(item).(models.IDable)
		if !ok {
			continue
		}
		if err := del(ctx, idable.GetID()); err != nil {
			return err
		}
	}
	return nil
}

// listGroupIDsForTenant returns the ids of every location group belonging to
// the tenant (any status). Used to fan out the two per-(tenant, group)-only
// deletes (currency-migration audit rows, group_notification_prefs) across the
// tenant's groups, since neither registry exposes a per-tenant delete.
func (r *TenantPurger) listGroupIDsForTenant(ctx context.Context, tenantID string) ([]string, error) {
	groups, err := r.fs.LocationGroupRegistry.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(groups))
	for _, g := range groups {
		if g == nil || g.GetTenantID() != tenantID {
			continue
		}
		ids = append(ids, g.ID)
	}
	return ids, nil
}

package store

type TableName string

type TableNames struct {
	Locations               func() TableName
	Areas                   func() TableName
	Commodities             func() TableName
	CommodityEvents         func() TableName
	Settings                func() TableName
	Exports                 func() TableName
	Files                   func() TableName
	RestoreSteps            func() TableName
	RestoreOperations       func() TableName
	Tenants                 func() TableName
	Users                   func() TableName
	ThumbnailGenerationJobs func() TableName
	UserConcurrencySlots    func() TableName
	OperationSlots          func() TableName
	RefreshTokens           func() TableName
	AuditLogs               func() TableName
	EmailVerifications      func() TableName
	PasswordResets          func() TableName
	LocationGroups          func() TableName
	GroupMemberships        func() TableName
	GroupInvites            func() TableName
	GroupInvitesAudit       func() TableName
	Tags                    func() TableName
	CommodityLoans          func() TableName
	CommodityServices       func() TableName
	WarrantyReminders       func() TableName
	CurrencyMigrations      func() TableName
	CurrencyMigrationAudit  func() TableName
}

var DefaultTableNames = TableNames{
	Locations:               func() TableName { return "locations" },
	Areas:                   func() TableName { return "areas" },
	Commodities:             func() TableName { return "commodities" },
	CommodityEvents:         func() TableName { return "commodity_events" },
	Settings:                func() TableName { return "settings" },
	Exports:                 func() TableName { return "exports" },
	Files:                   func() TableName { return "files" },
	RestoreSteps:            func() TableName { return "restore_steps" },
	RestoreOperations:       func() TableName { return "restore_operations" },
	Tenants:                 func() TableName { return "tenants" },
	Users:                   func() TableName { return "users" },
	ThumbnailGenerationJobs: func() TableName { return "thumbnail_generation_jobs" },
	UserConcurrencySlots:    func() TableName { return "user_concurrency_slots" },
	OperationSlots:          func() TableName { return "operation_slots" },
	RefreshTokens:           func() TableName { return "refresh_tokens" },
	AuditLogs:               func() TableName { return "audit_logs" },
	EmailVerifications:      func() TableName { return "email_verifications" },
	PasswordResets:          func() TableName { return "password_resets" },
	LocationGroups:          func() TableName { return "location_groups" },
	GroupMemberships:        func() TableName { return "group_memberships" },
	GroupInvites:            func() TableName { return "group_invites" },
	GroupInvitesAudit:       func() TableName { return "group_invites_audit" },
	Tags:                    func() TableName { return "tags" },
	CommodityLoans:          func() TableName { return "commodity_loans" },
	CommodityServices:       func() TableName { return "commodity_services" },
	WarrantyReminders:       func() TableName { return "warranty_reminders" },
	CurrencyMigrations:      func() TableName { return "currency_migrations" },
	CurrencyMigrationAudit:  func() TableName { return "currency_migration_audit_rows" },
}

// NewTableNames returns the default table names
func NewTableNames() TableNames {
	return DefaultTableNames
}

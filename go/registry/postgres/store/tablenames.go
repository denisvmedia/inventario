package store

type TableName string

type TableNames struct {
	Locations               func() TableName
	Areas                   func() TableName
	Commodities             func() TableName
	Settings                func() TableName
	Images                  func() TableName
	Invoices                func() TableName
	Manuals                 func() TableName
	Exports                 func() TableName
	Files                   func() TableName
	RestoreSteps            func() TableName
	RestoreOperations       func() TableName
	Tenants                 func() TableName
	Users                   func() TableName
	ThumbnailGenerationJobs func() TableName
	UserConcurrencySlots    func() TableName
}

var DefaultTableNames = TableNames{
	Locations:               func() TableName { return "locations" },
	Areas:                   func() TableName { return "areas" },
	Commodities:             func() TableName { return "commodities" },
	Settings:                func() TableName { return "settings" },
	Images:                  func() TableName { return "images" },
	Invoices:                func() TableName { return "invoices" },
	Manuals:                 func() TableName { return "manuals" },
	Exports:                 func() TableName { return "exports" },
	Files:                   func() TableName { return "files" },
	RestoreSteps:            func() TableName { return "restore_steps" },
	RestoreOperations:       func() TableName { return "restore_operations" },
	Tenants:                 func() TableName { return "tenants" },
	Users:                   func() TableName { return "users" },
	ThumbnailGenerationJobs: func() TableName { return "thumbnail_generation_jobs" },
	UserConcurrencySlots:    func() TableName { return "user_concurrency_slots" },
}

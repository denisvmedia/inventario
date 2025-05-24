package migrations

// RegisterMigrators registers all migrators
func RegisterMigrators() {
	Register("memory", NewMemoryMigrator)
	Register("boltdb", NewBoltDBMigrator)
	Register("postgresql", NewPostgreSQLMigrator)
}

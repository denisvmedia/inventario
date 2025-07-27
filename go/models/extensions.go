package models

// PostgreSQL Extensions required for advanced features
//
//migrator:schema:extension name="pg_trgm" if_not_exists="true" comment="Enable trigram similarity search"
//migrator:schema:extension name="btree_gin" if_not_exists="true" comment="Enable GIN indexes on btree types"
type DatabaseExtensions struct{}

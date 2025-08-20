package models

// PostgreSQL Extensions required for advanced features
//
//xmigrator:schema:extension name="pg_trgm" if_not_exists="true" comment="Enable trigram similarity search"
//xmigrator:schema:extension name="btree_gin" if_not_exists="true" comment="Enable GIN indexes on btree types"
type DatabaseExtensions struct{}

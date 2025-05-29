package stubs

// This file demonstrates the difference between old verbose syntax and new simplified syntax

// OLD VERBOSE SYNTAX (still supported for backward compatibility)
//migrator:schema:table name="old_syntax_users"
type OldSyntaxUser struct {
	//migrator:schema:field name="id" type="SERIAL" primary="true" not_null="true"
	ID int `db:"id"`

	//migrator:schema:field name="email" type="VARCHAR(255)" unique="true" not_null="true" index="true"
	Email string `db:"email"`

	//migrator:schema:field name="is_active" type="BOOLEAN" not_null="true" default="true"
	IsActive bool `db:"is_active"`

	//migrator:schema:field name="description" type="TEXT" nullable="true"
	Description *string `db:"description"`
}

// NEW SIMPLIFIED SYNTAX (recommended)
//migrator:schema:table name="new_syntax_users"
type NewSyntaxUser struct {
	//migrator:schema:field name="id" type="SERIAL" primary not_null
	ID int `db:"id"`

	//migrator:schema:field name="email" type="VARCHAR(255)" unique not_null index
	Email string `db:"email"`

	//migrator:schema:field name="is_active" type="BOOLEAN" not_null default="true"
	IsActive bool `db:"is_active"`

	//migrator:schema:field name="description" type="TEXT" nullable
	Description *string `db:"description"`
}

// MIXED SYNTAX (also supported - you can mix both styles)
//migrator:schema:table name="mixed_syntax_posts"
type MixedSyntaxPost struct {
	//migrator:schema:field name="id" type="SERIAL" primary not_null
	ID int `db:"id"`

	//migrator:schema:field name="title" type="VARCHAR(255)" not_null unique="true" index
	Title string `db:"title"`

	//migrator:schema:field name="content" type="TEXT" nullable="true"
	Content *string `db:"content"`

	//migrator:schema:field name="view_count" type="INTEGER" not_null default="0" check="view_count >= 0"
	ViewCount int `db:"view_count"`

	//migrator:schema:field name="is_published" type="BOOLEAN" not_null="false" default="false"
	IsPublished bool `db:"is_published"`
}

// EMBEDDED FIELDS WITH SIMPLIFIED SYNTAX
//migrator:schema:embed
type ModernTimestamps struct {
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null default="CURRENT_TIMESTAMP"
	CreatedAt string `db:"created_at"`

	//migrator:schema:field name="updated_at" type="TIMESTAMP" nullable
	UpdatedAt *string `db:"updated_at"`
}

//migrator:schema:table name="modern_articles"
type ModernArticle struct {
	//migrator:schema:field name="id" type="SERIAL" primary not_null
	ID int `db:"id"`

	//migrator:schema:field name="title" type="VARCHAR(255)" not_null unique index
	Title string `db:"title"`

	//migrator:embedded mode="inline"
	ModernTimestamps

	//migrator:embedded mode="json" name="metadata" type="JSONB" not_null platform.mysql.type="JSON"
	Metadata map[string]interface{} `json:"metadata"`
}

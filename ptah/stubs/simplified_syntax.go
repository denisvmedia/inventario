package stubs

import "time"

//migrator:schema:table name="simplified_users"
type SimplifiedUser struct {
	//migrator:schema:field name="id" type="SERIAL" primary not_null
	ID int `db:"id"`

	//migrator:schema:field name="email" type="VARCHAR(255)" unique not_null
	Email string `db:"email"`

	//migrator:schema:field name="username" type="VARCHAR(100)" unique not_null index
	Username string `db:"username"`

	//migrator:schema:field name="password_hash" type="TEXT" not_null
	PasswordHash string `db:"password_hash"`

	//migrator:schema:field name="is_active" type="BOOLEAN" not_null default="true"
	IsActive bool `db:"is_active"`

	//migrator:schema:field name="is_admin" type="BOOLEAN" not_null default="false"
	IsAdmin bool `db:"is_admin"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null default="CURRENT_TIMESTAMP"
	CreatedAt time.Time `db:"created_at"`

	//migrator:schema:field name="updated_at" type="TIMESTAMP" nullable
	UpdatedAt *time.Time `db:"updated_at"`
}

//migrator:schema:table name="simplified_posts"
type SimplifiedPost struct {
	//migrator:schema:field name="id" type="SERIAL" primary not_null
	ID int `db:"id"`

	//migrator:schema:field name="user_id" type="INTEGER" not_null foreign="simplified_users(id)" foreign_key_name="fk_post_user"
	UserID int `db:"user_id"`

	//migrator:schema:field name="title" type="VARCHAR(255)" not_null index
	Title string `db:"title"`

	//migrator:schema:field name="content" type="TEXT" nullable
	Content *string `db:"content"`

	//migrator:schema:field name="is_published" type="BOOLEAN" not_null default="false"
	IsPublished bool `db:"is_published"`

	//migrator:schema:field name="view_count" type="INTEGER" not_null default="0" check="view_count >= 0"
	ViewCount int `db:"view_count"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null default="CURRENT_TIMESTAMP"
	CreatedAt time.Time `db:"created_at"`
}

//migrator:schema:index name="idx_posts_user_published" fields="user_id,is_published"
var _ = SimplifiedPost{}

// Embedded types with simplified syntax
//migrator:schema:embed
type SimpleTimestamps struct {
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null default="CURRENT_TIMESTAMP"
	CreatedAt time.Time `db:"created_at"`

	//migrator:schema:field name="updated_at" type="TIMESTAMP" nullable
	UpdatedAt *time.Time `db:"updated_at"`
}

//migrator:schema:embed
type SimpleAudit struct {
	//migrator:schema:field name="created_by" type="VARCHAR(100)" nullable
	CreatedBy *string `db:"created_by"`

	//migrator:schema:field name="updated_by" type="VARCHAR(100)" nullable
	UpdatedBy *string `db:"updated_by"`
}

//migrator:schema:table name="simplified_articles"
type SimplifiedArticle struct {
	//migrator:schema:field name="id" type="SERIAL" primary not_null
	ID int `db:"id"`

	//migrator:schema:field name="title" type="VARCHAR(255)" not_null unique
	Title string `db:"title"`

	//migrator:schema:field name="slug" type="VARCHAR(255)" not_null unique index
	Slug string `db:"slug"`

	//migrator:embedded mode="inline"
	SimpleTimestamps

	//migrator:embedded mode="inline" prefix="audit_"
	SimpleAudit

	//migrator:embedded mode="json" name="metadata" type="JSONB" platform.mysql.type="JSON" platform.mariadb.type="LONGTEXT"
	Metadata map[string]interface{} `json:"metadata"`

	//migrator:embedded mode="relation" field="author_id" ref="simplified_users(id)" on_delete="CASCADE"
	Author SimplifiedUser
}

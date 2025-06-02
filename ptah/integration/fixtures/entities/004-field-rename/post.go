package entities

import "time"

//migrator:schema:table name="posts"
type Post struct {
	//migrator:schema:field name="id" type="SERIAL" primary="true"
	ID int64

	//migrator:schema:field name="title" type="VARCHAR(255)" not_null="true"
	Title string

	//migrator:schema:field name="content" type="TEXT" not_null="true"
	Content string

	//migrator:schema:field name="user_id" type="BIGINT" not_null="true"
	UserID int64

	//migrator:schema:field name="status" type="ENUM" enum="draft,published,archived" not_null="true" default="draft"
	Status string

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time
}

//migrator:schema:index table="posts" name="idx_posts_user_id" columns="user_id"
//migrator:schema:index table="posts" name="idx_posts_status" columns="status"
//migrator:schema:foreign_key table="posts" name="fk_posts_user_id" columns="user_id" ref_table="users" ref_columns="id" on_delete="CASCADE"

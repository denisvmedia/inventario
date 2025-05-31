package entities

import "time"

//migrator:schema:table name="posts"
type Post struct {
	//migrator:schema:field name="id" type="SERIAL" primary="true"
	ID int64

	//migrator:schema:field name="user_id" type="INTEGER" not_null="true" foreign_key="users(id)"
	UserID int64

	//migrator:schema:field name="title" type="VARCHAR(255)" not_null="true"
	Title string

	//migrator:schema:field name="content" type="TEXT"
	Content string

	//migrator:schema:field name="published" type="BOOLEAN" not_null="true" default_expr="false"
	Published bool

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time
}

//migrator:schema:index table="posts" name="idx_posts_user_id" columns="user_id"
//migrator:schema:index table="posts" name="idx_posts_published" columns="published"
//migrator:schema:index table="posts" name="idx_posts_title" columns="title"

package main

//migrator:schema:table name=users engine=InnoDB comment="User accounts"
type User struct {
	//migrator:schema:field name=id type="SERIAL" primary=true
	ID int

	//migrator:schema:field name=email type="TEXT" unique=true not_null=true
	Email string

	//migrator:schema:field name=password_hash type="TEXT" not_null=true
	PasswordHash string

	//migrator:schema:field name=role type="ENUM" enum="admin,user,guest" default="user"
	Role string

	//migrator:schema:field name=created_at type="TIMESTAMP" default_fn="NOW()" not_null=true
	CreatedAt string
}

//migrator:schema:table name=posts engine=InnoDB comment="User posts"
type Post struct {
	//migrator:schema:field name=id type="SERIAL" primary=true
	ID int

	//migrator:schema:field name=user_id type="INT" not_null=true foreign="users(id)" foreign_key_name="fk_posts_user"
	UserID int

	//migrator:schema:field name=title type="TEXT" not_null=true
	Title string

	//migrator:schema:field name=content type="TEXT"
	Content string

	//migrator:schema:field name=created_at type="TIMESTAMP" default_fn="NOW()" not_null=true
	CreatedAt string
}

//migrator:schema:index name=idx_posts_user fields="user_id"
var _ = Post{}

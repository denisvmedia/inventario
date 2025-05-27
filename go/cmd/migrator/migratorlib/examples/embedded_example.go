package examples

import (
	"fmt"
	"time"
)

// Example demonstrating embedded field functionality

// Embedded types
//
//migrator:schema:embed
type Timestamps struct {
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default="CURRENT_TIMESTAMP"
	CreatedAt time.Time `db:"created_at"`

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `db:"updated_at"`
}

//migrator:schema:embed
type AuditInfo struct {
	//migrator:schema:field name="by" type="TEXT"
	By string `db:"by"`

	//migrator:schema:field name="reason" type="TEXT"
	Reason string `db:"reason"`
}

//migrator:schema:embed
type Meta struct {
	Author string
	Source string
}

//migrator:schema:table name="users"
type User struct {
	//migrator:schema:field name="id" type="VARCHAR(36)" primary="true" not_null="true"
	ID int `db:"id"`

	//migrator:schema:field name="email" type="VARCHAR(255)" unique="true" not_null="true"
	Email string `db:"email"`
}

// Example table demonstrating all 4 embedded modes
//
//migrator:schema:table name="articles"
type Article struct {
	//migrator:schema:field name="id" type="INTEGER" primary="true" not_null="true"
	ID int `db:"id"`

	//migrator:schema:field name="title" type="VARCHAR(255)" not_null="true"
	Title string `db:"title"`

	// Mode 1: inline - Injects individual fields as separate columns
	//migrator:embedded mode="inline"
	Timestamps // Results in: created_at, updated_at columns

	// Mode 2: inline with prefix - Injects fields with prefix
	//migrator:embedded mode="inline" prefix="audit_"
	AuditInfo // Results in: audit_by, audit_reason columns

	// Mode 3: json - Serializes struct into one JSON/JSONB column
	//migrator:embedded mode="json" name="meta_data" type="JSONB"
	Meta // Results in: meta_data JSONB column

	// Mode 4: relation - Adds foreign key field + constraint
	//migrator:embedded mode="relation" field="author_id" ref="users(id)" on_delete="CASCADE"
	Author User // Results in: author_id INTEGER + FK constraint

	// Mode 5: skip - Ignores this embedded field completely
	//migrator:embedded mode="skip"
	SkippedField Meta // Results in: nothing (ignored)
}

// ExampleUsage demonstrates how to use embedded fields
// This example shows the 4 embedded field modes:
// 1. inline - Injects fields as separate columns
// 2. inline with prefix - Injects fields with prefix
// 3. json - Serializes to JSON/JSONB column
// 4. relation - Creates foreign key relationship
// 5. skip - Ignores the embedded field
//
// Run the migrator on this file to see the generated SQL:
// go run ../main.go embedded_example.go
func ExampleUsage() {
	fmt.Println("This is an example of embedded field usage")
}

package executor_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/schema/builder"
)

func TestGetOrderedCreateStatements(t *testing.T) {
	c := qt.New(t)

	result, err := builder.ParsePackageRecursively("../stubs")
	c.Assert(err, qt.IsNil)

	statements := executor.GetOrderedCreateStatements(result, "postgres")
	c.Assert(len(statements), qt.Equals, len(result.Tables))

	// Verify that each statement contains CREATE TABLE
	for _, statement := range statements {
		c.Assert(statement, qt.Contains, "CREATE TABLE")
	}
}

func TestEmbeddedFieldsInPackageParser(t *testing.T) {
	c := qt.New(t)

	result, err := builder.ParsePackageRecursively("../stubs")
	c.Assert(err, qt.IsNil)

	// Find the articles table statement
	statements := executor.GetOrderedCreateStatements(result, "postgres")
	var articlesSQL string
	for _, statement := range statements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			articlesSQL = statement
			break
		}
	}

	c.Assert(articlesSQL, qt.Not(qt.Equals), "")

	// Verify embedded fields are included
	c.Assert(articlesSQL, qt.Contains, "created_at", qt.Commentf("Should contain created_at from Timestamps"))
	c.Assert(articlesSQL, qt.Contains, "updated_at", qt.Commentf("Should contain updated_at from Timestamps"))
	c.Assert(articlesSQL, qt.Contains, "audit_by", qt.Commentf("Should contain audit_by from AuditInfo"))
	c.Assert(articlesSQL, qt.Contains, "audit_reason", qt.Commentf("Should contain audit_reason from AuditInfo"))
	c.Assert(articlesSQL, qt.Contains, "meta_data", qt.Commentf("Should contain meta_data from Meta"))
	c.Assert(articlesSQL, qt.Contains, "author_id", qt.Commentf("Should contain author_id from User relation"))
}

func TestPlatformSpecificOverrides(t *testing.T) {
	c := qt.New(t)

	result, err := builder.ParsePackageRecursively("../stubs")
	c.Assert(err, qt.IsNil)

	// Test PostgreSQL (default)
	postgresStatements := executor.GetOrderedCreateStatements(result, "postgres")
	var postgresArticlesSQL string
	for _, statement := range postgresStatements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			postgresArticlesSQL = statement
			break
		}
	}
	c.Assert(postgresArticlesSQL, qt.Contains, "meta_data JSONB")

	// Test MySQL (override)
	mysqlStatements := executor.GetOrderedCreateStatements(result, "mysql")
	var mysqlArticlesSQL string
	for _, statement := range mysqlStatements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			mysqlArticlesSQL = statement
			break
		}
	}
	c.Assert(mysqlArticlesSQL, qt.Contains, "meta_data JSON")

	// Test MariaDB (override with check constraint)
	mariadbStatements := executor.GetOrderedCreateStatements(result, "mariadb")
	var mariadbArticlesSQL string
	for _, statement := range mariadbStatements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			mariadbArticlesSQL = statement
			break
		}
	}
	c.Assert(mariadbArticlesSQL, qt.Contains, "meta_data LONGTEXT")
	c.Assert(mariadbArticlesSQL, qt.Contains, "JSON_VALID(meta_data)")
}

package executor

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/builder"
)

func TestGetOrderedCreateStatements(t *testing.T) {
	c := qt.New(t)

	result, err := builder.ParsePackageRecursively("../stubs")
	c.Assert(err, qt.IsNil)

	statements := GetOrderedCreateStatements(result, "postgres")
	c.Assert(len(statements), qt.Equals, len(result.Tables))

	// Verify that each statement contains CREATE TABLE
	for _, statement := range statements {
		c.Assert(strings.Contains(statement, "CREATE TABLE"), qt.IsTrue)
	}
}

func TestEmbeddedFieldsInPackageParser(t *testing.T) {
	c := qt.New(t)

	result, err := builder.ParsePackageRecursively("../stubs")
	c.Assert(err, qt.IsNil)

	// Find the articles table statement
	statements := GetOrderedCreateStatements(result, "postgres")
	var articlesSQL string
	for _, statement := range statements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			articlesSQL = statement
			break
		}
	}

	c.Assert(articlesSQL, qt.Not(qt.Equals), "")

	// Verify embedded fields are included
	c.Assert(strings.Contains(articlesSQL, "created_at"), qt.IsTrue, qt.Commentf("Should contain created_at from Timestamps"))
	c.Assert(strings.Contains(articlesSQL, "updated_at"), qt.IsTrue, qt.Commentf("Should contain updated_at from Timestamps"))
	c.Assert(strings.Contains(articlesSQL, "audit_by"), qt.IsTrue, qt.Commentf("Should contain audit_by from AuditInfo"))
	c.Assert(strings.Contains(articlesSQL, "audit_reason"), qt.IsTrue, qt.Commentf("Should contain audit_reason from AuditInfo"))
	c.Assert(strings.Contains(articlesSQL, "meta_data"), qt.IsTrue, qt.Commentf("Should contain meta_data from Meta"))
	c.Assert(strings.Contains(articlesSQL, "author_id"), qt.IsTrue, qt.Commentf("Should contain author_id from User relation"))
}

func TestPlatformSpecificOverrides(t *testing.T) {
	c := qt.New(t)

	result, err := builder.ParsePackageRecursively("../stubs")
	c.Assert(err, qt.IsNil)

	// Test PostgreSQL (default)
	postgresStatements := GetOrderedCreateStatements(result, "postgres")
	var postgresArticlesSQL string
	for _, statement := range postgresStatements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			postgresArticlesSQL = statement
			break
		}
	}
	c.Assert(strings.Contains(postgresArticlesSQL, "meta_data JSONB"), qt.IsTrue)

	// Test MySQL (override)
	mysqlStatements := GetOrderedCreateStatements(result, "mysql")
	var mysqlArticlesSQL string
	for _, statement := range mysqlStatements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			mysqlArticlesSQL = statement
			break
		}
	}
	c.Assert(strings.Contains(mysqlArticlesSQL, "meta_data JSON"), qt.IsTrue)

	// Test MariaDB (override with check constraint)
	mariadbStatements := GetOrderedCreateStatements(result, "mariadb")
	var mariadbArticlesSQL string
	for _, statement := range mariadbStatements {
		if strings.Contains(statement, "CREATE TABLE articles") {
			mariadbArticlesSQL = statement
			break
		}
	}
	c.Assert(strings.Contains(mariadbArticlesSQL, "meta_data LONGTEXT"), qt.IsTrue)
	c.Assert(strings.Contains(mariadbArticlesSQL, "JSON_VALID(meta_data)"), qt.IsTrue)
}

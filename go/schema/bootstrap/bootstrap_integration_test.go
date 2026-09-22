package bootstrap_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/lib/pq" // PostgreSQL driver

	"go.5x5.cz/inventario/schema/bootstrap"
)

func getPostgresDSNorSkip(t *testing.T) string {
	t.Helper()

	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN environment variable not set")
	}

	// Try to connect to verify PostgreSQL is available
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	return dsn
}

func TestMigrator_Apply_DatabaseConnection_HappyPath(t *testing.T) {
	dsn := getPostgresDSNorSkip(t)
	c := qt.New(t)
	migrator := bootstrap.New()

	args := bootstrap.ApplyArgs{
		DSN: dsn,
		Template: bootstrap.TemplateData{
			Username:                    "inventario",
			UsernameForMigrations:       "inventario",
			UsernameForBackgroundWorker: "inventario",
		},
		DryRun: false,
	}

	err := migrator.Apply(context.Background(), args)
	c.Assert(err, qt.IsNil, qt.Commentf("should successfully apply bootstrap migrations"))
}

func TestMigrator_Apply_Idempotent_HappyPath(t *testing.T) {
	dsn := getPostgresDSNorSkip(t)
	c := qt.New(t)
	migrator := bootstrap.New()

	args := bootstrap.ApplyArgs{
		DSN: dsn,
		Template: bootstrap.TemplateData{
			Username:                    "inventario",
			UsernameForMigrations:       "inventario",
			UsernameForBackgroundWorker: "inventario",
		},
		DryRun: false,
	}

	// Apply migrations first time
	err := migrator.Apply(context.Background(), args)
	c.Assert(err, qt.IsNil, qt.Commentf("first application should succeed"))

	// Apply migrations second time - should be idempotent
	err = migrator.Apply(context.Background(), args)
	c.Assert(err, qt.IsNil, qt.Commentf("second application should succeed (idempotent)"))

	// Apply migrations third time - should still be idempotent
	err = migrator.Apply(context.Background(), args)
	c.Assert(err, qt.IsNil, qt.Commentf("third application should succeed (idempotent)"))
}

func TestMigrator_Apply_TemplateSubstitution_Integration_HappyPath(t *testing.T) {
	dsn := getPostgresDSNorSkip(t)
	c := qt.New(t)
	migrator := bootstrap.New()

	args := bootstrap.ApplyArgs{
		DSN: dsn,
		Template: bootstrap.TemplateData{
			Username:                    "inventario",
			UsernameForMigrations:       "inventario",
			UsernameForBackgroundWorker: "inventario",
		},
		DryRun: false,
	}

	err := migrator.Apply(context.Background(), args)
	c.Assert(err, qt.IsNil, qt.Commentf("should apply with template substitution"))

	// Verify that the template variables were properly substituted by checking database state
	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	// Check if the roles were created (this verifies template substitution worked)
	var roleExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = 'inventario_app')").Scan(&roleExists)
	c.Assert(err, qt.IsNil)
	c.Assert(roleExists, qt.IsTrue, qt.Commentf("inventario_app role should exist"))

	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = 'inventario_migrator')").Scan(&roleExists)
	c.Assert(err, qt.IsNil)
	c.Assert(roleExists, qt.IsTrue, qt.Commentf("inventario_migrator role should exist"))

	// Check if extensions were created.
	var extensionExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm')").Scan(&extensionExists)
	c.Assert(err, qt.IsNil)
	c.Assert(extensionExists, qt.IsTrue, qt.Commentf("pg_trgm extension should exist"))
}

// The tables a migration creates must be readable by the app role, and default
// privileges only reach them when they name the login that creates them, not
// the group role it is a member of (#2520). Asserted against pg_default_acl
// because the login has no password here and cannot create a table to check.
func TestMigrator_Apply_DefaultPrivilegesFollowTheMigrationLogin(t *testing.T) {
	dsn := getPostgresDSNorSkip(t)
	c := qt.New(t)

	const login = "inventario_mig_acl_test"

	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	// Cleanups run last-registered-first, so the close must be registered
	// before the drops that need the connection. A `defer db.Close()` here
	// would run before both and leave the role behind — which would make a
	// second run of this test pass on the first run's leftovers.
	t.Cleanup(func() { db.Close() })
	t.Cleanup(func() {
		// The role owns nothing but its default-ACL entries, which hold the
		// DROP until they are gone.
		_, err := db.Exec("DROP OWNED BY " + login)
		c.Check(err, qt.IsNil, qt.Commentf("drop owned by %s", login))
		_, err = db.Exec("DROP ROLE IF EXISTS " + login)
		c.Check(err, qt.IsNil, qt.Commentf("drop role %s", login))
	})

	err = bootstrap.New().Apply(context.Background(), bootstrap.ApplyArgs{
		DSN: dsn,
		Template: bootstrap.TemplateData{
			Username:                    "inventario",
			UsernameForMigrations:       login,
			UsernameForBackgroundWorker: "inventario",
		},
	})
	c.Assert(err, qt.IsNil)

	for _, grantee := range []string{
		"inventario_app",
		"inventario_background_worker",
		"inventario_admin",
	} {
		var granted bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT 1
				FROM pg_default_acl d
				JOIN pg_roles r ON r.oid = d.defaclrole
				WHERE r.rolname = $1
				  AND d.defaclobjtype = 'r'
				  AND array_to_string(d.defaclacl, ',') LIKE '%' || $2 || '=%'
			)`, login, grantee).Scan(&granted)
		c.Assert(err, qt.IsNil)
		c.Assert(granted, qt.IsTrue,
			qt.Commentf("tables created by %s must grant to %s", login, grantee))
	}
}

func TestMigrator_Apply_InvalidDSN_UnhappyPath(t *testing.T) {
	//dsn := os.Getenv("POSTGRES_TEST_DSN")
	//if dsn == "" {
	//	t.Skip("POSTGRES_TEST_DSN environment variable not set")
	//}

	tests := []struct {
		name       string
		dsn        string
		expErrType error
	}{
		{
			name:       "invalid host should fail",
			dsn:        "postgres://inventario:inventario_password@invalid_host:5433/inventario?sslmode=disable",
			expErrType: &pgconn.ConnectError{},
		},
		{
			name:       "invalid port should fail",
			dsn:        "postgres://inventario:inventario_password@localhost:99999/inventario?sslmode=disable",
			expErrType: &pgconn.ParseConfigError{},
		},
		{
			name:       "invalid credentials should fail",
			dsn:        "postgres://invalid_user:invalid_pass@localhost:5433/inventario?sslmode=disable",
			expErrType: &pgconn.ConnectError{},
		},
		{
			name:       "invalid database should fail",
			dsn:        "postgres://inventario:inventario_password@localhost:5433/invalid_db?sslmode=disable",
			expErrType: &pgconn.ConnectError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			migrator := bootstrap.New()

			args := bootstrap.ApplyArgs{
				DSN: tt.dsn,
				Template: bootstrap.TemplateData{
					Username:              "inventario",
					UsernameForMigrations: "inventario",
				},
				DryRun: false,
			}

			err := migrator.Apply(context.Background(), args)
			c.Log(err)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err, qt.ErrorAs, &tt.expErrType)
		})
	}
}

func TestMigrator_Apply_DryRun_Integration_HappyPath(t *testing.T) {
	dsn := getPostgresDSNorSkip(t)
	c := qt.New(t)
	migrator := bootstrap.New()

	args := bootstrap.ApplyArgs{
		DSN: dsn,
		Template: bootstrap.TemplateData{
			Username:                    "inventario",
			UsernameForMigrations:       "inventario",
			UsernameForBackgroundWorker: "inventario",
		},
		DryRun: true,
	}

	err := migrator.Apply(context.Background(), args)
	c.Assert(err, qt.IsNil, qt.Commentf("dry run should succeed even with real DSN"))
}

func TestMigrator_Apply_TransactionRollback_UnhappyPath(t *testing.T) {
	c := qt.New(t)
	migrator := bootstrap.New()

	// Use a DSN with limited privileges to test transaction rollback
	// This test assumes the test user doesn't have SUPERUSER privileges
	limitedDSN := os.Getenv("POSTGRES_LIMITED_TEST_DSN")
	if limitedDSN == "" {
		t.Skip("POSTGRES_LIMITED_TEST_DSN environment variable not set")
	}

	args := bootstrap.ApplyArgs{
		DSN: limitedDSN,
		Template: bootstrap.TemplateData{
			Username:                    "inventario",
			UsernameForMigrations:       "inventario",
			UsernameForBackgroundWorker: "inventario",
		},
		DryRun: false,
	}

	err := migrator.Apply(context.Background(), args)
	// This should fail due to insufficient privileges, but the test verifies
	// that the error is handled gracefully and transactions are rolled back
	c.Assert(err, qt.IsNotNil, qt.Commentf("should fail with insufficient privileges"))
}

func TestMigrator_Apply_ContextCancellation_UnhappyPath(t *testing.T) {
	dsn := getPostgresDSNorSkip(t)
	c := qt.New(t)
	migrator := bootstrap.New()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	args := bootstrap.ApplyArgs{
		DSN: dsn,
		Template: bootstrap.TemplateData{
			Username:                    "inventario",
			UsernameForMigrations:       "inventario",
			UsernameForBackgroundWorker: "inventario",
		},
		DryRun: false,
	}

	err := migrator.Apply(ctx, args)
	c.Assert(err, qt.IsNotNil, qt.Commentf("should fail with cancelled context"))
}

// #2428: the guards compared a SQL string literal against a stored role name.
// CREATE USER interpolates the name unquoted, so PostgreSQL folded MyApp to
// myapp while the guard kept looking for MyApp — always false, so CREATE USER
// always ran. That broke the *first* apply, because the background-worker name
// defaults to the operational one: the second block tried to create a role the
// first had just made, and the single transaction rolled the whole file back.
func TestMigrator_Apply_MixedCaseUsernameIsIdempotent(t *testing.T) {
	dsn := getPostgresDSNorSkip(t)
	c := qt.New(t)

	args := bootstrap.ApplyArgs{
		DSN: dsn,
		Template: bootstrap.TemplateData{
			Username:                    "MixedCaseApp",
			UsernameForMigrations:       "MixedCaseMigrator",
			UsernameForBackgroundWorker: "MixedCaseApp",
		},
	}

	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	// Roles live in the cluster, not the database, so a test that leaves one
	// behind changes what the next run is testing.
	dropLoginRole(c, db, "mixedcaseapp")
	dropLoginRole(c, db, "mixedcasemigrator")
	defer func() {
		dropLoginRole(c, db, "mixedcaseapp")
		dropLoginRole(c, db, "mixedcasemigrator")
	}()

	err = bootstrap.New().Apply(context.Background(), args)
	c.Assert(err, qt.IsNil, qt.Commentf("the first apply is the one that used to fail"))
	err = bootstrap.New().Apply(context.Background(), args)
	c.Assert(err, qt.IsNil, qt.Commentf("and the second must be a no-op"))

	// The roles exist under the names PostgreSQL actually stores, which is what
	// the operator has to put in the application DSN.
	for _, role := range []string{"mixedcaseapp", "mixedcasemigrator"} {
		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", role).Scan(&exists)
		c.Assert(err, qt.IsNil)
		c.Check(exists, qt.IsTrue, qt.Commentf("role %s", role))
	}
	// And not under the spelling that was passed in.
	var unfolded bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = 'MixedCaseApp')").Scan(&unfolded)
	c.Assert(err, qt.IsNil)
	c.Check(unfolded, qt.IsFalse)
}

// pg_user lists login roles only. A NOLOGIN role holding the name was invisible
// to the old guard, so CREATE USER collided and took the whole file down with
// it — nothing provisioned at all. Reading pg_roles turns that into a skip, so
// the rest of the bootstrap still runs and the operator can grant LOGIN.
func TestMigrator_Apply_SurvivesANoLoginRoleHoldingTheName(t *testing.T) {
	dsn := getPostgresDSNorSkip(t)
	c := qt.New(t)

	db, err := sql.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	const name = "nologin_squatter"
	dropLoginRole(c, db, name)
	_, err = db.Exec("CREATE ROLE " + name + " WITH NOLOGIN")
	c.Assert(err, qt.IsNil)
	defer dropLoginRole(c, db, name)

	err = bootstrap.New().Apply(context.Background(), bootstrap.ApplyArgs{
		DSN: dsn,
		Template: bootstrap.TemplateData{
			Username:                    name,
			UsernameForMigrations:       name,
			UsernameForBackgroundWorker: name,
		},
	})
	c.Assert(err, qt.IsNil, qt.Commentf("a role already holding the name must not roll the file back"))

	// The service roles the file also creates are there, which is the point:
	// the failure used to leave the database untouched.
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = 'inventario_app')").Scan(&exists)
	c.Assert(err, qt.IsNil)
	c.Check(exists, qt.IsTrue)
}

// dropLoginRole removes a role the way bootstrap leaves it: DROP ROLE refuses
// while privileges are still granted to the name, and bootstrap grants it the
// service roles, so the privileges go first.
func dropLoginRole(c *qt.C, db *sql.DB, name string) {
	c.Helper()

	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", name).Scan(&exists)
	c.Assert(err, qt.IsNil)
	if !exists {
		return
	}
	_, err = db.Exec("DROP OWNED BY " + name)
	c.Assert(err, qt.IsNil)
	_, err = db.Exec("DROP ROLE " + name)
	c.Assert(err, qt.IsNil)
}

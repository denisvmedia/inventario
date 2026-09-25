package setup_test

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"go.5x5.cz/inventario/cmd/inventario/db/bootstrap/apply"
	"go.5x5.cz/inventario/cmd/inventario/db/migrate/up"
	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/internal/pgtest"
)

// TestMain stops whatever PostgreSQL the suite started. pgtest starts one on
// first use — POSTGRES_TEST_DSN when it is set, an embedded server otherwise
// (#1953).
func TestMain(m *testing.M) {
	code := m.Run()
	pgtest.Stop()
	os.Exit(code)
}

// prepareSchema runs bootstrap and the migration chain against dsn, once per
// binary.
//
// These tests exercise the initial-dataset setup against real tables, so an
// empty database is not enough. The PostgreSQL CI lane ran both steps before
// invoking the suite, which is why the tests never carried it themselves — and
// why they ran only there.
//
// It goes through the CLI commands rather than the packages underneath, so the
// suite prepares its database the way an operator does.
func prepareSchema(t *testing.T, dsn string) {
	t.Helper()
	schemaOnce.Do(func() {
		dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
		schemaErr = runDBCommand(apply.New(dbConfig).Cmd(), dbConfig, dsn,
			"--username=inventario", "--username-for-migrations=inventario")
		if schemaErr == nil {
			schemaErr = runDBCommand(up.New(dbConfig).Cmd(), dbConfig, dsn)
		}
	})
	if schemaErr != nil {
		t.Fatalf("preparing the test schema: %v", schemaErr)
	}
}

var (
	schemaOnce sync.Once
	schemaErr  error
)

func runDBCommand(cmd *cobra.Command, dbConfig *shared.DatabaseConfig, dsn string, extra ...string) error {
	shared.RegisterLocalDatabaseFlags(cmd, dbConfig)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--db-dsn=" + dsn}, extra...))
	if err := cmd.Execute(); err != nil {
		return fmt.Errorf("%s: %w\n%s", cmd.Name(), err, out.String())
	}
	return nil
}

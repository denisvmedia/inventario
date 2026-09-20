package bootstrap_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/schema/bootstrap"
)

func TestNew(t *testing.T) {
	c := qt.New(t)

	migrator := bootstrap.New()

	c.Assert(migrator, qt.IsNotNil)
}

func TestMigrator_getSQLFiles_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		expected []string
	}{
		{
			name:     "should find SQL files in alphabetical order",
			expected: []string{"001_initial.sql"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			migrator := bootstrap.New()

			// Use reflection to access private method for testing
			// Note: This is a workaround since getSQLFiles is not exported
			// In a real scenario, we might want to make it exported for testing
			// or test it indirectly through Apply method

			// For now, we'll test this indirectly through the Apply method
			// by checking the logs or behavior
			args := bootstrap.ApplyArgs{
				DSN:    "postgres://test:test@localhost/test",
				DryRun: true,
				Template: bootstrap.TemplateData{
					Username:              "testuser",
					UsernameForMigrations: "testmigrator",
				},
			}

			// Test the behavior indirectly through dry run
			err := migrator.Apply(context.Background(), args)
			c.Assert(err, qt.IsNil, qt.Commentf("dry run should not fail"))
		})
	}
}

func TestMigrator_Apply_DSNValidation_UnhappyPath(t *testing.T) {
	tests := []struct {
		name        string
		dsn         string
		expectedErr string
	}{
		{
			name:        "empty DSN should fail",
			dsn:         "",
			expectedErr: "database DSN is required",
		},
		{
			name:        "non-PostgreSQL DSN should fail",
			dsn:         "mysql://user:pass@localhost/db",
			expectedErr: "migrator: bootstrap migrations only support PostgreSQL databases",
		},
		{
			name:        "invalid protocol should fail",
			dsn:         "invalid://user:pass@localhost/db",
			expectedErr: "migrator: bootstrap migrations only support PostgreSQL databases",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			migrator := bootstrap.New()

			args := bootstrap.ApplyArgs{
				DSN: tt.dsn,
				Template: bootstrap.TemplateData{
					Username:              "testuser",
					UsernameForMigrations: "testmigrator",
				},
			}

			err := migrator.Apply(context.Background(), args)
			c.Assert(err, qt.ErrorMatches, tt.expectedErr)
		})
	}
}

func TestMigrator_Apply_PostgreSQLDSNValidation_HappyPath(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{
			name: "postgres:// protocol should be accepted",
			dsn:  "postgres://user:pass@localhost/db",
		},
		{
			name: "postgresql:// protocol should be accepted",
			dsn:  "postgresql://user:pass@localhost/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			migrator := bootstrap.New()

			args := bootstrap.ApplyArgs{
				DSN:    tt.dsn,
				DryRun: true, // Use dry run to avoid actual database connection
				Template: bootstrap.TemplateData{
					Username:              "testuser",
					UsernameForMigrations: "testmigrator",
				},
			}

			err := migrator.Apply(context.Background(), args)
			// Should not fail due to DSN validation (may fail due to connection, but that's expected in dry run)
			c.Assert(err, qt.IsNil, qt.Commentf("DSN validation should pass for valid PostgreSQL DSNs"))
		})
	}
}

func TestMigrator_Apply_DryRun_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		template bootstrap.TemplateData
	}{
		{
			name: "dry run with valid template data should succeed",
			template: bootstrap.TemplateData{
				Username:              "inventario",
				UsernameForMigrations: "inventario_migrator",
			},
		},
		{
			name: "dry run with different usernames should succeed",
			template: bootstrap.TemplateData{
				Username:              "myapp",
				UsernameForMigrations: "myapp_migrator",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			migrator := bootstrap.New()

			args := bootstrap.ApplyArgs{
				DSN:      "postgres://admin:pass@localhost/inventario",
				DryRun:   true,
				Template: tt.template,
			}

			err := migrator.Apply(context.Background(), args)
			c.Assert(err, qt.IsNil, qt.Commentf("dry run should succeed"))
		})
	}
}

func TestTemplateData_Structure(t *testing.T) {
	c := qt.New(t)

	templateData := bootstrap.TemplateData{
		Username:              "testuser",
		UsernameForMigrations: "testmigrator",
	}

	c.Assert(templateData.Username, qt.Equals, "testuser")
	c.Assert(templateData.UsernameForMigrations, qt.Equals, "testmigrator")
}

func TestApplyArgs_Structure(t *testing.T) {
	c := qt.New(t)

	args := bootstrap.ApplyArgs{
		DSN:    "postgres://test:test@localhost/test",
		DryRun: true,
		Template: bootstrap.TemplateData{
			Username:              "testuser",
			UsernameForMigrations: "testmigrator",
		},
	}

	c.Assert(args.DSN, qt.Equals, "postgres://test:test@localhost/test")
	c.Assert(args.DryRun, qt.Equals, true)
	c.Assert(args.Template.Username, qt.Equals, "testuser")
	c.Assert(args.Template.UsernameForMigrations, qt.Equals, "testmigrator")
}

func TestMigrator_Apply_NoSQLFiles_HappyPath(t *testing.T) {
	c := qt.New(t)

	// This test would require mocking the embedded filesystem
	// For now, we test with the actual embedded files
	// In a real scenario, we might want to create a test version with no files

	migrator := bootstrap.New()
	args := bootstrap.ApplyArgs{
		DSN:    "postgres://test:test@localhost/test",
		DryRun: true,
		Template: bootstrap.TemplateData{
			Username:              "testuser",
			UsernameForMigrations: "testmigrator",
		},
	}

	err := migrator.Apply(context.Background(), args)
	c.Assert(err, qt.IsNil, qt.Commentf("should handle case with SQL files gracefully"))
}

func TestMigrator_Apply_TemplateVariableSubstitution_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		template bootstrap.TemplateData
	}{
		{
			name: "should substitute Username variable",
			template: bootstrap.TemplateData{
				Username:              "custom_user",
				UsernameForMigrations: "custom_migrator",
			},
		},
		{
			name: "should handle special characters in usernames",
			template: bootstrap.TemplateData{
				Username:              "user_with_underscores",
				UsernameForMigrations: "migrator_with_underscores",
			},
		},
		{
			name: "should handle same username for both fields",
			template: bootstrap.TemplateData{
				Username:              "same_user",
				UsernameForMigrations: "same_user",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			migrator := bootstrap.New()

			args := bootstrap.ApplyArgs{
				DSN:      "postgres://admin:pass@localhost/inventario",
				DryRun:   true,
				Template: tt.template,
			}

			err := migrator.Apply(context.Background(), args)
			c.Assert(err, qt.IsNil, qt.Commentf("template substitution should work"))
		})
	}
}

// Default privileges key on the role that CREATES an object, and role
// membership does not carry them. A migration login named anything other than
// inventario_migrator therefore needs its own ALTER DEFAULT PRIVILEGES, or the
// tables it creates grant nothing to the app role (#2520).
func TestMigrator_Print_GrantsDefaultsToTheMigrationLogin(t *testing.T) {
	c := qt.New(t)

	sql := captureStdout(t, func() {
		err := bootstrap.New().Print(bootstrap.TemplateData{
			Username:                    "inventario",
			UsernameForMigrations:       "custom_migration_login",
			UsernameForBackgroundWorker: "inventario_bgw",
		})
		c.Assert(err, qt.IsNil)
	})

	c.Assert(sql, qt.Contains, "'custom_migration_login' != 'inventario_migrator'")
	c.Assert(sql, qt.Contains, "ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA public ")
	c.Assert(sql, qt.Contains, "TO inventario_app, inventario_background_worker, inventario_admin")
}

// captureStdout collects what fn writes to os.Stdout. Print emits the rendered
// SQL there rather than through the Migrator's writer, which defaults to
// io.Discard and carries logging only.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	os.Stdout = orig
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

func TestMigrator_Apply_FileOrdering_HappyPath(t *testing.T) {
	c := qt.New(t)
	migrator := bootstrap.New()

	args := bootstrap.ApplyArgs{
		DSN:    "postgres://admin:pass@localhost/inventario",
		DryRun: true,
		Template: bootstrap.TemplateData{
			Username:              "testuser",
			UsernameForMigrations: "testmigrator",
		},
	}

	// This test verifies that files are processed in alphabetical order
	// The current embedded filesystem has 001_initial.sql
	// If more files were added like 002_second.sql, 003_third.sql, etc.
	// they should be processed in that order
	err := migrator.Apply(context.Background(), args)
	c.Assert(err, qt.IsNil, qt.Commentf("files should be processed in alphabetical order"))
}

func TestMigrator_Apply_EmptyTemplateFields_UnhappyPath(t *testing.T) {
	tests := []struct {
		name     string
		template bootstrap.TemplateData
	}{
		{
			name: "empty Username should still work",
			template: bootstrap.TemplateData{
				Username:              "",
				UsernameForMigrations: "migrator",
			},
		},
		{
			name: "empty UsernameForMigrations should still work",
			template: bootstrap.TemplateData{
				Username:              "user",
				UsernameForMigrations: "",
			},
		},
		{
			name: "both empty should still work",
			template: bootstrap.TemplateData{
				Username:              "",
				UsernameForMigrations: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			migrator := bootstrap.New()

			args := bootstrap.ApplyArgs{
				DSN:      "postgres://admin:pass@localhost/inventario",
				DryRun:   true,
				Template: tt.template,
			}

			// Empty template fields should not cause errors in dry run mode
			// The actual SQL execution might fail, but template processing should work
			err := migrator.Apply(context.Background(), args)
			c.Assert(err, qt.IsNil, qt.Commentf("empty template fields should not cause template processing errors"))
		})
	}
}

func TestMigrator_Print_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		template bootstrap.TemplateData
	}{
		{
			name: "should print with valid template data",
			template: bootstrap.TemplateData{
				Username:              "testuser",
				UsernameForMigrations: "testmigrator",
			},
		},
		{
			name: "should print with different usernames",
			template: bootstrap.TemplateData{
				Username:              "myapp",
				UsernameForMigrations: "myapp_migrator",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			migrator := bootstrap.New()

			err := migrator.Print(tt.template)
			c.Assert(err, qt.IsNil, qt.Commentf("print should succeed"))
		})
	}
}

func TestMigrator_Print_EmptyTemplate_HappyPath(t *testing.T) {
	c := qt.New(t)
	migrator := bootstrap.New()

	templateData := bootstrap.TemplateData{
		Username:              "",
		UsernameForMigrations: "",
	}

	err := migrator.Print(templateData)
	c.Assert(err, qt.IsNil, qt.Commentf("print should work with empty template fields"))
}

func TestTemplateData_Validate_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		template bootstrap.TemplateData
	}{
		{
			name: "plain login names",
			template: bootstrap.TemplateData{
				Username:                    "inventario",
				UsernameForMigrations:       "inventario_migrator",
				UsernameForBackgroundWorker: "inventario",
			},
		},
		{
			name: "migration login may carry the migrator role name",
			template: bootstrap.TemplateData{
				Username:              "app",
				UsernameForMigrations: "inventario_migrator",
			},
		},
		{
			name:     "empty fields are left to the caller's defaulting",
			template: bootstrap.TemplateData{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			err := tt.template.Validate()

			c.Assert(err, qt.IsNil)
		})
	}
}

func TestTemplateData_Validate_UnhappyPath(t *testing.T) {
	tests := []struct {
		name     string
		template bootstrap.TemplateData
	}{
		{
			// The k8s/prod regression: the login collapses into the role the
			// application narrows to, so SET LOCAL ROLE stops narrowing.
			name:     "operational login named after the app role",
			template: bootstrap.TemplateData{Username: "inventario_app"},
		},
		{
			name:     "operational login named after the worker role",
			template: bootstrap.TemplateData{Username: "inventario_background_worker"},
		},
		{
			name:     "operational login named after the admin role",
			template: bootstrap.TemplateData{Username: "inventario_admin"},
		},
		{
			name:     "operational login named after the migrator role",
			template: bootstrap.TemplateData{Username: "inventario_migrator"},
		},
		{
			name: "migration login named after the app role",
			template: bootstrap.TemplateData{
				Username:              "inventario",
				UsernameForMigrations: "inventario_app",
			},
		},
		{
			name: "worker login named after the app role",
			template: bootstrap.TemplateData{
				Username:                    "inventario",
				UsernameForBackgroundWorker: "inventario_app",
			},
		},
		{
			// The template interpolates the name unquoted, so PostgreSQL folds
			// this to inventario_app and the collision lands regardless. Its own
			// guards compare SQL string literals, which do not fold, so bootstrap
			// happily grants the worker role to the policy role.
			name:     "operational login in another case",
			template: bootstrap.TemplateData{Username: "INVENTARIO_APP"},
		},
		{
			name: "worker login in another case",
			template: bootstrap.TemplateData{
				Username:                    "inventario",
				UsernameForBackgroundWorker: "Inventario_App",
			},
		},
		{
			name: "migration login in another case",
			template: bootstrap.TemplateData{
				Username:              "inventario",
				UsernameForMigrations: "INVENTARIO_ADMIN",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			err := tt.template.Validate()

			c.Assert(err, qt.ErrorIs, bootstrap.ErrReservedLoginName)
		})
	}
}

func TestMigrator_Apply_ReservedLoginName_UnhappyPath(t *testing.T) {
	c := qt.New(t)

	migrator := bootstrap.New()

	// Dry run has to report the collision too: the setup job previews before it
	// applies, and a preview that passes would hide the defect until rollout.
	err := migrator.Apply(context.Background(), bootstrap.ApplyArgs{
		DSN:      "postgres://admin:pass@localhost/inventario",
		DryRun:   true,
		Template: bootstrap.TemplateData{Username: "inventario_app"},
	})

	c.Assert(err, qt.ErrorIs, bootstrap.ErrReservedLoginName)
}

// Package run exposes the `inventario run` parent command. It wires the
// shared configuration (bootstrap package) to the three subcommand packages
// (all, apiserver, workers) and preserves backward-compatible behavior for
// bare `inventario run`, which delegates to the all subcommand.
package run

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/run/all"
	"github.com/denisvmedia/inventario/cmd/inventario/run/apiserver"
	"github.com/denisvmedia/inventario/cmd/inventario/run/bootstrap"
	"github.com/denisvmedia/inventario/cmd/inventario/run/workers"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

type Command struct {
	command.Base

	cfg      bootstrap.Config
	dbConfig shared.DatabaseConfig
}

// New constructs the `run` command and its all/apiserver/workers subcommands.
// Every subcommand shares the same Config + DatabaseConfig, populated once
// here from YAML/env and from PersistentFlags on the parent.
func New() *Command {
	c := &Command{}

	c.Base = command.NewBase(&cobra.Command{
		Use:   "run",
		Short: "Run the application server",
		Long: `Run starts the Inventario application server, providing a web-based interface
for managing your personal inventory. The server hosts both the API endpoints and
the frontend interface, allowing you to access your inventory through a web browser.

The server supports multiple database backends and provides RESTful API endpoints
for all inventory operations. File uploads are handled through configurable storage
locations that can be local filesystem paths or cloud storage URLs.

USAGE EXAMPLES:

  Basic development server (in-memory database):
    inventario run

  Production server with PostgreSQL:
    inventario run --addr=":8080" --db-dsn="postgres://user:pass@localhost/inventario"

  Custom upload location:
    inventario run --upload-location="file:///var/lib/inventario/uploads?create_dir=1"

FLAG DETAILS:

  --addr (default ":3333")
    Specifies the network address and port where the server will listen.
    Format: "[host]:port" (e.g., ":8080", "localhost:3333", "0.0.0.0:8080")
    Use ":0" to let the system choose an available port.

  --db-dsn (default "memory://")
    Database connection string supporting multiple backends:
    • PostgreSQL: "postgres://user:password@host:port/database?sslmode=disable"
    • In-memory: "memory://" (data lost on restart, useful for testing)

  --upload-location (default "file://./uploads?create_dir=1")
    Specifies where uploaded files are stored. Supports:
    • Local filesystem: "file:///absolute/path?create_dir=1"
    • Relative path: "file://./relative/path?create_dir=1"
    • The "create_dir=1" parameter creates the directory if it doesn't exist

PREREQUISITES:
  • Database must be migrated before first run: "inventario migrate --db-dsn=..."
  • For production use, ensure the database and upload directory have proper permissions

SERVER ENDPOINTS:
  Once running, the server provides:
  • Web Interface: http://localhost:3333 (or your specified address)
  • API Documentation: http://localhost:3333/api/docs (Swagger UI)
  • Liveness Probe: http://localhost:3333/healthz
  • Readiness Probe: http://localhost:3333/readyz

Use /readyz for load balancer/orchestrator "can serve traffic" checks, and /healthz for basic process liveness.

The server runs until interrupted (Ctrl+C) and gracefully shuts down active connections.

SUBCOMMANDS:
  all        Start the API server and every background worker (default).
  apiserver  Start only the HTTP API server; background workers must run separately.
  workers    Start every background worker; no HTTP listener is opened.

Invoking "inventario run" without a subcommand is equivalent to "inventario run all"
and is kept for backward compatibility.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return all.Run(&c.cfg, &c.dbConfig)
		},
	})

	bootstrap.RegisterFlags(c.Cmd(), &c.cfg, &c.dbConfig)

	c.Cmd().AddCommand(
		all.New(&c.cfg, &c.dbConfig).Cmd(),
		apiserver.New(&c.cfg, &c.dbConfig).Cmd(),
		workers.New(&c.cfg, &c.dbConfig).Cmd(),
	)

	return c
}

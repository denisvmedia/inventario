package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/internal/utcclock"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/postgres"
)

func registerDBBackends() (cleanup func() error) {
	// Register backends with the traditional registry system
	memory.Register()
	postgresCleanup := postgres.Register()

	// Combine cleanup functions
	cleanup = func() error {
		return postgresCleanup()
	}

	return cleanup
}

func configPath() string {
	// Get the user's config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "config.yaml"
	}

	// Define the config file path
	configFilePath := filepath.Join(configDir, "inventario", "config.yaml")

	// Check if the config file exists
	if _, err := os.Stat(configFilePath); err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	return configFilePath
}

func setupSlog() {
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     logLevel(os.Getenv("INVENTARIO_LOG_LEVEL")),
	}

	var handler slog.Handler
	if os.Getenv("INVENTARIO_LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// logLevel reads INVENTARIO_LOG_LEVEL. Empty or unparseable means info, which
// is what slog defaults to and what a deployment that never sets the variable
// keeps getting.
//
// Without this the level was fixed at info and every slog.Debug in the tree
// was unreachable — so the calls that belong at debug had to be written at
// info to be seen at all, which is how a per-read log line ends up in a
// production stream (#521).
//
// slog.Level.UnmarshalText accepts the four names case-insensitively plus the
// offset form ("debug+2"), so verbosity between the named levels is available
// without inventing a syntax for it.
func logLevel(raw string) slog.Level {
	level := slog.LevelInfo
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return level
	}
	if err := level.UnmarshalText([]byte(raw)); err != nil {
		// The logger is not up yet, so this cannot be a log line. Writing to
		// stderr keeps a typo from silently selecting info.
		fmt.Fprintf(os.Stderr, "INVENTARIO_LOG_LEVEL=%q is not a level name; using info\n", raw)
		return slog.LevelInfo
	}
	return level
}

func main() {
	// Before anything reads a clock. See the package comment: the schema's
	// timestamp columns carry no zone, so the process has to supply one.
	utcclock.Pin()

	shared.SetEnvPrefix("INVENTARIO")
	shared.SetConfigFile(configPath())

	setupSlog()

	cleanup := registerDBBackends()
	defer func() {
		err := cleanup()
		if err != nil {
			panic(err)
		}
	}()
	Execute()
}

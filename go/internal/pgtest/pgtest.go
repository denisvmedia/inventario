// Package pgtest provides a PostgreSQL instance for tests that need a real
// one.
//
// A test asks for a DSN with [DSN] and the package's TestMain calls [Stop]
// after m.Run. When POSTGRES_TEST_DSN is set the value is returned as-is, so
// CI keeps pointing the suite at its service container; otherwise an embedded
// server is downloaded once, cached, and started on a free port the first time
// a test asks. That is what lets postgres-only behaviour — row-level security,
// the SQL the memory registry does not run, constraint violations the Go side
// never reaches — be covered by the ordinary `go test ./...` pass rather than
// only by a lane with a service container attached (#1953).
//
// Starting on demand rather than from TestMain matters for packages that are
// mostly unit tests: apiserver or cmd should not pay for a server on a run
// that touches no database.
//
// Each package gets its own server, so package-level parallelism is safe. A
// single POSTGRES_TEST_DSN shared by parallel packages is not, which is why
// the PostgreSQL CI lane invokes those packages as separate steps.
//
// What it does not change: the role that owns the schema is the application
// role, here as in CI. Permission bugs that only appear when the database is
// owned by somebody else stay invisible to both, and reproducing those still
// needs an external database with a separate owner.
package pgtest

import (
	"fmt"
	"math"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

// DSNEnv is the override. Set it and Start hands it straight back, which is
// how the postgres CI lane keeps using its service container.
const DSNEnv = "POSTGRES_TEST_DSN"

const (
	user     = "inventario"
	password = "inventario"
	database = "inventario"
)

// DSN returns a DSN for the current test binary, starting the embedded server
// the first time it is asked and reusing it afterwards. Nothing starts until a
// test actually needs a database, so a package that is mostly unit tests pays
// nothing for having a few that are not.
//
// The package's TestMain has to call Stop after m.Run:
//
//	func TestMain(m *testing.M) {
//		code := m.Run()
//		pgtest.Stop()
//		os.Exit(code)
//	}
//
// Short mode skips the calling test rather than starting a server.
//
// The caller owns the schema: this hands over an empty database and does not
// bootstrap or migrate it.
func DSN(t testing.TB) string {
	t.Helper()
	SkipIfShort(t)
	if existing := os.Getenv(DSNEnv); existing != "" {
		return existing
	}
	once.Do(start)
	if startErr != nil {
		// Fatal rather than a skip. A test that silently stops running is the
		// thing this package exists to remove.
		t.Fatalf("pgtest: %v", startErr)
	}
	return runningDSN
}

// Stop shuts the embedded server down and removes its runtime directory. Safe
// to call when nothing was started.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if stopFn != nil {
		stopFn()
		stopFn = nil
	}
}

var (
	once       sync.Once
	mu         sync.Mutex
	runningDSN string
	startErr   error
	stopFn     func()
)

func start() {
	dsn, stop, err := launch()
	mu.Lock()
	defer mu.Unlock()
	runningDSN, stopFn, startErr = dsn, stop, err
}

func launch() (dsn string, stop func(), err error) {
	port, err := freePort()
	if err != nil {
		return "", nil, fmt.Errorf("no free port: %w", err)
	}

	// Each instance gets its own runtime directory: `go test ./...` runs
	// packages in parallel, and two servers sharing a data directory
	// corrupt each other. The binaries cache underneath the user's home is
	// shared on purpose — it is read-only once populated, and re-downloading
	// per package would dominate the run.
	runtimePath, err := os.MkdirTemp("", "pgtest-")
	if err != nil {
		return "", nil, fmt.Errorf("no runtime dir: %w", err)
	}

	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Username(user).
		Password(password).
		Database(database).
		Port(port).
		RuntimePath(runtimePath).
		// UTC, because the server's zone is observable: columns declared
		// TIMESTAMP WITHOUT TIME ZONE are stored verbatim and compared against
		// a NOW() the server reads in its own zone. An embedded server would
		// otherwise inherit the developer's zone and disagree with production
		// and with the CI container, both of which are UTC — and the
		// disagreement reads as a test failure in the code rather than in the
		// environment (see internal/utcclock, #2594).
		StartParameters(map[string]string{"timezone": "UTC"}).
		StartTimeout(90 * time.Second))

	if err := pg.Start(); err != nil {
		_ = os.RemoveAll(runtimePath)
		return "", nil, fmt.Errorf("start: %w", err)
	}

	dsn = fmt.Sprintf("postgres://%s:%s@localhost:%d/%s?sslmode=disable", user, password, port, database)
	return dsn, func() {
		_ = pg.Stop()
		_ = os.RemoveAll(runtimePath)
	}, nil
}

// SkipIfShort keeps `go test -short` free of the startup cost. The suites that
// use this package take seconds rather than milliseconds, which is what short
// mode is for.
func SkipIfShort(t testing.TB) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping: needs a PostgreSQL instance")
	}
}

// freePort asks the kernel for an unused port and immediately gives it back.
// The window between release and the server's bind is a race in principle;
// in practice the alternative is a fixed port, which collides for real as
// soon as two packages run at once.
func freePort() (uint32, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	if port <= 0 || port > math.MaxUint16 {
		return 0, fmt.Errorf("port out of range: %d", port)
	}
	return uint32(port), nil
}

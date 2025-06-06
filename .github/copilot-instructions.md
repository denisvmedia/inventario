Project layout:
  * /go - Backend Go code
    * /registry - Data storage implementations
      * /memory - In-memory storage implementation
      * /boltb - Boltdb storage implementation
      * /postgres - Postgres storage implementation
  * /frontend - Vue.js 3 + TypeScript frontend
  * /e2e - End-to-end tests

We use `github.com/denisvmedia/inventario/internal/errkit` for errors, but for sentitel errors we use std `errors` package.

We use `github.com/denisvmedia/inventario/internal/log` for loggig (and never `log` package). Using `log/slog` is not a mistake as well (but internal log should be preferred).

We use `github.com/frankban/quicktest` for in our tests. This package should always be imported with `qt` alias.

When changing go code, make sure you run `golangci-lint run --timeout=10m`, which must always be successful.

When changing go code, consider writing and/or updating unit tests.

When changing go code, make sure you test it using `go test`, all the tests must pass (it's ok if DB tests are skipped, when you are not testing the DB). If the DB tests are needed, check `.github/workflows/go-test-postgres.yml` to understand how to run them.

When changing frontend code, make sure you lint the code using `npm run lint:js` and `npm run lint:styles`.

When changing frontend code, make sure you testcode using `npm run test`.

When changing go API entities make sure you run `swag init --output docs` to generate swagger docs. `swag` to use version must be taken as `SWAG_VERSION=$(go list -m -f '{{.Version}}' github.com/swaggo/swag)`. To install it use `go install github.com/swaggo/swag/cmd/swag@${SWAG_VERSION}`.

When complex or breaking changes are done, make sure you run e2e tests. You can get more information on how to run them in `.github/workflows/e2e-tests.yml`.

Consider writing or updating e2e tests if your changes may need you to do so (check e2e directory). But remain rational, since too many e2e tests may take too much time to run, so only the most important parts should be tests.

If you update the tests, always run the corresponding command(s) to make sure they work the way you expect.

In all the approaches make sure you follow best practices (general ones or specific to the language or library/framework).

When making changes, make sure that the existing documentation is still actual. Modify it accordingly, if it's not.

When writing go code, make sure you have godoc comments. Keep good balance between verbosity and lack of documentation. Only be super detailed, where the complexity of the code demands that.

Make sure you always have ending newline in all go, ts, js and md files.

Make sure you don't have trailing space anywhere (unless it is required by the format or explicitely stated by the user).

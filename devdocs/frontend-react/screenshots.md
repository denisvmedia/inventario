# Local screenshots of the React frontend

How to spin up the new React frontend (#1397) end-to-end on your machine
and capture screenshots of every key page. Useful for PR demos, design
reviews, or anyone asking "what can you actually see right now?".

The script we use, `e2e/screenshots-react.mjs`, is the React-side variant
of the screenshot flow. The legacy Vue screenshot flow remains separate
and is left untouched.

## Prerequisites

- Node.js 24.14.1, npm
- Go 1.26+
- Playwright browsers installed in `e2e/` (one-time):
  ```bash
  cd e2e && npm install && npm run install-browsers
  ```

## Step-by-step

### 1. Build both frontend bundles

The Go binary embeds **both** the Vue and the React bundles at build time
(via `//go:embed` packages gated on the `with_frontend` build tag). The
runtime flag `--frontend-bundle=new|legacy` (or env `INVENTARIO_FRONTEND`)
just selects which one to serve at `/`. So both `dist/` directories must
exist before `go build` will succeed.

```bash
make build-frontend          # Vue → frontend/dist/
make build-frontend-react    # React → frontend-react/dist/
```

### 2. Build the binary

```bash
mkdir -p bin
cd go/cmd/inventario && \
  go build -tags with_frontend -o ../../../bin/inventario .
```

Skipping the tag (or the `make build-backend-nofe` target) gives you a
binary without any embedded frontend — useful for API-only work, useless
for screenshots.

### 3. Run the server with the React bundle

```bash
./bin/inventario run \
  --addr=":3333" \
  --db-dsn="memory://" \
  --frontend-bundle=new \
  --no-auth-rate-limit \
  --no-global-rate-limit
```

`memory://` means data lives in process memory and goes away on restart —
fine for screenshots, useless for anything else. Drop the rate-limit
flags in a real run; they're convenient when bursting requests during
seeding + scripted login.

Verify the React bundle is serving:

```bash
curl -s http://localhost:3333/ | grep -i "<title>"
# → <title>Inventario</title>      (both bundles use this title)
curl -s http://localhost:3333/ | grep -o 'index-[A-Za-z0-9_]*\.js'
# → matches the hash in frontend-react/dist/assets/
```

### 4. Seed the database

```bash
curl -X POST http://localhost:3333/api/v1/seed
# → {"message":"Database seeded successfully","status":"success"}
```

The seeder creates `admin@test-org.com` / `testpassword123` plus a "Test
Organization" tenant, a "Default" group, and a few locations / areas /
commodities so the dashboard isn't empty. Source: `go/debug/seeddata/`.

### 5. Run the screenshot script

```bash
BASE_URL=http://localhost:3333 \
  OUT=tmp-screenshots-react \
  node e2e/screenshots-react.mjs
```

What it captures (filename → page):

| File                       | URL pattern                       | What you see                                 |
| -------------------------- | --------------------------------- | -------------------------------------------- |
| `01-login.png`             | `/login`                          | Sign-in form (#1407)                         |
| `02-register.png`          | `/register`                       | Account creation (#1407)                     |
| `03-forgot-password.png`   | `/forgot-password`                | Password reset request (#1407)               |
| `04-not-found.png`         | `/some-nonexistent-route`         | Styled NotFound (#1404)                      |
| `10-dashboard.png`         | `/g/:slug/`                       | Dashboard with stat cards + recent (#1408)   |
| `11-locations.png`         | `/g/:slug/locations`              | Locations list with nested areas (#1409)     |
| `12-locations-new.png`     | `/g/:slug/locations/new`          | Location-create dialog opens via deep link   |
| `13-location-detail.png`   | `/g/:slug/locations/:id`          | Single-location detail (#1409)               |
| `14-area-detail.png`       | `/g/:slug/areas/:id`              | Area detail with parent breadcrumb (#1409)   |
| `20-profile.png`           | `/profile`                        | Profile (#1414)                              |
| `21-settings.png`          | `/settings`                       | Settings (#1414)                             |

The slug is read off the URL the server redirects to after login — the
dual-bundle handler points "/" at `/g/<first-slug>/` once a group is
active, so the script doesn't need to know it ahead of time.

### 6. Stop the server

```bash
PID=$(lsof -ti:3333) && kill "$PID"
```

(Works on Linux and macOS; no-ops silently when nothing is bound.
Or just Ctrl-C the foreground process if you ran it that way.)

## Common issues

- **`bind: address already in use` on port 3333.** Another inventario
  instance — or any other dev server — is bound. Kill it with
  `PID=$(lsof -ti:3333) && kill "$PID"` (Linux/macOS; no-ops if nothing
  is bound).
- **Login API returns 401.** The seed step didn't run or hit a different
  process. Re-run `curl -X POST .../api/v1/seed` against the same port.
- **Dashboard shows "0 items" + empty recent list.** The seed succeeded
  but the in-memory DB was reset by a server restart between seeding
  and screenshotting. Re-seed.
- **`bundle=new` log line says `legacy`.** You forgot
  `--frontend-bundle=new` (or `INVENTARIO_FRONTEND=new`). The default
  selection comes from `go/cmd/inventario/run/bootstrap/config.go`.
- **`go:embed` build error: `pattern "all:dist": no matching files`.**
  You didn't run `make build-frontend` and `make build-frontend-react`
  first. Both `dist/` directories must exist for the embed to compile.
- **Sidebar shows `common:nav.preferences` literally.** Known: the
  preferences nav entry resolves a key the catalog doesn't have yet.
  Tracked separately; doesn't affect the rest of the UI.

## Updating the URL list

Add a row to the `groupPages` array (or the public-routes block) in
`e2e/screenshots-react.mjs`. Keep the filename prefix monotonic so the
output ordering reads top-to-bottom for a reviewer.

For pages that need a specific record (e.g. a commodity with a long
name), drive Playwright into the page first via the seeded fixture and
take the shot there — don't hard-code IDs that the seeder regenerates.

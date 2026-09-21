# Load testing

The k6 profile in `k6/api-load.js` drives the authenticated read path: the group
list, the three list endpoints a session opens, and a search.

## Running it

k6 is a single binary — `brew install k6`, or see
[k6.io/docs](https://grafana.com/docs/k6/latest/set-up/install-k6/).

```bash
# Against a local stack (docker compose up), smoke profile.
k6 run load/k6/api-load.js

# The profiles #848 asks for.
PROFILE=load   k6 run load/k6/api-load.js    # 100 VUs, 5 min
PROFILE=stress k6 run load/k6/api-load.js    # 200 VUs, 5 min

# Somewhere else, as someone else.
BASE_URL=https://inventario.example.com \
USER_EMAIL=loadtest@example.com \
USER_PASSWORD='...' \
  PROFILE=load k6 run load/k6/api-load.js
```

The user must already be a member of at least one group; `setup()` fails loudly
if not, rather than reporting a fast run against nothing.

## Thresholds, and where they are meaningful

| Metric | Target |
| --- | --- |
| `http_req_duration` p95 | < 500 ms |
| `http_req_failed` | < 1% |
| login p95 | < 2 s (bcrypt, once per VU) |
| search p95 | < 800 ms |

These are properties of the hardware as much as of the code. A GitHub-hosted
runner gives two vCPUs, shared with Postgres, Redis and the app itself; it will
not hold 500 ms at 200 VUs, and a failure there says nothing about production.

So CI runs the **smoke** profile only, weekly and on demand, and what it checks
is that the script and the stack still work together and that nothing has become
catastrophically slow. Take the numbers that matter on hardware you would
actually deploy on, and record them in the issue rather than in a threshold.

## Reading a run

The per-endpoint trends (`inventario_list_duration`,
`inventario_search_duration`) and the `endpoint` tag exist because a global p95
tells you the system is slow without telling you where. Start there, then go to
`pg_stat_statements` on the database — most of what this profile can find is a
query, not a handler.

## Why there are no writes

A write profile has to clean up after itself or the dataset grows between runs,
and two runs over different amounts of data are not comparable. Comparability is
the one thing a load test has to have, so the writes are left out until there is
a fixture that resets between runs.

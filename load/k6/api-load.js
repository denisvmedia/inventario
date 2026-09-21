// k6 load profile for the authenticated API (#848).
//
// What it exercises is the read path a signed-in user actually produces:
// the group list, then the three list endpoints a session opens, then a
// search. Writes are deliberately absent — a write profile has to clean up
// after itself or it changes the shape of the data between runs, and a
// growing dataset makes two runs incomparable, which is the one thing a
// load test has to be.
//
// Run it:
//
//   k6 run load/k6/api-load.js                       # smoke, the default
//   PROFILE=load  k6 run load/k6/api-load.js         # 100 VUs, 5 min
//   PROFILE=stress k6 run load/k6/api-load.js        # 200 VUs, 5 min
//
// Against something other than a local stack:
//
//   BASE_URL=https://inventario.example.com \
//   USER_EMAIL=... USER_PASSWORD=... k6 run load/k6/api-load.js
//
// The thresholds below are #848's targets. They are a property of the
// hardware as much as of the code: a shared CI runner sharing two vCPUs with
// Postgres will not hold them at 200 VUs, and a run there that fails proves
// nothing about production. That is why CI runs the smoke profile only and
// the real numbers are taken on real hardware — see load/README.md.
import http from "k6/http"
import { check, fail, sleep } from "k6"
import { Trend } from "k6/metrics"

const BASE_URL = __ENV.BASE_URL || "http://localhost:3333"
const API = `${BASE_URL}/api/v1`
const EMAIL = __ENV.USER_EMAIL || "admin@smoketest.example"
const PASSWORD = __ENV.USER_PASSWORD || "SmokeTestPass1!"
const PROFILE = __ENV.PROFILE || "smoke"

const PROFILES = {
  // Enough to prove the script and the stack still work together.
  smoke: [
    { duration: "15s", target: 5 },
    { duration: "30s", target: 5 },
    { duration: "15s", target: 0 },
  ],
  load: [
    { duration: "1m", target: 100 },
    { duration: "5m", target: 100 },
    { duration: "1m", target: 0 },
  ],
  stress: [
    { duration: "1m", target: 100 },
    { duration: "2m", target: 200 },
    { duration: "5m", target: 200 },
    { duration: "1m", target: 0 },
  ],
}

const stages = PROFILES[PROFILE]
if (!stages) {
  throw new Error(`unknown PROFILE "${PROFILE}"; expected one of ${Object.keys(PROFILES)}`)
}

// Per-endpoint timing, because a global p95 hides which call is slow. The
// aggregate threshold is what gates; these are what you read afterwards.
const listTrend = new Trend("inventario_list_duration", true)
const searchTrend = new Trend("inventario_search_duration", true)

export const options = {
  stages,
  thresholds: {
    // #848's targets.
    http_req_duration: ["p(95)<500"],
    http_req_failed: ["rate<0.01"],
    // Login happens once, in setup, and involves bcrypt, so it is slower by
    // design and would drag the aggregate around if it were not separated out.
    "http_req_duration{endpoint:login}": ["p(95)<2000"],
    "http_req_duration{endpoint:list}": ["p(95)<500"],
    "http_req_duration{endpoint:search}": ["p(95)<800"],
  },
}

// setup logs in once and resolves a group slug, so the VUs do not spend the
// run re-deriving the same two facts. The token it returns is shared by every
// VU, which is deliberate: sign-in is rate limited per account, so a VU that
// logs in for itself gets a 429 as soon as the profile is wider than that
// limit. Modelling N concurrent *sessions* needs N accounts, which is a
// different fixture than this profile carries.
export function setup() {
  const session = login()
  const res = http.get(`${API}/groups`, authHeaders(session))
  if (res.status !== 200) {
    fail(`GET /groups returned ${res.status}: ${res.body}`)
  }
  const groups = res.json("data") || []
  if (groups.length === 0) {
    fail("the load user is a member of no group; seed one before running")
  }
  const slug = groups[0].attributes.slug
  if (!slug) {
    fail("the first group has no slug")
  }
  return { slug, token: session.token }
}

export default function (data) {
  // The token comes from setup, so the run measures the read path rather
  // than bcrypt — which is what this profile is for.
  const headers = authHeaders({ token: data.token })
  const g = `${API}/g/${encodeURIComponent(data.slug)}`

  for (const path of ["/locations", "/areas", "/commodities"]) {
    const res = http.get(`${g}${path}`, { ...headers, tags: { endpoint: "list" } })
    check(res, { [`GET ${path} is 200`]: (r) => r.status === 200 })
    listTrend.add(res.timings.duration)
  }

  const search = http.get(`${g}/search?q=a&limit=20`, {
    ...headers,
    tags: { endpoint: "search" },
  })
  check(search, { "GET /search is 200": (r) => r.status === 200 })
  searchTrend.add(search.timings.duration)

  // Think time. Without it the VU count stops meaning "concurrent users"
  // and starts meaning "request generators", which overstates the load a
  // real user produces by an order of magnitude.
  sleep(1)
}

function login() {
  const res = http.post(
    `${API}/auth/login`,
    JSON.stringify({ email: EMAIL, password: PASSWORD }),
    { headers: { "Content-Type": "application/json" }, tags: { endpoint: "login" } }
  )
  if (res.status !== 200) {
    fail(`login as ${EMAIL} returned ${res.status}: ${res.body}`)
  }
  const token = res.json("access_token")
  if (!token) {
    fail("login succeeded but returned no access_token")
  }
  return { token }
}

function authHeaders(session) {
  return {
    headers: {
      Authorization: `Bearer ${session.token}`,
      Accept: "application/json",
    },
  }
}

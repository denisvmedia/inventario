/**
 * Where the Inventario stack is reachable for e2e tests.
 *
 * Two modes:
 *   - Dev mode (default): `go run` backend on :3333 + Vite dev server on :5173.
 *     Playwright hits Vite, which proxies /api to :3333.
 *   - Pre-built mode (USE_PREBUILT=true): a Docker image serves backend and the
 *     embedded SPA from :3333. No Vite. Playwright hits :3333 directly.
 *
 * `E2E_BASE_URL` overrides both defaults (e.g. pointing tests at a deployed env).
 */
export const USE_PREBUILT = process.env.USE_PREBUILT === 'true';

export const BACKEND_URL = process.env.E2E_BACKEND_URL || 'http://localhost:3333';

export const BASE_URL =
  process.env.E2E_BASE_URL ||
  (USE_PREBUILT ? BACKEND_URL : 'http://localhost:5173');

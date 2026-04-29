import { setupServer } from "msw/node"

// Tests register handlers per-case via `server.use(...)`. The shared instance
// keeps Node's request listeners reused across the whole suite.
export const server = setupServer()

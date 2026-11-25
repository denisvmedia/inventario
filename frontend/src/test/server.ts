import { setupServer } from "msw/node"

import { defaults as itemTypeDefaults } from "./handlers/itemTypes"

// Tests register handlers per-case via `server.use(...)`. The shared instance
// keeps Node's request listeners reused across the whole suite.
//
// The handlers given here survive `server.resetHandlers()`. GET /item-types
// is one of them: every page that shows an item resolves the item's type
// through the group's types (#2802), and a test that cares about the types
// overrides it with `itemTypeHandlers.list`.
export const server = setupServer(...itemTypeDefaults())

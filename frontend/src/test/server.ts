import { setupServer } from "msw/node"

import { defaults as customFieldDefaults } from "./handlers/customFields"
import { defaults as dataExportDefaults } from "./handlers/dataExports"
import { defaults as featureFlagDefaults } from "./handlers/featureFlags"
import { documentDefaults as intakeDocumentDefaults } from "./handlers/intake"
import { defaults as itemTypeDefaults } from "./handlers/itemTypes"

// Tests register handlers per-case via `server.use(...)`. The shared instance
// keeps Node's request listeners reused across the whole suite.
//
// The handlers given here survive `server.resetHandlers()`. GET /item-types
// is one of them: every page that shows an item resolves the item's type
// through the group's types (#2802), and a test that cares about the types
// overrides it with `itemTypeHandlers.list`. GET /custom-fields is another:
// the item form and the item page read the group's fields (#2803). GET
// /intake/documents is the third: the item page asks which other system each
// item came from (#2809), and the default answers that none did. GET
// /users/me/data-exports is the fourth: Account settings lists the user's
// personal data exports (#2177), and the default answers that there are none.
// /feature-flags is the fifth: GroupProvider reads the plans flag (#1389),
// and the default answers with every flag off.
export const server = setupServer(
  ...itemTypeDefaults(),
  ...customFieldDefaults(),
  ...intakeDocumentDefaults(),
  ...dataExportDefaults(),
  ...featureFlagDefaults()
)

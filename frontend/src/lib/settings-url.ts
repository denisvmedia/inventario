// The Settings page reads its open section from `?section=`, which makes a
// section linkable (#1384). Two call sites point at the help one — the
// sidebar's Help row and the /help route, which exists only to redirect — so
// the target lives here rather than being spelled twice.
export const SETTINGS_HELP_URL = "/settings?section=help"

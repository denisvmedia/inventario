-- Issue #1255: the location_groups.icon field used to accept any string up
-- to 10 chars, which let typos and nonsense slip in. The picker and the
-- server-side validator now constrain it to a curated set — this migration
-- resets any pre-existing value outside that set to the empty string so
-- stale free-text doesn't linger and render as plain text in the UI.
--
-- The whitelist must match models.ValidGroupIcons. Any future edit to that
-- slice that adds an emoji is backward-compatible; removing or changing an
-- emoji needs a follow-up data migration to clear the stale value.
--
-- This is a hand-written data migration (not Ptah-generated).

UPDATE location_groups
SET icon = ''
WHERE icon <> ''
  AND icon NOT IN (
    '🏠', '🏡', '🏘️', '🏰', '🛏️', '🛋️',
    '🏢', '🏛️', '💼', '💻', '🖨️', '📇',
    '📦', '🗄️', '🗃️', '🗂️', '📁', '🧰',
    '🌳', '🌲', '🌵', '🌷', '🍀', '⛰️',
    '🚗', '🚚', '🚲', '✈️', '🧳', '⛵',
    '🎨', '🎸', '🎮', '📚', '📷', '⚽', '🎲', '🧩',
    '⭐', '💎', '🔥', '⚙️', '🛠️', '🎯', '🏷️', '💡',
    '🛡️', '🔒', '🧪', '🧷'
  );

-- Migration: backfill users.default_group_id (issue #1592).
-- Direction: UP
--
-- Enforces the invariant from #1592 on existing rows:
--
--   default_group_id is NULL only when the user has zero memberships.
--
-- For every user that currently has default_group_id IS NULL but does have
-- at least one group_memberships row, promote the deterministic earliest
-- joined_at membership (ties broken by group_id ASC) to default. This
-- mirrors EnsureUserDefaultGroup in services/group_service.go.
--
-- Idempotent: re-running selects no rows once the invariant holds.

UPDATE users u
SET default_group_id = picked.group_id,
    updated_at = CURRENT_TIMESTAMP
FROM (
    SELECT DISTINCT ON (gm.member_user_id)
           gm.member_user_id AS user_id,
           gm.group_id
    FROM group_memberships gm
    ORDER BY gm.member_user_id, gm.joined_at ASC, gm.group_id ASC
) AS picked
WHERE u.id = picked.user_id
  AND u.default_group_id IS NULL;

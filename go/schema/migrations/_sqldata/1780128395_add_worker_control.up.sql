-- Migration generated from schema differences
-- Generated on: 2026-05-30T08:06:35Z
-- Direction: UP

-- POSTGRES TABLE: worker_control --
CREATE TABLE worker_control (
  worker_type TEXT NOT NULL,
  paused BOOLEAN NOT NULL DEFAULT 'false',
  paused_by TEXT,
  paused_at TIMESTAMP,
  reason TEXT,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_worker_control_uuid ON worker_control (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS worker_control_worker_type_idx ON worker_control (worker_type);
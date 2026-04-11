CREATE TABLE IF NOT EXISTS import_plan_items (
  id TEXT PRIMARY KEY,
  plan_id TEXT NOT NULL REFERENCES import_plans (id) ON DELETE CASCADE,
  session_id TEXT NOT NULL REFERENCES import_sessions (id) ON DELETE CASCADE,
  entry_type TEXT NOT NULL CHECK (entry_type IN ('FILE', 'DIRECTORY')),
  relative_path TEXT NOT NULL,
  name TEXT NOT NULL,
  target_mount_ids TEXT[] NOT NULL DEFAULT '{}'::text[],
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (plan_id, relative_path)
);

CREATE INDEX IF NOT EXISTS idx_import_plan_items_plan
  ON import_plan_items (plan_id, relative_path);

TRUNCATE import_session_entries;

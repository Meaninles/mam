ALTER TABLE agents
  ADD COLUMN IF NOT EXISTS callback_base_url TEXT NOT NULL DEFAULT '';

UPDATE agents
SET callback_base_url = 'http://127.0.0.1:61337'
WHERE callback_base_url = '';

CREATE TABLE IF NOT EXISTS import_sessions (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL REFERENCES agents (agent_id) ON DELETE RESTRICT,
  device_key TEXT NOT NULL,
  source_type TEXT NOT NULL CHECK (source_type IN ('REMOVABLE_VOLUME', 'LOCAL_DIRECTORY')),
  device_label TEXT NOT NULL,
  device_type TEXT NOT NULL,
  source_path TEXT NOT NULL,
  mount_path TEXT NOT NULL,
  volume_name TEXT,
  file_system TEXT,
  capacity_bytes BIGINT,
  available_bytes BIGINT,
  scan_status TEXT NOT NULL CHECK (scan_status IN ('PENDING', 'SCANNING', 'READY', 'FAILED')),
  session_status TEXT NOT NULL CHECK (session_status IN ('PENDING_SCAN', 'READY', 'IMPORTING', 'PARTIAL_SUCCESS', 'ISSUE', 'DISCONNECTED')),
  last_error_code TEXT,
  last_error_message TEXT,
  source_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  connected_at TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ NOT NULL,
  disconnected_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (agent_id, device_key)
);

CREATE INDEX IF NOT EXISTS idx_import_sessions_agent_updated
  ON import_sessions (agent_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS import_plans (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL UNIQUE REFERENCES import_sessions (id) ON DELETE CASCADE,
  library_id TEXT REFERENCES libraries (id) ON DELETE SET NULL,
  status TEXT NOT NULL CHECK (status IN ('DRAFT', 'READY', 'SUBMITTED', 'IMPORTING', 'COMPLETED', 'FAILED', 'CANCELED')),
  target_strategy TEXT NOT NULL,
  destination_root_path TEXT NOT NULL DEFAULT '/',
  has_blocking_issues BOOLEAN NOT NULL DEFAULT false,
  selected_file_count INTEGER NOT NULL DEFAULT 0,
  total_size_bytes BIGINT NOT NULL DEFAULT 0,
  precheck_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
  last_prechecked_at TIMESTAMPTZ,
  submitted_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS import_session_entries (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES import_sessions (id) ON DELETE CASCADE,
  entry_type TEXT NOT NULL CHECK (entry_type IN ('FILE', 'DIRECTORY')),
  relative_path TEXT NOT NULL,
  parent_relative_path TEXT,
  name TEXT NOT NULL,
  extension TEXT,
  file_kind TEXT NOT NULL,
  size_bytes BIGINT NOT NULL DEFAULT 0,
  modified_at TIMESTAMPTZ,
  target_mount_ids TEXT[] NOT NULL DEFAULT '{}'::text[],
  import_status TEXT NOT NULL CHECK (import_status IN ('PENDING', 'QUEUED', 'RUNNING', 'VERIFYING', 'COMPLETED', 'FAILED', 'CONFLICT', 'SKIPPED')),
  latest_error_code TEXT,
  latest_error_message TEXT,
  note TEXT,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (session_id, relative_path)
);

CREATE INDEX IF NOT EXISTS idx_import_entries_session
  ON import_session_entries (session_id, entry_type, relative_path);

CREATE TABLE IF NOT EXISTS import_reports (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES import_sessions (id) ON DELETE CASCADE,
  plan_id TEXT NOT NULL REFERENCES import_plans (id) ON DELETE CASCADE,
  job_id TEXT NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
  library_id TEXT REFERENCES libraries (id) ON DELETE SET NULL,
  status TEXT NOT NULL CHECK (status IN ('QUEUED', 'RUNNING', 'PARTIAL_SUCCESS', 'FAILED', 'COMPLETED')),
  title TEXT NOT NULL,
  verify_summary TEXT NOT NULL,
  target_summaries JSONB NOT NULL DEFAULT '[]'::jsonb,
  issue_ids TEXT[] NOT NULL DEFAULT '{}'::text[],
  success_count INTEGER NOT NULL DEFAULT 0,
  failed_count INTEGER NOT NULL DEFAULT 0,
  partial_count INTEGER NOT NULL DEFAULT 0,
  file_count INTEGER NOT NULL DEFAULT 0,
  submitted_at TIMESTAMPTZ NOT NULL,
  finished_at TIMESTAMPTZ,
  latest_updated_at TIMESTAMPTZ NOT NULL,
  note TEXT,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_import_reports_session_updated
  ON import_reports (session_id, latest_updated_at DESC);

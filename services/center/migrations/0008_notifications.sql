CREATE TABLE notifications (
  id TEXT PRIMARY KEY,
  source_type TEXT NOT NULL CHECK (source_type IN ('JOB', 'ISSUE')),
  source_id TEXT NOT NULL,
  job_id TEXT REFERENCES jobs(id) ON DELETE CASCADE,
  issue_id TEXT REFERENCES issues(id) ON DELETE CASCADE,
  library_id TEXT REFERENCES libraries(id),
  kind TEXT NOT NULL CHECK (kind IN ('ACTION_REQUIRED', 'REMINDER')),
  lifecycle_status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (lifecycle_status IN ('ACTIVE', 'STALE')),
  default_target_kind TEXT NOT NULL CHECK (default_target_kind IN ('issues', 'task-center', 'file-center', 'storage-nodes', 'import-center')),
  title TEXT NOT NULL,
  summary TEXT NOT NULL,
  severity TEXT NOT NULL CHECK (severity IN ('CRITICAL', 'WARNING', 'INFO', 'SUCCESS')),
  object_label TEXT NOT NULL,
  source_payload JSONB,
  capabilities_payload JSONB NOT NULL,
  jump_params JSONB NOT NULL,
  stale_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (source_type, source_id)
);

CREATE INDEX idx_notifications_job_id ON notifications (job_id);
CREATE INDEX idx_notifications_issue_id ON notifications (issue_id);
CREATE INDEX idx_notifications_library_kind ON notifications (library_id, kind);
CREATE INDEX idx_notifications_lifecycle_updated ON notifications (lifecycle_status, updated_at DESC);
CREATE INDEX idx_notifications_source_type_updated ON notifications (source_type, updated_at DESC);

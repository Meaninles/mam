CREATE TABLE IF NOT EXISTS governance_audit_logs (
  id TEXT PRIMARY KEY,
  action_kind TEXT NOT NULL,
  subject_type TEXT NOT NULL,
  subject_id TEXT NOT NULL,
  reason TEXT NOT NULL,
  payload_json JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_governance_audit_logs_created_at
  ON governance_audit_logs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_governance_audit_logs_subject
  ON governance_audit_logs (action_kind, subject_type, subject_id);

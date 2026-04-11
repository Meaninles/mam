CREATE TABLE issues (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  library_id TEXT REFERENCES libraries(id),
  issue_category TEXT NOT NULL CHECK (issue_category IN ('CONFLICT', 'TRANSFER', 'VERIFY', 'NODE_PERMISSION', 'CAPACITY_RESOURCE', 'SCAN_PARSE', 'CLEANUP_GOVERNANCE')),
  issue_type TEXT NOT NULL,
  nature TEXT NOT NULL CHECK (nature IN ('BLOCKING', 'RISK')),
  source_domain TEXT NOT NULL CHECK (source_domain IN ('TRANSFER_JOB', 'MAINTENANCE_JOB', 'FILE_CENTER', 'STORAGE_DOMAIN', 'SYSTEM_GOVERNANCE', 'IMPORT_DOMAIN')),
  severity TEXT NOT NULL CHECK (severity IN ('CRITICAL', 'WARNING', 'INFO')),
  status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'AWAITING_CONFIRMATION', 'IN_PROGRESS', 'IGNORED', 'RESOLVED', 'ARCHIVED')),
  dedupe_key TEXT,
  title TEXT NOT NULL,
  summary TEXT NOT NULL,
  object_label TEXT NOT NULL,
  asset_label TEXT,
  suggested_action TEXT,
  suggested_action_label TEXT,
  suggestion TEXT,
  detail TEXT,
  source_snapshot JSONB,
  impact_snapshot JSONB,
  first_detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_status_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at TIMESTAMPTZ,
  archived_at TIMESTAMPTZ,
  latest_event_at TIMESTAMPTZ,
  latest_error_code TEXT,
  latest_error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE issue_events (
  id TEXT PRIMARY KEY,
  issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  sequence_no INTEGER NOT NULL,
  event_type TEXT NOT NULL CHECK (event_type IN ('DETECTED', 'UPDATED', 'STATUS_CHANGED', 'RETRY_REQUESTED', 'CONFIRMED', 'IGNORED', 'RESOLVED', 'ARCHIVED', 'AUTO_REOPENED')),
  action_key TEXT,
  from_status TEXT,
  to_status TEXT,
  actor_type TEXT NOT NULL CHECK (actor_type IN ('USER', 'SYSTEM', 'SERVICE', 'AGENT', 'SCHEDULER')),
  actor_ref_id TEXT,
  operator_label TEXT,
  message TEXT,
  payload JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (issue_id, sequence_no)
);

CREATE TABLE issue_object_links (
  id TEXT PRIMARY KEY,
  issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  link_role TEXT NOT NULL CHECK (link_role IN ('SOURCE_JOB', 'SOURCE_JOB_ITEM', 'PRIMARY_SUBJECT', 'AFFECTED_ASSET', 'AFFECTED_REPLICA', 'AFFECTED_DIRECTORY', 'AFFECTED_MOUNT', 'AFFECTED_STORAGE_NODE', 'RELATED_OBJECT')),
  object_type TEXT NOT NULL CHECK (object_type IN ('JOB', 'JOB_ITEM', 'ASSET', 'ASSET_REPLICA', 'DIRECTORY', 'MOUNT', 'STORAGE_NODE', 'EXTERNAL_REF')),
  job_id TEXT REFERENCES jobs(id) ON DELETE CASCADE,
  job_item_id TEXT REFERENCES job_items(id) ON DELETE CASCADE,
  asset_id TEXT REFERENCES assets(id) ON DELETE CASCADE,
  asset_replica_id TEXT REFERENCES asset_replicas(id) ON DELETE CASCADE,
  directory_id TEXT REFERENCES library_directories(id) ON DELETE CASCADE,
  mount_id TEXT REFERENCES mounts(id) ON DELETE CASCADE,
  storage_node_id TEXT REFERENCES storage_nodes(id) ON DELETE CASCADE,
  external_ref_type TEXT,
  external_ref_id TEXT,
  object_label TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_issue_object_links_single_target CHECK (
    (CASE WHEN job_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN job_item_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN asset_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN asset_replica_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN directory_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN mount_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN storage_node_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN external_ref_id IS NULL THEN 0 ELSE 1 END) = 1
  )
);

CREATE INDEX idx_issues_library_status ON issues (library_id, status);
CREATE INDEX idx_issues_category_status ON issues (issue_category, status);
CREATE INDEX idx_issues_source_domain_status ON issues (source_domain, status);
CREATE INDEX idx_issues_nature_status ON issues (nature, status);
CREATE INDEX idx_issues_severity_status ON issues (severity, status);
CREATE INDEX idx_issues_last_detected ON issues (last_detected_at DESC);
CREATE INDEX idx_issues_updated ON issues (updated_at DESC);
CREATE INDEX idx_issues_dedupe_key ON issues (dedupe_key);

CREATE INDEX idx_issue_events_issue_created ON issue_events (issue_id, created_at DESC);
CREATE INDEX idx_issue_events_type_created ON issue_events (event_type, created_at DESC);
CREATE INDEX idx_issue_events_actor_created ON issue_events (actor_type, created_at DESC);

CREATE INDEX idx_issue_object_links_issue ON issue_object_links (issue_id);
CREATE INDEX idx_issue_object_links_job ON issue_object_links (job_id);
CREATE INDEX idx_issue_object_links_job_item ON issue_object_links (job_item_id);
CREATE INDEX idx_issue_object_links_asset ON issue_object_links (asset_id);
CREATE INDEX idx_issue_object_links_replica ON issue_object_links (asset_replica_id);
CREATE INDEX idx_issue_object_links_directory ON issue_object_links (directory_id);
CREATE INDEX idx_issue_object_links_mount ON issue_object_links (mount_id);
CREATE INDEX idx_issue_object_links_storage_node ON issue_object_links (storage_node_id);
CREATE INDEX idx_issue_object_links_role_object ON issue_object_links (link_role, object_type);

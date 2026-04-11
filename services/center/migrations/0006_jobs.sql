DROP TABLE IF EXISTS job_object_links;
DROP TABLE IF EXISTS job_events;
DROP TABLE IF EXISTS job_attempts;
DROP TABLE IF EXISTS job_items;
DROP TABLE IF EXISTS jobs;

CREATE TABLE jobs (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  library_id TEXT REFERENCES libraries(id),
  job_family TEXT NOT NULL CHECK (job_family IN ('TRANSFER', 'MAINTENANCE')),
  job_intent TEXT NOT NULL CHECK (job_intent IN ('IMPORT', 'REPLICATE', 'DELETE_REPLICA', 'DELETE_ASSET', 'SCAN_DIRECTORY', 'VERIFY_REPLICA', 'VERIFY_ASSET', 'EXTRACT_METADATA', 'CONNECTION_TEST')),
  route_type TEXT CHECK (route_type IN ('COPY', 'UPLOAD', 'DOWNLOAD')),
  status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'QUEUED', 'RUNNING', 'PAUSED', 'WAITING_CONFIRMATION', 'WAITING_RETRY', 'PARTIAL_SUCCESS', 'FAILED', 'COMPLETED', 'CANCELED')),
  priority TEXT NOT NULL DEFAULT 'NORMAL' CHECK (priority IN ('LOW', 'NORMAL', 'HIGH')),
  title TEXT NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  source_domain TEXT NOT NULL CHECK (source_domain IN ('FILE_CENTER', 'IMPORT_CENTER', 'STORAGE_NODES', 'SYSTEM_POLICY', 'ISSUE_CENTER', 'SCHEDULED')),
  source_ref_id TEXT,
  source_snapshot JSONB,
  progress_percent NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (progress_percent >= 0 AND progress_percent <= 100),
  speed_bps BIGINT,
  eta_seconds INTEGER,
  total_items INTEGER NOT NULL DEFAULT 0 CHECK (total_items >= 0),
  success_items INTEGER NOT NULL DEFAULT 0 CHECK (success_items >= 0),
  failed_items INTEGER NOT NULL DEFAULT 0 CHECK (failed_items >= 0),
  skipped_items INTEGER NOT NULL DEFAULT 0 CHECK (skipped_items >= 0),
  issue_count INTEGER NOT NULL DEFAULT 0 CHECK (issue_count >= 0),
  latest_error_code TEXT,
  latest_error_message TEXT,
  outcome_summary TEXT,
  created_by_type TEXT NOT NULL CHECK (created_by_type IN ('USER', 'SYSTEM', 'AGENT')),
  created_by_ref TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  canceled_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE job_items (
  id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
  parent_item_id TEXT REFERENCES job_items(id) ON DELETE CASCADE,
  item_key TEXT NOT NULL,
  item_type TEXT NOT NULL CHECK (item_type IN ('ASSET_REPLICA_TRANSFER', 'DIRECTORY_SCAN', 'REPLICA_VERIFY', 'ASSET_METADATA_EXTRACT', 'REPLICA_DELETE', 'ASSET_DELETE_STEP', 'CONNECTIVITY_CHECK')),
  route_type TEXT CHECK (route_type IN ('COPY', 'UPLOAD', 'DOWNLOAD')),
  status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'QUEUED', 'RUNNING', 'PAUSED', 'WAITING_CONFIRMATION', 'WAITING_RETRY', 'SKIPPED', 'FAILED', 'COMPLETED', 'CANCELED')),
  phase TEXT,
  title TEXT NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  source_path TEXT,
  target_path TEXT,
  progress_percent NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (progress_percent >= 0 AND progress_percent <= 100),
  speed_bps BIGINT,
  eta_seconds INTEGER,
  bytes_total BIGINT CHECK (bytes_total IS NULL OR bytes_total >= 0),
  bytes_done BIGINT CHECK (bytes_done IS NULL OR bytes_done >= 0),
  attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  issue_count INTEGER NOT NULL DEFAULT 0 CHECK (issue_count >= 0),
  latest_error_code TEXT,
  latest_error_message TEXT,
  result_summary TEXT,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  canceled_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (job_id, item_key)
);

CREATE TABLE job_attempts (
  id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
  job_item_id TEXT REFERENCES job_items(id) ON DELETE CASCADE,
  attempt_no INTEGER NOT NULL CHECK (attempt_no > 0),
  status TEXT NOT NULL CHECK (status IN ('RUNNING', 'SUCCEEDED', 'FAILED', 'CANCELED')),
  worker_type TEXT NOT NULL DEFAULT 'CENTER' CHECK (worker_type IN ('CENTER', 'AGENT')),
  worker_ref TEXT,
  error_code TEXT,
  error_message TEXT,
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ
);

CREATE TABLE job_events (
  id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
  job_item_id TEXT REFERENCES job_items(id) ON DELETE CASCADE,
  job_attempt_id TEXT REFERENCES job_attempts(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL,
  message TEXT NOT NULL,
  payload JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE job_object_links (
  id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
  job_item_id TEXT REFERENCES job_items(id) ON DELETE CASCADE,
  link_role TEXT NOT NULL CHECK (link_role IN ('SUBJECT_ASSET', 'SUBJECT_REPLICA', 'SOURCE_DIRECTORY', 'TARGET_DIRECTORY', 'SOURCE_MOUNT', 'TARGET_MOUNT', 'SOURCE_STORAGE_NODE', 'TARGET_STORAGE_NODE')),
  object_type TEXT NOT NULL CHECK (object_type IN ('ASSET', 'ASSET_REPLICA', 'DIRECTORY', 'MOUNT', 'STORAGE_NODE')),
  asset_id TEXT REFERENCES assets(id) ON DELETE CASCADE,
  asset_replica_id TEXT REFERENCES asset_replicas(id) ON DELETE CASCADE,
  directory_id TEXT REFERENCES library_directories(id) ON DELETE CASCADE,
  mount_id TEXT REFERENCES mounts(id) ON DELETE CASCADE,
  storage_node_id TEXT REFERENCES storage_nodes(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_job_object_links_single_target CHECK (
    (CASE WHEN asset_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN asset_replica_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN directory_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN mount_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN storage_node_id IS NULL THEN 0 ELSE 1 END) = 1
  )
);

CREATE INDEX idx_jobs_library_status ON jobs (library_id, status);
CREATE INDEX idx_jobs_family_status ON jobs (job_family, status);
CREATE INDEX idx_jobs_intent_status ON jobs (job_intent, status);
CREATE INDEX idx_jobs_priority_status ON jobs (priority, status);
CREATE INDEX idx_jobs_created_at ON jobs (created_at DESC);
CREATE INDEX idx_jobs_updated_at ON jobs (updated_at DESC);
CREATE INDEX idx_jobs_source_domain_created_at ON jobs (source_domain, created_at DESC);

CREATE INDEX idx_job_items_job_status ON job_items (job_id, status);
CREATE INDEX idx_job_items_job_type ON job_items (job_id, item_type);
CREATE INDEX idx_job_items_parent_item_id ON job_items (parent_item_id);
CREATE INDEX idx_job_items_updated_at ON job_items (updated_at DESC);

CREATE INDEX idx_job_attempts_job_started_at ON job_attempts (job_id, started_at DESC);
CREATE INDEX idx_job_attempts_job_item_started_at ON job_attempts (job_item_id, started_at DESC);

CREATE INDEX idx_job_events_job_created_at ON job_events (job_id, created_at DESC);
CREATE INDEX idx_job_events_job_item_created_at ON job_events (job_item_id, created_at DESC);
CREATE INDEX idx_job_events_event_type_created_at ON job_events (event_type, created_at DESC);

CREATE INDEX idx_job_object_links_job_id ON job_object_links (job_id);
CREATE INDEX idx_job_object_links_job_item_id ON job_object_links (job_item_id);
CREATE INDEX idx_job_object_links_asset_id ON job_object_links (asset_id);
CREATE INDEX idx_job_object_links_asset_replica_id ON job_object_links (asset_replica_id);
CREATE INDEX idx_job_object_links_directory_id ON job_object_links (directory_id);
CREATE INDEX idx_job_object_links_mount_id ON job_object_links (mount_id);
CREATE INDEX idx_job_object_links_storage_node_id ON job_object_links (storage_node_id);

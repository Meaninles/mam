ALTER TABLE agents
  ADD COLUMN IF NOT EXISTS capabilities TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];

CREATE TABLE IF NOT EXISTS storage_nodes (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  node_type TEXT NOT NULL CHECK (node_type IN ('LOCAL', 'NAS', 'CLOUD')),
  vendor TEXT,
  address TEXT,
  access_mode TEXT NOT NULL DEFAULT 'DIRECT',
  account_alias TEXT,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS storage_node_credentials (
  id TEXT PRIMARY KEY,
  storage_node_id TEXT NOT NULL REFERENCES storage_nodes(id),
  credential_kind TEXT NOT NULL DEFAULT 'NONE',
  username TEXT,
  secret_ciphertext TEXT,
  secret_ref TEXT,
  token_status TEXT NOT NULL DEFAULT 'UNKNOWN',
  expires_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (storage_node_id)
);

CREATE TABLE IF NOT EXISTS storage_node_runtime (
  id TEXT PRIMARY KEY,
  storage_node_id TEXT NOT NULL REFERENCES storage_nodes(id),
  health_status TEXT NOT NULL DEFAULT 'UNKNOWN',
  auth_status TEXT NOT NULL DEFAULT 'UNKNOWN',
  last_check_at TIMESTAMPTZ,
  last_success_at TIMESTAMPTZ,
  last_error_code TEXT,
  last_error_message TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (storage_node_id)
);

CREATE TABLE IF NOT EXISTS mounts (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  library_id TEXT NOT NULL,
  library_name TEXT NOT NULL,
  storage_node_id TEXT NOT NULL REFERENCES storage_nodes(id),
  name TEXT NOT NULL,
  mount_source_type TEXT NOT NULL CHECK (mount_source_type IN ('LOCAL_PATH', 'NAS_SHARE', 'CLOUD_FOLDER')),
  mount_mode TEXT NOT NULL CHECK (mount_mode IN ('READ_ONLY', 'READ_WRITE')),
  source_path TEXT NOT NULL,
  relative_root_path TEXT NOT NULL DEFAULT '/',
  heartbeat_policy TEXT NOT NULL CHECK (heartbeat_policy IN ('NEVER', 'HOURLY', 'DAILY', 'WEEKLY')),
  scan_policy TEXT NOT NULL DEFAULT 'MANUAL' CHECK (scan_policy IN ('MANUAL', 'ON_START', 'SCHEDULED')),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS mount_runtime (
  id TEXT PRIMARY KEY,
  mount_id TEXT NOT NULL REFERENCES mounts(id),
  scan_status TEXT NOT NULL DEFAULT 'IDLE' CHECK (scan_status IN ('IDLE', 'RUNNING', 'SUCCESS', 'FAILED')),
  last_scan_at TIMESTAMPTZ,
  last_scan_summary TEXT,
  next_heartbeat_at TIMESTAMPTZ,
  capacity_bytes BIGINT,
  available_bytes BIGINT,
  auth_status TEXT NOT NULL DEFAULT 'NOT_REQUIRED' CHECK (auth_status IN ('UNKNOWN', 'AUTHORIZED', 'EXPIRED', 'FAILED', 'NOT_REQUIRED')),
  health_status TEXT NOT NULL DEFAULT 'UNKNOWN' CHECK (health_status IN ('UNKNOWN', 'ONLINE', 'DEGRADED', 'OFFLINE', 'ERROR')),
  last_error_code TEXT,
  last_error_message TEXT,
  last_check_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (mount_id)
);

CREATE TABLE IF NOT EXISTS mount_scan_histories (
  id TEXT PRIMARY KEY,
  mount_id TEXT NOT NULL REFERENCES mounts(id),
  started_at TIMESTAMPTZ NOT NULL,
  finished_at TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('SUCCESS', 'FAILED')),
  summary TEXT NOT NULL,
  trigger TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

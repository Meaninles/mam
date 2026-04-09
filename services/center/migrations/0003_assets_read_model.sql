CREATE TABLE IF NOT EXISTS libraries (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  root_label TEXT NOT NULL DEFAULT '/',
  status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'DISABLED', 'ARCHIVED')),
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  archived_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS library_directories (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL REFERENCES libraries(id),
  relative_path TEXT NOT NULL,
  name TEXT NOT NULL,
  parent_path TEXT,
  depth INTEGER NOT NULL DEFAULT 0,
  source_kind TEXT NOT NULL DEFAULT 'SCANNED' CHECK (source_kind IN ('SCANNED', 'MANUAL')),
  status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'HIDDEN', 'DELETED')),
  sort_name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (library_id, relative_path),
  CONSTRAINT chk_library_directories_root CHECK (
    (relative_path = '/' AND parent_path IS NULL AND depth = 0)
    OR (relative_path <> '/' AND parent_path IS NOT NULL AND depth > 0)
  )
);

CREATE TABLE IF NOT EXISTS assets (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL REFERENCES libraries(id),
  directory_id TEXT NOT NULL REFERENCES library_directories(id),
  relative_path TEXT NOT NULL,
  name TEXT NOT NULL,
  extension TEXT,
  size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
  mime_type TEXT,
  file_kind TEXT NOT NULL DEFAULT 'DOCUMENT' CHECK (file_kind IN ('IMAGE', 'VIDEO', 'AUDIO', 'DOCUMENT', 'ARCHIVE', 'OTHER')),
  lifecycle_state TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (lifecycle_state IN ('ACTIVE', 'DELETE_PENDING', 'DELETED')),
  rating SMALLINT NOT NULL DEFAULT 0 CHECK (rating BETWEEN 0 AND 5),
  color_label TEXT NOT NULL DEFAULT 'NONE' CHECK (color_label IN ('NONE', 'RED', 'YELLOW', 'GREEN', 'BLUE', 'PURPLE')),
  note TEXT,
  canonical_modified_at TIMESTAMPTZ,
  content_changed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (library_id, relative_path)
);

CREATE TABLE IF NOT EXISTS asset_replicas (
  id TEXT PRIMARY KEY,
  asset_id TEXT NOT NULL REFERENCES assets(id),
  mount_id TEXT NOT NULL REFERENCES mounts(id),
  physical_path TEXT NOT NULL,
  size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
  modified_at TIMESTAMPTZ,
  replica_state TEXT NOT NULL DEFAULT 'AVAILABLE' CHECK (replica_state IN ('AVAILABLE', 'MISSING', 'ERROR', 'DELETE_PENDING', 'DELETED')),
  sync_state TEXT NOT NULL DEFAULT 'IN_SYNC' CHECK (sync_state IN ('IN_SYNC', 'OUT_OF_SYNC', 'UNKNOWN')),
  verification_state TEXT NOT NULL DEFAULT 'UNVERIFIED' CHECK (verification_state IN ('UNVERIFIED', 'PASSED', 'FAILED', 'MISMATCH')),
  quick_hash TEXT,
  quick_hash_algorithm TEXT,
  quick_hash_at TIMESTAMPTZ,
  hash_verified_at TIMESTAMPTZ,
  last_seen_at TIMESTAMPTZ NOT NULL,
  missing_detected_at TIMESTAMPTZ,
  delete_requested_at TIMESTAMPTZ,
  last_error_code TEXT,
  last_error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (asset_id, mount_id),
  UNIQUE (mount_id, physical_path)
);

CREATE TABLE IF NOT EXISTS directory_presences (
  id TEXT PRIMARY KEY,
  directory_id TEXT NOT NULL REFERENCES library_directories(id),
  mount_id TEXT NOT NULL REFERENCES mounts(id),
  physical_path TEXT NOT NULL,
  presence_state TEXT NOT NULL DEFAULT 'PRESENT' CHECK (presence_state IN ('PRESENT', 'MISSING', 'UNKNOWN')),
  last_seen_at TIMESTAMPTZ NOT NULL,
  missing_detected_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (directory_id, mount_id)
);

CREATE TABLE IF NOT EXISTS asset_metadata (
  id TEXT PRIMARY KEY,
  asset_id TEXT NOT NULL REFERENCES assets(id),
  namespace TEXT NOT NULL,
  meta_key TEXT NOT NULL,
  meta_value JSONB NOT NULL,
  source TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (asset_id, namespace, meta_key)
);

CREATE INDEX IF NOT EXISTS idx_library_directories_library_parent
  ON library_directories (library_id, parent_path);

CREATE INDEX IF NOT EXISTS idx_library_directories_library_sort
  ON library_directories (library_id, parent_path, sort_name);

CREATE INDEX IF NOT EXISTS idx_assets_library_directory
  ON assets (library_id, directory_id);

CREATE INDEX IF NOT EXISTS idx_assets_library_name
  ON assets (library_id, name);

CREATE INDEX IF NOT EXISTS idx_assets_library_kind
  ON assets (library_id, file_kind);

CREATE INDEX IF NOT EXISTS idx_assets_library_updated
  ON assets (library_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_asset_replicas_asset_state
  ON asset_replicas (asset_id, replica_state);

CREATE INDEX IF NOT EXISTS idx_asset_replicas_mount_state
  ON asset_replicas (mount_id, replica_state);

CREATE INDEX IF NOT EXISTS idx_directory_presences_mount_state
  ON directory_presences (mount_id, presence_state);

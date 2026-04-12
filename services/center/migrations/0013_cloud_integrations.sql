CREATE TABLE IF NOT EXISTS cloud_node_profiles (
  id TEXT PRIMARY KEY,
  storage_node_id TEXT NOT NULL UNIQUE REFERENCES storage_nodes(id) ON DELETE CASCADE,
  provider_vendor TEXT NOT NULL,
  auth_method TEXT NOT NULL CHECK (auth_method IN ('TOKEN', 'QR')),
  remote_root_path TEXT NOT NULL,
  provider_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  last_auth_at TIMESTAMPTZ,
  last_auth_error_code TEXT,
  last_auth_error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cloud_node_profiles_vendor
  ON cloud_node_profiles (provider_vendor);

CREATE TABLE IF NOT EXISTS integration_gateways (
  id TEXT PRIMARY KEY,
  gateway_type TEXT NOT NULL UNIQUE CHECK (gateway_type IN ('CD2')),
  display_name TEXT NOT NULL,
  base_url TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  runtime_status TEXT NOT NULL DEFAULT 'UNKNOWN' CHECK (runtime_status IN ('UNKNOWN', 'ONLINE', 'DEGRADED', 'ERROR', 'DISABLED')),
  last_test_at TIMESTAMPTZ,
  last_error_code TEXT,
  last_error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS integration_gateway_credentials (
  id TEXT PRIMARY KEY,
  gateway_id TEXT NOT NULL UNIQUE REFERENCES integration_gateways(id) ON DELETE CASCADE,
  username TEXT NOT NULL,
  password_ciphertext TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_integration_gateways_runtime
  ON integration_gateways (gateway_type, runtime_status);

INSERT INTO cloud_node_profiles (
  id,
  storage_node_id,
  provider_vendor,
  auth_method,
  remote_root_path,
  provider_payload,
  created_at,
  updated_at
)
SELECT
  'cloud-node-profile-' || sn.id,
  sn.id,
  COALESCE(NULLIF(sn.vendor, ''), '115'),
  CASE
    WHEN sn.access_mode = 'TOKEN' THEN 'TOKEN'
    ELSE 'QR'
  END,
  COALESCE(NULLIF(sn.address, ''), '/'),
  '{}'::jsonb,
  COALESCE(snc.created_at, sn.created_at, NOW()),
  COALESCE(snc.updated_at, sn.updated_at, NOW())
FROM storage_nodes sn
LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
WHERE sn.node_type = 'CLOUD'
  AND NOT EXISTS (
    SELECT 1
    FROM cloud_node_profiles profile
    WHERE profile.storage_node_id = sn.id
  );

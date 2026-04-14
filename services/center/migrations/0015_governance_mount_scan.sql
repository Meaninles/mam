ALTER TABLE mount_runtime
  ADD COLUMN IF NOT EXISTS next_scan_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_mount_runtime_next_scan_at
  ON mount_runtime (next_scan_at)
  WHERE next_scan_at IS NOT NULL;

ALTER TABLE job_items
  ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_job_items_retry_ready
  ON job_items (job_id, status, next_retry_at)
  WHERE status = 'WAITING_RETRY';

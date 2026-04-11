ALTER TABLE issues
  ADD COLUMN IF NOT EXISTS occurrence_count INTEGER NOT NULL DEFAULT 1 CHECK (occurrence_count >= 1),
  ADD COLUMN IF NOT EXISTS last_detection_key TEXT;

UPDATE issues
SET occurrence_count = 1
WHERE occurrence_count IS NULL OR occurrence_count < 1;

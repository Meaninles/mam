CREATE TABLE IF NOT EXISTS directory_tag_links (
  directory_id TEXT NOT NULL REFERENCES library_directories(id) ON DELETE CASCADE,
  tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  order_index INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (directory_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_directory_tag_links_tag
  ON directory_tag_links (tag_id, directory_id);

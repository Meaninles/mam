CREATE TABLE IF NOT EXISTS tag_groups (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  order_index INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tags (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  normalized_name TEXT NOT NULL UNIQUE,
  group_id TEXT NOT NULL REFERENCES tag_groups(id),
  order_index INTEGER NOT NULL DEFAULT 0,
  is_pinned BOOLEAN NOT NULL DEFAULT FALSE,
  usage_count INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tag_library_scopes (
  tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  library_id TEXT NOT NULL REFERENCES libraries(id),
  PRIMARY KEY (tag_id, library_id)
);

CREATE TABLE IF NOT EXISTS asset_tag_links (
  asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
  tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  order_index INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (asset_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_tag_groups_order
  ON tag_groups (order_index, name);

CREATE INDEX IF NOT EXISTS idx_tags_group_pinned_order
  ON tags (group_id, is_pinned DESC, order_index, name);

CREATE INDEX IF NOT EXISTS idx_tag_library_scopes_library
  ON tag_library_scopes (library_id, tag_id);

CREATE INDEX IF NOT EXISTS idx_asset_tag_links_tag
  ON asset_tag_links (tag_id, asset_id);

INSERT INTO tag_groups (id, name, order_index)
VALUES ('tag-group-ungrouped', '未分组', 0)
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name;

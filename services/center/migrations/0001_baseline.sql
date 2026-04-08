CREATE TABLE IF NOT EXISTS agents (
  agent_id TEXT PRIMARY KEY,
  version TEXT NOT NULL,
  hostname TEXT NOT NULL,
  platform TEXT NOT NULL,
  mode TEXT NOT NULL,
  process_id BIGINT NOT NULL,
  registered_at TIMESTAMPTZ NOT NULL,
  last_heartbeat_at TIMESTAMPTZ NOT NULL
);

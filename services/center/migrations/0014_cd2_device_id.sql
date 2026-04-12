ALTER TABLE integration_gateways
  ADD COLUMN IF NOT EXISTS gateway_payload JSONB NOT NULL DEFAULT '{}'::jsonb;

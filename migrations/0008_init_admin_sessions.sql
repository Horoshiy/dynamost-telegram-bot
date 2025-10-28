-- +goose Up
CREATE TABLE IF NOT EXISTS admin_sessions (
  admin_tg_id BIGINT PRIMARY KEY,
  current_flow TEXT NULL,
  flow_state JSONB NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS admin_sessions;

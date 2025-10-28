-- +goose Up
CREATE TABLE IF NOT EXISTS tournaments (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NULL,
  status TEXT NOT NULL DEFAULT 'active', -- planned/active/finished
  start_date DATE NULL,
  end_date DATE NULL,
  note TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS tournaments;

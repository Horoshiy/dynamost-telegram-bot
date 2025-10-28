-- +goose Up
CREATE TABLE IF NOT EXISTS players (
  id BIGSERIAL PRIMARY KEY,
  full_name TEXT NOT NULL,
  birth_date DATE NULL,
  position TEXT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  note TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS players;

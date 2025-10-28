-- +goose Up
CREATE TABLE IF NOT EXISTS match_lineups (
  id BIGSERIAL PRIMARY KEY,
  match_id BIGINT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
  player_id BIGINT NOT NULL REFERENCES players(id),
  role TEXT NOT NULL, -- 'start' | 'sub'
  number_override INT NULL,
  note TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (match_id, player_id)
);

-- +goose Down
DROP TABLE IF EXISTS match_lineups;

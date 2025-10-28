-- +goose Up
CREATE TABLE IF NOT EXISTS tournament_roster (
  id BIGSERIAL PRIMARY KEY,
  tournament_id BIGINT NOT NULL REFERENCES tournaments(id),
  team_id BIGINT NOT NULL REFERENCES teams(id),
  player_id BIGINT NOT NULL REFERENCES players(id),
  tournament_number INT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tournament_id, team_id, player_id)
);

-- +goose Down
DROP TABLE IF EXISTS tournament_roster;

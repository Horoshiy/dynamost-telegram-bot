-- +goose Up
CREATE TABLE IF NOT EXISTS matches (
  id BIGSERIAL PRIMARY KEY,
  tournament_id BIGINT NOT NULL REFERENCES tournaments(id),
  team_id BIGINT NOT NULL REFERENCES teams(id),
  opponent_name TEXT NOT NULL,
  start_time TIMESTAMPTZ NOT NULL,
  location TEXT NULL,
  status TEXT NOT NULL DEFAULT 'scheduled', -- scheduled/played/canceled
  score_ht TEXT NULL,
  score_ft TEXT NULL,
  score_et TEXT NULL,
  score_pen TEXT NULL,
  score_final_us INT NULL,
  score_final_them INT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS matches;

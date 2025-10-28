-- +goose Up
CREATE TABLE IF NOT EXISTS match_events (
  id BIGSERIAL PRIMARY KEY,
  match_id BIGINT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL, -- 'goal' | 'card' | 'sub'
  event_time TEXT NOT NULL,
  player_id_main BIGINT NULL REFERENCES players(id),
  player_id_alt BIGINT NULL REFERENCES players(id),
  card_type TEXT NULL, -- 'yellow' | 'red'
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS match_events;

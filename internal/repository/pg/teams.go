package pg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dynamost/telegram-bot/internal/models"
	"github.com/dynamost/telegram-bot/internal/repository"
)

// Teams ----------------------------------------------------------------------

type TeamsRepo struct {
	pool *pgxpool.Pool
}

func NewTeamsRepo(pool *pgxpool.Pool) repository.TeamsRepository {
	return &TeamsRepo{pool: pool}
}

func (r *TeamsRepo) ListActive(ctx context.Context) ([]models.Team, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, short_code, active, note, created_at, updated_at
		FROM teams
		WHERE active = TRUE
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Team
	for rows.Next() {
		var team models.Team
		var note *string
		if err := rows.Scan(
			&team.ID,
			&team.Name,
			&team.ShortCode,
			&team.Active,
			&note,
			&team.CreatedAt,
			&team.UpdatedAt,
		); err != nil {
			return nil, err
		}
		team.Note = note
		items = append(items, team)
	}
	return items, rows.Err()
}

func (r *TeamsRepo) Get(ctx context.Context, id int64) (*models.Team, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, short_code, active, note, created_at, updated_at
		FROM teams
		WHERE id = $1`, id)

	var team models.Team
	var note *string
	if err := row.Scan(
		&team.ID,
		&team.Name,
		&team.ShortCode,
		&team.Active,
		&note,
		&team.CreatedAt,
		&team.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, models.ErrNotFound
		}
		return nil, err
	}
	team.Note = note
	return &team, nil
}

func (r *TeamsRepo) Create(ctx context.Context, team models.Team) (int64, error) {
	var id int64
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO teams (name, short_code, active, note)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		team.Name,
		team.ShortCode,
		team.Active,
		team.Note,
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *TeamsRepo) Update(ctx context.Context, id int64, patch models.TeamPatch) error {
	set, args := buildUpdateSet([]column{
		{name: "name", value: patch.Name},
		{name: "short_code", value: patch.ShortCode},
		{name: "active", value: patch.Active},
		{name: "note", value: patch.Note},
	})
	if len(set) == 0 {
		return nil
	}
	query := fmt.Sprintf("UPDATE teams SET %s WHERE id=$%d", set, len(args)+1)
	args = append(args, id)
	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

// Players --------------------------------------------------------------------

type PlayersRepo struct {
	pool *pgxpool.Pool
}

func NewPlayersRepo(pool *pgxpool.Pool) repository.PlayersRepository {
	return &PlayersRepo{pool: pool}
}

func (r *PlayersRepo) List(ctx context.Context, pagination models.Pagination) ([]models.Player, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, full_name, birth_date, position, active, note, created_at, updated_at
		FROM players
		ORDER BY full_name
		LIMIT $1 OFFSET $2`, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Player
	for rows.Next() {
		var (
			player   models.Player
			birth    *time.Time
			position *string
			note     *string
		)
		if err := rows.Scan(
			&player.ID,
			&player.FullName,
			&birth,
			&position,
			&player.Active,
			&note,
			&player.CreatedAt,
			&player.UpdatedAt,
		); err != nil {
			return nil, err
		}
		player.BirthDate = birth
		player.Position = position
		player.Note = note
		items = append(items, player)
	}
	return items, rows.Err()
}

func (r *PlayersRepo) Count(ctx context.Context) (int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM players`).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *PlayersRepo) Get(ctx context.Context, id int64) (*models.Player, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, full_name, birth_date, position, active, note, created_at, updated_at
		FROM players WHERE id=$1`, id)

	var (
		player   models.Player
		birth    *time.Time
		position *string
		note     *string
	)
	if err := row.Scan(
		&player.ID,
		&player.FullName,
		&birth,
		&position,
		&player.Active,
		&note,
		&player.CreatedAt,
		&player.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, models.ErrNotFound
		}
		return nil, err
	}
	player.BirthDate = birth
	player.Position = position
	player.Note = note
	return &player, nil
}

func (r *PlayersRepo) Create(ctx context.Context, player models.Player) (int64, error) {
	var id int64
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO players (full_name, birth_date, position, active, note)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		player.FullName,
		player.BirthDate,
		player.Position,
		player.Active,
		player.Note,
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *PlayersRepo) Update(ctx context.Context, id int64, patch models.PlayerPatch) error {
	set, args := buildUpdateSet([]column{
		{name: "full_name", value: patch.FullName},
		{name: "birth_date", value: patch.BirthDate},
		{name: "position", value: patch.Position},
		{name: "active", value: patch.Active},
		{name: "note", value: patch.Note},
	})
	if len(set) == 0 {
		return nil
	}
	query := fmt.Sprintf("UPDATE players SET %s WHERE id=$%d", set, len(args)+1)
	args = append(args, id)
	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *PlayersRepo) ListAssignments(ctx context.Context, playerID int64) ([]models.TournamentRosterEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tr.id, tr.tournament_id, tr.team_id, tr.player_id, tr.tournament_number,
		       tr.created_at, tr.updated_at, t.name
		FROM tournament_roster tr
		JOIN teams t ON t.id = tr.team_id
		WHERE tr.player_id = $1
		ORDER BY tr.updated_at DESC`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.TournamentRosterEntry
	for rows.Next() {
		var entry models.TournamentRosterEntry
		var number *int
		if err := rows.Scan(
			&entry.ID,
			&entry.TournamentID,
			&entry.TeamID,
			&entry.PlayerID,
			&number,
			&entry.CreatedAt,
			&entry.UpdatedAt,
			&entry.PlayerName,
		); err != nil {
			return nil, err
		}
		entry.TournamentNumber = number
		items = append(items, entry)
	}
	return items, rows.Err()
}

// Tournaments ----------------------------------------------------------------

type TournamentsRepo struct {
	pool *pgxpool.Pool
}

func NewTournamentsRepo(pool *pgxpool.Pool) repository.TournamentsRepository {
	return &TournamentsRepo{pool: pool}
}

func (r *TournamentsRepo) List(ctx context.Context, status *models.TournamentStatus) ([]models.Tournament, error) {
	query := `
		SELECT id, name, type, status, start_date, end_date, note, created_at, updated_at
		FROM tournaments`
	args := []any{}
	if status != nil {
		query += " WHERE status = $1"
		args = append(args, *status)
	}
	query += " ORDER BY start_date NULLS LAST, name"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Tournament
	for rows.Next() {
		var (
			tournament models.Tournament
			typ        *string
			start      *time.Time
			end        *time.Time
			note       *string
		)
		if err := rows.Scan(
			&tournament.ID,
			&tournament.Name,
			&typ,
			&tournament.Status,
			&start,
			&end,
			&note,
			&tournament.CreatedAt,
			&tournament.UpdatedAt,
		); err != nil {
			return nil, err
		}
		tournament.Type = typ
		tournament.StartDate = start
		tournament.EndDate = end
		tournament.Note = note
		items = append(items, tournament)
	}
	return items, rows.Err()
}

func (r *TournamentsRepo) Get(ctx context.Context, id int64) (*models.Tournament, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, type, status, start_date, end_date, note, created_at, updated_at
		FROM tournaments WHERE id=$1`, id)

	var (
		tournament models.Tournament
		typ        *string
		start      *time.Time
		end        *time.Time
		note       *string
	)
	if err := row.Scan(
		&tournament.ID,
		&tournament.Name,
		&typ,
		&tournament.Status,
		&start,
		&end,
		&note,
		&tournament.CreatedAt,
		&tournament.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, models.ErrNotFound
		}
		return nil, err
	}
	tournament.Type = typ
	tournament.StartDate = start
	tournament.EndDate = end
	tournament.Note = note
	return &tournament, nil
}

func (r *TournamentsRepo) Create(ctx context.Context, tournament models.Tournament) (int64, error) {
	var id int64
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO tournaments (name, type, status, start_date, end_date, note)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		tournament.Name,
		tournament.Type,
		tournament.Status,
		tournament.StartDate,
		tournament.EndDate,
		tournament.Note,
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *TournamentsRepo) Update(ctx context.Context, id int64, patch models.TournamentPatch) error {
	set, args := buildUpdateSet([]column{
		{name: "name", value: patch.Name},
		{name: "type", value: patch.Type},
		{name: "status", value: patch.Status},
		{name: "start_date", value: patch.StartDate},
		{name: "end_date", value: patch.EndDate},
		{name: "note", value: patch.Note},
	})
	if len(set) == 0 {
		return nil
	}
	query := fmt.Sprintf("UPDATE tournaments SET %s WHERE id=$%d", set, len(args)+1)
	args = append(args, id)
	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

// Rosters --------------------------------------------------------------------

type RostersRepo struct {
	pool *pgxpool.Pool
}

func NewRostersRepo(pool *pgxpool.Pool) repository.RostersRepository {
	return &RostersRepo{pool: pool}
}

func (r *RostersRepo) ListTeams(ctx context.Context, tournamentID int64) ([]models.TournamentTeam, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT t.id, t.name, t.short_code
		FROM tournament_roster tr
		JOIN teams t ON t.id = tr.team_id
		WHERE tr.tournament_id = $1
		ORDER BY t.name`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.TournamentTeam
	for rows.Next() {
		var team models.TournamentTeam
		if err := rows.Scan(&team.TeamID, &team.TeamName, &team.ShortCode); err != nil {
			return nil, err
		}
		items = append(items, team)
	}
	return items, rows.Err()
}

func (r *RostersRepo) ListRoster(ctx context.Context, tournamentID, teamID int64) ([]models.TournamentRosterEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tr.id, tr.tournament_id, tr.team_id, tr.player_id, tr.tournament_number,
		       tr.created_at, tr.updated_at, p.full_name
		FROM tournament_roster tr
		JOIN players p ON p.id = tr.player_id
		WHERE tr.tournament_id = $1 AND tr.team_id = $2
		ORDER BY COALESCE(tr.tournament_number, 999), p.full_name`, tournamentID, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.TournamentRosterEntry
	for rows.Next() {
		var entry models.TournamentRosterEntry
		var number *int
		if err := rows.Scan(
			&entry.ID,
			&entry.TournamentID,
			&entry.TeamID,
			&entry.PlayerID,
			&number,
			&entry.CreatedAt,
			&entry.UpdatedAt,
			&entry.PlayerName,
		); err != nil {
			return nil, err
		}
		entry.TournamentNumber = number
		items = append(items, entry)
	}
	return items, rows.Err()
}

func (r *RostersRepo) AddPlayer(ctx context.Context, tournamentID, teamID, playerID int64, number *int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tournament_roster (tournament_id, team_id, player_id, tournament_number)
		VALUES ($1, $2, $3, $4)`,
		tournamentID, teamID, playerID, number,
	)
	return err
}

func (r *RostersRepo) UpdateNumber(ctx context.Context, tournamentID, teamID, playerID int64, number *int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE tournament_roster
		SET tournament_number = $4, updated_at = NOW()
		WHERE tournament_id = $1 AND team_id = $2 AND player_id = $3`,
		tournamentID, teamID, playerID, number,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *RostersRepo) RemovePlayer(ctx context.Context, tournamentID, teamID, playerID int64) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM tournament_roster
		WHERE tournament_id = $1 AND team_id = $2 AND player_id = $3`,
		tournamentID, teamID, playerID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *RostersRepo) PlayerParticipation(ctx context.Context, tournamentID, teamID, playerID int64) (bool, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM match_lineups ml
		JOIN matches m ON m.id = ml.match_id
		WHERE m.tournament_id = $1 AND m.team_id = $2 AND ml.player_id = $3`,
		tournamentID, teamID, playerID,
	).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM match_events me
		JOIN matches m ON m.id = me.match_id
		WHERE m.tournament_id = $1 AND m.team_id = $2
		  AND (me.player_id_main = $3 OR me.player_id_alt = $3)`,
		tournamentID, teamID, playerID,
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *RostersRepo) TeamPlayerCount(ctx context.Context, tournamentID, teamID int64) (int, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tournament_roster
		WHERE tournament_id = $1 AND team_id = $2`,
		tournamentID, teamID,
	).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *RostersRepo) IsPlayerInRoster(ctx context.Context, tournamentID, teamID, playerID int64) (bool, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tournament_roster
		WHERE tournament_id = $1 AND team_id = $2 AND player_id = $3`,
		tournamentID, teamID, playerID,
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// Matches --------------------------------------------------------------------

type MatchesRepo struct {
	pool *pgxpool.Pool
}

func NewMatchesRepo(pool *pgxpool.Pool) repository.MatchesRepository {
	return &MatchesRepo{pool: pool}
}

func (r *MatchesRepo) List(ctx context.Context, tournamentID, teamID int64) ([]models.Match, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tournament_id, team_id, opponent_name, start_time, location,
		       status, score_ht, score_ft, score_et, score_pen,
		       score_final_us, score_final_them, created_at, updated_at
		FROM matches
		WHERE tournament_id = $1 AND team_id = $2
		ORDER BY start_time`, tournamentID, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Match
	for rows.Next() {
		var (
			match     models.Match
			location  *string
			scoreHT   *string
			scoreFT   *string
			scoreET   *string
			scorePEN  *string
			scoreUs   *int
			scoreThem *int
			status    string
		)
		if err := rows.Scan(
			&match.ID,
			&match.TournamentID,
			&match.TeamID,
			&match.OpponentName,
			&match.StartTime,
			&location,
			&status,
			&scoreHT,
			&scoreFT,
			&scoreET,
			&scorePEN,
			&scoreUs,
			&scoreThem,
			&match.CreatedAt,
			&match.UpdatedAt,
		); err != nil {
			return nil, err
		}
		match.Location = location
		match.Status = models.MatchStatus(status)
		match.ScoreHT = scoreHT
		match.ScoreFT = scoreFT
		match.ScoreET = scoreET
		match.ScorePEN = scorePEN
		match.ScoreFinalUs = scoreUs
		match.ScoreFinalThem = scoreThem
		items = append(items, match)
	}
	return items, rows.Err()
}

func (r *MatchesRepo) Get(ctx context.Context, id int64) (*models.Match, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tournament_id, team_id, opponent_name, start_time, location,
		       status, score_ht, score_ft, score_et, score_pen,
		       score_final_us, score_final_them, created_at, updated_at
		FROM matches WHERE id=$1`, id)

	var (
		match     models.Match
		location  *string
		scoreHT   *string
		scoreFT   *string
		scoreET   *string
		scorePEN  *string
		scoreUs   *int
		scoreThem *int
		status    string
	)
	if err := row.Scan(
		&match.ID,
		&match.TournamentID,
		&match.TeamID,
		&match.OpponentName,
		&match.StartTime,
		&location,
		&status,
		&scoreHT,
		&scoreFT,
		&scoreET,
		&scorePEN,
		&scoreUs,
		&scoreThem,
		&match.CreatedAt,
		&match.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, models.ErrNotFound
		}
		return nil, err
	}
	match.Location = location
	match.Status = models.MatchStatus(status)
	match.ScoreHT = scoreHT
	match.ScoreFT = scoreFT
	match.ScoreET = scoreET
	match.ScorePEN = scorePEN
	match.ScoreFinalUs = scoreUs
	match.ScoreFinalThem = scoreThem
	return &match, nil
}

func (r *MatchesRepo) Create(ctx context.Context, match models.Match) (int64, error) {
	var id int64
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO matches (tournament_id, team_id, opponent_name, start_time, location, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		match.TournamentID,
		match.TeamID,
		match.OpponentName,
		match.StartTime,
		match.Location,
		match.Status,
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *MatchesRepo) Update(ctx context.Context, id int64, patch models.MatchPatch) error {
	set, args := buildUpdateSet([]column{
		{name: "start_time", value: patch.StartTime},
		{name: "location", value: patch.Location},
		{name: "status", value: patch.Status},
		{name: "score_ht", value: patch.ScoreHT},
		{name: "score_ft", value: patch.ScoreFT},
		{name: "score_et", value: patch.ScoreET},
		{name: "score_pen", value: patch.ScorePEN},
		{name: "score_final_us", value: patch.ScoreFinalUs},
		{name: "score_final_them", value: patch.ScoreFinalThem},
		{name: "opponent_name", value: patch.OpponentName},
	})
	if len(set) == 0 {
		return nil
	}
	query := fmt.Sprintf("UPDATE matches SET %s WHERE id=$%d", set, len(args)+1)
	args = append(args, id)
	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

// Lineups --------------------------------------------------------------------

type LineupRepo struct {
	pool *pgxpool.Pool
}

func NewLineupRepo(pool *pgxpool.Pool) repository.LineupRepository {
	return &LineupRepo{pool: pool}
}

func (r *LineupRepo) Get(ctx context.Context, matchID int64) ([]models.MatchLineup, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ml.id, ml.match_id, ml.player_id, ml.role, ml.number_override, ml.note,
		       ml.created_at, ml.updated_at, p.full_name, tr.tournament_number
		FROM match_lineups ml
		JOIN players p ON p.id = ml.player_id
		LEFT JOIN matches m ON m.id = ml.match_id
		LEFT JOIN tournament_roster tr
		       ON tr.tournament_id = m.tournament_id
		      AND tr.team_id = m.team_id
		      AND tr.player_id = ml.player_id
		WHERE ml.match_id = $1
		ORDER BY ml.role, COALESCE(ml.number_override, tr.tournament_number, 999), p.full_name`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.MatchLineup
	for rows.Next() {
		var (
			item      models.MatchLineup
			role      string
			override  *int
			note      *string
			rosterNum *int
		)
		if err := rows.Scan(
			&item.ID,
			&item.MatchID,
			&item.PlayerID,
			&role,
			&override,
			&note,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.PlayerName,
			&rosterNum,
		); err != nil {
			return nil, err
		}
		item.Role = models.LineupRole(role)
		item.NumberOverride = override
		item.Note = note
		item.RosterNumber = rosterNum
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *LineupRepo) Upsert(ctx context.Context, matchID, playerID int64, role models.LineupRole, numberOverride *int, note *string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO match_lineups (match_id, player_id, role, number_override, note)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (match_id, player_id)
		DO UPDATE SET role = EXCLUDED.role,
		              number_override = EXCLUDED.number_override,
		              note = EXCLUDED.note,
		              updated_at = NOW()`,
		matchID, playerID, role, numberOverride, note,
	)
	return err
}

func (r *LineupRepo) Update(ctx context.Context, matchID, playerID int64, patch models.LineupPatch) error {
	set, args := buildUpdateSet([]column{
		{name: "role", value: patch.Role},
		{name: "number_override", value: patch.NumberOverride},
		{name: "note", value: patch.Note},
	})
	if len(set) == 0 {
		return nil
	}
	query := fmt.Sprintf("UPDATE match_lineups SET %s WHERE match_id=$%d AND player_id=$%d", set, len(args)+1, len(args)+2)
	args = append(args, matchID, playerID)
	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *LineupRepo) Remove(ctx context.Context, matchID, playerID int64) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM match_lineups WHERE match_id = $1 AND player_id = $2`,
		matchID, playerID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *LineupRepo) HasPlayer(ctx context.Context, matchID, playerID int64) (bool, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM match_lineups WHERE match_id = $1 AND player_id = $2`,
		matchID, playerID,
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *LineupRepo) ListPlayers(ctx context.Context, matchID int64) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT player_id FROM match_lineups WHERE match_id = $1`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Events ---------------------------------------------------------------------

type EventsRepo struct {
	pool *pgxpool.Pool
}

func NewEventsRepo(pool *pgxpool.Pool) repository.EventsRepository {
	return &EventsRepo{pool: pool}
}

func (r *EventsRepo) List(ctx context.Context, matchID int64) ([]models.MatchEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT me.id, me.match_id, me.event_type, me.event_time,
		       me.player_id_main, me.player_id_alt, me.card_type,
		       me.created_at,
		       p1.full_name AS player_main_name,
		       p2.full_name AS player_alt_name
		FROM match_events me
		LEFT JOIN players p1 ON p1.id = me.player_id_main
		LEFT JOIN players p2 ON p2.id = me.player_id_alt
		WHERE me.match_id = $1
		ORDER BY me.created_at`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.MatchEvent
	for rows.Next() {
		var (
			event     models.MatchEvent
			eventType string
			mainID    *int64
			altID     *int64
			cardType  *string
			mainName  *string
			altName   *string
		)
		if err := rows.Scan(
			&event.ID,
			&event.MatchID,
			&eventType,
			&event.EventTimeText,
			&mainID,
			&altID,
			&cardType,
			&event.CreatedAt,
			&mainName,
			&altName,
		); err != nil {
			return nil, err
		}
		event.EventType = models.MatchEventType(eventType)
		event.PlayerMainID = mainID
		event.PlayerAltID = altID
		if cardType != nil {
			ct := models.CardType(*cardType)
			event.CardType = &ct
		}
		event.PlayerMain = mainName
		event.PlayerAlt = altName
		items = append(items, event)
	}
	return items, rows.Err()
}

func (r *EventsRepo) Add(ctx context.Context, event models.MatchEvent) (int64, error) {
	var card *string
	if event.CardType != nil {
		ct := string(*event.CardType)
		card = &ct
	}
	var id int64
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO match_events (match_id, event_type, event_time, player_id_main, player_id_alt, card_type)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		event.MatchID,
		event.EventType,
		event.EventTimeText,
		event.PlayerMainID,
		event.PlayerAltID,
		card,
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *EventsRepo) PlayersInEvents(ctx context.Context, matchID int64, playerID int64) (bool, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM match_events
		WHERE match_id = $1 AND (player_id_main = $2 OR player_id_alt = $2)`,
		matchID, playerID,
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// Sessions -------------------------------------------------------------------

type SessionsRepo struct {
	pool *pgxpool.Pool
}

func NewSessionsRepo(pool *pgxpool.Pool) repository.SessionsRepository {
	return &SessionsRepo{pool: pool}
}

func (r *SessionsRepo) Get(ctx context.Context, adminID int64) (*models.AdminSession, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT admin_tg_id, current_flow, flow_state, updated_at
		FROM admin_sessions
		WHERE admin_tg_id = $1`, adminID)
	var (
		session models.AdminSession
		flow    *string
		state   []byte
	)
	if err := row.Scan(
		&session.AdminID,
		&flow,
		&state,
		&session.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, models.ErrNotFound
		}
		return nil, err
	}
	session.CurrentFlow = flow
	session.FlowState = state
	return &session, nil
}

func (r *SessionsRepo) Upsert(ctx context.Context, session models.AdminSession) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO admin_sessions (admin_tg_id, current_flow, flow_state, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (admin_tg_id)
		DO UPDATE SET current_flow = EXCLUDED.current_flow,
		              flow_state = EXCLUDED.flow_state,
		              updated_at = NOW()`,
		session.AdminID,
		session.CurrentFlow,
		session.FlowState,
	)
	return err
}

func (r *SessionsRepo) Delete(ctx context.Context, adminID int64) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM admin_sessions WHERE admin_tg_id = $1`, adminID)
	return err
}

// Shared helpers -------------------------------------------------------------

type column struct {
	name  string
	value any
}

func buildUpdateSet(cols []column) (string, []any) {
	var (
		clauses []string
		args    []any
		idx     = 1
	)
	for _, col := range cols {
		switch v := col.value.(type) {
		case nil:
			continue
		case *string:
			if v == nil {
				continue
			}
			clauses = append(clauses, fmt.Sprintf("%s=$%d", col.name, idx))
			args = append(args, *v)
			idx++
		case *bool:
			if v == nil {
				continue
			}
			clauses = append(clauses, fmt.Sprintf("%s=$%d", col.name, idx))
			args = append(args, *v)
			idx++
		case *models.MatchStatus:
			if v == nil {
				continue
			}
			clauses = append(clauses, fmt.Sprintf("%s=$%d", col.name, idx))
			args = append(args, *v)
			idx++
		case *models.TournamentStatus:
			if v == nil {
				continue
			}
			clauses = append(clauses, fmt.Sprintf("%s=$%d", col.name, idx))
			args = append(args, *v)
			idx++
		case *models.LineupRole:
			if v == nil {
				continue
			}
			clauses = append(clauses, fmt.Sprintf("%s=$%d", col.name, idx))
			args = append(args, *v)
			idx++
		case *int:
			if v == nil {
				continue
			}
			clauses = append(clauses, fmt.Sprintf("%s=$%d", col.name, idx))
			args = append(args, *v)
			idx++
		case *time.Time:
			if v == nil {
				continue
			}
			clauses = append(clauses, fmt.Sprintf("%s=$%d", col.name, idx))
			args = append(args, *v)
			idx++
		case models.OptionalString:
			if !v.Set {
				continue
			}
			clauses = append(clauses, fmt.Sprintf("%s=$%d", col.name, idx))
			args = append(args, v.Value)
			idx++
		case models.OptionalTime:
			if !v.Set {
				continue
			}
			clauses = append(clauses, fmt.Sprintf("%s=$%d", col.name, idx))
			args = append(args, v.Value)
			idx++
		case models.OptionalInt:
			if !v.Set {
				continue
			}
			clauses = append(clauses, fmt.Sprintf("%s=$%d", col.name, idx))
			args = append(args, v.Value)
			idx++
		default:
			clauses = append(clauses, fmt.Sprintf("%s=$%d", col.name, idx))
			args = append(args, v)
			idx++
		}
	}
	if len(clauses) == 0 {
		return "", nil
	}
	clauses = append(clauses, "updated_at=NOW()")
	return strings.Join(clauses, ", "), args
}

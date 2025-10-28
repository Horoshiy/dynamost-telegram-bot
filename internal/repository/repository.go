package repository

import (
	"context"

	"github.com/dynamost/telegram-bot/internal/models"
)

type TeamsRepository interface {
	ListActive(ctx context.Context) ([]models.Team, error)
	Get(ctx context.Context, id int64) (*models.Team, error)
	Create(ctx context.Context, team models.Team) (int64, error)
	Update(ctx context.Context, id int64, patch models.TeamPatch) error
}

type PlayersRepository interface {
	List(ctx context.Context, pagination models.Pagination) ([]models.Player, error)
	Count(ctx context.Context) (int, error)
	Get(ctx context.Context, id int64) (*models.Player, error)
	Create(ctx context.Context, player models.Player) (int64, error)
	Update(ctx context.Context, id int64, patch models.PlayerPatch) error
	ListAssignments(ctx context.Context, playerID int64) ([]models.TournamentRosterEntry, error)
}

type TournamentsRepository interface {
	List(ctx context.Context, status *models.TournamentStatus) ([]models.Tournament, error)
	Get(ctx context.Context, id int64) (*models.Tournament, error)
	Create(ctx context.Context, tournament models.Tournament) (int64, error)
	Update(ctx context.Context, id int64, patch models.TournamentPatch) error
}

type RostersRepository interface {
	ListTeams(ctx context.Context, tournamentID int64) ([]models.TournamentTeam, error)
	ListRoster(ctx context.Context, tournamentID, teamID int64) ([]models.TournamentRosterEntry, error)
	AddPlayer(ctx context.Context, tournamentID, teamID, playerID int64, number *int) error
	UpdateNumber(ctx context.Context, tournamentID, teamID, playerID int64, number *int) error
	RemovePlayer(ctx context.Context, tournamentID, teamID, playerID int64) error
	PlayerParticipation(ctx context.Context, tournamentID, teamID, playerID int64) (bool, error)
	TeamPlayerCount(ctx context.Context, tournamentID, teamID int64) (int, error)
	IsPlayerInRoster(ctx context.Context, tournamentID, teamID, playerID int64) (bool, error)
}

type MatchesRepository interface {
	List(ctx context.Context, tournamentID, teamID int64) ([]models.Match, error)
	Get(ctx context.Context, id int64) (*models.Match, error)
	Create(ctx context.Context, match models.Match) (int64, error)
	Update(ctx context.Context, id int64, patch models.MatchPatch) error
}

type LineupRepository interface {
	Get(ctx context.Context, matchID int64) ([]models.MatchLineup, error)
	Upsert(ctx context.Context, matchID, playerID int64, role models.LineupRole, numberOverride *int, note *string) error
	Update(ctx context.Context, matchID, playerID int64, patch models.LineupPatch) error
	Remove(ctx context.Context, matchID, playerID int64) error
	HasPlayer(ctx context.Context, matchID, playerID int64) (bool, error)
	ListPlayers(ctx context.Context, matchID int64) ([]int64, error)
}

type EventsRepository interface {
	List(ctx context.Context, matchID int64) ([]models.MatchEvent, error)
	Add(ctx context.Context, event models.MatchEvent) (int64, error)
	PlayersInEvents(ctx context.Context, matchID int64, playerID int64) (bool, error)
}

type SessionsRepository interface {
	Get(ctx context.Context, adminID int64) (*models.AdminSession, error)
	Upsert(ctx context.Context, session models.AdminSession) error
	Delete(ctx context.Context, adminID int64) error
}

type Logger interface {
	Info(action string, entity string, entityID int64, adminID int64, status string)
	Error(err error, action string, entity string, entityID int64, adminID int64)
}

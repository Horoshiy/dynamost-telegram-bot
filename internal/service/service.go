package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dynamost/telegram-bot/internal/models"
	"github.com/dynamost/telegram-bot/internal/repository"
)

// Teams ----------------------------------------------------------------------

type TeamsService interface {
	ListActive(ctx context.Context) ([]models.Team, error)
	Get(ctx context.Context, id int64) (*models.Team, error)
	Create(ctx context.Context, input CreateTeamInput) (int64, error)
	Update(ctx context.Context, id int64, patch models.TeamPatch) error
}

type CreateTeamInput struct {
	Name      string
	ShortCode string
	Active    bool
	Note      *string
}

type teamsService struct {
	repo repository.TeamsRepository
}

func NewTeamsService(repo repository.TeamsRepository) TeamsService {
	return &teamsService{repo: repo}
}

func (s *teamsService) ListActive(ctx context.Context) ([]models.Team, error) {
	return s.repo.ListActive(ctx)
}

func (s *teamsService) Get(ctx context.Context, id int64) (*models.Team, error) {
	return s.repo.Get(ctx, id)
}

func (s *teamsService) Create(ctx context.Context, input CreateTeamInput) (int64, error) {
	if input.Name == "" {
		return 0, fmt.Errorf("name: %w", models.ErrValidation)
	}
	if input.ShortCode == "" {
		return 0, fmt.Errorf("short_code: %w", models.ErrValidation)
	}
	team := models.Team{
		Name:      input.Name,
		ShortCode: input.ShortCode,
		Active:    input.Active,
		Note:      input.Note,
	}
	return s.repo.Create(ctx, team)
}

func (s *teamsService) Update(ctx context.Context, id int64, patch models.TeamPatch) error {
	return s.repo.Update(ctx, id, patch)
}

// Players --------------------------------------------------------------------

type PlayersService interface {
	List(ctx context.Context, page, perPage int) ([]models.Player, bool, error)
	Get(ctx context.Context, id int64) (*models.Player, error)
	Create(ctx context.Context, input CreatePlayerInput) (int64, error)
	Update(ctx context.Context, id int64, patch models.PlayerPatch) error
	ListAssignments(ctx context.Context, playerID int64) ([]models.TournamentRosterEntry, error)
}

type CreatePlayerInput struct {
	FullName string
	Birth    *time.Time
	Position *string
	Active   bool
	Note     *string
}

type playersService struct {
	repo repository.PlayersRepository
}

func NewPlayersService(repo repository.PlayersRepository) PlayersService {
	return &playersService{repo: repo}
}

func (s *playersService) List(ctx context.Context, page, perPage int) ([]models.Player, bool, error) {
	pagination := models.NewPagination(page, perPage)
	items, err := s.repo.List(ctx, pagination)
	if err != nil {
		return nil, false, err
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, false, err
	}
	next := pagination.Offset+len(items) < total
	return items, next, nil
}

func (s *playersService) Get(ctx context.Context, id int64) (*models.Player, error) {
	return s.repo.Get(ctx, id)
}

func (s *playersService) Create(ctx context.Context, input CreatePlayerInput) (int64, error) {
	if input.FullName == "" {
		return 0, fmt.Errorf("full_name: %w", models.ErrValidation)
	}
	player := models.Player{
		FullName:  input.FullName,
		BirthDate: input.Birth,
		Position:  input.Position,
		Active:    input.Active,
		Note:      input.Note,
	}
	return s.repo.Create(ctx, player)
}

func (s *playersService) Update(ctx context.Context, id int64, patch models.PlayerPatch) error {
	return s.repo.Update(ctx, id, patch)
}

func (s *playersService) ListAssignments(ctx context.Context, playerID int64) ([]models.TournamentRosterEntry, error) {
	return s.repo.ListAssignments(ctx, playerID)
}

// Tournaments ----------------------------------------------------------------

type TournamentsService interface {
	List(ctx context.Context, status *models.TournamentStatus) ([]models.Tournament, error)
	Get(ctx context.Context, id int64) (*models.Tournament, error)
	Create(ctx context.Context, input CreateTournamentInput) (int64, error)
	Update(ctx context.Context, id int64, patch models.TournamentPatch) error
}

type CreateTournamentInput struct {
	Name      string
	Type      *string
	Status    models.TournamentStatus
	StartDate *time.Time
	EndDate   *time.Time
	Note      *string
}

type tournamentsService struct {
	repo repository.TournamentsRepository
}

func NewTournamentsService(repo repository.TournamentsRepository) TournamentsService {
	return &tournamentsService{repo: repo}
}

func (s *tournamentsService) List(ctx context.Context, status *models.TournamentStatus) ([]models.Tournament, error) {
	return s.repo.List(ctx, status)
}

func (s *tournamentsService) Get(ctx context.Context, id int64) (*models.Tournament, error) {
	return s.repo.Get(ctx, id)
}

func (s *tournamentsService) Create(ctx context.Context, input CreateTournamentInput) (int64, error) {
	if input.Name == "" {
		return 0, fmt.Errorf("name: %w", models.ErrValidation)
	}
	if input.Status == "" {
		input.Status = models.TournamentStatusPlanned
	}
	tournament := models.Tournament{
		Name:      input.Name,
		Type:      input.Type,
		Status:    input.Status,
		StartDate: input.StartDate,
		EndDate:   input.EndDate,
		Note:      input.Note,
	}
	return s.repo.Create(ctx, tournament)
}

func (s *tournamentsService) Update(ctx context.Context, id int64, patch models.TournamentPatch) error {
	return s.repo.Update(ctx, id, patch)
}

// Rosters --------------------------------------------------------------------

type RostersService interface {
	ListTeamsInTournament(ctx context.Context, tournamentID int64) ([]models.TournamentTeam, error)
	ListRoster(ctx context.Context, tournamentID, teamID int64) ([]models.TournamentRosterEntry, error)
	AddPlayer(ctx context.Context, tournamentID, teamID, playerID int64, number *int) error
	UpdateNumber(ctx context.Context, tournamentID, teamID, playerID int64, number *int) error
	RemovePlayer(ctx context.Context, tournamentID, teamID, playerID int64) error
	EnsureTeamHasPlayers(ctx context.Context, tournamentID, teamID int64) (bool, error)
	IsPlayerInRoster(ctx context.Context, tournamentID, teamID, playerID int64) (bool, error)
}

type rostersService struct {
	repo repository.RostersRepository
}

func NewRostersService(repo repository.RostersRepository) RostersService {
	return &rostersService{repo: repo}
}

func (s *rostersService) ListTeamsInTournament(ctx context.Context, tournamentID int64) ([]models.TournamentTeam, error) {
	return s.repo.ListTeams(ctx, tournamentID)
}

func (s *rostersService) ListRoster(ctx context.Context, tournamentID, teamID int64) ([]models.TournamentRosterEntry, error) {
	return s.repo.ListRoster(ctx, tournamentID, teamID)
}

func (s *rostersService) AddPlayer(ctx context.Context, tournamentID, teamID, playerID int64, number *int) error {
	return s.repo.AddPlayer(ctx, tournamentID, teamID, playerID, number)
}

func (s *rostersService) UpdateNumber(ctx context.Context, tournamentID, teamID, playerID int64, number *int) error {
	return s.repo.UpdateNumber(ctx, tournamentID, teamID, playerID, number)
}

func (s *rostersService) RemovePlayer(ctx context.Context, tournamentID, teamID, playerID int64) error {
	involved, err := s.repo.PlayerParticipation(ctx, tournamentID, teamID, playerID)
	if err != nil {
		return err
	}
	if involved {
		return fmt.Errorf("player has participation records: %w", models.ErrValidation)
	}
	return s.repo.RemovePlayer(ctx, tournamentID, teamID, playerID)
}

func (s *rostersService) EnsureTeamHasPlayers(ctx context.Context, tournamentID, teamID int64) (bool, error) {
	count, err := s.repo.TeamPlayerCount(ctx, tournamentID, teamID)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *rostersService) IsPlayerInRoster(ctx context.Context, tournamentID, teamID, playerID int64) (bool, error) {
	return s.repo.IsPlayerInRoster(ctx, tournamentID, teamID, playerID)
}

// Matches --------------------------------------------------------------------

type MatchesService interface {
	List(ctx context.Context, tournamentID, teamID int64) ([]models.Match, error)
	Get(ctx context.Context, id int64) (*models.Match, error)
	Create(ctx context.Context, input CreateMatchInput) (int64, error)
	Update(ctx context.Context, id int64, patch models.MatchPatch) error
}

type CreateMatchInput struct {
	TournamentID int64
	TeamID       int64
	Opponent     string
	StartTime    time.Time
	Location     *string
	Status       models.MatchStatus
}

type matchesService struct {
	repo        repository.MatchesRepository
	rostersRepo repository.RostersRepository
}

func NewMatchesService(repo repository.MatchesRepository, rosters repository.RostersRepository) MatchesService {
	return &matchesService{repo: repo, rostersRepo: rosters}
}

func (s *matchesService) List(ctx context.Context, tournamentID, teamID int64) ([]models.Match, error) {
	return s.repo.List(ctx, tournamentID, teamID)
}

func (s *matchesService) Get(ctx context.Context, id int64) (*models.Match, error) {
	return s.repo.Get(ctx, id)
}

func (s *matchesService) Create(ctx context.Context, input CreateMatchInput) (int64, error) {
	if input.TournamentID == 0 || input.TeamID == 0 {
		return 0, fmt.Errorf("tournament/team: %w", models.ErrValidation)
	}
	if input.Opponent == "" {
		return 0, fmt.Errorf("opponent: %w", models.ErrValidation)
	}
	if input.StartTime.IsZero() {
		return 0, fmt.Errorf("start_time: %w", models.ErrValidation)
	}
	hasPlayers, err := s.rostersRepo.TeamPlayerCount(ctx, input.TournamentID, input.TeamID)
	if err != nil {
		return 0, err
	}
	if hasPlayers == 0 {
		return 0, fmt.Errorf("team has no players in roster: %w", models.ErrValidation)
	}
	match := models.Match{
		TournamentID: input.TournamentID,
		TeamID:       input.TeamID,
		OpponentName: input.Opponent,
		StartTime:    input.StartTime,
		Location:     input.Location,
		Status:       input.Status,
	}
	if match.Status == "" {
		match.Status = models.MatchStatusScheduled
	}
	return s.repo.Create(ctx, match)
}

func (s *matchesService) Update(ctx context.Context, id int64, patch models.MatchPatch) error {
	if patch.Status != nil && *patch.Status == models.MatchStatusCanceled {
		patch.ScoreHT = models.NewOptionalString(nil)
		patch.ScoreFT = models.NewOptionalString(nil)
		patch.ScoreET = models.NewOptionalString(nil)
		patch.ScorePEN = models.NewOptionalString(nil)
		patch.ScoreFinalUs = models.NewOptionalInt(nil)
		patch.ScoreFinalThem = models.NewOptionalInt(nil)
	}
	return s.repo.Update(ctx, id, patch)
}

// Lineup ---------------------------------------------------------------------

type LineupService interface {
	Get(ctx context.Context, matchID int64) ([]models.MatchLineup, error)
	Upsert(ctx context.Context, matchID, playerID int64, role models.LineupRole, numberOverride *int, note *string) error
	Update(ctx context.Context, matchID, playerID int64, patch models.LineupPatch) error
	Remove(ctx context.Context, matchID, playerID int64) error
}

type lineupService struct {
	repo        repository.LineupRepository
	matchesRepo repository.MatchesRepository
	rosterRepo  repository.RostersRepository
}

func NewLineupService(repo repository.LineupRepository, matches repository.MatchesRepository, rosters repository.RostersRepository) LineupService {
	return &lineupService{repo: repo, matchesRepo: matches, rosterRepo: rosters}
}

func (s *lineupService) Get(ctx context.Context, matchID int64) ([]models.MatchLineup, error) {
	return s.repo.Get(ctx, matchID)
}

func (s *lineupService) Upsert(ctx context.Context, matchID, playerID int64, role models.LineupRole, numberOverride *int, note *string) error {
	match, err := s.matchesRepo.Get(ctx, matchID)
	if err != nil {
		return err
	}
	inRoster, err := s.rosterRepo.IsPlayerInRoster(ctx, match.TournamentID, match.TeamID, playerID)
	if err != nil {
		return err
	}
	if !inRoster {
		return fmt.Errorf("player not in tournament roster: %w", models.ErrValidation)
	}
	if role != models.LineupRoleStart && role != models.LineupRoleSub {
		return fmt.Errorf("invalid role: %w", models.ErrValidation)
	}
	return s.repo.Upsert(ctx, matchID, playerID, role, numberOverride, note)
}

func (s *lineupService) Update(ctx context.Context, matchID, playerID int64, patch models.LineupPatch) error {
	if patch.Role != nil {
		if *patch.Role != models.LineupRoleStart && *patch.Role != models.LineupRoleSub {
			return fmt.Errorf("invalid role: %w", models.ErrValidation)
		}
	}
	return s.repo.Update(ctx, matchID, playerID, patch)
}

func (s *lineupService) Remove(ctx context.Context, matchID, playerID int64) error {
	return s.repo.Remove(ctx, matchID, playerID)
}

// Events ---------------------------------------------------------------------

type EventsService interface {
	List(ctx context.Context, matchID int64) ([]models.MatchEvent, error)
	AddGoal(ctx context.Context, matchID, playerID int64, timeText string) error
	AddCard(ctx context.Context, matchID, playerID int64, cardType models.CardType, timeText string) error
	AddSub(ctx context.Context, matchID, playerOutID, playerInID int64, timeText string) error
}

type eventsService struct {
	repo        repository.EventsRepository
	matchesRepo repository.MatchesRepository
	rosterRepo  repository.RostersRepository
}

func NewEventsService(repo repository.EventsRepository, matches repository.MatchesRepository, rosters repository.RostersRepository) EventsService {
	return &eventsService{repo: repo, matchesRepo: matches, rosterRepo: rosters}
}

func (s *eventsService) List(ctx context.Context, matchID int64) ([]models.MatchEvent, error) {
	return s.repo.List(ctx, matchID)
}

func (s *eventsService) AddGoal(ctx context.Context, matchID, playerID int64, timeText string) error {
	if timeText == "" {
		return fmt.Errorf("event_time: %w", models.ErrValidation)
	}
	match, err := s.matchesRepo.Get(ctx, matchID)
	if err != nil {
		return err
	}
	if err := s.ensureRoster(ctx, match, []int64{playerID}); err != nil {
		return err
	}
	event := models.MatchEvent{
		MatchID:       matchID,
		EventType:     models.MatchEventGoal,
		EventTimeText: timeText,
		PlayerMainID:  &playerID,
	}
	_, err = s.repo.Add(ctx, event)
	return err
}

func (s *eventsService) AddCard(ctx context.Context, matchID, playerID int64, cardType models.CardType, timeText string) error {
	if cardType != models.CardTypeYellow && cardType != models.CardTypeRed {
		return fmt.Errorf("card_type: %w", models.ErrValidation)
	}
	if timeText == "" {
		return fmt.Errorf("event_time: %w", models.ErrValidation)
	}
	match, err := s.matchesRepo.Get(ctx, matchID)
	if err != nil {
		return err
	}
	if err := s.ensureRoster(ctx, match, []int64{playerID}); err != nil {
		return err
	}
	event := models.MatchEvent{
		MatchID:       matchID,
		EventType:     models.MatchEventCard,
		EventTimeText: timeText,
		PlayerMainID:  &playerID,
	}
	event.CardType = &cardType
	_, err = s.repo.Add(ctx, event)
	return err
}

func (s *eventsService) AddSub(ctx context.Context, matchID, playerOutID, playerInID int64, timeText string) error {
	if timeText == "" {
		return fmt.Errorf("event_time: %w", models.ErrValidation)
	}
	match, err := s.matchesRepo.Get(ctx, matchID)
	if err != nil {
		return err
	}
	if err := s.ensureRoster(ctx, match, []int64{playerOutID, playerInID}); err != nil {
		return err
	}
	if playerOutID == playerInID {
		return fmt.Errorf("players identical: %w", models.ErrValidation)
	}
	event := models.MatchEvent{
		MatchID:       matchID,
		EventType:     models.MatchEventSub,
		EventTimeText: timeText,
		PlayerMainID:  &playerOutID,
		PlayerAltID:   &playerInID,
	}
	_, err = s.repo.Add(ctx, event)
	return err
}

func (s *eventsService) ensureRoster(ctx context.Context, match *models.Match, playerIDs []int64) error {
	for _, id := range playerIDs {
		inRoster, err := s.rosterRepo.IsPlayerInRoster(ctx, match.TournamentID, match.TeamID, id)
		if err != nil {
			return err
		}
		if !inRoster {
			return fmt.Errorf("player %d not in roster: %w", id, models.ErrValidation)
		}
	}
	return nil
}

// Sessions -------------------------------------------------------------------

type SessionService interface {
	Get(ctx context.Context, adminID int64) (*models.AdminSession, error)
	Save(ctx context.Context, session models.AdminSession) error
	Delete(ctx context.Context, adminID int64) error
}

type sessionService struct {
	repo repository.SessionsRepository
}

func NewSessionService(repo repository.SessionsRepository) SessionService {
	return &sessionService{repo: repo}
}

func (s *sessionService) Get(ctx context.Context, adminID int64) (*models.AdminSession, error) {
	session, err := s.repo.Get(ctx, adminID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return session, nil
}

func (s *sessionService) Save(ctx context.Context, session models.AdminSession) error {
	return s.repo.Upsert(ctx, session)
}

func (s *sessionService) Delete(ctx context.Context, adminID int64) error {
	return s.repo.Delete(ctx, adminID)
}

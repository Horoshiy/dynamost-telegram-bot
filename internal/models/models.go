package models

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates absence of a record.
	ErrNotFound = errors.New("not found")
	// ErrConflict indicates uniqueness or state conflict.
	ErrConflict = errors.New("conflict")
	// ErrValidation indicates business rule violation.
	ErrValidation = errors.New("validation error")
)

type NavigationEntry struct {
	Action string            `json:"action"`
	Params map[string]string `json:"params,omitempty"`
}

type Team struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	ShortCode string    `json:"short_code"`
	Active    bool      `json:"active"`
	Note      *string   `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TeamPatch struct {
	Name      *string
	ShortCode *string
	Active    *bool
	Note      OptionalString
}

type Player struct {
	ID        int64      `json:"id"`
	FullName  string     `json:"full_name"`
	BirthDate *time.Time `json:"birth_date,omitempty"`
	Position  *string    `json:"position,omitempty"`
	Active    bool       `json:"active"`
	Note      *string    `json:"note,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type PlayerPatch struct {
	FullName  *string
	BirthDate OptionalTime
	Position  OptionalString
	Active    *bool
	Note      OptionalString
}

type TournamentStatus string

const (
	TournamentStatusPlanned  TournamentStatus = "planned"
	TournamentStatusActive   TournamentStatus = "active"
	TournamentStatusFinished TournamentStatus = "finished"
)

type Tournament struct {
	ID        int64            `json:"id"`
	Name      string           `json:"name"`
	Type      *string          `json:"type,omitempty"`
	Status    TournamentStatus `json:"status"`
	StartDate *time.Time       `json:"start_date,omitempty"`
	EndDate   *time.Time       `json:"end_date,omitempty"`
	Note      *string          `json:"note,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type TournamentPatch struct {
	Name      *string
	Type      OptionalString
	Status    *TournamentStatus
	StartDate OptionalTime
	EndDate   OptionalTime
	Note      OptionalString
}

type TournamentRosterEntry struct {
	ID               int64     `json:"id"`
	TournamentID     int64     `json:"tournament_id"`
	TeamID           int64     `json:"team_id"`
	PlayerID         int64     `json:"player_id"`
	PlayerName       string    `json:"player_name"`
	TournamentNumber *int      `json:"tournament_number,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type TournamentTeam struct {
	TeamID    int64  `json:"team_id"`
	TeamName  string `json:"team_name"`
	ShortCode string `json:"short_code"`
}

type MatchStatus string

const (
	MatchStatusScheduled MatchStatus = "scheduled"
	MatchStatusPlayed    MatchStatus = "played"
	MatchStatusCanceled  MatchStatus = "canceled"
)

type Match struct {
	ID             int64       `json:"id"`
	TournamentID   int64       `json:"tournament_id"`
	TeamID         int64       `json:"team_id"`
	OpponentName   string      `json:"opponent_name"`
	StartTime      time.Time   `json:"start_time"`
	Location       *string     `json:"location,omitempty"`
	Status         MatchStatus `json:"status"`
	ScoreHT        *string     `json:"score_ht,omitempty"`
	ScoreFT        *string     `json:"score_ft,omitempty"`
	ScoreET        *string     `json:"score_et,omitempty"`
	ScorePEN       *string     `json:"score_pen,omitempty"`
	ScoreFinalUs   *int        `json:"score_final_us,omitempty"`
	ScoreFinalThem *int        `json:"score_final_them,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type MatchPatch struct {
	StartTime      OptionalTime
	Location       OptionalString
	Status         *MatchStatus
	ScoreHT        OptionalString
	ScoreFT        OptionalString
	ScoreET        OptionalString
	ScorePEN       OptionalString
	ScoreFinalUs   OptionalInt
	ScoreFinalThem OptionalInt
	OpponentName   *string
}

type LineupRole string

const (
	LineupRoleStart LineupRole = "start"
	LineupRoleSub   LineupRole = "sub"
)

type MatchLineup struct {
	ID             int64      `json:"id"`
	MatchID        int64      `json:"match_id"`
	PlayerID       int64      `json:"player_id"`
	PlayerName     string     `json:"player_name"`
	RosterNumber   *int       `json:"roster_number,omitempty"`
	Role           LineupRole `json:"role"`
	NumberOverride *int       `json:"number_override,omitempty"`
	Note           *string    `json:"note,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type LineupPatch struct {
	Role           *LineupRole
	NumberOverride OptionalInt
	Note           OptionalString
}

type MatchEventType string

const (
	MatchEventGoal MatchEventType = "goal"
	MatchEventCard MatchEventType = "card"
	MatchEventSub  MatchEventType = "sub"
)

type CardType string

const (
	CardTypeYellow CardType = "yellow"
	CardTypeRed    CardType = "red"
)

type MatchEvent struct {
	ID            int64          `json:"id"`
	MatchID       int64          `json:"match_id"`
	EventType     MatchEventType `json:"event_type"`
	EventTimeText string         `json:"event_time"`
	PlayerMainID  *int64         `json:"player_id_main,omitempty"`
	PlayerAltID   *int64         `json:"player_id_alt,omitempty"`
	CardType      *CardType      `json:"card_type,omitempty"`
	PlayerMain    *string        `json:"player_main,omitempty"`
	PlayerAlt     *string        `json:"player_alt,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

type Pagination struct {
	Limit  int
	Offset int
}

func NewPagination(page, perPage int) Pagination {
	if perPage <= 0 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	if page < 1 {
		page = 1
	}
	return Pagination{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}
}

type AdminSession struct {
	AdminID     int64
	CurrentFlow *string
	FlowState   []byte
	UpdatedAt   time.Time
}

type OptionalString struct {
	Set   bool
	Value *string
}

func NewOptionalString(v *string) OptionalString {
	return OptionalString{Set: true, Value: v}
}

type OptionalTime struct {
	Set   bool
	Value *time.Time
}

func NewOptionalTime(v *time.Time) OptionalTime {
	return OptionalTime{Set: true, Value: v}
}

type OptionalInt struct {
	Set   bool
	Value *int
}

func NewOptionalInt(v *int) OptionalInt {
	return OptionalInt{Set: true, Value: v}
}

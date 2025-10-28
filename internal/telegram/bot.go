package telegram

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/dynamost/telegram-bot/internal/models"
	"github.com/dynamost/telegram-bot/internal/repository"
	"github.com/dynamost/telegram-bot/internal/service"
	"github.com/dynamost/telegram-bot/internal/session"
)

const (
	perPage     = 20
	maxNavDepth = 10
)

const (
	flowCreateTournament   = "create_tournament"
	flowEditTournament     = "edit_tournament"
	flowCreateTeam         = "create_team"
	flowEditTeam           = "edit_team"
	flowCreatePlayer       = "create_player"
	flowEditPlayer         = "edit_player"
	flowRosterAddPlayer    = "roster_add_player"
	flowRosterChangeNumber = "roster_change_number"
	flowMatchCreate        = "match_create"
	flowMatchEdit          = "match_edit"
	flowLineupNumber       = "lineup_number"
	flowEventGoal          = "event_goal"
	flowEventCard          = "event_card"
	flowEventSub           = "event_sub"
)

type Services struct {
	Teams       service.TeamsService
	Players     service.PlayersService
	Tournaments service.TournamentsService
	Rosters     service.RostersService
	Matches     service.MatchesService
	Lineup      service.LineupService
	Events      service.EventsService
	Sessions    *session.Store
}

type navEntry = models.NavigationEntry

type Bot struct {
	api     *tgbotapi.BotAPI
	admins  map[int64]struct{}
	svc     Services
	logger  repository.Logger
	loc     *time.Location
	timeNow func() time.Time
	navMu   sync.Mutex
	nav     map[int64][]navEntry
}

func NewBot(api *tgbotapi.BotAPI, adminIDs []int64, loc *time.Location, svc Services, logger repository.Logger) *Bot {
	adminMap := make(map[int64]struct{}, len(adminIDs))
	for _, id := range adminIDs {
		adminMap[id] = struct{}{}
	}
	return &Bot{
		api:     api,
		admins:  adminMap,
		loc:     loc,
		svc:     svc,
		logger:  logger,
		timeNow: time.Now,
		nav:     make(map[int64][]navEntry),
	}
}

func (b *Bot) pushNav(ctx context.Context, adminID int64, entry navEntry) {
	if entry.Action == "" {
		return
	}
	b.navMu.Lock()
	stack := b.nav[adminID]
	if len(stack) > 0 {
		last := stack[len(stack)-1]
		if last.Action == entry.Action && compareParamMaps(last.Params, entry.Params) {
			b.navMu.Unlock()
			return
		}
	}
	stack = append(stack, entry)
	if len(stack) > maxNavDepth {
		stack = stack[1:]
	}
	b.nav[adminID] = stack
	snapshot := make([]navEntry, len(stack))
	copy(snapshot, stack)
	b.navMu.Unlock()
	b.persistNav(ctx, adminID, snapshot)
}

func (b *Bot) popNav(ctx context.Context, adminID int64) (navEntry, bool) {
	b.navMu.Lock()
	stack := b.nav[adminID]
	if len(stack) == 0 {
		b.navMu.Unlock()
		return navEntry{}, false
	}
	entry := stack[len(stack)-1]
	stack = stack[:len(stack)-1]
	if len(stack) == 0 {
		delete(b.nav, adminID)
	} else {
		b.nav[adminID] = stack
	}
	snapshot := make([]navEntry, len(stack))
	copy(snapshot, stack)
	b.navMu.Unlock()
	b.persistNav(ctx, adminID, snapshot)
	return entry, true
}

func (b *Bot) clearNav(ctx context.Context, adminID int64) {
	b.navMu.Lock()
	_, existed := b.nav[adminID]
	delete(b.nav, adminID)
	b.navMu.Unlock()
	if existed {
		b.persistNav(ctx, adminID, nil)
	}
}

func (b *Bot) persistNav(ctx context.Context, adminID int64, stack []navEntry) {
	var entries []models.NavigationEntry
	if len(stack) > 0 {
		entries = make([]models.NavigationEntry, len(stack))
		for i, entry := range stack {
			entries[i] = models.NavigationEntry(entry)
		}
	}
	if err := b.svc.Sessions.Save(ctx, adminID, nil, nil, entries); err != nil {
		b.logger.Error(err, "persist_nav", "nav", int64(len(stack)), adminID)
	}
}

func (b *Bot) snapshotNav(adminID int64) []models.NavigationEntry {
	b.navMu.Lock()
	defer b.navMu.Unlock()
	stack := b.nav[adminID]
	if len(stack) == 0 {
		return nil
	}
	entries := make([]models.NavigationEntry, len(stack))
	for i, entry := range stack {
		entries[i] = models.NavigationEntry(entry)
	}
	return entries
}

func (b *Bot) restoreNav(adminID int64, entries []models.NavigationEntry) {
	b.navMu.Lock()
	defer b.navMu.Unlock()
	if len(entries) == 0 {
		delete(b.nav, adminID)
		return
	}
	if len(entries) > maxNavDepth {
		entries = entries[len(entries)-maxNavDepth:]
	}
	stack := make([]navEntry, len(entries))
	for i, entry := range entries {
		stack[i] = navEntry(entry)
	}
	b.nav[adminID] = stack
}

func (b *Bot) saveSession(ctx context.Context, adminID int64, flowName *string, state any) error {
	return b.svc.Sessions.Save(ctx, adminID, flowName, state, b.snapshotNav(adminID))
}

func (b *Bot) handleNavEntry(ctx context.Context, chatID int64, entry navEntry) error {
	switch entry.Action {
	case "tournaments_page":
		page := parseIntParam(entry.Params, "page", 1)
		return b.sendTournamentList(ctx, chatID, page)
	case "teams_menu":
		return b.sendTeams(ctx, chatID)
	case "players_menu":
		page := parseIntParam(entry.Params, "page", 1)
		return b.sendPlayersPage(ctx, chatID, page)
	case "games_open_team":
		tournamentID := parseInt64(entry.Params["t"])
		teamID := parseInt64(entry.Params["team"])
		return b.sendGamesMatches(ctx, chatID, tournamentID, teamID)
	case "games_open_tournament":
		tournamentID := parseInt64(entry.Params["id"])
		return b.sendGamesTeams(ctx, chatID, tournamentID)
	case "roster_open_tournament":
		tournamentID := parseInt64(entry.Params["id"])
		return b.sendRosterTeams(ctx, chatID, tournamentID)
	default:
		b.sendSimple(chatID, "Вернуться не удалось.")
		return nil
	}
}

func (b *Bot) Run(ctx context.Context) error {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := b.api.GetUpdatesChan(updateConfig)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			if err := b.handleUpdate(ctx, update); err != nil {
				b.logger.Error(err, "handle_update", "update", int64(update.UpdateID), 0)
			}
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if update.Message != nil {
		return b.handleMessage(ctx, update.Message)
	}
	if update.CallbackQuery != nil {
		return b.handleCallback(ctx, update.CallbackQuery)
	}
	return nil
}

func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) error {
	if msg.From == nil {
		return nil
	}
	adminID := msg.From.ID
	if !b.isAdmin(adminID) {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "У вас нет прав. Обратитесь к директору клуба.")
		reply.ReplyToMessageID = msg.MessageID
		_, _ = b.api.Send(reply)
		return nil
	}

	if msg.IsCommand() {
		b.clearNav(ctx, adminID)
		switch msg.Command() {
		case "start":
			b.sendSimple(msg.Chat.ID, "Доступные разделы: /tournaments, /teams, /players, /tournament_rosters, /games.")
		case "tournaments":
			return b.sendTournamentList(ctx, msg.Chat.ID, 1)
		case "teams":
			return b.sendTeams(ctx, msg.Chat.ID)
		case "players":
			return b.sendPlayersPage(ctx, msg.Chat.ID, 1)
		case "tournament_rosters":
			return b.sendRosterTournaments(ctx, msg.Chat.ID)
		case "games":
			return b.sendGamesTournaments(ctx, msg.Chat.ID)
		default:
			b.sendSimple(msg.Chat.ID, "Неизвестная команда.")
		}
		return nil
	}

	// Attempt to continue a wizard flow.
	sessionState := &wizardState{}
	var navState []models.NavigationEntry
	stored, err := b.svc.Sessions.Load(ctx, adminID, sessionState, &navState)
	if err != nil {
		return err
	}
	b.restoreNav(adminID, navState)
	if stored == nil || sessionState.Flow == "" {
		// Plain message without wizard – ignore.
		return nil
	}

	return b.advanceWizard(ctx, msg, sessionState)
}

func (b *Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) error {
	if cb.From == nil {
		return nil
	}
	adminID := cb.From.ID
	if !b.isAdmin(adminID) {
		_, _ = b.api.Request(tgbotapi.NewCallback(cb.ID, "Недостаточно прав"))
		return nil
	}

	payload, err := parseCallback(cb.Data)
	if err != nil {
		_, _ = b.api.Request(tgbotapi.NewCallback(cb.ID, "Некорректная кнопка"))
		return nil
	}

	switch payload.Action {
	case "open_tournament":
		id, _ := strconv.ParseInt(payload.Params["id"], 10, 64)
		page := parseIntParam(payload.Params, "page", 1)
		b.pushNav(ctx, adminID, navEntry{
			Action: "tournaments_page",
			Params: map[string]string{"page": strconv.Itoa(page)},
		})
		if err := b.showTournament(ctx, cb.Message.Chat.ID, cb.Message.MessageID, id); err != nil {
			return err
		}
	case "tournaments_page":
		page, _ := strconv.Atoi(payload.Params["page"])
		if page < 1 {
			page = 1
		}
		return b.sendTournamentList(ctx, cb.Message.Chat.ID, page)
	case "tournaments_start_create":
		return b.startTournamentWizard(ctx, cb.Message.Chat.ID, cb.From.ID)
	case "tournament_edit":
		tournamentID := parseInt64(payload.Params["id"])
		return b.startTournamentEditWizard(ctx, cb.Message.Chat.ID, cb.From.ID, tournamentID)
	case "teams_start_create":
		return b.startTeamWizard(ctx, cb.Message.Chat.ID, cb.From.ID)
	case "team_open":
		teamID := parseInt64(payload.Params["id"])
		b.pushNav(ctx, adminID, navEntry{Action: "teams_menu"})
		return b.showTeam(ctx, cb.Message.Chat.ID, teamID)
	case "teams_menu":
		return b.sendTeams(ctx, cb.Message.Chat.ID)
	case "team_edit":
		teamID := parseInt64(payload.Params["id"])
		return b.startTeamEditWizard(ctx, cb.Message.Chat.ID, cb.From.ID, teamID)
	case "players_page":
		page, _ := strconv.Atoi(payload.Params["page"])
		if page < 1 {
			page = 1
		}
		return b.sendPlayersPage(ctx, cb.Message.Chat.ID, page)
	case "players_start_create":
		return b.startPlayerWizard(ctx, cb.Message.Chat.ID, cb.From.ID)
	case "player_open":
		playerID := parseInt64(payload.Params["id"])
		page := parseIntParam(payload.Params, "page", 1)
		b.pushNav(ctx, adminID, navEntry{
			Action: "players_menu",
			Params: map[string]string{"page": strconv.Itoa(page)},
		})
		return b.showPlayer(ctx, cb.Message.Chat.ID, playerID, page)
	case "players_menu":
		page := parseInt64(payload.Params["page"])
		if page < 1 {
			page = 1
		}
		return b.sendPlayersPage(ctx, cb.Message.Chat.ID, int(page))
	case "player_edit":
		playerID := parseInt64(payload.Params["id"])
		page := parseInt64(payload.Params["page"])
		return b.startPlayerEditWizard(ctx, cb.Message.Chat.ID, cb.From.ID, playerID, int(page))
	case "roster_open_tournament":
		tournamentID := parseInt64(payload.Params["id"])
		return b.sendRosterTeams(ctx, cb.Message.Chat.ID, tournamentID)
	case "roster_open_team":
		tournamentID := parseInt64(payload.Params["t"])
		teamID := parseInt64(payload.Params["team"])
		b.pushNav(ctx, adminID, navEntry{
			Action: "roster_open_tournament",
			Params: map[string]string{"id": strconv.FormatInt(tournamentID, 10)},
		})
		return b.showRoster(ctx, cb.Message.Chat.ID, tournamentID, teamID)
	case "roster_add_player":
		tournamentID := parseInt64(payload.Params["t"])
		teamID := parseInt64(payload.Params["team"])
		page, _ := strconv.Atoi(payload.Params["page"])
		if page < 1 {
			page = 1
		}
		return b.sendRosterAddPlayerList(ctx, cb.Message.Chat.ID, tournamentID, teamID, page)
	case "roster_add_pick":
		tournamentID := parseInt64(payload.Params["t"])
		teamID := parseInt64(payload.Params["team"])
		playerID := parseInt64(payload.Params["player"])
		return b.startRosterAddWizard(ctx, cb.Message.Chat.ID, cb.From.ID, tournamentID, teamID, playerID)
	case "roster_change_number":
		tournamentID := parseInt64(payload.Params["t"])
		teamID := parseInt64(payload.Params["team"])
		playerID := parseInt64(payload.Params["player"])
		return b.startRosterChangeWizard(ctx, cb.Message.Chat.ID, cb.From.ID, tournamentID, teamID, playerID)
	case "roster_remove_player":
		tournamentID := parseInt64(payload.Params["t"])
		teamID := parseInt64(payload.Params["team"])
		playerID := parseInt64(payload.Params["player"])
		if err := b.svc.Rosters.RemovePlayer(ctx, tournamentID, teamID, playerID); err != nil {
			b.sendSimple(cb.Message.Chat.ID, fmt.Sprintf("Не удалось удалить игрока: %v", err))
		} else {
			b.sendSimple(cb.Message.Chat.ID, "Игрок удалён из заявки.")
		}
		return b.showRoster(ctx, cb.Message.Chat.ID, tournamentID, teamID)
	case "games_open_tournament":
		tournamentID := parseInt64(payload.Params["id"])
		return b.sendGamesTeams(ctx, cb.Message.Chat.ID, tournamentID)
	case "games_open_team":
		tournamentID := parseInt64(payload.Params["t"])
		teamID := parseInt64(payload.Params["team"])
		b.pushNav(ctx, adminID, navEntry{
			Action: "games_open_tournament",
			Params: map[string]string{"id": strconv.FormatInt(tournamentID, 10)},
		})
		return b.sendGamesMatches(ctx, cb.Message.Chat.ID, tournamentID, teamID)
	case "match_start_create":
		tournamentID := parseInt64(payload.Params["t"])
		teamID := parseInt64(payload.Params["team"])
		return b.startMatchCreateWizard(ctx, cb.Message.Chat.ID, cb.From.ID, tournamentID, teamID)
	case "open_match":
		matchID := parseInt64(payload.Params["id"])
		match, err := b.svc.Matches.Get(ctx, matchID)
		if err != nil {
			return err
		}
		b.pushNav(ctx, adminID, navEntry{
			Action: "games_open_team",
			Params: map[string]string{
				"t":    strconv.FormatInt(match.TournamentID, 10),
				"team": strconv.FormatInt(match.TeamID, 10),
			},
		})
		return b.renderMatch(ctx, cb.Message.Chat.ID, match)
	case "match_edit":
		matchID := parseInt64(payload.Params["id"])
		return b.startMatchEditWizard(ctx, cb.Message.Chat.ID, cb.From.ID, matchID)
	case "match_lineup_menu":
		matchID := parseInt64(payload.Params["match"])
		return b.sendLineupMenu(ctx, cb.Message.Chat.ID, matchID)
	case "match_lineup_add":
		matchID := parseInt64(payload.Params["match"])
		page, _ := strconv.Atoi(payload.Params["page"])
		if page < 1 {
			page = 1
		}
		return b.sendLineupAddList(ctx, cb.Message.Chat.ID, matchID, page)
	case "match_lineup_add_pick":
		matchID := parseInt64(payload.Params["match"])
		playerID := parseInt64(payload.Params["player"])
		return b.addPlayerToLineup(ctx, cb.Message.Chat.ID, matchID, playerID)
	case "match_lineup_remove":
		matchID := parseInt64(payload.Params["match"])
		playerID := parseInt64(payload.Params["player"])
		return b.removePlayerFromLineup(ctx, cb.Message.Chat.ID, matchID, playerID)
	case "match_lineup_role_toggle":
		matchID := parseInt64(payload.Params["match"])
		playerID := parseInt64(payload.Params["player"])
		return b.toggleLineupRole(ctx, cb.Message.Chat.ID, matchID, playerID)
	case "match_lineup_number":
		matchID := parseInt64(payload.Params["match"])
		playerID := parseInt64(payload.Params["player"])
		return b.startLineupNumberWizard(ctx, cb.Message.Chat.ID, cb.From.ID, matchID, playerID)
	case "match_events_menu":
		matchID := parseInt64(payload.Params["match"])
		return b.sendEventsMenu(ctx, cb.Message.Chat.ID, matchID)
	case "match_events_add_goal":
		matchID := parseInt64(payload.Params["match"])
		return b.sendEventsGoalPlayerList(ctx, cb.Message.Chat.ID, matchID)
	case "match_events_goal_pick":
		matchID := parseInt64(payload.Params["match"])
		playerID := parseInt64(payload.Params["player"])
		return b.startEventGoalWizard(ctx, cb.Message.Chat.ID, cb.From.ID, matchID, playerID)
	case "match_events_add_card":
		matchID := parseInt64(payload.Params["match"])
		return b.sendEventsCardPlayerList(ctx, cb.Message.Chat.ID, matchID)
	case "match_events_card_pick":
		matchID := parseInt64(payload.Params["match"])
		playerID := parseInt64(payload.Params["player"])
		return b.sendEventsCardTypeMenu(ctx, cb.Message.Chat.ID, matchID, playerID)
	case "match_events_card_type":
		matchID := parseInt64(payload.Params["match"])
		playerID := parseInt64(payload.Params["player"])
		cardType := payload.Params["type"]
		return b.startEventCardWizard(ctx, cb.Message.Chat.ID, cb.From.ID, matchID, playerID, cardType)
	case "match_events_add_sub":
		matchID := parseInt64(payload.Params["match"])
		return b.sendEventSubOutList(ctx, cb.Message.Chat.ID, matchID)
	case "match_events_sub_pick_out":
		matchID := parseInt64(payload.Params["match"])
		outID := parseInt64(payload.Params["player"])
		return b.sendEventSubInList(ctx, cb.Message.Chat.ID, matchID, outID)
	case "match_events_sub_pick_in":
		matchID := parseInt64(payload.Params["match"])
		outID := parseInt64(payload.Params["out"])
		inID := parseInt64(payload.Params["player"])
		return b.startEventSubWizard(ctx, cb.Message.Chat.ID, cb.From.ID, matchID, outID, inID)
	case "match_status_set":
		matchID := parseInt64(payload.Params["id"])
		status := payload.Params["status"]
		return b.setMatchStatus(ctx, cb.Message.Chat.ID, matchID, status)
	case "match_scores_reset":
		matchID := parseInt64(payload.Params["id"])
		return b.resetMatchScores(ctx, cb.Message.Chat.ID, matchID)
	case "nav_back":
		entry, ok := b.popNav(ctx, adminID)
		if !ok {
			b.sendSimple(cb.Message.Chat.ID, "История экранов пуста.")
			return nil
		}
		return b.handleNavEntry(ctx, cb.Message.Chat.ID, entry)
	default:
		_, _ = b.api.Request(tgbotapi.NewCallback(cb.ID, "Функция в разработке"))
	}
	_, _ = b.api.Request(tgbotapi.NewCallback(cb.ID, ""))
	return nil
}

func (b *Bot) isAdmin(id int64) bool {
	_, ok := b.admins[id]
	return ok
}

// ----------------------------------------------------------------------------
// Renderers

func (b *Bot) sendSimple(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	_, _ = b.api.Send(msg)
}

func (b *Bot) sendTournamentList(ctx context.Context, chatID int64, page int) error {
	tournaments, err := b.svc.Tournaments.List(ctx, nil)
	if err != nil {
		return err
	}
	start := (page - 1) * perPage
	if start >= len(tournaments) {
		start = 0
		page = 1
	}
	end := start + perPage
	if end > len(tournaments) {
		end = len(tournaments)
	}
	var builder strings.Builder
	builder.WriteString("*Турниры*\n")
	if len(tournaments) == 0 {
		builder.WriteString("Нет турниров. Нажмите кнопку, чтобы создать.\n")
	} else {
		for _, t := range tournaments[start:end] {
			builder.WriteString(fmt.Sprintf("- %s (%s)\n", escape(t.Name), t.Status))
		}
	}
	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, t := range tournaments[start:end] {
		data := fmt.Sprintf("open_tournament|id=%d|page=%d", t.ID, page)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Открыть %s", t.Name), data),
		})
	}
	if len(tournaments) > perPage {
		row := []tgbotapi.InlineKeyboardButton{}
		if page > 1 {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", fmt.Sprintf("tournaments_page|page=%d", page-1)))
		}
		if end < len(tournaments) {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData("Вперёд ➡", fmt.Sprintf("tournaments_page|page=%d", page+1)))
		}
		if len(row) > 0 {
			keyboard = append(keyboard, row)
		}
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("➕ Создать турнир", "tournaments_start_create"),
	})

	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) showTournament(ctx context.Context, chatID int64, messageID int, id int64) error {
	t, err := b.svc.Tournaments.Get(ctx, id)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("*%s*\n", escape(t.Name)))
	if t.Type != nil {
		builder.WriteString(fmt.Sprintf("_%s_\n", escape(*t.Type)))
	}
	builder.WriteString(fmt.Sprintf("Статус: %s\n", t.Status))
	if t.StartDate != nil {
		builder.WriteString(fmt.Sprintf("Старт: %s\n", t.StartDate.Format("02.01.2006")))
	}
	if t.EndDate != nil {
		builder.WriteString(fmt.Sprintf("Финиш: %s\n", t.EndDate.Format("02.01.2006")))
	}
	if t.Note != nil && *t.Note != "" {
		builder.WriteString(fmt.Sprintf("Заметка: %s\n", escape(*t.Note)))
	}
	teams, err := b.svc.Rosters.ListTeamsInTournament(ctx, t.ID)
	if err == nil && len(teams) > 0 {
		builder.WriteString("\n*Команды в турнире:*\n")
		for _, tm := range teams {
			builder.WriteString(fmt.Sprintf("- %s\n", escape(tm.TeamName)))
		}
		upcoming := b.collectUpcomingMatches(ctx, t.ID, teams)
		if len(upcoming) > 0 {
			builder.WriteString("\n*Ближайшие матчи:*\n")
			for _, info := range upcoming {
				builder.WriteString(fmt.Sprintf("- %s • %s vs %s (%s)\n",
					info.Match.StartTime.In(b.loc).Format("02.01 15:04"),
					escape(info.TeamName),
					escape(info.Match.OpponentName),
					statusLabel(info.Match.Status)))
			}
		}
	}
	msg := tgbotapi.NewEditMessageText(chatID, messageID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			{tgbotapi.NewInlineKeyboardButtonData("✏ Редактировать", fmt.Sprintf("tournament_edit|id=%d", t.ID))},
			{
				tgbotapi.NewInlineKeyboardButtonData("👥 Заявки", fmt.Sprintf("roster_open_tournament|id=%d", t.ID)),
				tgbotapi.NewInlineKeyboardButtonData("🏟 Матчи", fmt.Sprintf("games_open_tournament|id=%d", t.ID)),
			},
			{tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "nav_back")},
		},
	}
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendTeams(ctx context.Context, chatID int64) error {
	teams, err := b.svc.Teams.ListActive(ctx)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString("*Команды*\n")
	if len(teams) == 0 {
		builder.WriteString("Активных команд пока нет.\n")
	}
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(teams)+1)
	for _, team := range teams {
		label := fmt.Sprintf("%s (%s)", escape(team.Name), escape(team.ShortCode))
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("team_open|id=%d", team.ID)),
		})
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("➕ Создать команду", "teams_start_create"),
	})
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) showTeam(ctx context.Context, chatID int64, teamID int64) error {
	team, err := b.svc.Teams.Get(ctx, teamID)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("*%s*\n", escape(team.Name)))
	builder.WriteString(fmt.Sprintf("Код: `%s`\n", escape(team.ShortCode)))
	statusText := "Неактивна"
	if team.Active {
		statusText = "Активна"
	}
	builder.WriteString(fmt.Sprintf("Статус: %s\n", statusText))
	if team.Note != nil && *team.Note != "" {
		builder.WriteString(fmt.Sprintf("Заметка: %s\n", escape(*team.Note)))
	}
	if tournaments, err := b.svc.Tournaments.List(ctx, nil); err == nil && len(tournaments) > 0 {
		var participation []string
		for _, t := range tournaments {
			roster, err := b.svc.Rosters.ListRoster(ctx, t.ID, team.ID)
			if err != nil || len(roster) == 0 {
				continue
			}
			participation = append(participation, fmt.Sprintf("- %s (%d игроков)", escape(t.Name), len(roster)))
		}
		if len(participation) > 0 {
			builder.WriteString("\n*Участие в турнирах:*\n")
			for _, line := range participation {
				builder.WriteString(line + "\n")
			}
		}
		if upcoming := b.collectTeamUpcomingMatches(ctx, tournaments, team.ID); len(upcoming) > 0 {
			builder.WriteString("\n*Ближайшие матчи:*\n")
			for _, info := range upcoming {
				builder.WriteString(fmt.Sprintf("- %s • %s vs %s (%s)\n",
					info.Match.StartTime.In(b.loc).Format("02.01 15:04"),
					escape(info.TournamentName),
					escape(info.Match.OpponentName),
					statusLabel(info.Match.Status)))
			}
		}
	}
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("✏ Редактировать", fmt.Sprintf("team_edit|id=%d", team.ID)),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "nav_back"),
		},
	)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendPlayersPage(ctx context.Context, chatID int64, page int) error {
	items, hasNext, err := b.svc.Players.List(ctx, page, perPage)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("*Игроки — страница %d*\n", page))
	if len(items) == 0 {
		builder.WriteString("Пока пусто.")
	}
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(items)+2)
	for _, p := range items {
		line := fmt.Sprintf("- %s", escape(p.FullName))
		if p.Position != nil {
			line += fmt.Sprintf(" (%s)", escape(*p.Position))
		}
		builder.WriteString(line + "\n")
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Открыть %s", truncateLabel(p.FullName, 25)), fmt.Sprintf("player_open|id=%d|page=%d", p.ID, page)),
		})
	}
	row := []tgbotapi.InlineKeyboardButton{}
	if page > 1 {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", fmt.Sprintf("players_page|page=%d", page-1)))
	}
	if hasNext {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("Вперёд ➡", fmt.Sprintf("players_page|page=%d", page+1)))
	}
	markup := tgbotapi.InlineKeyboardMarkup{}
	if len(row) > 0 {
		markup.InlineKeyboard = append(markup.InlineKeyboard, row)
	}
	markup.InlineKeyboard = append(markup.InlineKeyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("➕ Создать игрока", "players_start_create"),
	})
	markup.InlineKeyboard = append(markup.InlineKeyboard, keyboard...)
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = markup
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) showPlayer(ctx context.Context, chatID int64, playerID int64, page int) error {
	player, err := b.svc.Players.Get(ctx, playerID)
	if err != nil {
		return err
	}
	assignments, err := b.svc.Players.ListAssignments(ctx, playerID)
	if err != nil {
		assignments = nil
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("*%s*\n", escape(player.FullName)))
	if player.BirthDate != nil {
		builder.WriteString(fmt.Sprintf("Дата рождения: %s\n", player.BirthDate.Format("02.01.2006")))
	}
	if player.Position != nil && *player.Position != "" {
		builder.WriteString(fmt.Sprintf("Позиция: %s\n", escape(*player.Position)))
	}
	statusText := "Неактивен"
	if player.Active {
		statusText = "Активен"
	}
	builder.WriteString(fmt.Sprintf("Статус: %s\n", statusText))
	if player.Note != nil && *player.Note != "" {
		builder.WriteString(fmt.Sprintf("Заметка: %s\n", escape(*player.Note)))
	}
	if len(assignments) > 0 {
		builder.WriteString("\n*Заявки:*\n")
		for _, a := range assignments {
			title := escape(a.PlayerName)
			line := fmt.Sprintf("- Турнир #%d, Команда %s", a.TournamentID, title)
			if a.TournamentNumber != nil {
				line += fmt.Sprintf(", № %d", *a.TournamentNumber)
			}
			builder.WriteString(line + "\n")
		}
	}
	if page < 1 {
		page = 1
	}
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("✏ Редактировать", fmt.Sprintf("player_edit|id=%d|page=%d", player.ID, page)),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "nav_back"),
		},
	)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendRosterTournaments(ctx context.Context, chatID int64) error {
	tournaments, err := b.svc.Tournaments.List(ctx, nil)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString("*Заявки — выберите турнир*\n")
	if len(tournaments) == 0 {
		builder.WriteString("Турниров пока нет.")
	}
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(tournaments))
	for _, t := range tournaments {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s (%s)", escape(t.Name), t.Status),
				fmt.Sprintf("roster_open_tournament|id=%d", t.ID)),
		})
	}
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	if len(keyboard) > 0 {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	}
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendRosterTeams(ctx context.Context, chatID int64, tournamentID int64) error {
	teams, err := b.svc.Rosters.ListTeamsInTournament(ctx, tournamentID)
	if err != nil {
		return err
	}

	teamMap := make(map[int64]models.TournamentTeam, len(teams))
	for _, team := range teams {
		teamMap[team.TeamID] = team
	}

	var available []models.Team
	if b.svc.Teams != nil {
		if allTeams, err := b.svc.Teams.ListActive(ctx); err == nil {
			for _, team := range allTeams {
				if _, exists := teamMap[team.ID]; !exists {
					available = append(available, team)
				}
			}
			sort.Slice(available, func(i, j int) bool {
				return strings.ToLower(available[i].Name) < strings.ToLower(available[j].Name)
			})
		}
	}

	var builder strings.Builder
	builder.WriteString("*Заявка — выберите команду*\n")
	if len(teams) == 0 {
		builder.WriteString("В этом турнире пока нет заявленных команд.\n")
	}
	if len(available) > 0 {
		builder.WriteString("\n*Команды без заявки:*\n")
		for _, team := range available {
			builder.WriteString(fmt.Sprintf("- %s\n", escape(team.Name)))
		}
		builder.WriteString("Нажмите, чтобы начать заполнять заявку.\n")
	}
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(teams)+len(available)+1)
	for _, team := range teams {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s (`%s`)", escape(team.TeamName), escape(team.ShortCode)),
				fmt.Sprintf("roster_open_team|t=%d|team=%d", tournamentID, team.TeamID)),
		})
	}
	for _, team := range available {
		label := fmt.Sprintf("➕ %s", escape(team.Name))
		if team.ShortCode != "" {
			label = fmt.Sprintf("➕ %s (`%s`)", escape(team.Name), escape(team.ShortCode))
		}
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				label,
				fmt.Sprintf("roster_open_team|t=%d|team=%d", tournamentID, team.ID)),
		})
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "tournament_rosters"),
	})
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) showRoster(ctx context.Context, chatID int64, tournamentID, teamID int64) error {
	entries, err := b.svc.Rosters.ListRoster(ctx, tournamentID, teamID)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString("*Состав заявки*\n")
	if len(entries) == 0 {
		builder.WriteString("Игроков пока нет.\n")
	}
	for _, entry := range entries {
		if entry.TournamentNumber != nil {
			builder.WriteString(fmt.Sprintf("- #%d %s\n", *entry.TournamentNumber, escape(entry.PlayerName)))
		} else {
			builder.WriteString(fmt.Sprintf("- %s\n", escape(entry.PlayerName)))
		}
	}
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(entries)+2)
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("➕ Добавить игрока", fmt.Sprintf("roster_add_player|t=%d|team=%d|page=1", tournamentID, teamID)),
	})
	for _, entry := range entries {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("✏ Номер", fmt.Sprintf("roster_change_number|t=%d|team=%d|player=%d", tournamentID, teamID, entry.PlayerID)),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", fmt.Sprintf("roster_remove_player|t=%d|team=%d|player=%d", tournamentID, teamID, entry.PlayerID)),
		})
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "nav_back"),
	})
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendRosterAddPlayerList(ctx context.Context, chatID int64, tournamentID, teamID int64, page int) error {
	current, err := b.svc.Rosters.ListRoster(ctx, tournamentID, teamID)
	if err != nil {
		return err
	}
	inRoster := make(map[int64]struct{}, len(current))
	for _, entry := range current {
		inRoster[entry.PlayerID] = struct{}{}
	}
	players, hasNext, err := b.svc.Players.List(ctx, page, perPage)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString("*Выберите игрока*\n")
	keyboard := [][]tgbotapi.InlineKeyboardButton{}
	showPlayers := players
	for _, player := range showPlayers {
		if _, exists := inRoster[player.ID]; exists {
			builder.WriteString(fmt.Sprintf("✅ %s\n", escape(player.FullName)))
			continue
		}
		builder.WriteString(fmt.Sprintf("- %s\n", escape(player.FullName)))
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("➕ %s", truncateLabel(player.FullName, 25)),
				fmt.Sprintf("roster_add_pick|t=%d|team=%d|player=%d", tournamentID, teamID, player.ID)),
		})
	}
	pagination := []tgbotapi.InlineKeyboardButton{}
	if page > 1 {
		pagination = append(pagination, tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", fmt.Sprintf("roster_add_player|t=%d|team=%d|page=%d", tournamentID, teamID, page-1)))
	}
	if hasNext {
		pagination = append(pagination, tgbotapi.NewInlineKeyboardButtonData("Вперёд ➡", fmt.Sprintf("roster_add_player|t=%d|team=%d|page=%d", tournamentID, teamID, page+1)))
	}
	if len(pagination) > 0 {
		keyboard = append(keyboard, pagination)
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ К заявке", fmt.Sprintf("roster_open_team|t=%d|team=%d", tournamentID, teamID)),
	})
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendGamesTournaments(ctx context.Context, chatID int64) error {
	tournaments, err := b.svc.Tournaments.List(ctx, nil)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString("*Матчи — выберите турнир*\n")
	if len(tournaments) == 0 {
		builder.WriteString("Пока нет турниров.")
	}
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(tournaments))
	for _, t := range tournaments {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s (%s)", escape(t.Name), t.Status),
				fmt.Sprintf("games_open_tournament|id=%d", t.ID)),
		})
	}
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	if len(keyboard) > 0 {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	}
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendGamesTeams(ctx context.Context, chatID int64, tournamentID int64) error {
	teams, err := b.svc.Rosters.ListTeamsInTournament(ctx, tournamentID)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString("*Матчи — выберите команду*\n")
	if len(teams) == 0 {
		builder.WriteString("Нет команд с заявкой в этом турнире.")
	}
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(teams)+1)
	for _, team := range teams {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				team.TeamName,
				fmt.Sprintf("games_open_team|t=%d|team=%d", tournamentID, team.TeamID)),
		})
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "games"),
	})
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendGamesMatches(ctx context.Context, chatID int64, tournamentID, teamID int64) error {
	matches, err := b.svc.Matches.List(ctx, tournamentID, teamID)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString("*Матчи команды*\n")
	if len(matches) == 0 {
		builder.WriteString("Матчей пока нет.\n")
	}
	for _, m := range matches {
		builder.WriteString(fmt.Sprintf("- %s — %s (%s)\n",
			m.StartTime.In(b.loc).Format("02.01 15:04"),
			escape(m.OpponentName),
			m.Status))
	}
	keyboard := [][]tgbotapi.InlineKeyboardButton{}
	for _, m := range matches {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Открыть матч", fmt.Sprintf("open_match|id=%d", m.ID)),
		})
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("➕ Создать матч", fmt.Sprintf("match_start_create|t=%d|team=%d", tournamentID, teamID)),
	})
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "nav_back"),
	})
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) showMatch(ctx context.Context, chatID int64, matchID int64) error {
	match, err := b.svc.Matches.Get(ctx, matchID)
	if err != nil {
		return err
	}
	return b.renderMatch(ctx, chatID, match)
}

func (b *Bot) renderMatch(ctx context.Context, chatID int64, match *models.Match) error {
	matchID := match.ID
	lineup, err := b.svc.Lineup.Get(ctx, matchID)
	if err != nil {
		return err
	}
	events, err := b.svc.Events.List(ctx, matchID)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString("*Матч*\n")
	builder.WriteString(fmt.Sprintf("%s vs %s\n", match.StartTime.In(b.loc).Format("02.01.2006 15:04"), escape(match.OpponentName)))
	if match.Location != nil && *match.Location != "" {
		builder.WriteString(fmt.Sprintf("Место: %s\n", escape(*match.Location)))
	}
	builder.WriteString(fmt.Sprintf("Статус: %s\n", match.Status))
	if match.ScoreHT != nil {
		builder.WriteString(fmt.Sprintf("HT: %s\n", *match.ScoreHT))
	}
	if match.ScoreFT != nil {
		builder.WriteString(fmt.Sprintf("FT: %s\n", *match.ScoreFT))
	}
	if match.ScoreET != nil {
		builder.WriteString(fmt.Sprintf("ET: %s\n", *match.ScoreET))
	}
	if match.ScorePEN != nil {
		builder.WriteString(fmt.Sprintf("PEN: %s\n", *match.ScorePEN))
	}
	if match.ScoreFinalUs != nil || match.ScoreFinalThem != nil {
		builder.WriteString(fmt.Sprintf("Итог: %d:%d\n", safeInt(match.ScoreFinalUs), safeInt(match.ScoreFinalThem)))
	}
	builder.WriteString("\n*Состав*\n")
	if len(lineup) == 0 {
		builder.WriteString("Пока пусто.\n")
	} else {
		for _, l := range lineup {
			role := string(l.Role)
			if l.NumberOverride != nil {
				builder.WriteString(fmt.Sprintf("- %s #%d (%s)\n", escape(l.PlayerName), *l.NumberOverride, role))
			} else if l.RosterNumber != nil {
				builder.WriteString(fmt.Sprintf("- %s #%d (%s)\n", escape(l.PlayerName), *l.RosterNumber, role))
			} else {
				builder.WriteString(fmt.Sprintf("- %s (%s)\n", escape(l.PlayerName), role))
			}
		}
	}
	builder.WriteString("\n*События*\n")
	if len(events) == 0 {
		builder.WriteString("Пока нет событий.\n")
	} else {
		for _, e := range events {
			line := fmt.Sprintf("%s — %s", e.EventType, escape(e.EventTimeText))
			if e.PlayerMain != nil {
				line += fmt.Sprintf(" %s", escape(*e.PlayerMain))
			}
			if e.EventType == models.MatchEventSub && e.PlayerAlt != nil {
				line += fmt.Sprintf(" ↔ %s", escape(*e.PlayerAlt))
			}
			if e.EventType == models.MatchEventCard && e.CardType != nil {
				line += fmt.Sprintf(" (%s)", *e.CardType)
			}
			builder.WriteString("- " + line + "\n")
		}
	}
	statusRow := []tgbotapi.InlineKeyboardButton{
		matchStatusButton(matchID, match.Status, models.MatchStatusScheduled),
		matchStatusButton(matchID, match.Status, models.MatchStatusPlayed),
		matchStatusButton(matchID, match.Status, models.MatchStatusCanceled),
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("✏ Редактировать", fmt.Sprintf("match_edit|id=%d", matchID)),
		},
		statusRow,
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("👥 Состав", fmt.Sprintf("match_lineup_menu|match=%d", matchID)),
			tgbotapi.NewInlineKeyboardButtonData("⚽ События", fmt.Sprintf("match_events_menu|match=%d", matchID)),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("🔁 Сбросить счёт", fmt.Sprintf("match_scores_reset|id=%d", matchID)),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "nav_back"),
		},
	)
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendLineupMenu(ctx context.Context, chatID int64, matchID int64) error {
	lineup, err := b.svc.Lineup.Get(ctx, matchID)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString("*Состав матча*\n")
	if len(lineup) == 0 {
		builder.WriteString("Пока пусто.\n")
	} else {
		for _, l := range lineup {
			role := string(l.Role)
			if l.NumberOverride != nil {
				builder.WriteString(fmt.Sprintf("- %s #%d (%s)\n", escape(l.PlayerName), *l.NumberOverride, role))
			} else if l.RosterNumber != nil {
				builder.WriteString(fmt.Sprintf("- %s #%d (%s)\n", escape(l.PlayerName), *l.RosterNumber, role))
			} else {
				builder.WriteString(fmt.Sprintf("- %s (%s)\n", escape(l.PlayerName), role))
			}
		}
	}
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(lineup)+2)
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("➕ Добавить из заявки", fmt.Sprintf("match_lineup_add|match=%d|page=1", matchID)),
	})
	for _, l := range lineup {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("↕ Роль", fmt.Sprintf("match_lineup_role_toggle|match=%d|player=%d", matchID, l.PlayerID)),
			tgbotapi.NewInlineKeyboardButtonData("№", fmt.Sprintf("match_lineup_number|match=%d|player=%d", matchID, l.PlayerID)),
			tgbotapi.NewInlineKeyboardButtonData("🗑", fmt.Sprintf("match_lineup_remove|match=%d|player=%d", matchID, l.PlayerID)),
		})
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ К матчу", fmt.Sprintf("open_match|id=%d", matchID)),
	})
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendLineupAddList(ctx context.Context, chatID int64, matchID int64, page int) error {
	match, err := b.svc.Matches.Get(ctx, matchID)
	if err != nil {
		return err
	}
	roster, err := b.svc.Rosters.ListRoster(ctx, match.TournamentID, match.TeamID)
	if err != nil {
		return err
	}
	lineup, err := b.svc.Lineup.Get(ctx, matchID)
	if err != nil {
		return err
	}
	inLineup := make(map[int64]struct{}, len(lineup))
	for _, l := range lineup {
		inLineup[l.PlayerID] = struct{}{}
	}
	available := make([]models.TournamentRosterEntry, 0, len(roster))
	for _, entry := range roster {
		if _, exists := inLineup[entry.PlayerID]; !exists {
			available = append(available, entry)
		}
	}
	if len(available) == 0 {
		b.sendSimple(chatID, "Все игроки заявки уже в составе.")
		return b.sendLineupMenu(ctx, chatID, matchID)
	}
	total := len(available)
	if total == 0 {
		b.sendSimple(chatID, "Все игроки заявки уже в составе.")
		return b.sendLineupMenu(ctx, chatID, matchID)
	}
	if page < 1 {
		page = 1
	}
	start := (page - 1) * perPage
	if start >= total {
		page = 1
		start = 0
	}
	end := start + perPage
	if end > total {
		end = total
	}
	pageItems := available[start:end]
	var builder strings.Builder
	builder.WriteString("*Добавить в состав*\n")
	for _, entry := range pageItems {
		title := entry.PlayerName
		if entry.TournamentNumber != nil {
			title = fmt.Sprintf("#%d %s", *entry.TournamentNumber, entry.PlayerName)
		}
		builder.WriteString(fmt.Sprintf("- %s\n", escape(title)))
	}
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(pageItems)+2)
	for _, entry := range pageItems {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("➕ %s", truncateLabel(entry.PlayerName, 25)),
				fmt.Sprintf("match_lineup_add_pick|match=%d|player=%d", matchID, entry.PlayerID)),
		})
	}
	pagination := []tgbotapi.InlineKeyboardButton{}
	if page > 1 {
		pagination = append(pagination, tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", fmt.Sprintf("match_lineup_add|match=%d|page=%d", matchID, page-1)))
	}
	if end < total {
		pagination = append(pagination, tgbotapi.NewInlineKeyboardButtonData("Вперёд ➡", fmt.Sprintf("match_lineup_add|match=%d|page=%d", matchID, page+1)))
	}
	if len(pagination) > 0 {
		keyboard = append(keyboard, pagination)
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ К составу", fmt.Sprintf("match_lineup_menu|match=%d", matchID)),
	})
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) addPlayerToLineup(ctx context.Context, chatID int64, matchID, playerID int64) error {
	if err := b.svc.Lineup.Upsert(ctx, matchID, playerID, models.LineupRoleStart, nil, nil); err != nil {
		b.sendSimple(chatID, fmt.Sprintf("Не удалось добавить игрока: %v", err))
		return nil
	}
	b.sendSimple(chatID, "Игрок добавлен в состав.")
	return b.sendLineupMenu(ctx, chatID, matchID)
}

func (b *Bot) removePlayerFromLineup(ctx context.Context, chatID int64, matchID, playerID int64) error {
	if err := b.svc.Lineup.Remove(ctx, matchID, playerID); err != nil {
		b.sendSimple(chatID, fmt.Sprintf("Не удалось удалить игрока: %v", err))
		return nil
	}
	b.sendSimple(chatID, "Игрок удалён из состава.")
	return b.sendLineupMenu(ctx, chatID, matchID)
}

func (b *Bot) toggleLineupRole(ctx context.Context, chatID int64, matchID, playerID int64) error {
	lineup, err := b.svc.Lineup.Get(ctx, matchID)
	if err != nil {
		return err
	}
	var current *models.MatchLineup
	for _, l := range lineup {
		if l.PlayerID == playerID {
			current = &l
			break
		}
	}
	if current == nil {
		b.sendSimple(chatID, "Игрок не найден в составе.")
		return nil
	}
	newRole := models.LineupRoleStart
	if current.Role == models.LineupRoleStart {
		newRole = models.LineupRoleSub
	}
	if err := b.svc.Lineup.Update(ctx, matchID, playerID, models.LineupPatch{Role: &newRole}); err != nil {
		b.sendSimple(chatID, fmt.Sprintf("Не удалось изменить роль: %v", err))
		return nil
	}
	b.sendSimple(chatID, "Роль обновлена.")
	return b.sendLineupMenu(ctx, chatID, matchID)
}

func (b *Bot) startLineupNumberWizard(ctx context.Context, chatID, adminID int64, matchID, playerID int64) error {
	state := &wizardState{
		Flow: flowLineupNumber,
		Step: 0,
		Data: map[string]string{
			"match_id":  strconv.FormatInt(matchID, 10),
			"player_id": strconv.FormatInt(playerID, 10),
		},
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, "Введите номер игрока на матч (или '-' чтобы очистить).")
	return nil
}

func (b *Bot) sendEventsMenu(ctx context.Context, chatID int64, matchID int64) error {
	events, err := b.svc.Events.List(ctx, matchID)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString("*События матча*\n")
	if len(events) == 0 {
		builder.WriteString("Пока нет событий.\n")
	} else {
		for _, e := range events {
			line := fmt.Sprintf("%s — %s", e.EventType, escape(e.EventTimeText))
			if e.PlayerMain != nil {
				line += fmt.Sprintf(" %s", escape(*e.PlayerMain))
			}
			if e.EventType == models.MatchEventSub && e.PlayerAlt != nil {
				line += fmt.Sprintf(" ↔ %s", escape(*e.PlayerAlt))
			}
			if e.EventType == models.MatchEventCard && e.CardType != nil {
				line += fmt.Sprintf(" (%s)", *e.CardType)
			}
			builder.WriteString("- " + line + "\n")
		}
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⚽ Гол", fmt.Sprintf("match_events_add_goal|match=%d", matchID)),
			tgbotapi.NewInlineKeyboardButtonData("🟥 Карточка", fmt.Sprintf("match_events_add_card|match=%d", matchID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Замена", fmt.Sprintf("match_events_add_sub|match=%d", matchID)),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅ К матчу", fmt.Sprintf("open_match|id=%d", matchID)),
		},
	)
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendEventsGoalPlayerList(ctx context.Context, chatID int64, matchID int64) error {
	lineup, err := b.svc.Lineup.Get(ctx, matchID)
	if err != nil {
		return err
	}
	if len(lineup) == 0 {
		b.sendSimple(chatID, "Добавьте игроков в состав перед фиксацией событий.")
		return nil
	}
	var builder strings.Builder
	builder.WriteString("Выберите автора гола:")
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(lineup)+1)
	for _, l := range lineup {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				escape(truncateLabel(l.PlayerName, 25)),
				fmt.Sprintf("match_events_goal_pick|match=%d|player=%d", matchID, l.PlayerID)),
		})
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", fmt.Sprintf("match_events_menu|match=%d", matchID)),
	})
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendEventsCardPlayerList(ctx context.Context, chatID int64, matchID int64) error {
	lineup, err := b.svc.Lineup.Get(ctx, matchID)
	if err != nil {
		return err
	}
	if len(lineup) == 0 {
		b.sendSimple(chatID, "Добавьте игроков в состав перед фиксацией событий.")
		return nil
	}
	var builder strings.Builder
	builder.WriteString("Выберите игрока для карточки:")
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(lineup)+1)
	for _, l := range lineup {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				escape(truncateLabel(l.PlayerName, 25)),
				fmt.Sprintf("match_events_card_pick|match=%d|player=%d", matchID, l.PlayerID)),
		})
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", fmt.Sprintf("match_events_menu|match=%d", matchID)),
	})
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendEventsCardTypeMenu(ctx context.Context, chatID int64, matchID, playerID int64) error {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("🟨 Жёлтая", fmt.Sprintf("match_events_card_type|match=%d|player=%d|type=yellow", matchID, playerID)),
			tgbotapi.NewInlineKeyboardButtonData("🟥 Красная", fmt.Sprintf("match_events_card_type|match=%d|player=%d|type=red", matchID, playerID)),
		},
	)
	msg := tgbotapi.NewMessage(chatID, "Выберите тип карточки:")
	msg.ReplyMarkup = keyboard
	_, err := b.api.Send(msg)
	return err
}

func (b *Bot) sendEventSubOutList(ctx context.Context, chatID int64, matchID int64) error {
	lineup, err := b.svc.Lineup.Get(ctx, matchID)
	if err != nil {
		return err
	}
	if len(lineup) == 0 {
		b.sendSimple(chatID, "Нет игроков в составе для замены.")
		return nil
	}
	var builder strings.Builder
	builder.WriteString("Выберите игрока, который уходит:")
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(lineup)+1)
	for _, l := range lineup {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				escape(truncateLabel(l.PlayerName, 25)),
				fmt.Sprintf("match_events_sub_pick_out|match=%d|player=%d", matchID, l.PlayerID)),
		})
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", fmt.Sprintf("match_events_menu|match=%d", matchID)),
	})
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) sendEventSubInList(ctx context.Context, chatID int64, matchID, outPlayerID int64) error {
	match, err := b.svc.Matches.Get(ctx, matchID)
	if err != nil {
		return err
	}
	roster, err := b.svc.Rosters.ListRoster(ctx, match.TournamentID, match.TeamID)
	if err != nil {
		return err
	}
	var builder strings.Builder
	builder.WriteString("Выберите игрока, который выходит на поле:")
	keyboard := make([][]tgbotapi.InlineKeyboardButton, 0, len(roster)+1)
	for _, entry := range roster {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				escape(truncateLabel(entry.PlayerName, 25)),
				fmt.Sprintf("match_events_sub_pick_in|match=%d|out=%d|player=%d", matchID, outPlayerID, entry.PlayerID)),
		})
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", fmt.Sprintf("match_events_menu|match=%d", matchID)),
	})
	msg := tgbotapi.NewMessage(chatID, builder.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = b.api.Send(msg)
	return err
}

// ----------------------------------------------------------------------------
// Wizards (minimal)

type wizardState struct {
	Flow string            `json:"flow"`
	Step int               `json:"step"`
	Data map[string]string `json:"data"`
}

func (b *Bot) startTournamentWizard(ctx context.Context, chatID, adminID int64) error {
	state := &wizardState{
		Flow: flowCreateTournament,
		Step: 0,
		Data: make(map[string]string),
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, "Создание турнира: введите название.")
	return nil
}

func (b *Bot) advanceWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	switch state.Flow {
	case flowCreateTournament:
		return b.advanceTournamentWizard(ctx, msg, state)
	case flowEditTournament:
		return b.advanceTournamentEditWizard(ctx, msg, state)
	case flowCreateTeam:
		return b.advanceTeamWizard(ctx, msg, state)
	case flowEditTeam:
		return b.advanceTeamEditWizard(ctx, msg, state)
	case flowCreatePlayer:
		return b.advancePlayerWizard(ctx, msg, state)
	case flowEditPlayer:
		return b.advancePlayerEditWizard(ctx, msg, state)
	case flowRosterAddPlayer:
		return b.advanceRosterAddWizard(ctx, msg, state)
	case flowRosterChangeNumber:
		return b.advanceRosterChangeWizard(ctx, msg, state)
	case flowMatchCreate:
		return b.advanceMatchCreateWizard(ctx, msg, state)
	case flowMatchEdit:
		return b.advanceMatchEditWizard(ctx, msg, state)
	case flowLineupNumber:
		return b.advanceLineupNumberWizard(ctx, msg, state)
	case flowEventGoal:
		return b.advanceEventGoalWizard(ctx, msg, state)
	case flowEventCard:
		return b.advanceEventCardWizard(ctx, msg, state)
	case flowEventSub:
		return b.advanceEventSubWizard(ctx, msg, state)
	default:
		return nil
	}
}

func (b *Bot) advanceTournamentWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	text := strings.TrimSpace(msg.Text)
	adminID := msg.From.ID
	chatID := msg.Chat.ID

	switch state.Step {
	case 0:
		state.Data["name"] = text
		state.Step++
		b.sendSimple(chatID, "Введите тип турнира (или '-' для пропуска).")
	case 1:
		if text != "-" && text != "" {
			state.Data["type"] = text
		}
		state.Step++
		b.sendSimple(chatID, "Укажите статус (planned/active/finished).")
	case 2:
		if text == "" {
			text = "planned"
		}
		switch models.TournamentStatus(text) {
		case models.TournamentStatusPlanned, models.TournamentStatusActive, models.TournamentStatusFinished:
			state.Data["status"] = text
		default:
			b.sendSimple(chatID, "Неверный статус. Попробуйте ещё раз.")
			return nil
		}
		state.Step++
		b.sendSimple(chatID, "Введите дату начала в формате YYYY-MM-DD или '-' для пропуска.")
	case 3:
		if text != "-" && text != "" {
			if _, err := time.Parse("2006-01-02", text); err != nil {
				b.sendSimple(chatID, "Неверный формат даты. Укажите YYYY-MM-DD.")
				return nil
			}
			state.Data["start_date"] = text
		}
		state.Step++
		b.sendSimple(chatID, "Введите дату окончания в формате YYYY-MM-DD или '-' для пропуска.")
	case 4:
		if text != "-" && text != "" {
			if _, err := time.Parse("2006-01-02", text); err != nil {
				b.sendSimple(chatID, "Неверный формат даты. Укажите YYYY-MM-DD.")
				return nil
			}
			state.Data["end_date"] = text
		}
		state.Step++
		b.sendSimple(chatID, "Введите примечание (или '-' для пропуска).")
	case 5:
		if text != "-" && text != "" {
			state.Data["note"] = text
		}
		if err := b.finishTournamentWizard(ctx, state); err != nil {
			return err
		}
		b.sendSimple(chatID, "Турнир создан.")
		return b.svc.Sessions.Clear(ctx, adminID)
	}

	return b.saveSession(ctx, adminID, &state.Flow, state)
}

func (b *Bot) finishTournamentWizard(ctx context.Context, state *wizardState) error {
	name := state.Data["name"]
	if name == "" {
		return errors.New("empty name")
	}
	var typ *string
	if val := state.Data["type"]; val != "" {
		typ = &val
	}
	status := models.TournamentStatus(state.Data["status"])
	if status == "" {
		status = models.TournamentStatusPlanned
	}
	var start *time.Time
	if val := state.Data["start_date"]; val != "" {
		parsed, _ := time.ParseInLocation("2006-01-02", val, b.loc)
		start = &parsed
	}
	var end *time.Time
	if val := state.Data["end_date"]; val != "" {
		parsed, _ := time.ParseInLocation("2006-01-02", val, b.loc)
		end = &parsed
	}
	var note *string
	if val := state.Data["note"]; val != "" {
		note = &val
	}
	_, err := b.svc.Tournaments.Create(ctx, service.CreateTournamentInput{
		Name:      name,
		Type:      typ,
		Status:    status,
		StartDate: start,
		EndDate:   end,
		Note:      note,
	})
	return err
}

func (b *Bot) startTournamentEditWizard(ctx context.Context, chatID, adminID int64, tournamentID int64) error {
	tournament, err := b.svc.Tournaments.Get(ctx, tournamentID)
	if err != nil {
		return err
	}
	state := &wizardState{
		Flow: flowEditTournament,
		Step: 0,
		Data: map[string]string{
			"id":          strconv.FormatInt(tournamentID, 10),
			"orig_status": string(tournament.Status),
		},
	}
	if tournament.Type != nil {
		state.Data["orig_type"] = *tournament.Type
	}
	if tournament.StartDate != nil {
		state.Data["orig_start"] = tournament.StartDate.Format("2006-01-02")
	}
	if tournament.EndDate != nil {
		state.Data["orig_end"] = tournament.EndDate.Format("2006-01-02")
	}
	if tournament.Note != nil {
		state.Data["orig_note"] = *tournament.Note
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, fmt.Sprintf("Текущее название: %s\nВведите новое название (или '-' чтобы оставить).", escape(tournament.Name)))
	return nil
}

func (b *Bot) advanceTournamentEditWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	text := strings.TrimSpace(msg.Text)
	adminID := msg.From.ID
	chatID := msg.Chat.ID

	switch state.Step {
	case 0:
		if text != "" && text != "-" {
			state.Data["name_new"] = text
		}
		state.Step++
		current := "(пусто)"
		if val := state.Data["orig_type"]; val != "" {
			current = escape(val)
		}
		b.sendSimple(chatID, fmt.Sprintf("Текущий тип: %s\nВведите новый тип (или '-' чтобы оставить, 'удалить' чтобы очистить).", current))
	case 1:
		if text != "" && text != "-" {
			if strings.EqualFold(text, "удалить") {
				state.Data["type_action"] = "delete"
			} else {
				state.Data["type_new"] = text
			}
		}
		state.Step++
		status := state.Data["orig_status"]
		if status == "" {
			status = string(models.TournamentStatusPlanned)
		}
		b.sendSimple(chatID, fmt.Sprintf("Текущий статус: %s\nВведите новый статус (planned/active/finished) или '-' чтобы оставить.", status))
	case 2:
		if text != "" && text != "-" {
			normalized := strings.ToLower(text)
			switch models.TournamentStatus(normalized) {
			case models.TournamentStatusPlanned, models.TournamentStatusActive, models.TournamentStatusFinished:
				state.Data["status_new"] = normalized
			default:
				b.sendSimple(chatID, "Недопустимый статус. Используйте planned/active/finished или '-'.")
				return nil
			}
		}
		state.Step++
		current := state.Data["orig_start"]
		if current == "" {
			current = "(не задана)"
		}
		b.sendSimple(chatID, fmt.Sprintf("Текущая дата начала: %s\nВведите новую дату (YYYY-MM-DD), '-' чтобы оставить, 'удалить' чтобы очистить.", current))
	case 3:
		if text != "" && text != "-" {
			if strings.EqualFold(text, "удалить") {
				state.Data["start_action"] = "delete"
			} else {
				if _, err := time.Parse("2006-01-02", text); err != nil {
					b.sendSimple(chatID, "Неверный формат даты. Используйте YYYY-MM-DD, '-' или 'удалить'.")
					return nil
				}
				state.Data["start_new"] = text
			}
		}
		state.Step++
		current := state.Data["orig_end"]
		if current == "" {
			current = "(не задана)"
		}
		b.sendSimple(chatID, fmt.Sprintf("Текущая дата окончания: %s\nВведите новую дату (YYYY-MM-DD), '-' чтобы оставить, 'удалить' чтобы очистить.", current))
	case 4:
		if text != "" && text != "-" {
			if strings.EqualFold(text, "удалить") {
				state.Data["end_action"] = "delete"
			} else {
				if _, err := time.Parse("2006-01-02", text); err != nil {
					b.sendSimple(chatID, "Неверный формат даты. Используйте YYYY-MM-DD, '-' или 'удалить'.")
					return nil
				}
				state.Data["end_new"] = text
			}
		}
		state.Step++
		current := state.Data["orig_note"]
		if current == "" {
			current = "(пусто)"
		} else {
			current = escape(current)
		}
		b.sendSimple(chatID, fmt.Sprintf("Текущее примечание: %s\nВведите новое примечание, '-' чтобы оставить, 'удалить' чтобы очистить.", current))
	case 5:
		if text != "" {
			if strings.EqualFold(text, "удалить") {
				state.Data["note_action"] = "delete"
			} else if text != "-" {
				state.Data["note_new"] = text
			}
		}
		if err := b.finishTournamentEditWizard(ctx, state); err != nil {
			b.sendSimple(chatID, fmt.Sprintf("Ошибка обновления турнира: %v", err))
			return nil
		}
		b.sendSimple(chatID, "Турнир обновлён.")
		_ = b.sendTournamentList(ctx, chatID, 1)
		return b.svc.Sessions.Clear(ctx, adminID)
	}
	return b.saveSession(ctx, adminID, &state.Flow, state)
}

func (b *Bot) finishTournamentEditWizard(ctx context.Context, state *wizardState) error {
	tournamentID := parseInt64(state.Data["id"])
	patch := models.TournamentPatch{}

	if v := state.Data["name_new"]; v != "" {
		val := v
		patch.Name = &val
	}
	if action := state.Data["type_action"]; action == "delete" {
		patch.Type = models.NewOptionalString(nil)
	} else if v := state.Data["type_new"]; v != "" {
		val := v
		patch.Type = models.NewOptionalString(&val)
	}
	if v := state.Data["status_new"]; v != "" {
		status := models.TournamentStatus(v)
		patch.Status = &status
	}
	if action := state.Data["start_action"]; action == "delete" {
		patch.StartDate = models.NewOptionalTime(nil)
	} else if v := state.Data["start_new"]; v != "" {
		parsed, err := time.ParseInLocation("2006-01-02", v, b.loc)
		if err != nil {
			return err
		}
		patch.StartDate = models.NewOptionalTime(&parsed)
	}
	if action := state.Data["end_action"]; action == "delete" {
		patch.EndDate = models.NewOptionalTime(nil)
	} else if v := state.Data["end_new"]; v != "" {
		parsed, err := time.ParseInLocation("2006-01-02", v, b.loc)
		if err != nil {
			return err
		}
		patch.EndDate = models.NewOptionalTime(&parsed)
	}
	if action := state.Data["note_action"]; action == "delete" {
		patch.Note = models.NewOptionalString(nil)
	} else if v := state.Data["note_new"]; v != "" {
		val := v
		patch.Note = models.NewOptionalString(&val)
	}

	return b.svc.Tournaments.Update(ctx, tournamentID, patch)
}

func (b *Bot) startTeamWizard(ctx context.Context, chatID, adminID int64) error {
	state := &wizardState{
		Flow: flowCreateTeam,
		Step: 0,
		Data: make(map[string]string),
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, "Создание команды: введите название.")
	return nil
}

func (b *Bot) advanceTeamWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	text := strings.TrimSpace(msg.Text)
	adminID := msg.From.ID
	chatID := msg.Chat.ID

	switch state.Step {
	case 0:
		if text == "" {
			b.sendSimple(chatID, "Название не может быть пустым. Повторите ввод.")
			return nil
		}
		state.Data["name"] = text
		state.Step++
		b.sendSimple(chatID, "Введите короткий код (например, U12).")
	case 1:
		if text == "" {
			b.sendSimple(chatID, "Короткий код не может быть пустым.")
			return nil
		}
		state.Data["short_code"] = text
		state.Step++
		b.sendSimple(chatID, "Команда активна? (да/нет, по умолчанию да).")
	case 2:
		if text == "" {
			state.Data["active"] = "true"
		} else {
			val, ok := parseYesNo(text)
			if !ok {
				b.sendSimple(chatID, "Введите 'да' или 'нет'.")
				return nil
			}
			state.Data["active"] = strconv.FormatBool(val)
		}
		state.Step++
		b.sendSimple(chatID, "Введите примечание (или '-' для пропуска).")
	case 3:
		if text != "-" && text != "" {
			state.Data["note"] = text
		}
		if err := b.finishTeamWizard(ctx, state); err != nil {
			b.sendSimple(chatID, fmt.Sprintf("Не удалось создать команду: %v", err))
		} else {
			b.sendSimple(chatID, "Команда создана.")
			_ = b.sendTeams(ctx, chatID)
		}
		return b.svc.Sessions.Clear(ctx, adminID)
	}
	return b.saveSession(ctx, adminID, &state.Flow, state)
}

func (b *Bot) finishTeamWizard(ctx context.Context, state *wizardState) error {
	active := true
	if val := state.Data["active"]; val != "" {
		parsed, err := strconv.ParseBool(val)
		if err == nil {
			active = parsed
		}
	}
	var note *string
	if val := state.Data["note"]; val != "" {
		note = &val
	}
	_, err := b.svc.Teams.Create(ctx, service.CreateTeamInput{
		Name:      state.Data["name"],
		ShortCode: state.Data["short_code"],
		Active:    active,
		Note:      note,
	})
	return err
}

func (b *Bot) startTeamEditWizard(ctx context.Context, chatID, adminID int64, teamID int64) error {
	team, err := b.svc.Teams.Get(ctx, teamID)
	if err != nil {
		return err
	}
	state := &wizardState{
		Flow: flowEditTeam,
		Step: 0,
		Data: map[string]string{
			"id": strconv.FormatInt(team.ID, 10),
		},
	}
	if team.Note != nil {
		state.Data["orig_note"] = *team.Note
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, fmt.Sprintf("Текущее название: %s\nВведите новое название (или '-' чтобы оставить).", escape(team.Name)))
	return nil
}

func (b *Bot) advanceTeamEditWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	text := strings.TrimSpace(msg.Text)
	adminID := msg.From.ID
	chatID := msg.Chat.ID

	switch state.Step {
	case 0:
		if text != "" && text != "-" {
			state.Data["name_new"] = text
		}
		state.Step++
		b.sendSimple(chatID, "Введите новый короткий код (или '-' чтобы оставить).")
	case 1:
		if text != "" && text != "-" {
			state.Data["code_new"] = text
		}
		state.Step++
		b.sendSimple(chatID, "Команда активна? (да/нет, '-' чтобы оставить текущее значение).")
	case 2:
		if text != "" && text != "-" {
			val, ok := parseYesNo(text)
			if !ok {
				b.sendSimple(chatID, "Введите 'да', 'нет' или '-' чтобы оставить без изменений.")
				return nil
			}
			state.Data["active_new"] = strconv.FormatBool(val)
		}
		state.Step++
		current := state.Data["orig_note"]
		if current == "" {
			current = "(пусто)"
		} else {
			current = escape(current)
		}
		b.sendSimple(chatID, fmt.Sprintf("Текущее примечание: %s\nВведите новое примечание, '-' чтобы оставить, 'удалить' чтобы очистить.", current))
	case 3:
		if text != "" {
			if strings.EqualFold(text, "удалить") {
				state.Data["note_action"] = "delete"
			} else if text != "-" {
				state.Data["note_new"] = text
			}
		}
		if err := b.finishTeamEditWizard(ctx, state); err != nil {
			b.sendSimple(chatID, fmt.Sprintf("Не удалось обновить команду: %v", err))
			return nil
		}
		b.sendSimple(chatID, "Команда обновлена.")
		teamID := parseInt64(state.Data["id"])
		_ = b.showTeam(ctx, chatID, teamID)
		return b.svc.Sessions.Clear(ctx, adminID)
	}
	return b.saveSession(ctx, adminID, &state.Flow, state)
}

func (b *Bot) finishTeamEditWizard(ctx context.Context, state *wizardState) error {
	teamID := parseInt64(state.Data["id"])
	patch := models.TeamPatch{}
	if v := state.Data["name_new"]; v != "" {
		val := v
		patch.Name = &val
	}
	if v := state.Data["code_new"]; v != "" {
		val := v
		patch.ShortCode = &val
	}
	if v := state.Data["active_new"]; v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			value := parsed
			patch.Active = &value
		} else {
			return err
		}
	}
	if state.Data["note_action"] == "delete" {
		patch.Note = models.NewOptionalString(nil)
	} else if v := state.Data["note_new"]; v != "" {
		val := v
		patch.Note = models.NewOptionalString(&val)
	}
	return b.svc.Teams.Update(ctx, teamID, patch)
}

func (b *Bot) startPlayerWizard(ctx context.Context, chatID, adminID int64) error {
	state := &wizardState{
		Flow: flowCreatePlayer,
		Step: 0,
		Data: make(map[string]string),
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, "Создание игрока: укажите ФИО.")
	return nil
}

func (b *Bot) advancePlayerWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	text := strings.TrimSpace(msg.Text)
	adminID := msg.From.ID
	chatID := msg.Chat.ID

	switch state.Step {
	case 0:
		if text == "" {
			b.sendSimple(chatID, "ФИО не может быть пустым.")
			return nil
		}
		state.Data["full_name"] = text
		state.Step++
		b.sendSimple(chatID, "Введите дату рождения (YYYY-MM-DD) или '-' для пропуска.")
	case 1:
		if text != "-" && text != "" {
			if _, err := time.Parse("2006-01-02", text); err != nil {
				b.sendSimple(chatID, "Неверный формат даты. Используйте YYYY-MM-DD.")
				return nil
			}
			state.Data["birth_date"] = text
		}
		state.Step++
		b.sendSimple(chatID, "Введите игровую позицию (или '-' для пропуска).")
	case 2:
		if text != "-" && text != "" {
			state.Data["position"] = text
		}
		state.Step++
		b.sendSimple(chatID, "Введите примечание (или '-' для пропуска).")
	case 3:
		if text != "-" && text != "" {
			state.Data["note"] = text
		}
		if err := b.finishPlayerWizard(ctx, state); err != nil {
			b.sendSimple(chatID, fmt.Sprintf("Не удалось создать игрока: %v", err))
		} else {
			b.sendSimple(chatID, "Игрок создан.")
			_ = b.sendPlayersPage(ctx, chatID, 1)
		}
		return b.svc.Sessions.Clear(ctx, adminID)
	}
	return b.saveSession(ctx, adminID, &state.Flow, state)
}

func (b *Bot) finishPlayerWizard(ctx context.Context, state *wizardState) error {
	var birth *time.Time
	if val := state.Data["birth_date"]; val != "" {
		parsed, err := time.ParseInLocation("2006-01-02", val, b.loc)
		if err == nil {
			birth = &parsed
		}
	}
	var position *string
	if val := state.Data["position"]; val != "" {
		position = &val
	}
	var note *string
	if val := state.Data["note"]; val != "" {
		note = &val
	}
	_, err := b.svc.Players.Create(ctx, service.CreatePlayerInput{
		FullName: state.Data["full_name"],
		Birth:    birth,
		Position: position,
		Active:   true,
		Note:     note,
	})
	return err
}

func (b *Bot) startPlayerEditWizard(ctx context.Context, chatID, adminID int64, playerID int64, page int) error {
	player, err := b.svc.Players.Get(ctx, playerID)
	if err != nil {
		return err
	}
	state := &wizardState{
		Flow: flowEditPlayer,
		Step: 0,
		Data: map[string]string{
			"id":          strconv.FormatInt(playerID, 10),
			"return_page": strconv.FormatInt(int64(page), 10),
		},
	}
	if player.BirthDate != nil {
		state.Data["orig_birth"] = player.BirthDate.Format("2006-01-02")
	}
	if player.Position != nil {
		state.Data["orig_position"] = *player.Position
	}
	if player.Note != nil {
		state.Data["orig_note"] = *player.Note
	}
	state.Data["orig_active"] = strconv.FormatBool(player.Active)

	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, fmt.Sprintf("Текущее ФИО: %s\nВведите новое ФИО (или '-' чтобы оставить).", escape(player.FullName)))
	return nil
}

func (b *Bot) advancePlayerEditWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	text := strings.TrimSpace(msg.Text)
	adminID := msg.From.ID
	chatID := msg.Chat.ID

	switch state.Step {
	case 0:
		if text != "" && text != "-" {
			state.Data["name_new"] = text
		}
		state.Step++
		current := state.Data["orig_birth"]
		if current == "" {
			current = "(не задана)"
		}
		b.sendSimple(chatID, fmt.Sprintf("Текущая дата рождения: %s\nВведите новую дату (YYYY-MM-DD), '-' чтобы оставить, 'удалить' чтобы очистить.", current))
	case 1:
		if text != "" && text != "-" {
			if strings.EqualFold(text, "удалить") {
				state.Data["birth_action"] = "delete"
			} else {
				if _, err := time.Parse("2006-01-02", text); err != nil {
					b.sendSimple(chatID, "Неверный формат даты. Используйте YYYY-MM-DD, '-' или 'удалить'.")
					return nil
				}
				state.Data["birth_new"] = text
			}
		}
		state.Step++
		current := state.Data["orig_position"]
		if current == "" {
			current = "(пусто)"
		} else {
			current = escape(current)
		}
		b.sendSimple(chatID, fmt.Sprintf("Текущая позиция: %s\nВведите новую позицию, '-' чтобы оставить, 'удалить' чтобы очистить.", current))
	case 2:
		if text != "" && text != "-" {
			if strings.EqualFold(text, "удалить") {
				state.Data["position_action"] = "delete"
			} else {
				state.Data["position_new"] = text
			}
		}
		state.Step++
		currentStatus := "активен"
		if state.Data["orig_active"] == "false" {
			currentStatus = "неактивен"
		}
		b.sendSimple(chatID, fmt.Sprintf("Игрок сейчас %s.\nВведите 'да'/'нет' чтобы изменить активность или '-' чтобы оставить.", currentStatus))
	case 3:
		if text != "" && text != "-" {
			val, ok := parseYesNo(text)
			if !ok {
				b.sendSimple(chatID, "Введите 'да', 'нет' или '-' чтобы оставить без изменений.")
				return nil
			}
			state.Data["active_new"] = strconv.FormatBool(val)
		}
		state.Step++
		current := state.Data["orig_note"]
		if current == "" {
			current = "(пусто)"
		} else {
			current = escape(current)
		}
		b.sendSimple(chatID, fmt.Sprintf("Текущее примечание: %s\nВведите новое примечание, '-' чтобы оставить, 'удалить' чтобы очистить.", current))
	case 4:
		if text != "" {
			if strings.EqualFold(text, "удалить") {
				state.Data["note_action"] = "delete"
			} else if text != "-" {
				state.Data["note_new"] = text
			}
		}
		if err := b.finishPlayerEditWizard(ctx, state); err != nil {
			b.sendSimple(chatID, fmt.Sprintf("Не удалось обновить игрока: %v", err))
			return nil
		}
		b.sendSimple(chatID, "Игрок обновлён.")
		playerID := parseInt64(state.Data["id"])
		page := int(parseInt64(state.Data["return_page"]))
		if page < 1 {
			page = 1
		}
		_ = b.showPlayer(ctx, chatID, playerID, page)
		return b.svc.Sessions.Clear(ctx, adminID)
	}
	return b.saveSession(ctx, adminID, &state.Flow, state)
}

func (b *Bot) finishPlayerEditWizard(ctx context.Context, state *wizardState) error {
	playerID := parseInt64(state.Data["id"])
	patch := models.PlayerPatch{}
	if v := state.Data["name_new"]; v != "" {
		val := v
		patch.FullName = &val
	}
	if action := state.Data["birth_action"]; action == "delete" {
		patch.BirthDate = models.NewOptionalTime(nil)
	} else if v := state.Data["birth_new"]; v != "" {
		parsed, err := time.ParseInLocation("2006-01-02", v, b.loc)
		if err != nil {
			return err
		}
		patch.BirthDate = models.NewOptionalTime(&parsed)
	}
	if action := state.Data["position_action"]; action == "delete" {
		patch.Position = models.NewOptionalString(nil)
	} else if v := state.Data["position_new"]; v != "" {
		val := v
		patch.Position = models.NewOptionalString(&val)
	}
	if v := state.Data["active_new"]; v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			value := parsed
			patch.Active = &value
		} else {
			return err
		}
	}
	if action := state.Data["note_action"]; action == "delete" {
		patch.Note = models.NewOptionalString(nil)
	} else if v := state.Data["note_new"]; v != "" {
		val := v
		patch.Note = models.NewOptionalString(&val)
	}
	return b.svc.Players.Update(ctx, playerID, patch)
}

func (b *Bot) startRosterAddWizard(ctx context.Context, chatID, adminID int64, tournamentID, teamID, playerID int64) error {
	state := &wizardState{
		Flow: flowRosterAddPlayer,
		Step: 0,
		Data: map[string]string{
			"tournament_id": strconv.FormatInt(tournamentID, 10),
			"team_id":       strconv.FormatInt(teamID, 10),
			"player_id":     strconv.FormatInt(playerID, 10),
		},
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, "Введите номер игрока в турнире (или '-' для пропуска).")
	return nil
}

func (b *Bot) advanceRosterAddWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	text := strings.TrimSpace(msg.Text)
	adminID := msg.From.ID
	chatID := msg.Chat.ID

	if state.Step != 0 {
		return nil
	}
	var (
		number *int
		err    error
	)
	if text != "-" && text != "" {
		number, err = parseOptionalInt(text)
		if err != nil {
			b.sendSimple(chatID, "Номер должен быть целым числом или '-' для пропуска.")
			return nil
		}
	}
	tournamentID := parseInt64(state.Data["tournament_id"])
	teamID := parseInt64(state.Data["team_id"])
	playerID := parseInt64(state.Data["player_id"])

	if err := b.svc.Rosters.AddPlayer(ctx, tournamentID, teamID, playerID, number); err != nil {
		b.sendSimple(chatID, fmt.Sprintf("Не удалось добавить игрока: %v", err))
	} else {
		b.sendSimple(chatID, "Игрок добавлен в заявку.")
		_ = b.showRoster(ctx, chatID, tournamentID, teamID)
	}
	return b.svc.Sessions.Clear(ctx, adminID)
}

func (b *Bot) startRosterChangeWizard(ctx context.Context, chatID, adminID int64, tournamentID, teamID, playerID int64) error {
	state := &wizardState{
		Flow: flowRosterChangeNumber,
		Step: 0,
		Data: map[string]string{
			"tournament_id": strconv.FormatInt(tournamentID, 10),
			"team_id":       strconv.FormatInt(teamID, 10),
			"player_id":     strconv.FormatInt(playerID, 10),
		},
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, "Введите новый номер игрока (или '-' для удаления номера).")
	return nil
}

func (b *Bot) advanceRosterChangeWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	text := strings.TrimSpace(msg.Text)
	adminID := msg.From.ID
	chatID := msg.Chat.ID
	if state.Step != 0 {
		return nil
	}
	var (
		number *int
		err    error
	)
	if text != "-" && text != "" {
		number, err = parseOptionalInt(text)
		if err != nil {
			b.sendSimple(chatID, "Номер должен быть целым числом или '-' для удаления.")
			return nil
		}
	}
	tournamentID := parseInt64(state.Data["tournament_id"])
	teamID := parseInt64(state.Data["team_id"])
	playerID := parseInt64(state.Data["player_id"])

	if err := b.svc.Rosters.UpdateNumber(ctx, tournamentID, teamID, playerID, number); err != nil {
		b.sendSimple(chatID, fmt.Sprintf("Не удалось изменить номер: %v", err))
	} else {
		b.sendSimple(chatID, "Номер обновлён.")
		_ = b.showRoster(ctx, chatID, tournamentID, teamID)
	}
	return b.svc.Sessions.Clear(ctx, adminID)
}

func (b *Bot) startMatchCreateWizard(ctx context.Context, chatID, adminID int64, tournamentID, teamID int64) error {
	state := &wizardState{
		Flow: flowMatchCreate,
		Step: 0,
		Data: map[string]string{
			"tournament_id": strconv.FormatInt(tournamentID, 10),
			"team_id":       strconv.FormatInt(teamID, 10),
		},
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, "Создание матча: укажите соперника.")
	return nil
}

func (b *Bot) advanceMatchCreateWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	text := strings.TrimSpace(msg.Text)
	adminID := msg.From.ID
	chatID := msg.Chat.ID

	switch state.Step {
	case 0:
		if text == "" {
			b.sendSimple(chatID, "Соперник не может быть пустым.")
			return nil
		}
		state.Data["opponent"] = text
		state.Step++
		b.sendSimple(chatID, "Введите дату матча (YYYY-MM-DD).")
	case 1:
		if _, err := time.Parse("2006-01-02", text); err != nil {
			b.sendSimple(chatID, "Неверный формат. Используйте YYYY-MM-DD.")
			return nil
		}
		state.Data["date"] = text
		state.Step++
		b.sendSimple(chatID, "Введите время матча (HH:MM).")
	case 2:
		if _, err := time.Parse("15:04", text); err != nil {
			b.sendSimple(chatID, "Неверный формат времени. Используйте HH:MM (24 часа).")
			return nil
		}
		state.Data["time"] = text
		state.Step++
		b.sendSimple(chatID, "Введите место проведения (или '-' для пропуска).")
	case 3:
		if text != "-" && text != "" {
			state.Data["location"] = text
		}
		if err := b.finishMatchCreateWizard(ctx, state); err != nil {
			b.sendSimple(chatID, fmt.Sprintf("Не удалось создать матч: %v", err))
		} else {
			b.sendSimple(chatID, "Матч создан.")
			tournamentID := parseInt64(state.Data["tournament_id"])
			teamID := parseInt64(state.Data["team_id"])
			_ = b.sendGamesMatches(ctx, chatID, tournamentID, teamID)
		}
		return b.svc.Sessions.Clear(ctx, adminID)
	}
	return b.saveSession(ctx, adminID, &state.Flow, state)
}

func (b *Bot) finishMatchCreateWizard(ctx context.Context, state *wizardState) error {
	tournamentID := parseInt64(state.Data["tournament_id"])
	teamID := parseInt64(state.Data["team_id"])
	date := state.Data["date"]
	timePart := state.Data["time"]
	start, err := time.ParseInLocation("2006-01-02 15:04", fmt.Sprintf("%s %s", date, timePart), b.loc)
	if err != nil {
		return err
	}
	var location *string
	if val := state.Data["location"]; val != "" {
		location = &val
	}
	_, err = b.svc.Matches.Create(ctx, service.CreateMatchInput{
		TournamentID: tournamentID,
		TeamID:       teamID,
		Opponent:     state.Data["opponent"],
		StartTime:    start,
		Location:     location,
		Status:       models.MatchStatusScheduled,
	})
	return err
}

func (b *Bot) advanceLineupNumberWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	if state.Step != 0 {
		return nil
	}
	text := strings.TrimSpace(msg.Text)
	adminID := msg.From.ID
	chatID := msg.Chat.ID

	matchID := parseInt64(state.Data["match_id"])
	playerID := parseInt64(state.Data["player_id"])

	var patch models.LineupPatch
	switch text {
	case "-":
		patch.NumberOverride = models.NewOptionalInt(nil)
	default:
		if text == "" {
			b.sendSimple(chatID, "Введите номер или '-' для удаления.")
			return nil
		}
		num, err := strconv.Atoi(text)
		if err != nil {
			b.sendSimple(chatID, "Номер должен быть целым числом.")
			return nil
		}
		patch.NumberOverride = models.NewOptionalInt(&num)
	}

	if err := b.svc.Lineup.Update(ctx, matchID, playerID, patch); err != nil {
		b.sendSimple(chatID, fmt.Sprintf("Не удалось обновить номер: %v", err))
		return nil
	}
	b.sendSimple(chatID, "Номер в составе обновлён.")
	_ = b.sendLineupMenu(ctx, chatID, matchID)
	return b.svc.Sessions.Clear(ctx, adminID)
}

func (b *Bot) startEventGoalWizard(ctx context.Context, chatID, adminID int64, matchID, playerID int64) error {
	state := &wizardState{
		Flow: flowEventGoal,
		Step: 0,
		Data: map[string]string{
			"match_id":  strconv.FormatInt(matchID, 10),
			"player_id": strconv.FormatInt(playerID, 10),
		},
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, "Введите время гола (например, 45+2).")
	return nil
}

func (b *Bot) advanceEventGoalWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	if state.Step != 0 {
		return nil
	}
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		b.sendSimple(msg.Chat.ID, "Время события не может быть пустым.")
		return nil
	}
	matchID := parseInt64(state.Data["match_id"])
	playerID := parseInt64(state.Data["player_id"])
	if err := b.svc.Events.AddGoal(ctx, matchID, playerID, text); err != nil {
		b.sendSimple(msg.Chat.ID, fmt.Sprintf("Не удалось добавить гол: %v", err))
		return nil
	}
	b.sendSimple(msg.Chat.ID, "Гол добавлен.")
	_ = b.showMatch(ctx, msg.Chat.ID, matchID)
	return b.svc.Sessions.Clear(ctx, msg.From.ID)
}

func (b *Bot) startEventCardWizard(ctx context.Context, chatID, adminID int64, matchID, playerID int64, cardType string) error {
	cardType = strings.ToLower(cardType)
	if cardType != "yellow" && cardType != "red" {
		b.sendSimple(chatID, "Неизвестный тип карточки.")
		return nil
	}
	state := &wizardState{
		Flow: flowEventCard,
		Step: 0,
		Data: map[string]string{
			"match_id":  strconv.FormatInt(matchID, 10),
			"player_id": strconv.FormatInt(playerID, 10),
			"card_type": cardType,
		},
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, "Введите время карточки (например, 12 или 90+3).")
	return nil
}

func (b *Bot) advanceEventCardWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	if state.Step != 0 {
		return nil
	}
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		b.sendSimple(msg.Chat.ID, "Время события не может быть пустым.")
		return nil
	}
	matchID := parseInt64(state.Data["match_id"])
	playerID := parseInt64(state.Data["player_id"])
	cardType := models.CardType(state.Data["card_type"])
	if cardType != models.CardTypeYellow && cardType != models.CardTypeRed {
		b.sendSimple(msg.Chat.ID, "Неизвестный тип карточки.")
		return nil
	}
	if err := b.svc.Events.AddCard(ctx, matchID, playerID, cardType, text); err != nil {
		b.sendSimple(msg.Chat.ID, fmt.Sprintf("Не удалось добавить карточку: %v", err))
		return nil
	}
	b.sendSimple(msg.Chat.ID, "Карточка добавлена.")
	_ = b.showMatch(ctx, msg.Chat.ID, matchID)
	return b.svc.Sessions.Clear(ctx, msg.From.ID)
}

func (b *Bot) startEventSubWizard(ctx context.Context, chatID, adminID int64, matchID, outPlayerID, inPlayerID int64) error {
	if outPlayerID == inPlayerID {
		b.sendSimple(chatID, "Игроки замены должны отличаться.")
		return nil
	}
	state := &wizardState{
		Flow: flowEventSub,
		Step: 0,
		Data: map[string]string{
			"match_id": strconv.FormatInt(matchID, 10),
			"out_id":   strconv.FormatInt(outPlayerID, 10),
			"in_id":    strconv.FormatInt(inPlayerID, 10),
		},
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, "Введите время замены (например, 60).")
	return nil
}

func (b *Bot) advanceEventSubWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	if state.Step != 0 {
		return nil
	}
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		b.sendSimple(msg.Chat.ID, "Время события не может быть пустым.")
		return nil
	}
	matchID := parseInt64(state.Data["match_id"])
	outID := parseInt64(state.Data["out_id"])
	inID := parseInt64(state.Data["in_id"])
	if err := b.svc.Events.AddSub(ctx, matchID, outID, inID, text); err != nil {
		b.sendSimple(msg.Chat.ID, fmt.Sprintf("Не удалось добавить замену: %v", err))
		return nil
	}
	b.sendSimple(msg.Chat.ID, "Замена добавлена.")
	_ = b.showMatch(ctx, msg.Chat.ID, matchID)
	return b.svc.Sessions.Clear(ctx, msg.From.ID)
}

func (b *Bot) startMatchEditWizard(ctx context.Context, chatID, adminID int64, matchID int64) error {
	state := &wizardState{
		Flow: flowMatchEdit,
		Step: 0,
		Data: map[string]string{
			"match_id": strconv.FormatInt(matchID, 10),
		},
	}
	if err := b.saveSession(ctx, adminID, &state.Flow, state); err != nil {
		return err
	}
	b.sendSimple(chatID, "Введите статус (scheduled/played/canceled) или '-' чтобы оставить без изменений.")
	return nil
}

func (b *Bot) advanceMatchEditWizard(ctx context.Context, msg *tgbotapi.Message, state *wizardState) error {
	text := strings.TrimSpace(msg.Text)
	adminID := msg.From.ID
	chatID := msg.Chat.ID

	switch state.Step {
	case 0:
		if text != "" && text != "-" {
			normalized := strings.ToLower(text)
			if normalized != string(models.MatchStatusScheduled) &&
				normalized != string(models.MatchStatusPlayed) &&
				normalized != string(models.MatchStatusCanceled) {
				b.sendSimple(chatID, "Допустимые статусы: scheduled, played, canceled или '-'.")
				return nil
			}
			state.Data["status"] = normalized
		}
		state.Step++
		b.sendSimple(chatID, "Введите дату матча (YYYY-MM-DD) или '-' чтобы оставить без изменений.")
	case 1:
		if text != "" && text != "-" {
			if _, err := time.Parse("2006-01-02", text); err != nil {
				b.sendSimple(chatID, "Неверный формат даты. Используйте YYYY-MM-DD или '-'.")
				return nil
			}
			state.Data["date"] = text
		}
		state.Step++
		b.sendSimple(chatID, "Введите время матча (HH:MM) или '-' чтобы оставить без изменений.")
	case 2:
		if text != "" && text != "-" {
			if _, err := time.Parse("15:04", text); err != nil {
				b.sendSimple(chatID, "Неверный формат времени. Используйте HH:MM или '-'.")
				return nil
			}
			state.Data["time"] = text
		}
		state.Step++
		b.sendSimple(chatID, "Введите место проведения или '-' чтобы очистить (оставьте пустым для без изменений).")
	case 3:
		if text != "" {
			state.Data["location"] = text
		}
		state.Step++
		b.sendSimple(chatID, "Введите счёты через пробел: HT FT ET PEN FINAL_US FINAL_THEM. Используйте '-' для каждого значения или '-' целиком чтобы пропустить.")
	case 4:
		if text != "" {
			state.Data["scores"] = text
		}
		if err := b.finishMatchEditWizard(ctx, state); err != nil {
			b.sendSimple(chatID, fmt.Sprintf("Не удалось обновить матч: %v. Повторите ввод счётов.", err))
			return nil
		}
		b.sendSimple(chatID, "Матч обновлён.")
		matchID := parseInt64(state.Data["match_id"])
		_ = b.showMatch(ctx, chatID, matchID)
		return b.svc.Sessions.Clear(ctx, adminID)
	}
	return b.saveSession(ctx, adminID, &state.Flow, state)
}

func (b *Bot) finishMatchEditWizard(ctx context.Context, state *wizardState) error {
	matchID := parseInt64(state.Data["match_id"])
	match, err := b.svc.Matches.Get(ctx, matchID)
	if err != nil {
		return err
	}
	patch := models.MatchPatch{}

	if status := state.Data["status"]; status != "" && status != "-" {
		ms := models.MatchStatus(status)
		if ms != models.MatchStatusScheduled && ms != models.MatchStatusPlayed && ms != models.MatchStatusCanceled {
			return fmt.Errorf("неверный статус: %s", status)
		}
		patch.Status = &ms
	}

	dateVal := state.Data["date"]
	timeVal := state.Data["time"]
	if dateVal == "" {
		dateVal = "-"
	}
	if timeVal == "" {
		timeVal = "-"
	}
	if dateVal != "-" || timeVal != "-" {
		start := match.StartTime.In(b.loc)
		year, month, day := start.Date()
		hour, minute, _ := start.Clock()

		if dateVal != "-" {
			parsed, err := time.ParseInLocation("2006-01-02", dateVal, b.loc)
			if err != nil {
				return fmt.Errorf("неверный формат даты: %w", err)
			}
			year, month, day = parsed.Date()
		}
		if timeVal != "-" {
			parsed, err := time.Parse("15:04", timeVal)
			if err != nil {
				return fmt.Errorf("неверный формат времени: %w", err)
			}
			hour, minute, _ = parsed.Clock()
		}
		newStart := time.Date(year, month, day, hour, minute, 0, 0, b.loc).UTC()
		patch.StartTime = models.NewOptionalTime(&newStart)
	}

	if loc := state.Data["location"]; loc != "" {
		if loc == "-" {
			patch.Location = models.NewOptionalString(nil)
		} else {
			value := loc
			patch.Location = models.NewOptionalString(&value)
		}
	}

	if patch.Status != nil && *patch.Status == models.MatchStatusCanceled {
		patch.ScoreHT = models.NewOptionalString(nil)
		patch.ScoreFT = models.NewOptionalString(nil)
		patch.ScoreET = models.NewOptionalString(nil)
		patch.ScorePEN = models.NewOptionalString(nil)
		patch.ScoreFinalUs = models.NewOptionalInt(nil)
		patch.ScoreFinalThem = models.NewOptionalInt(nil)
	} else if scores := state.Data["scores"]; scores != "" && scores != "-" {
		tokens := strings.Fields(scores)
		if len(tokens) != 6 {
			return fmt.Errorf("ожидалось 6 значений для счётов, получено %d", len(tokens))
		}
		if patch.ScoreHT, err = optionalScoreString(tokens[0]); err != nil {
			return err
		}
		if patch.ScoreFT, err = optionalScoreString(tokens[1]); err != nil {
			return err
		}
		if patch.ScoreET, err = optionalScoreString(tokens[2]); err != nil {
			return err
		}
		if patch.ScorePEN, err = optionalScoreString(tokens[3]); err != nil {
			return err
		}
		if patch.ScoreFinalUs, err = optionalScoreInt(tokens[4]); err != nil {
			return err
		}
		if patch.ScoreFinalThem, err = optionalScoreInt(tokens[5]); err != nil {
			return err
		}
	}

	return b.svc.Matches.Update(ctx, matchID, patch)
}

func (b *Bot) setMatchStatus(ctx context.Context, chatID int64, matchID int64, statusText string) error {
	status := models.MatchStatus(strings.ToLower(statusText))
	switch status {
	case models.MatchStatusScheduled, models.MatchStatusPlayed, models.MatchStatusCanceled:
	default:
		b.sendSimple(chatID, "Неизвестный статус.")
		return nil
	}
	if err := b.svc.Matches.Update(ctx, matchID, models.MatchPatch{Status: &status}); err != nil {
		b.sendSimple(chatID, fmt.Sprintf("Не удалось обновить статус: %v", err))
		return nil
	}
	b.sendSimple(chatID, fmt.Sprintf("Статус матча: %s", statusLabel(status)))
	return b.showMatch(ctx, chatID, matchID)
}

func (b *Bot) resetMatchScores(ctx context.Context, chatID int64, matchID int64) error {
	patch := models.MatchPatch{
		ScoreHT:        models.NewOptionalString(nil),
		ScoreFT:        models.NewOptionalString(nil),
		ScoreET:        models.NewOptionalString(nil),
		ScorePEN:       models.NewOptionalString(nil),
		ScoreFinalUs:   models.NewOptionalInt(nil),
		ScoreFinalThem: models.NewOptionalInt(nil),
	}
	if err := b.svc.Matches.Update(ctx, matchID, patch); err != nil {
		b.sendSimple(chatID, fmt.Sprintf("Не удалось сбросить счёт: %v", err))
		return nil
	}
	b.sendSimple(chatID, "Счёт матча сброшен.")
	return b.showMatch(ctx, chatID, matchID)
}

type matchSummary struct {
	Match          models.Match
	TeamName       string
	TournamentName string
}

func (b *Bot) collectUpcomingMatches(ctx context.Context, tournamentID int64, teams []models.TournamentTeam) []matchSummary {
	cutoff := b.timeNow().Add(-1 * time.Hour)
	teamNames := make(map[int64]string, len(teams))
	for _, tm := range teams {
		teamNames[tm.TeamID] = tm.TeamName
	}
	var summaries []matchSummary
	for _, tm := range teams {
		matches, err := b.svc.Matches.List(ctx, tournamentID, tm.TeamID)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if m.Status == models.MatchStatusScheduled && m.StartTime.After(cutoff) {
				summaries = append(summaries, matchSummary{
					Match:    m,
					TeamName: teamNames[tm.TeamID],
				})
			}
		}
	}
	if len(summaries) == 0 {
		return nil
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Match.StartTime.Before(summaries[j].Match.StartTime)
	})
	if len(summaries) > 3 {
		summaries = summaries[:3]
	}
	return summaries
}

func (b *Bot) collectTeamUpcomingMatches(ctx context.Context, tournaments []models.Tournament, teamID int64) []matchSummary {
	cutoff := b.timeNow().Add(-1 * time.Hour)
	var summaries []matchSummary
	for _, t := range tournaments {
		matches, err := b.svc.Matches.List(ctx, t.ID, teamID)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if m.Status == models.MatchStatusScheduled && m.StartTime.After(cutoff) {
				summaries = append(summaries, matchSummary{
					Match:          m,
					TournamentName: t.Name,
				})
			}
		}
	}
	if len(summaries) == 0 {
		return nil
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Match.StartTime.Before(summaries[j].Match.StartTime)
	})
	if len(summaries) > 3 {
		summaries = summaries[:3]
	}
	return summaries
}

// ----------------------------------------------------------------------------

type callbackPayload struct {
	Action string
	Params map[string]string
}

func parseCallback(data string) (*callbackPayload, error) {
	parts := strings.Split(data, "|")
	if len(parts) == 0 {
		return nil, errors.New("empty callback")
	}
	payload := &callbackPayload{
		Action: parts[0],
		Params: map[string]string{},
	}
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		payload.Params[kv[0]] = kv[1]
	}
	return payload, nil
}

func escape(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"`", "\\`",
		"[", "\\[",
	)
	return replacer.Replace(s)
}

func parseInt64(value string) int64 {
	if value == "" {
		return 0
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func parseOptionalInt(value string) (*int, error) {
	if value == "" {
		return nil, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return nil, err
	}
	return &number, nil
}

func parseYesNo(text string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "да", "yes", "y", "true", "1":
		return true, true
	case "нет", "no", "n", "false", "0":
		return false, true
	default:
		return false, false
	}
}

func truncateLabel(label string, max int) string {
	if len(label) <= max {
		return label
	}
	runes := []rune(label)
	if len(runes) <= max {
		return label
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func safeInt(val *int) int {
	if val == nil {
		return 0
	}
	return *val
}

func matchStatusButton(matchID int64, current models.MatchStatus, target models.MatchStatus) tgbotapi.InlineKeyboardButton {
	label := statusLabel(target)
	prefix := "⚪"
	if current == target {
		prefix = "✅"
	}
	return tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%s %s", prefix, label), fmt.Sprintf("match_status_set|id=%d|status=%s", matchID, target))
}

func statusLabel(status models.MatchStatus) string {
	switch status {
	case models.MatchStatusScheduled:
		return "План"
	case models.MatchStatusPlayed:
		return "Сыгран"
	case models.MatchStatusCanceled:
		return "Отменён"
	default:
		return string(status)
	}
}

func optionalScoreString(token string) (models.OptionalString, error) {
	if token == "-" {
		return models.NewOptionalString(nil), nil
	}
	if !strings.Contains(token, ":") {
		return models.OptionalString{}, fmt.Errorf("некорректный формат счёта %q", token)
	}
	value := token
	return models.NewOptionalString(&value), nil
}

func optionalScoreInt(token string) (models.OptionalInt, error) {
	if token == "-" {
		return models.NewOptionalInt(nil), nil
	}
	value, err := strconv.Atoi(token)
	if err != nil {
		return models.OptionalInt{}, fmt.Errorf("некорректное значение счёта %q", token)
	}
	return models.NewOptionalInt(&value), nil
}

func compareParamMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func parseIntParam(params map[string]string, key string, def int) int {
	if params == nil {
		return def
	}
	val, ok := params[key]
	if !ok || val == "" {
		return def
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return parsed
}

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/dynamost/telegram-bot/internal/config"
	"github.com/dynamost/telegram-bot/internal/repository/pg"
	"github.com/dynamost/telegram-bot/internal/service"
	"github.com/dynamost/telegram-bot/internal/session"
	"github.com/dynamost/telegram-bot/internal/telegram"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	settings, pool, err := config.Load(ctx)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	defer pool.Close()

	logger := config.NewLogger()

	teamsRepo := pg.NewTeamsRepo(pool)
	playersRepo := pg.NewPlayersRepo(pool)
	tournamentsRepo := pg.NewTournamentsRepo(pool)
	rostersRepo := pg.NewRostersRepo(pool)
	matchesRepo := pg.NewMatchesRepo(pool)
	lineupRepo := pg.NewLineupRepo(pool)
	eventsRepo := pg.NewEventsRepo(pool)
	sessionsRepo := pg.NewSessionsRepo(pool)

	teamsSvc := service.NewTeamsService(teamsRepo)
	playersSvc := service.NewPlayersService(playersRepo)
	tournamentsSvc := service.NewTournamentsService(tournamentsRepo)
	rostersSvc := service.NewRostersService(rostersRepo)
	matchesSvc := service.NewMatchesService(matchesRepo, rostersRepo)
	lineupSvc := service.NewLineupService(lineupRepo, matchesRepo, rostersRepo)
	eventsSvc := service.NewEventsService(eventsRepo, matchesRepo, rostersRepo)
	sessionSvc := service.NewSessionService(sessionsRepo)
	sessionStore := session.NewStore(sessionSvc)

	botAPI, err := tgbotapi.NewBotAPI(settings.BotToken)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}
	botAPI.Debug = os.Getenv("DEBUG") == "1"

	bot := telegram.NewBot(botAPI, settings.AdminIDs, settings.Location, telegram.Services{
		Teams:       teamsSvc,
		Players:     playersSvc,
		Tournaments: tournamentsSvc,
		Rosters:     rostersSvc,
		Matches:     matchesSvc,
		Lineup:      lineupSvc,
		Events:      eventsSvc,
		Sessions:    sessionStore,
	}, logger)

	if err := bot.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("bot stopped: %v", err)
	}
}

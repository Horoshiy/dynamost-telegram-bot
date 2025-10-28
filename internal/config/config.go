package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type Settings struct {
	BotToken string
	DBDSN    string
	AdminIDs []int64
	Location *time.Location
}

func Load(ctx context.Context) (*Settings, *pgxpool.Pool, error) {
	_ = godotenv.Load()

	set := &Settings{}
	set.BotToken = strings.TrimSpace(os.Getenv("BOT_TOKEN"))
	if set.BotToken == "" {
		return nil, nil, fmt.Errorf("BOT_TOKEN is required")
	}

	set.DBDSN = strings.TrimSpace(os.Getenv("DB_DSN"))
	if set.DBDSN == "" {
		return nil, nil, fmt.Errorf("DB_DSN is required")
	}

	adminRaw := strings.TrimSpace(os.Getenv("ADMIN_IDS"))
	if adminRaw == "" {
		return nil, nil, fmt.Errorf("ADMIN_IDS is required")
	}
	for _, part := range strings.Split(adminRaw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		val, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid admin id %q: %w", part, err)
		}
		set.AdminIDs = append(set.AdminIDs, val)
	}
	if len(set.AdminIDs) == 0 {
		return nil, nil, fmt.Errorf("ADMIN_IDS must contain at least one value")
	}

	tz := strings.TrimSpace(os.Getenv("CLUB_TZ"))
	if tz == "" {
		return nil, nil, fmt.Errorf("CLUB_TZ is required")
	}
	location, err := time.LoadLocation(tz)
	if err != nil {
		return nil, nil, fmt.Errorf("load CLUB_TZ: %w", err)
	}
	set.Location = location

	cfg, err := pgxpool.ParseConfig(set.DBDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("parse db dsn: %w", err)
	}
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("connect db: %w", err)
	}
	return set, pool, nil
}

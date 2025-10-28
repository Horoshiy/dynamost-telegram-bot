# Dynamost Telegram Bot

Go ≥ 1.22 Telegram bot for managing the club’s tournaments, teams, players, rosters, matches, lineups, and match events via inline keyboards.

## Requirements

- Go 1.22 or newer
- PostgreSQL 14+
- `goose` CLI (for migrations)
- Optional: `golangci-lint`, `air` or similar for dev autoreload

## Setup

```bash
cp .env.example .env
go mod tidy
```

Edit `.env` with a valid `BOT_TOKEN`, local `DB_DSN`, comma-separated `ADMIN_IDS`, and `CLUB_TZ` (IANA timezone identifier, e.g. `Europe/Moscow`).

## Database

Create an empty database and run migrations:

```bash
export DB_DSN=postgres://user:pass@localhost:5432/football?sslmode=disable
make migrate-up
```

Rollback if necessary:

```bash
make migrate-down
```

## Development

Run the bot locally:

```bash
make run
```

The entrypoint is `cmd/bot/main.go`. Core packages live under `internal/`:

- `config` – parses `.env`, wires dependencies, creates loggers
- `telegram` – slash-command handlers, inline rendering, callback routing, wizard flows
- `service` – business logic and validation rules
- `repository/pg` – pgx-based data access
- `session` – admin wizard persistence backed by `admin_sessions`
- `models` – shared domain DTOs

Refer to `docs/telegram-football-bot_TZ_v3.md` for functional requirements and UX flows.

## Testing & Quality

Write table-driven tests alongside packages (`*_test.go`). Run all tests before pushing:

```bash
go test ./...
```

Lint (optional):

```bash
make lint
```

## Deployment

### Production Deployment to Ubuntu Server

Для развертывания бота на продакшн сервере с Ubuntu:

```bash
# Быстрый старт (10-15 минут)
```

1. **Подготовьте сервер:**
   ```bash
   wget https://raw.githubusercontent.com/dynamost/telegram-bot/main/deploy/setup-server.sh
   chmod +x setup-server.sh
   sudo ./setup-server.sh
   ```

2. **Настройте GitHub Secrets** (см. [QUICK_DEPLOY.md](docs/QUICK_DEPLOY.md))

3. **Запустите деплой:**
   ```bash
   git push origin main
   ```

📖 **Документация:**
- [Быстрый старт деплоя](docs/QUICK_DEPLOY.md) — пошаговая инструкция за 15 минут
- [Полная документация по деплою](docs/DEPLOYMENT.md) — подробное руководство с troubleshooting

### Что включает автоматический деплой:

- ✅ Сборка бинарника для Linux
- ✅ Копирование на сервер по SSH
- ✅ Автоматическое применение миграций БД
- ✅ Настройка systemd сервиса
- ✅ Автоматический перезапуск бота

### Управление ботом на сервере:

```bash
# Проверка статуса
sudo systemctl status dynamost-bot

# Просмотр логов
sudo journalctl -u dynamost-bot -f

# Перезапуск
sudo systemctl restart dynamost-bot
```

## Manual Smoke Checklist

1. Add your Telegram ID to `ADMIN_IDS`, run the bot, and trigger `/tournaments`, `/teams`, `/players`.
2. Create a tournament, team, and player with the wizards.
3. Build a tournament roster for a team and attach numbers.
4. Schedule a match, manage lineup entries, and log match events.
5. Cancel a match and verify that all score fields reset to `NULL`.
6. Inspect stdout logs for `timestamp admin_tg_id action entity entity_id status`.

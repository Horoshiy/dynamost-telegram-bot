# Codex: Генерация проекта Telegram-бота (Go + PostgreSQL)

Этот документ — **инструкция и промпт для Codex**, который должен использовать требования `telegram-football-bot_TZ_v3.md` для сборки минимально работоспособного репозитория.

---

## 1) Цели генерации
1. Собрать каркас Go-проекта (Go ≥ 1.22) с четким разделением: Telegram-слой → сервисы → репозитории → PostgreSQL.
2. Подготовить SQL-миграции для всех сущностей из ТЗ, включая хранение состояний wizard в `admin_sessions`.
3. Сгенерировать доменные модели, репозитории и сервисы с бизнес-проверками (заявки, составы, события, очистка счетов).
4. Реализовать Telegram-слой: входные slash-команды, inline-навигацию, пагинацию по 20 элементов, пошаговые мастеры, парсер `callback_data`, сохранение состояний wizard.
5. Настроить конфигурацию и инфраструктуру: загрузка `.env`, логирование в stdout, запуск миграций и бота через Makefile.
6. Обеспечить выполнение Definition of Done из ТЗ и оформить инструкции в README.

---

## 2) Структура репозитория (обязательная)

```
/cmd/bot/main.go
/internal/config/           # парсинг env и настройка зависимостей
/internal/telegram/         # обработчики апдейтов, рендер экранов, inline-кнопки
/internal/session/          # работа с admin_sessions, state machine
/internal/models/           # доменные структуры
/internal/repository/       # доступ к БД (pgx), реализации интерфейсов
/internal/service/          # бизнес-логика Teams/Players/Tournaments/Rosters/Matches/Lineup/Events
/migrations/                # SQL миграции goose
/docs/telegram-football-bot_TZ_v3.md
.env.example
Makefile
go.mod
README.md
```

---

## 3) Конфигурация, окружение и логирование

`.env.example`:

```
BOT_TOKEN=YOUR_TELEGRAM_BOT_TOKEN
DB_DSN=postgres://user:pass@localhost:5432/football?sslmode=disable
ADMIN_IDS=12345,67890
CLUB_TZ=Europe/Moscow
```

`/internal/config/` должен:

- Читать `.env`, парсить `BOT_TOKEN`, `DB_DSN`, `ADMIN_IDS` (список `int64`) и `CLUB_TZ` (IANA time zone).
- Поднимать соединение с PostgreSQL (pgx `pool` или `conn`).
- Предоставлять зависимости сервисам (репозитории, session storage).
- Готовить логгер, пишущий в stdout строки вида `timestamp admin_tg_id action entity entity_id status`.

Все ответы бота — краткие, на русском, без лишнего текста. Неадминистратору отвечать «У вас нет прав. Обратитесь к директору клуба.» и скрывать кнопки.

---

## 4) Схема БД и миграции (goose)

Файлы миграций:

```
0001_init_teams.sql
0002_init_players.sql
0003_init_tournaments.sql
0004_init_tournament_roster.sql
0005_init_matches.sql
0006_init_match_lineups.sql
0007_init_match_events.sql
0008_init_admin_sessions.sql
```

**0001_init_teams.sql**
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS teams (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  short_code TEXT NOT NULL UNIQUE,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  note TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS teams;
```

**0002_init_players.sql**
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS players (
  id BIGSERIAL PRIMARY KEY,
  full_name TEXT NOT NULL,
  birth_date DATE NULL,
  position TEXT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  note TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS players;
```

**0003_init_tournaments.sql**
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS tournaments (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NULL,
  status TEXT NOT NULL DEFAULT 'active', -- planned/active/finished
  start_date DATE NULL,
  end_date DATE NULL,
  note TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS tournaments;
```

**0004_init_tournament_roster.sql**
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS tournament_roster (
  id BIGSERIAL PRIMARY KEY,
  tournament_id BIGINT NOT NULL REFERENCES tournaments(id),
  team_id BIGINT NOT NULL REFERENCES teams(id),
  player_id BIGINT NOT NULL REFERENCES players(id),
  tournament_number INT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tournament_id, team_id, player_id)
);

-- +goose Down
DROP TABLE IF EXISTS tournament_roster;
```

**0005_init_matches.sql**
```sql
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
```

**0006_init_match_lineups.sql**
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS match_lineups (
  id BIGSERIAL PRIMARY KEY,
  match_id BIGINT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
  player_id BIGINT NOT NULL REFERENCES players(id),
  role TEXT NOT NULL, -- 'start' | 'sub'
  number_override INT NULL,
  note TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (match_id, player_id)
);

-- +goose Down
DROP TABLE IF EXISTS match_lineups;
```

**0007_init_match_events.sql**
```sql
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
```

**0008_init_admin_sessions.sql**
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS admin_sessions (
  admin_tg_id BIGINT PRIMARY KEY,
  current_flow TEXT NULL,
  flow_state JSONB NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS admin_sessions;
```

---

## 5) Сервисы, репозитории и бизнес-правила

`/internal/repository/` — pgx-реализации: TeamsRepo, PlayersRepo, TournamentsRepo, RostersRepo, MatchesRepo, LineupRepo, EventsRepo, SessionRepo.

`/internal/service/` — интерфейсы и реализации:

- `TeamsService`, `PlayersService`, `TournamentsService`, `RostersService`, `MatchesService`, `LineupService`, `EventsService`, `SessionService`.

Ключевые проверки и правила:

- Авторизация происходит в Telegram-слое до вызова сервисов.
- Создание матча запрещено, если у `(tournament_id, team_id)` нет игроков в `tournament_roster`.
- Добавление/обновление состава и событий допускается только для игроков из заявки соответствующей пары.
- Изменение матча со статусом `canceled` обязано обнулять все поля счёта (HT/FT/ET/PEN и итог).
- `match_lineups` используют `UNIQUE (match_id, player_id)` — повторное добавление означает `UPDATE`.
- `match_events` валидируют обязательные поля: `goal` → `player_id_main`; `card` → `player_id_main + card_type`; `sub` → оба `player_id`.
- Удаление из `tournament_roster` запрещено, если игрок участвовал в составе или событиях матчей турнира (проверить связи).
- Сервисы не знают о Telegram-структурах; возвращают чистые модели и ошибки.

При генерации можно ориентироваться на контракт из ТЗ:

```go
type MatchesService interface {
    List(tournamentID, teamID int64) ([]Match, error)
    Get(id int64) (*Match, error)
    Create(tournamentID, teamID int64, opponent string, start time.Time, location *string) (int64, error)
    Update(id int64, patch MatchPatch) error
}
```

(аналогично для остальных сервисов).

---

## 6) Telegram-слой и UX

Обработчики slash-команд:

### `/tournaments`
- Список `planned` + `active` турниров, кнопки «Открыть…», «➕ Создать турнир».
- Карточка турнира: данные, команды из `tournament_roster`, кнопки `✏ Редактировать`, `👥 Заявки`, `🏟 Матчи`, `⬅ Назад`.
- Wizard создания: Название → Тип → Статус → Дата начала → Дата окончания → Примечание.

### `/teams`
- Список активных команд, кнопки «Открыть…», «➕ Создать команду».
- Карточка: `name`, `short_code`, `active`, `note`, кнопки `✏ Редактировать`, `⬅ Назад`.
- Wizard создания: Название → Короткий код → Активна? → Примечание?.

### `/players`
- Список игроков с пагинацией (20 шт.), кнопки «Открыть игрока…», «➡ Далее», «➕ Создать игрока».
- Карточка: ФИО, дата рождения, позиция, активность, примечание, список заявок (турнир/команда/номер).
- Wizard создания: ФИО → Дата рождения (или пропуск) → Позиция (или пропуск) → Примечание (или пропуск).

### `/tournament_rosters`
- Поток: выбрать турнир → выбрать команду → видеть заявку.
- Кнопки: «➕ Добавить игрока», «✏ Изменить номер игрока», «🗑 Удалить игрока», «⬅ Назад».
- Добавление: выбрать игрока из списка (с пагинацией) → запросить номер (опционально) → создать запись.
- Изменение номера: выбрать игрока → ввести новый номер → обновить.
- Удаление: выбрать → подтверждение → удалить (с проверками связей).

### `/games`
- Поток: выбрать турнир → выбрать команду → увидеть список матчей (`Открыть матч`, `➕ Создать матч`, `⬅ Назад`).
- Карточка матча: соперник, дата/время, место, статус, счета (HT/FT/ET/PEN/итог), краткий состав, события.
- Кнопки: `✏ Редактировать матч`, `👥 Состав`, `⚽ События`, `⬅ Назад`.
- Wizard создания матча: соперник → дата → время → локация (проверить наличие игроков в заявке перед созданием).
- Управление составом: добавление из заявки, изменение роли (`start/sub`), установка номера на матч, удаление.
- Управление событиями: добавление goal/card/sub, указание времени (строка «45+2», «90» и т.п.), выбор игроков, просмотр списка.

### Общие требования Telegram-слоя

- Формат `callback_data`: `action|k1=v1|k2=v2`. Примеры: `open_tournament|id=7`, `open_match|id=91`, `events_add_goal|match=91|player=123`.
- Пагинация: 20 элементов, кнопки «⬅ Назад страница», «Вперёд ➡», хранить `page=N` в callback.
- State machine: хранить состояние wizard и стек экранов в `admin_sessions` (`flow_state JSONB`), предусмотреть `StartFlow`, `AdvanceFlow`, `CancelFlow`.
- Кнопка «⬅ Назад» возвращает на предыдущий экран (использовать стек в `admin_sessions`).
- Авторизация: любая команда/кнопка проверяет `ADMIN_IDS`; отказ → сообщение об отсутствии прав.
- Ошибки логируются, пользователю возвращается понятное сообщение.

---

## 7) Инфраструктура и запуск

`Makefile` цели:

```
run:        # go run ./cmd/bot
migrate-up: # goose -dir ./migrations postgres "$(DB_DSN)" up
migrate-down: # откат миграций
lint:       # golangci-lint run (если добавите)
```

`README.md` должен описывать:

- Требования: Go ≥ 1.22, PostgreSQL.
- Инициализацию: `go mod init`, список зависимостей (telegram-bot-api v5, pgx/v5, goose, godotenv, golangci-lint по желанию).
- Настройку `.env`, запуск миграций и бота: `make migrate-up && make run`.
- Замечание, что после входных команд взаимодействие идет через inline-кнопки.

---

## 8) Definition of Done и проверки

Этап считается выполненным, если админ, используя только кнопки:

- Создает и редактирует турниры, команды, игроков.
- Через `/tournament_rosters` управляет заявкой: добавляет, меняет номер, удаляет (с проверками).
- Через `/games` создает матчи, редактирует дату/время/место/статус/счеты, управляет составом и событиями.
- Все ограничения из раздела «Валидация и бизнес-правила» соблюдаются.
- Пагинация (20 элементов) и кнопка «⬅ Назад» работают на всех списках.
- Статус `canceled` очищает счета.
- Логи в stdout фиксируют действия в формате `timestamp admin_tg_id action entity entity_id status`.
- Бот игнорирует некорректные апдейты без падений.

Перед сдачей вручную пройти ключевые сценарии (создание сущностей, настройка заявки, матч, событие, отмена).

---

## 9) Промпт для Codex (скопируй целиком)

**Роль**: Senior Go engineer.  
**Задача**: Сгенерировать каркас и минимальную реализацию проекта Telegram-бота в соответствии с `telegram-football-bot_TZ_v3.md` и инструкцией `codex_prompt_generation.md`.  
**Ограничения**: Go ≥ 1.22, PostgreSQL, чистая архитектура (telegram → services → repository), inline-кнопки, пагинация по 20 элементов, хранение wizard-состояния в `admin_sessions`, ответы бота на русском.

**Выполни:**
1. Создай структуру репозитория из раздела «Структура репозитория».
2. Создай все миграции из раздела «Схема БД и миграции» (с полным SQL).
3. Сформируй `go.mod`, подключи зависимости (pgx/v5, goose, tgbotapi, godotenv и др.).
4. Реализуй конфигурацию: загрузка `.env`, парсинг `BOT_TOKEN`, `DB_DSN`, `ADMIN_IDS`, инициализация БД.
5. Реализуй репозитории и сервисы с описанными бизнес-проверками.
6. Реализуй Telegram-слой: slash-команды, inline-экраны, callback-парсер, пагинацию, wizards с сохранением в `admin_sessions`.
7. Реализуй хранение и восстановление состояний wizard (`internal/session` + таблица `admin_sessions`).
8. Подготовь `Makefile` и `README.md` с командами запуска.
9. Убедись, что сценарии Definition of Done выполнимы и логирование соответствует требованиям.

Выводи все файлы и их содержимое в порядке, удобном для сохранения на диск.

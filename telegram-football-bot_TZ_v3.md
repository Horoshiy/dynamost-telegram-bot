# Техническое задание (v3) — Telegram-бот управления футбольным клубом

**Дата:** 2025-10-26  
**Язык реализации:** Go ≥ 1.22  
**БД:** PostgreSQL  
**Интерфейс:** Telegram Bot API с inline-кнопками (без длинных команд с параметрами)  

---

## Оглавление

1. [Цель проекта](#цель-проекта)  
2. [Роли доступа](#роли-доступа)  
3. [Сущности домена](#сущности-домена)  
   - [Team (Команда)](#team-команда)  
   - [Player (Игрок)](#player-игрок)  
   - [Tournament (Турнир)](#tournament-турнир)  
   - [TournamentRoster (Заявка на турнир)](#tournamentroster-заявка-на-турнир)  
   - [Match (Матч)](#match-матч)  
   - [MatchLineupItem (Состав на матч)](#matchlineupitem-состав-на-матч)  
   - [MatchEvent (Событие матча)](#matchevent-событие-матча)  
4. [Навигация в боте (UX)](#навигация-в-боте-ux)  
   - [/tournaments](#/tournaments)  
   - [/teams](#/teams)  
   - [/players](#/players)  
   - [/tournament_rosters](#/tournament_rosters)  
   - [/games](#/games)  
5. [Схема БД (PostgreSQL)](#схема-бд-postgresql)  
6. [Валидация и бизнес-правила](#валидация-и-бизнес-правила)  
7. [Нефункциональные требования](#нефункциональные-требования)  
8. [Конфигурация и окружение](#конфигурация-и-окружение)  
9. [Структура репозитория](#структура-репозитория)  
10. [Миграции](#миграции)  
11. [Definition of Done](#definition-of-done)  
12. [Документация для Codex](#документация-для-codex)  
    - [Callback-паттерны](#callback-паттерны)  
    - [State machine / wizard](#state-machine--wizard)  
    - [Сервисы (контракты)](#сервисы-контракты)  
    - [Задачи для генерации Codex](#задачи-для-генерации-codex)  
13. [Дальнейшие расширения (не в этом этапе)](#дальнейшие-расширения-не-в-этом-этапе)

---

## Цель проекта

Создать Telegram-бот, который позволяет администраторам клуба управлять командами, игроками, турнирами, заявками игроков на турнир, матчами, составами и событиями матча через удобные **inline-кнопки**.  
Текстовые slash-команды используются **только** как вход в разделы:

- `/tournaments`
- `/teams`
- `/players`
- `/tournament_rosters`
- `/games`

Далее вся работа — кнопками и пошаговыми диалогами.

---

## Роли доступа

### Администратор
- Telegram `user_id` должен быть в списке `ADMIN_TELEGRAM_IDS` (конфиг/БД).
- Имеет доступ ко всем CRUD-операциям и навигации.

### Не администратор
- Получает ответ: `У вас нет прав. Обратитесь к директору клуба.`
- Админские кнопки не показываются.

---

## Сущности домена

### Team (Команда)

Адм. единица: «Динамчики Юг», «Центр», «204-й квартал».  
**Не** храним тренера, год рождения, возраст.

**Поля:**
- `id`
- `name` — полное название
- `short_code` — короткий уникальный код
- `active` — флаг активности
- `note` — примечание
- `created_at`, `updated_at`

---

### Player (Игрок)

Футболист клуба. **Номер не хранится в Player.**

**Поля:**
- `id`
- `full_name`
- `birth_date` (NULLable)
- `position` (NULLable, текст)
- `active` (bool)
- `note` (NULLable)
- `created_at`, `updated_at`

---

### Tournament (Турнир)

**Поля:**
- `id`
- `name`
- `type` (строка — «лига», «кубок», «товарищеский», …)
- `status` — `planned` / `active` / `finished`
- `start_date`, `end_date` (NULLable)
- `note` (NULLable)
- `created_at`, `updated_at`

---

### TournamentRoster (Заявка на турнир)

Ключевая сущность: связь «Игрок — Команда — Турнир», где хранится **номер игрока на турнир**.

**Поля:**
- `id`
- `tournament_id` → `tournaments.id`
- `team_id` → `teams.id`
- `player_id` → `players.id`
- `tournament_number` (INT, NULLable) — номер игрока в **этом** турнире
- `created_at`, `updated_at`

**Ограничение:**
- `UNIQUE (tournament_id, team_id, player_id)` — нельзя дублировать игрока в заявке одной команды на один турнир.

---

### Match (Матч)

Матч всегда привязан к `(tournament_id, team_id)`. Создавать матч можно **только**, если для этой пары есть игроки в `tournament_roster`.

**Поля:**
- `id`
- `tournament_id` (обязателен)
- `team_id` (обязателен)
- `opponent_name` (строка)
- `start_time` (TIMESTAMPTZ)
- `location` (NULLable)
- `status` — `scheduled` / `played` / `canceled`

**Счёты:**
- `score_ht`  — строка «X:Y» (1-й тайм)
- `score_ft`  — строка «X:Y» (после 2-го тайма, итог основного времени)
- `score_et`  — строка «X:Y» (после доп. времени, если было)
- `score_pen` — строка «X:Y» (серия пенальти, если была)
- `score_final_us`   — итоговые голы нашей команды (INT)
- `score_final_them` — итоговые голы соперника (INT)

**Правило:**
- При `status = canceled` все поля счёта должны быть `NULL`.

---

### MatchLineupItem (Состав на матч)

Список игроков, заявленных на **конкретный матч**, и их роли.  
Игрок должен быть в `tournament_roster` для пары `(match.tournament_id, match.team_id)`.

**Поля:**
- `id`
- `match_id`
- `player_id`
- `role` — `"start"` | `"sub"`
- `number_override` (INT, NULLable) — номер **именно** на этот матч  
  (если пусто — показываем `tournament_roster.tournament_number`)
- `note` (NULLable)
- `created_at`, `updated_at`

**Ограничение:**
- `UNIQUE (match_id, player_id)`

---

### MatchEvent (Событие матча)

Допустимые типы: **`goal`**, **`card`**, **`sub`**.

**Поля:**
- `id`
- `match_id`
- `event_type` — `"goal" | "card" | "sub"`
- `event_time` — строка («5 мин», «45+2», «1 тайм» …)
- `player_id_main`
- `player_id_alt` — только для `sub`
- `card_type` — только для `card`: `"yellow" | "red"`
- `created_at`

**Валидация:**
- `goal` → `player_id_main` обязателен
- `card` → `player_id_main` и `card_type` обязательны
- `sub` → `player_id_main` и `player_id_alt` обязательны  
Все игроки должны присутствовать в `tournament_roster` пары `(tournament_id, team_id)` матча.

---

## Навигация в боте (UX)

Базовые входные команды → экраны со списками и кнопками. Дальше — навигация по **inline-кнопкам**.  
Текст запрашивается **только** когда бот просит в рамках wizard.

### `/tournaments`

- Показывает список турниров (обычно `planned` + `active`) с кнопками:
  - «Открыть турнир …»
  - «➕ Создать турнир»
- Экран турнира:
  - данные турнира (name/type/status/dates/note)
  - список команд, участвующих (по `tournament_roster`)
  - кнопки:
    - `✏ Редактировать`
    - `👥 Заявки (составы команд)`
    - `🏟 Матчи`
    - `⬅ Назад`

**Wizard «Создать турнир»:**
1) Название → 2) Тип → 3) Статус → 4) Дата начала → 5) Дата окончания → 6) Примечание → Создать.

---

### `/teams`

- Список активных команд с кнопками:
  - «Открыть …»
  - «➕ Создать команду»
- Экран команды:
  - `name / short_code / active / note`
  - `✏ Редактировать` / `⬅ Назад`

**Wizard «Создать команду»:**
1) Название → 2) Короткий код → 3) Активна? → 4) Примечание?

---

### `/players`

- Список игроков (пагинация):
  - «Открыть игрока …»
  - «➡ Далее»
  - «➕ Создать игрока»
- Экран игрока:
  - ФИО, дата рождения, позиция, активность, примечание
  - Список текущих заявок: Турнир → Команда → Номер (из `tournament_roster`)
  - `✏ Редактировать` / `⬅ Назад`

**Wizard «Создать игрока»:**
1) ФИО → 2) Дата рождения (или пропустить) → 3) Позиция (или пропустить) → 4) Примечание (или пропустить).

---

### `/tournament_rosters`

Управление заявками на турнир.

Поток:
1) Выбор турнира → 2) Выбор команды в этом турнире → 3) Экран заявки (список игроков + номера).

Кнопки:
- «➕ Добавить игрока»
- «✏ Изменить номер игрока»
- «🗑 Удалить игрока из заявки»
- «⬅ Назад»

**Добавить игрока:**
- Показать список активных игроков (пагинация) → выбрать → запросить номер (можно пропустить) → создать запись `tournament_roster`.

**Изменить номер игрока:**
- Показать игроков из заявки → выбрать → запросить новый номер → UPDATE `tournament_number`.

**Удалить игрока из заявки:**
- Показать игроков из заявки → выбрать → подтверждение → DELETE.

---

### `/games`

Работа с матчами.

Поток:
1) Выбор турнира → 2) Выбор команды (которая участвует в турнире) → 3) Список матчей этой команды.

Список матчей (пример):
- `ID=91 vs "СКА Ставрополь" 27.10 11:00 — scheduled`
- `ID=92 vs "Академия" 02.11 10:30 — played`
Кнопки:
- «Открыть матч #…»
- «➕ Создать матч»
- «⬅ Назад»

**Экран матча:**
- дата/время, место, статус
- счёт: HT / FT / ET / PEN / Итог
- состав: количество start/sub (и быстрый просмотр)
- события: список с таймштампами
- кнопки:
  - `✏ Редактировать матч` (дата/время/место/статус/ввод счётов)
  - `👥 Состав` (добавить из заявки, изменить роль, изменить номер на матч, удалить)
  - `⚽ События` (добавить goal/card/sub, посмотреть список)
  - `⬅ Назад`

**Создать матч (wizard):**
1) Соперник → 2) Дата → 3) Время → 4) Локация → Создать  
(Перед созданием проверить, что у пары `(tournament_id, team_id)` есть запись(и) в `tournament_roster`).

---

## Схема БД (PostgreSQL)

```sql
-- teams
id          BIGSERIAL PRIMARY KEY,
name        TEXT NOT NULL,
short_code  TEXT NOT NULL UNIQUE,
active      BOOLEAN NOT NULL DEFAULT TRUE,
note        TEXT NULL,
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- players
id          BIGSERIAL PRIMARY KEY,
full_name   TEXT NOT NULL,
birth_date  DATE NULL,
position    TEXT NULL,
active      BOOLEAN NOT NULL DEFAULT TRUE,
note        TEXT NULL,
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- tournaments
id          BIGSERIAL PRIMARY KEY,
name        TEXT NOT NULL,
type        TEXT NULL,
status      TEXT NOT NULL DEFAULT 'active',  -- planned/active/finished
start_date  DATE NULL,
end_date    DATE NULL,
note        TEXT NULL,
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- tournament_roster (заявка на турнир)
id                  BIGSERIAL PRIMARY KEY,
tournament_id       BIGINT NOT NULL REFERENCES tournaments(id),
team_id             BIGINT NOT NULL REFERENCES teams(id),
player_id           BIGINT NOT NULL REFERENCES players(id),
tournament_number   INT NULL,
created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (tournament_id, team_id, player_id);

-- matches
id                  BIGSERIAL PRIMARY KEY,
tournament_id       BIGINT NOT NULL REFERENCES tournaments(id),
team_id             BIGINT NOT NULL REFERENCES teams(id),
opponent_name       TEXT NOT NULL,
start_time          TIMESTAMPTZ NOT NULL,
location            TEXT NULL,
status              TEXT NOT NULL DEFAULT 'scheduled',  -- scheduled/played/canceled
score_ht            TEXT NULL,
score_ft            TEXT NULL,
score_et            TEXT NULL,
score_pen           TEXT NULL,
score_final_us      INT NULL,
score_final_them    INT NULL,
created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- match_lineups
id                  BIGSERIAL PRIMARY KEY,
match_id            BIGINT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
player_id           BIGINT NOT NULL REFERENCES players(id),
role                TEXT NOT NULL,     -- 'start' | 'sub'
number_override     INT NULL,
note                TEXT NULL,
created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
UNIQUE (match_id, player_id);

-- match_events
id                  BIGSERIAL PRIMARY KEY,
match_id            BIGINT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
event_type          TEXT NOT NULL,     -- 'goal' | 'card' | 'sub'
event_time          TEXT NOT NULL,
player_id_main      BIGINT NULL REFERENCES players(id),
player_id_alt       BIGINT NULL REFERENCES players(id),
card_type           TEXT NULL,         -- 'yellow' | 'red'
created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW();
```

---

## Валидация и бизнес-правила

1. Авторизация: все действия доступны **только** админам.
2. Создание матча: у пары `(tournament_id, team_id)` должен быть хотя бы один игрок в `tournament_roster`.
3. Добавление в состав матча: `player_id` обязан присутствовать в `tournament_roster` той же пары.
4. Добавление события: все фигурирующие игроки должны присутствовать в `tournament_roster` пары матча.
5. `status = canceled` у матча → все поля счёта очищаются до `NULL`.
6. Форматы счётов:
   - `score_ht/ft/et/pen` — строка «X:Y» (без парсинга в INT).
   - `score_final_us/score_final_them` — INT.
7. `MatchLineupItem` — `UNIQUE (match_id, player_id)`: повтор — это **update**, а не дублирование.

---

## Нефункциональные требования

- Бот не падает на некорректных апдейтах; ошибки логируются и игнорируются для следующего апдейта.
- Логи в stdout: `timestamp, admin_tg_id, action, entity, entity_id, status`.
- Ответы человеку — краткие и операционные.
- Язык ответов бота — русский.
- Код модульный: сервисы не знают о Telegram; Telegram-слой вызывает сервисы.

---

## Конфигурация и окружение

Переменные окружения:
- `BOT_TOKEN` — токен Telegram-бота
- `DB_DSN` — DSN PostgreSQL (например: `postgres://user:pass@host:5432/db?sslmode=disable`)
- `ADMIN_IDS` — список Telegram user_id через запятую: `12345,67890`

Режимы запуска:
- локально (dev)
- контейнер (Docker) — по желанию

---

## Структура репозитория

```
/cmd/bot/main.go
/internal/config/           # парсинг env, конфиг
/internal/telegram/         # хендлеры, рендер экранов, inline-кнопки, callback_data
/internal/session/          # admin_sessions (state machine, стек навигации)
/internal/models/           # доменные модели
/internal/repository/       # SQL доступ (CRUD)
/internal/service/          # бизнес-логика (Teams/Players/Tournaments/Rosters/Matches/Lineup/Events)
/migrations/                # SQL миграции
/docs/                      # доп. документация, при необходимости
```

**admin_sessions (рекомендация):**
- `admin_tg_id BIGINT`
- `current_flow TEXT`
- `flow_state JSONB`
- `updated_at TIMESTAMPTZ`

---

## Миграции

- Инструмент: `goose` (или аналог), формат файлов: `0001_init.sql`, `0002_...sql` и т.д.
- Миграции обязательны и должны применяться при старте приложения/через Makefile.

---

## Definition of Done

Этап считается выполненным, если админ может, используя **только кнопки**:

- Создавать и редактировать **турниры**.
- Создавать и редактировать **команды**.
- Создавать и редактировать **игроков**.
- Через `/tournament_rosters`: управлять заявками (добавлять/удалять игрока, менять номер).
- Через `/games`: по выбранному турниру и команде
  - создавать матчи,
  - редактировать дату/время/место/статус/счёты (HT/FT/ET/PEN/итог),
  - управлять составом (добавлять из заявки, менять роль, номер на матч, удалять),
  - добавлять события матча типов `goal`, `card (yellow/red)`, `sub`,
  - просматривать список событий.
- Не-админ видит только сообщение об отсутствии прав.
- Логирование в stdout работает.

---

## Документация для Codex

### Callback-паттерны

- Использовать стабильные `callback_data` вида:  
  `action|k1=v1|k2=v2`  
  Примеры:  
  - `open_tournament|id=7`  
  - `open_roster|t=7|team=12`  
  - `add_roster_player|t=7|team=12|page=2`  
  - `open_match|id=91`  
  - `lineup_add|match=91`

- Нужен универсальный парсер `callback_data` → `struct { Action string; Args map[string]string }`.

### State machine / wizard

- Таблица `admin_sessions` для хранения текущего сценария.
- Универсальные хелперы:
  - `StartFlow(adminID, flowName, initialState)`
  - `AdvanceFlow(adminID, userText)`
  - `CancelFlow(adminID)`
- Навигация «⬅ Назад» через стек экранов в `flow_state`:
  - пример структуры: `{"screens":[{...}, {...}]}`

### Сервисы (контракты)

```go
type TeamsService interface {
    ListActive() ([]Team, error)
    Get(id int64) (*Team, error)
    Create(name, shortCode string, active bool, note *string) (int64, error)
    Update(id int64, patch TeamPatch) error
}

type PlayersService interface {
    List(p Page) ([]Player, error)
    Get(id int64) (*Player, error)
    Create(fullName string, birth *time.Time, position *string, active bool, note *string) (int64, error)
    Update(id int64, patch PlayerPatch) error
    ListAssignments(playerID int64) ([]PlayerAssignment, error) // турнир/команда/номер
}

type TournamentsService interface {
    List(status *string) ([]Tournament, error)
    Get(id int64) (*Tournament, error)
    Create(name, typ, status string, start, end *time.Time, note *string) (int64, error)
    Update(id int64, patch TournamentPatch) error
}

type RostersService interface {
    ListTeamsInTournament(tournamentID int64) ([]Team, error)
    ListRoster(tournamentID, teamID int64) ([]RosterRow, error)
    AddPlayer(tournamentID, teamID, playerID int64, number *int) error
    UpdateNumber(tournamentID, teamID, playerID int64, number *int) error
    RemovePlayer(tournamentID, teamID, playerID int64) error
    EnsureTeamHasPlayers(tournamentID, teamID int64) (bool, error)
}

type MatchesService interface {
    List(tournamentID, teamID int64) ([]Match, error)
    Get(id int64) (*Match, error)
    Create(tournamentID, teamID int64, opponent string, start time.Time, location *string) (int64, error)
    Update(id int64, patch MatchPatch) error // дата/время/место/статус/счёты
}

type LineupService interface {
    Get(matchID int64) (Lineup, error)
    Upsert(matchID, playerID int64, role string, numberOverride *int, note *string) error
    Update(matchID, playerID int64, patch LineupPatch) error
    Remove(matchID, playerID int64) error
}

type EventsService interface {
    List(matchID int64) ([]MatchEvent, error)
    AddGoal(matchID, playerID int64, timeText string) error
    AddCard(matchID, playerID int64, cardType, timeText string) error // 'yellow' or 'red'
    AddSub(matchID, playerOutID, playerInID int64, timeText string) error
}
```

**Правила проверок в сервисах:**
- Авторизация — до входа в сервис (в Telegram-слое).
- MatchesService.Create → `EnsureTeamHasPlayers(...)`
- LineupService.Upsert/Update → игрок ∈ `tournament_roster` для пары `(match.tournament_id, match.team_id)`
- EventsService.Add* → все игроки ∈ `tournament_roster` для пары матча
- Update матча со `status=canceled` → очистить все счёты

### Задачи для генерации Codex

1. Сгенерировать миграции SQL для всех таблиц в `/migrations`.  
2. Сгенерировать доменные модели (`/internal/models`).  
3. Сгенерировать репозитории (CRUD) для всех сущностей.  
4. Сгенерировать сервисы и интерфейсы с реализацией проверок.  
5. Сгенерировать Telegram-слой:
   - обработчик `/tournaments`, `/teams`, `/players`, `/tournament_rosters`, `/games`
   - рендер списков и карточек
   - создание inline-кнопок и парсер `callback_data`
   - state machine (wizard + стек экранов «⬅ Назад»)
6. Реализовать `.env` парсер и валидацию настроек (`BOT_TOKEN`, `DB_DSN`, `ADMIN_IDS`).  
7. Написать `Makefile`/скрипты запуска (применение миграций, запуск бота).

---

## Дальнейшие расширения (не в этом этапе)

- Таблица турнира (очки/разница/место) и автообновление от результатов матчей.
- Публичные витрины (просмотр расписания/результатов/таблиц).
- Персональная статистика игрока по турнирам (голы/карточки/минуты).
- Исторические посты, сториз, интеграции с сайтом.
- Конкурс прогнозов.
- Роли «тренер команды», «редактор» с ограничениями по видимости и правам.
- Экспорт CSV/Excel, отчёты, печатные формы составов/протоколов.

---

**Конец документа.**

# 📦 Сводка по деплою

## Что было создано

### GitHub Actions Workflow
- **`.github/workflows/deploy.yml`** — автоматический деплой при push в main
  - Сборка бинарника для Linux
  - Подключение к серверу по SSH
  - Копирование файлов
  - Применение миграций БД
  - Перезапуск systemd сервиса

### Конфигурация сервера
- **`deploy/dynamost-bot.service`** — systemd unit для управления ботом
  - Автозапуск при перезагрузке сервера
  - Автоматический перезапуск при сбое
  - Логирование в systemd journal
  - Изоляция процесса (security hardening)

- **`deploy/setup-server.sh`** — скрипт автоматической настройки сервера
  - Установка PostgreSQL
  - Создание БД и пользователя
  - Установка goose для миграций
  - Создание системного пользователя
  - Настройка директорий

### Документация
- **`docs/DEPLOYMENT.md`** — полное руководство по деплою (400+ строк)
  - Подготовка сервера
  - Настройка БД
  - Настройка GitHub Secrets
  - Управление ботом
  - Мониторинг и логи
  - Troubleshooting
  - Резервное копирование
  - Безопасность

- **`docs/QUICK_DEPLOY.md`** — быстрый старт за 15 минут
  - Пошаговая инструкция
  - Минимум текста
  - Только необходимые команды

- **`DEPLOYMENT_CHECKLIST.md`** — чеклист для деплоя
  - 6 этапов с подробным описанием
  - Проверочные пункты
  - Команды для диагностики

- **`.github/DEPLOY_SECRETS.md`** — справочник по GitHub Secrets
  - Описание всех 7 секретов
  - Инструкции по получению значений
  - Примеры и форматы

### Обновления существующих файлов
- **`README.md`** — добавлен раздел Deployment с ссылками на документацию

---

## Архитектура деплоя

```
┌─────────────────┐
│  GitHub Push    │
│   to main       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ GitHub Actions  │
│  - Build Binary │
│  - Run Tests    │
└────────┬────────┘
         │
         ▼ SSH
┌─────────────────────────┐
│   Ubuntu Server         │
│                         │
│  ┌──────────────────┐   │
│  │ /opt/dynamost-   │   │
│  │ bot/             │   │
│  │  ├─ bot          │◄──┼── Binary
│  │  ├─ migrations/  │◄──┼── SQL files
│  │  └─ .env         │◄──┼── Secrets
│  └──────────────────┘   │
│           │              │
│           ▼              │
│  ┌──────────────────┐   │
│  │ goose migrate    │   │
│  │  up              │   │
│  └──────────────────┘   │
│           │              │
│           ▼              │
│  ┌──────────────────┐   │
│  │ systemd          │   │
│  │ restart service  │   │
│  └──────────────────┘   │
│           │              │
│           ▼              │
│  ┌──────────────────┐   │
│  │   PostgreSQL     │   │
│  │   Database       │   │
│  └──────────────────┘   │
│           │              │
│           ▼              │
│  ┌──────────────────┐   │
│  │ Telegram Bot API │   │
│  └──────────────────┘   │
└─────────────────────────┘
```

---

## Требуемые GitHub Secrets

| # | Секрет | Описание | Источник |
|---|--------|----------|----------|
| 1 | `SSH_PRIVATE_KEY` | Приватный SSH ключ | `~/.ssh/github_deploy` |
| 2 | `SERVER_HOST` | IP/домен сервера | Хостинг провайдер |
| 3 | `SERVER_USER` | SSH пользователь | `ubuntu` или `root` |
| 4 | `BOT_TOKEN` | Telegram токен | @BotFather |
| 5 | `DB_DSN` | PostgreSQL DSN | setup-server.sh |
| 6 | `ADMIN_IDS` | ID администраторов | @userinfobot |
| 7 | `CLUB_TZ` | Временная зона | `Europe/Moscow` |

---

## Процесс деплоя (автоматический)

### 1. Push в main
```bash
git push origin main
```

### 2. GitHub Actions выполняет:
- ✅ Checkout кода
- ✅ Установка Go 1.25.1
- ✅ Сборка бинарника (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`)
- ✅ Настройка SSH ключа
- ✅ Копирование файлов на сервер
- ✅ Создание `.env` файла
- ✅ Установка goose (если нужно)
- ✅ Применение миграций (`goose up`)
- ✅ Копирование systemd service
- ✅ Перезапуск сервиса (`systemctl restart`)
- ✅ Проверка статуса

### 3. Результат
Бот обновлен и работает на сервере!

---

## Управление ботом

### Основные команды

```bash
# Статус
sudo systemctl status dynamost-bot

# Запуск
sudo systemctl start dynamost-bot

# Остановка
sudo systemctl stop dynamost-bot

# Перезапуск
sudo systemctl restart dynamost-bot

# Логи (последние 50 строк)
sudo journalctl -u dynamost-bot -n 50

# Логи в реальном времени
sudo journalctl -u dynamost-bot -f

# Автозапуск при загрузке
sudo systemctl enable dynamost-bot
```

### Работа с миграциями

```bash
# Переход в директорию
cd /opt/dynamost-bot

# Загрузка переменных окружения
source .env

# Статус миграций
goose -dir ./migrations postgres "$DB_DSN" status

# Применить миграции
goose -dir ./migrations postgres "$DB_DSN" up

# Откатить последнюю миграцию
goose -dir ./migrations postgres "$DB_DSN" down

# Откатить до версии
goose -dir ./migrations postgres "$DB_DSN" down-to 3
```

---

## Файловая структура на сервере

```
/opt/dynamost-bot/
├── bot                  # Бинарный файл бота
├── .env                 # Переменные окружения (секреты)
└── migrations/          # SQL миграции
    ├── 0001_init_teams.sql
    ├── 0002_init_players.sql
    ├── 0003_init_tournaments.sql
    ├── 0004_init_tournament_roster.sql
    ├── 0005_init_matches.sql
    ├── 0006_init_match_lineups.sql
    ├── 0007_init_match_events.sql
    └── 0008_init_admin_sessions.sql

/etc/systemd/system/
└── dynamost-bot.service # Systemd unit file
```

---

## Мониторинг

### Проверка работы бота

```bash
# Статус сервиса
sudo systemctl status dynamost-bot

# Проверка процесса
ps aux | grep dynamost-bot

# Использование ресурсов
top -p $(pgrep -f dynamost-bot)

# Проверка подключения к БД
psql "$DB_DSN" -c "SELECT COUNT(*) FROM teams;"

# Проверка версии миграций
psql "$DB_DSN" -c "SELECT * FROM goose_db_version ORDER BY id DESC LIMIT 5;"
```

### Логи

```bash
# Последние ошибки
sudo journalctl -u dynamost-bot -p err -n 20

# Логи за сегодня
sudo journalctl -u dynamost-bot --since today

# Логи за последний час
sudo journalctl -u dynamost-bot --since "1 hour ago"

# Поиск по тексту
sudo journalctl -u dynamost-bot | grep "error"

# Экспорт логов
sudo journalctl -u dynamost-bot -o json > logs.json
```

---

## Резервное копирование

### Ручной бэкап БД

```bash
# Создание дампа
pg_dump "$DB_DSN" > backup_$(date +%Y%m%d_%H%M%S).sql

# Создание сжатого дампа
pg_dump "$DB_DSN" | gzip > backup_$(date +%Y%m%d_%H%M%S).sql.gz
```

### Автоматический бэкап (cron)

```bash
# Создайте директорию для бэкапов
sudo mkdir -p /backups
sudo chown dynamost:dynamost /backups

# Добавьте в crontab
crontab -e

# Ежедневный бэкап в 3:00
0 3 * * * pg_dump "postgres://..." | gzip > /backups/db_$(date +\%Y\%m\%d).sql.gz

# Очистка старых бэкапов (старше 30 дней)
0 4 * * * find /backups -name "db_*.sql.gz" -mtime +30 -delete
```

---

## Безопасность

### Настройка firewall

```bash
# Разрешить SSH
sudo ufw allow 22/tcp

# Включить firewall
sudo ufw enable

# Проверка статуса
sudo ufw status
```

### Обновление системы

```bash
# Обновление пакетов
sudo apt update && sudo apt upgrade -y

# Автоматические обновления безопасности
sudo apt install unattended-upgrades
sudo dpkg-reconfigure -plow unattended-upgrades
```

### Ротация логов

```bash
# Настройка journald (если нужно)
sudo vi /etc/systemd/journald.conf

# Ограничить размер логов
SystemMaxUse=500M
MaxRetentionSec=7day

# Перезапуск journald
sudo systemctl restart systemd-journald
```

---

## Частые проблемы и решения

### Бот не запускается

```bash
# Проверьте логи
sudo journalctl -u dynamost-bot -n 100

# Проверьте .env
cat /opt/dynamost-bot/.env

# Проверьте права
ls -la /opt/dynamost-bot/bot

# Попробуйте запустить вручную
cd /opt/dynamost-bot
source .env
./bot
```

### Ошибка подключения к БД

```bash
# Проверьте PostgreSQL
sudo systemctl status postgresql

# Проверьте подключение
psql "$DB_DSN" -c "SELECT 1;"

# Проверьте логи PostgreSQL
sudo journalctl -u postgresql -n 50
```

### Миграции не применяются

```bash
# Проверьте наличие goose
which goose

# Проверьте статус миграций
cd /opt/dynamost-bot
source .env
goose -dir ./migrations postgres "$DB_DSN" status

# Примените вручную
goose -dir ./migrations postgres "$DB_DSN" up -v
```

---

## Следующие шаги

После успешного деплоя:

1. ✅ Настройте автоматические бэкапы БД
2. ✅ Настройте мониторинг (например, uptimerobot.com)
3. ✅ Настройте алерты при падении бота
4. ✅ Документируйте изменения в конфигурации
5. ✅ Регулярно обновляйте систему
6. ✅ Периодически проверяйте логи
7. ✅ Тестируйте восстановление из бэкапов

---

## Ссылки на документацию

- 📖 [DEPLOYMENT.md](./DEPLOYMENT.md) — полное руководство
- 🚀 [QUICK_DEPLOY.md](./QUICK_DEPLOY.md) — быстрый старт
- ✅ [DEPLOYMENT_CHECKLIST.md](../DEPLOYMENT_CHECKLIST.md) — чеклист
- 🔐 [DEPLOY_SECRETS.md](../.github/DEPLOY_SECRETS.md) — описание секретов

---

**Удачи с деплоем! ⚽️🤖**


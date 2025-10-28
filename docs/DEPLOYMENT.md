# Инструкция по деплою на Ubuntu сервер

Данная инструкция описывает процесс автоматического деплоя Telegram-бота Dynamost на сервер Ubuntu с использованием GitHub Actions.

## Оглавление

1. [Подготовка сервера](#подготовка-сервера)
2. [Настройка базы данных PostgreSQL](#настройка-базы-данных-postgresql)
3. [Настройка GitHub Secrets](#настройка-github-secrets)
4. [Процесс деплоя](#процесс-деплоя)
5. [Управление ботом](#управление-ботом)
6. [Мониторинг и логи](#мониторинг-и-логи)
7. [Откат изменений](#откат-изменений)

---

## Подготовка сервера

### Требования

- Ubuntu 20.04 LTS или новее
- Минимум 1GB RAM
- Минимум 10GB свободного места на диске
- Права sudo
- Открытый SSH доступ

### Автоматическая настройка

На сервере выполните скрипт автоматической настройки:

```bash
# Скачайте скрипт на сервер
wget https://raw.githubusercontent.com/dynamost/telegram-bot/main/deploy/setup-server.sh

# Дайте права на выполнение
chmod +x setup-server.sh

# Запустите с правами sudo
sudo ./setup-server.sh
```

Скрипт автоматически:
- Обновит систему
- Установит PostgreSQL
- Создаст системного пользователя `dynamost`
- Установит инструмент миграций `goose`
- Создаст базу данных и пользователя
- Выведет строку подключения DB_DSN

### Ручная настройка (альтернатива)

Если вы предпочитаете ручную настройку:

```bash
# Обновление системы
sudo apt update && sudo apt upgrade -y

# Установка PostgreSQL
sudo apt install -y postgresql postgresql-contrib

# Создание пользователя для бота
sudo useradd -r -s /bin/bash -d /opt/dynamost-bot dynamost
sudo mkdir -p /opt/dynamost-bot
sudo chown dynamost:dynamost /opt/dynamost-bot

# Установка goose
curl -fsSL https://raw.githubusercontent.com/pressly/goose/master/install.sh | sh
sudo mv ./bin/goose /usr/local/bin/
```

---

## Настройка базы данных PostgreSQL

### Создание базы данных и пользователя

#### Важно: Как работает аутентификация PostgreSQL на Ubuntu

При установке PostgreSQL на Ubuntu пароль для пользователя `postgres` **не устанавливается**. Используется аутентификация `peer` — доступ только через `sudo -u postgres psql`.

Для бота нужно создать **отдельного пользователя** с паролем:

```bash
# Подключитесь к PostgreSQL от имени postgres (пароль не нужен)
sudo -u postgres psql

# В консоли PostgreSQL выполните:
CREATE USER dynamost WITH PASSWORD 'your_secure_password';
CREATE DATABASE dynamost_bot OWNER dynamost;
GRANT ALL PRIVILEGES ON DATABASE dynamost_bot TO dynamost;

# Подключитесь к созданной БД
\c dynamost_bot

# Выдайте права на схему
GRANT ALL ON SCHEMA public TO dynamost;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO dynamost;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO dynamost;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO dynamost;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO dynamost;

# Выход
\q
```

#### Если забыли пароль пользователя БД

Сбросьте пароль:

```bash
sudo -u postgres psql -c "ALTER USER dynamost WITH PASSWORD 'новый_пароль';"
```

#### Проверка существующих пользователей

```bash
# Список пользователей
sudo -u postgres psql -c "\du"

# Список баз данных
sudo -u postgres psql -c "\l"
```

### Строка подключения

После создания БД сформируйте строку подключения в формате:

```
postgres://dynamost:your_secure_password@localhost:5432/dynamost_bot?sslmode=disable
```

Эта строка понадобится для `DB_DSN` в GitHub Secrets.

### Проверка подключения

Проверьте подключение к базе данных:

```bash
psql "postgres://dynamost:your_secure_password@localhost:5432/dynamost_bot?sslmode=disable" -c "SELECT version();"
```

---

## Настройка GitHub Secrets

Добавьте следующие секреты в настройках репозитория GitHub:

**Settings → Secrets and variables → Actions → New repository secret**

### Обязательные секреты

| Секрет | Описание | Пример |
|--------|----------|--------|
| `SSH_PRIVATE_KEY` | Приватный SSH ключ для деплоя | Содержимое файла `~/.ssh/id_ed25519` |
| `SERVER_HOST` | IP адрес или домен сервера | `123.45.67.89` или `bot.example.com` |
| `SERVER_USER` | Имя пользователя для SSH | `ubuntu` или `root` |
| `BOT_TOKEN` | Токен Telegram бота | `1234567890:ABCdefGHIjklMNOpqrsTUVwxyz` |
| `DB_DSN` | Строка подключения к PostgreSQL | `postgres://dynamost:password@localhost:5432/dynamost_bot?sslmode=disable` |
| `ADMIN_IDS` | ID администраторов через запятую | `123456789,987654321` |
| `CLUB_TZ` | Временная зона клуба | `Europe/Moscow` или `UTC` |

### Получение значений секретов

#### 1. SSH_PRIVATE_KEY

Сгенерируйте SSH ключ на вашей локальной машине:

```bash
# Генерация ключа
ssh-keygen -t ed25519 -C "github-deploy" -f ~/.ssh/github_deploy

# Вывод приватного ключа (скопируйте весь вывод)
cat ~/.ssh/github_deploy

# Вывод публичного ключа
cat ~/.ssh/github_deploy.pub
```

Добавьте **публичный** ключ на сервер:

```bash
# На сервере
echo "ssh-ed25519 AAAA... github-deploy" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

В GitHub Secret добавьте содержимое **приватного** ключа (включая строки `-----BEGIN` и `-----END`).

#### 2. BOT_TOKEN

Получите токен у [@BotFather](https://t.me/BotFather):
1. Отправьте `/newbot` или `/token` (для существующего бота)
2. Скопируйте полученный токен

#### 3. ADMIN_IDS

Узнайте свой Telegram ID:
1. Напишите боту [@userinfobot](https://t.me/userinfobot)
2. Скопируйте значение `Id`
3. Для нескольких администраторов перечислите через запятую: `123456789,987654321`

#### 4. CLUB_TZ

Список временных зон: [en.wikipedia.org/wiki/List_of_tz_database_time_zones](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones)

Для России:
- Москва: `Europe/Moscow` (UTC+3)
- Санкт-Петербург: `Europe/Moscow` (UTC+3)
- Екатеринбург: `Asia/Yekaterinburg` (UTC+5)
- Новосибирск: `Asia/Novosibirsk` (UTC+7)
- Владивосток: `Asia/Vladivostok` (UTC+10)

---

## Процесс деплоя

### Автоматический деплой

После настройки GitHub Secrets деплой происходит автоматически:

1. **При push в ветку `main`:**
   ```bash
   git push origin main
   ```

2. **Ручной запуск через GitHub Actions:**
   - Перейдите в раздел **Actions** репозитория
   - Выберите workflow **"Deploy to Ubuntu Server"**
   - Нажмите **"Run workflow"**

### Что происходит при деплое

1. Сборка бинарника для Linux
2. Подключение к серверу по SSH
3. Копирование файлов:
   - Бинарник бота → `/opt/dynamost-bot/bot`
   - Миграции → `/opt/dynamost-bot/migrations/`
   - Systemd service → `/etc/systemd/system/dynamost-bot.service`
4. Создание `.env` файла с секретами
5. Установка goose (если отсутствует)
6. **Применение миграций БД**
7. Перезапуск сервиса
8. Проверка статуса

---

## Управление ботом

### Systemd команды

```bash
# Запуск бота
sudo systemctl start dynamost-bot

# Остановка бота
sudo systemctl stop dynamost-bot

# Перезапуск бота
sudo systemctl restart dynamost-bot

# Проверка статуса
sudo systemctl status dynamost-bot

# Включить автозапуск при загрузке системы
sudo systemctl enable dynamost-bot

# Отключить автозапуск
sudo systemctl disable dynamost-bot
```

---

## Мониторинг и логи

### Просмотр логов

```bash
# Последние 50 строк логов
sudo journalctl -u dynamost-bot -n 50

# Логи в реальном времени
sudo journalctl -u dynamost-bot -f

# Логи за сегодня
sudo journalctl -u dynamost-bot --since today

# Логи за последний час
sudo journalctl -u dynamost-bot --since "1 hour ago"

# Поиск ошибок
sudo journalctl -u dynamost-bot -p err
```

### Проверка работы бота

1. Напишите вашему боту в Telegram
2. Отправьте команду `/start`
3. Проверьте, что бот отвечает

### Проверка базы данных

```bash
# Подключение к БД
psql "postgres://dynamost:password@localhost:5432/dynamost_bot"

# Список таблиц
\dt

# Проверка версии миграций
SELECT * FROM goose_db_version;

# Выход
\q
```

---

## Откат изменений

### Откат миграций

Если нужно откатить последнюю миграцию:

```bash
# На сервере
cd /opt/dynamost-bot
source .env
goose -dir ./migrations postgres "$DB_DSN" down
```

### Откат к предыдущей версии бота

1. Найдите нужный коммит в GitHub
2. Создайте новую ветку от этого коммита
3. Сделайте push в `main` или запустите workflow вручную

Или на сервере вручную:

```bash
# Остановите бота
sudo systemctl stop dynamost-bot

# Замените бинарник на резервную копию
sudo cp /opt/dynamost-bot/bot.backup /opt/dynamost-bot/bot

# Запустите бота
sudo systemctl start dynamost-bot
```

---

## Troubleshooting

### Бот не запускается

Проверьте логи:
```bash
sudo journalctl -u dynamost-bot -n 100
```

Частые причины:
- Неверный `BOT_TOKEN` — проверьте секреты в GitHub
- Не удается подключиться к БД — проверьте `DB_DSN` и доступность PostgreSQL
- Не применены миграции — запустите `make migrate-up` вручную

### Ошибка подключения к БД

```bash
# Проверьте статус PostgreSQL
sudo systemctl status postgresql

# Проверьте подключение
psql "$DB_DSN" -c "SELECT 1;"

# Проверьте логи PostgreSQL
sudo journalctl -u postgresql -n 50
```

### Ошибка SSH при деплое

- Убедитесь, что публичный SSH ключ добавлен в `~/.ssh/authorized_keys` на сервере
- Проверьте права на файл: `chmod 600 ~/.ssh/authorized_keys`
- Убедитесь, что в GitHub Secret добавлен полный приватный ключ

### Миграции не применяются

```bash
# Проверьте версию миграций
cd /opt/dynamost-bot
source .env
goose -dir ./migrations postgres "$DB_DSN" status

# Примените миграции вручную
goose -dir ./migrations postgres "$DB_DSN" up
```

---

## Безопасность

1. **Никогда не коммитьте** файл `.env` в репозиторий
2. Используйте **сильные пароли** для БД
3. Настройте **firewall** на сервере:
   ```bash
   sudo ufw allow 22/tcp   # SSH
   sudo ufw allow 5432/tcp # PostgreSQL (только если нужен внешний доступ)
   sudo ufw enable
   ```
4. Регулярно обновляйте систему:
   ```bash
   sudo apt update && sudo apt upgrade -y
   ```
5. Настройте автоматические резервные копии БД

---

## Резервное копирование

### Создание бэкапа БД

```bash
# Создание дампа
pg_dump "postgres://dynamost:password@localhost:5432/dynamost_bot" > backup_$(date +%Y%m%d_%H%M%S).sql

# Или с сжатием
pg_dump "postgres://dynamost:password@localhost:5432/dynamost_bot" | gzip > backup_$(date +%Y%m%d_%H%M%S).sql.gz
```

### Восстановление из бэкапа

```bash
# Из обычного дампа
psql "postgres://dynamost:password@localhost:5432/dynamost_bot" < backup_20241028_120000.sql

# Из сжатого дампа
gunzip -c backup_20241028_120000.sql.gz | psql "postgres://dynamost:password@localhost:5432/dynamost_bot"
```

### Автоматические бэкапы

Создайте cron задание:

```bash
# Редактируйте crontab
crontab -e

# Добавьте строку для ежедневного бэкапа в 3:00
0 3 * * * pg_dump "postgres://dynamost:password@localhost:5432/dynamost_bot" | gzip > /backups/dynamost_$(date +\%Y\%m\%d).sql.gz
```

---

## Контакты и поддержка

При возникновении проблем:
1. Проверьте раздел [Troubleshooting](#troubleshooting)
2. Изучите логи бота: `sudo journalctl -u dynamost-bot -n 100`
3. Создайте Issue в GitHub репозитории

---

**Удачного деплоя! ⚽️**


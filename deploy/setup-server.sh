#!/bin/bash

# Скрипт для первоначальной настройки сервера Ubuntu
# Запускать с правами sudo

set -e

echo "=== Настройка сервера для Dynamost Telegram Bot ==="

# Обновление системы
echo "Обновление системы..."
apt update && apt upgrade -y

# Установка PostgreSQL
echo "Установка PostgreSQL..."
apt install -y postgresql postgresql-contrib

# Запуск PostgreSQL
systemctl start postgresql
systemctl enable postgresql

# Создание пользователя для бота
echo "Создание системного пользователя dynamost..."
if ! id -u dynamost >/dev/null 2>&1; then
    useradd -r -s /bin/bash -d /opt/dynamost-bot dynamost
fi

# Создание директории для бота
mkdir -p /opt/dynamost-bot
chown dynamost:dynamost /opt/dynamost-bot

# Установка goose для миграций
echo "Установка goose..."
if ! command -v goose &> /dev/null; then
    curl -fsSL https://raw.githubusercontent.com/pressly/goose/master/install.sh | sh
    mv ./bin/goose /usr/local/bin/
    chmod +x /usr/local/bin/goose
fi

echo ""
echo "=== Настройка PostgreSQL ==="
echo "Создание базы данных и пользователя..."

# Запрос учетных данных для БД
read -p "Введите имя базы данных [dynamost_bot]: " DB_NAME
DB_NAME=${DB_NAME:-dynamost_bot}

read -p "Введите имя пользователя БД [dynamost]: " DB_USER
DB_USER=${DB_USER:-dynamost}

read -sp "Введите пароль для пользователя БД: " DB_PASSWORD
echo ""

# Создание БД и пользователя
sudo -u postgres psql << EOF
-- Создание пользователя
CREATE USER $DB_USER WITH PASSWORD '$DB_PASSWORD';

-- Создание базы данных
CREATE DATABASE $DB_NAME OWNER $DB_USER;

-- Выдача прав
GRANT ALL PRIVILEGES ON DATABASE $DB_NAME TO $DB_USER;

-- Подключение к БД и выдача прав на схему public
\c $DB_NAME
GRANT ALL ON SCHEMA public TO $DB_USER;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO $DB_USER;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO $DB_USER;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO $DB_USER;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO $DB_USER;
EOF

echo ""
echo "✅ База данных настроена!"
echo ""
echo "DB_DSN строка подключения:"
echo "postgres://$DB_USER:$DB_PASSWORD@localhost:5432/$DB_NAME?sslmode=disable"
echo ""
echo "=== Настройка SSH ключа для деплоя ==="
echo "Добавьте публичный SSH ключ для пользователя, под которым будет проходить деплой:"
echo "1. На локальной машине сгенерируйте ключ (если нет): ssh-keygen -t ed25519 -C 'deploy-key'"
echo "2. Скопируйте публичный ключ: cat ~/.ssh/id_ed25519.pub"
echo "3. Добавьте его в ~/.ssh/authorized_keys на сервере для нужного пользователя"
echo ""
echo "=== Следующие шаги ==="
echo "1. Добавьте следующие секреты в GitHub Secrets:"
echo "   - SSH_PRIVATE_KEY: приватный SSH ключ для деплоя"
echo "   - SERVER_HOST: IP адрес или домен сервера"
echo "   - SERVER_USER: имя пользователя для SSH подключения"
echo "   - BOT_TOKEN: токен Telegram бота от @BotFather"
echo "   - DB_DSN: строка подключения к БД (см. выше)"
echo "   - ADMIN_IDS: ID администраторов через запятую (например: 123456789,987654321)"
echo "   - CLUB_TZ: временная зона (например: Europe/Moscow)"
echo ""
echo "2. Сделайте push в ветку main для запуска автоматического деплоя"
echo ""
echo "✅ Настройка сервера завершена!"


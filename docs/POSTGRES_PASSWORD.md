# 🔐 Работа с паролями PostgreSQL на Ubuntu

## Важно понять:

**PostgreSQL на Ubuntu использует аутентификацию `peer`** — доступ к пользователю `postgres` через `sudo` без пароля.

Для приложений нужен **отдельный пользователь с паролем**.

---

## 🚀 Быстрые команды

### Создать пользователя и БД с паролем

```bash
sudo -u postgres psql << EOF
CREATE USER dynamost WITH PASSWORD 'ваш_надежный_пароль';
CREATE DATABASE dynamost_bot OWNER dynamost;
GRANT ALL PRIVILEGES ON DATABASE dynamost_bot TO dynamost;
\c dynamost_bot
GRANT ALL ON SCHEMA public TO dynamost;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO dynamost;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO dynamost;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO dynamost;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO dynamost;
EOF
```

Ваша `DB_DSN`:
```
postgres://dynamost:ваш_надежный_пароль@localhost:5432/dynamost_bot?sslmode=disable
```

---

### Сбросить пароль существующего пользователя

```bash
sudo -u postgres psql -c "ALTER USER dynamost WITH PASSWORD 'новый_пароль';"
```

---

### Узнать каких пользователей и БД

```bash
# Список пользователей
sudo -u postgres psql -c "\du"

# Список баз данных
sudo -u postgres psql -c "\l"

# Подробная информация о пользователе
sudo -u postgres psql -c "SELECT usename, usecreatedb, usesuper FROM pg_user WHERE usename = 'dynamost';"
```

---

### Проверить подключение с паролем

```bash
# Попробуйте подключиться
psql "postgres://dynamost:ваш_пароль@localhost:5432/dynamost_bot" -c "SELECT version();"

# Или так
PGPASSWORD='ваш_пароль' psql -h localhost -U dynamost -d dynamost_bot -c "SELECT 1;"
```

Если подключение успешно — пароль правильный! ✅

---

## 📋 Пошаговая инструкция

### Шаг 1: Подключитесь к PostgreSQL

```bash
sudo -u postgres psql
```

Вы увидите приглашение:
```
postgres=#
```

### Шаг 2: Создайте пользователя

```sql
CREATE USER dynamost WITH PASSWORD 'МойСуперПароль123!';
```

### Шаг 3: Создайте базу данных

```sql
CREATE DATABASE dynamost_bot OWNER dynamost;
```

### Шаг 4: Выдайте права

```sql
GRANT ALL PRIVILEGES ON DATABASE dynamost_bot TO dynamost;
```

### Шаг 5: Подключитесь к БД

```sql
\c dynamost_bot
```

### Шаг 6: Выдайте права на схему

```sql
GRANT ALL ON SCHEMA public TO dynamost;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO dynamost;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO dynamost;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO dynamost;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO dynamost;
```

### Шаг 7: Выйдите

```sql
\q
```

### Шаг 8: Сформируйте DB_DSN

```
postgres://dynamost:МойСуперПароль123!@localhost:5432/dynamost_bot?sslmode=disable
```

---

## 🔍 Если пароль был установлен, но вы его забыли

### Вариант 1: Сбросить пароль

```bash
sudo -u postgres psql -c "ALTER USER dynamost WITH PASSWORD 'новый_пароль';"
```

### Вариант 2: Удалить и создать заново

```bash
# Подключитесь
sudo -u postgres psql

# Удалите пользователя (если нет активных подключений)
DROP USER IF EXISTS dynamost;

# Удалите БД (ОСТОРОЖНО! Все данные будут потеряны)
DROP DATABASE IF EXISTS dynamost_bot;

# Создайте заново (см. шаги выше)
```

---

## 🛡️ Безопасность паролей

### Правила надежного пароля:

- ✅ Минимум 16 символов
- ✅ Заглавные и строчные буквы
- ✅ Цифры
- ✅ Специальные символы (`!@#$%^&*`)
- ❌ Не используйте словарные слова
- ❌ Не используйте `password`, `123456`, `admin`

### Генерация надежного пароля:

```bash
# Вариант 1: openssl
openssl rand -base64 32

# Вариант 2: pwgen (нужно установить)
sudo apt install pwgen
pwgen 32 1

# Вариант 3: /dev/urandom
tr -dc 'A-Za-z0-9!@#$%^&*' < /dev/urandom | head -c 32; echo
```

---

## 📖 Конфигурация аутентификации PostgreSQL

PostgreSQL использует файл `pg_hba.conf` для настройки методов аутентификации:

```bash
# Посмотреть текущие настройки
sudo cat /etc/postgresql/*/main/pg_hba.conf | grep -v "^#" | grep -v "^$"
```

Типичный вывод:
```
local   all             postgres                                peer
local   all             all                                     peer
host    all             all             127.0.0.1/32            scram-sha-256
host    all             all             ::1/128                 scram-sha-256
```

**Что это значит:**
- `local ... peer` — локальное подключение без пароля (только через sudo)
- `host ... scram-sha-256` — TCP подключение с паролем (шифрование)

**Для работы бота достаточно настроек по умолчанию!**

---

## 🔧 Устранение проблем

### Ошибка: "password authentication failed"

```bash
# Проверьте существует ли пользователь
sudo -u postgres psql -c "\du dynamost"

# Сбросьте пароль
sudo -u postgres psql -c "ALTER USER dynamost WITH PASSWORD 'новый_пароль';"

# Попробуйте подключиться снова
psql "postgres://dynamost:новый_пароль@localhost:5432/dynamost_bot" -c "SELECT 1;"
```

### Ошибка: "database does not exist"

```bash
# Проверьте список БД
sudo -u postgres psql -c "\l"

# Создайте БД
sudo -u postgres psql -c "CREATE DATABASE dynamost_bot OWNER dynamost;"
```

### Ошибка: "role does not exist"

```bash
# Создайте пользователя
sudo -u postgres psql -c "CREATE USER dynamost WITH PASSWORD 'ваш_пароль';"
```

### Ошибка: "connection refused"

```bash
# Проверьте статус PostgreSQL
sudo systemctl status postgresql

# Запустите если остановлен
sudo systemctl start postgresql

# Проверьте что слушает на порту 5432
sudo ss -tlnp | grep 5432
```

---

## 💡 Полезные SQL команды

```sql
-- Посмотреть текущего пользователя
SELECT current_user;

-- Посмотреть все БД
SELECT datname FROM pg_database;

-- Посмотреть все таблицы в текущей БД
\dt

-- Посмотреть права пользователя
\du dynamost

-- Посмотреть версию PostgreSQL
SELECT version();

-- Посмотреть активные подключения
SELECT * FROM pg_stat_activity;

-- Посмотреть размер БД
SELECT pg_size_pretty(pg_database_size('dynamost_bot'));
```

---

## 📝 Чеклист для настройки БД

- [ ] PostgreSQL установлен и запущен
- [ ] Создан пользователь `dynamost` с паролем
- [ ] Создана БД `dynamost_bot`
- [ ] Выданы все необходимые права
- [ ] Подключение с паролем работает
- [ ] Сформирована строка `DB_DSN`
- [ ] `DB_DSN` добавлена в GitHub Secrets
- [ ] Миграции применены успешно

---

## 🚀 Автоматизация (рекомендуется)

Используйте готовый скрипт:

```bash
sudo ./deploy/setup-server.sh
```

Скрипт автоматически:
- ✅ Установит PostgreSQL
- ✅ Создаст пользователя с вашим паролем
- ✅ Создаст БД
- ✅ Выдаст все права
- ✅ Выведет готовую строку `DB_DSN`

---

**Готово! Теперь у вас есть полное понимание работы с паролями PostgreSQL.** 🎉


# ✅ Чеклист деплоя на Ubuntu сервер

## 📋 Что было создано

В репозитории добавлены следующие файлы:

- ✅ `.github/workflows/deploy.yml` — GitHub Actions workflow для автоматического деплоя
- ✅ `deploy/dynamost-bot.service` — Systemd сервис для управления ботом
- ✅ `deploy/setup-server.sh` — Скрипт автоматической настройки сервера
- ✅ `docs/DEPLOYMENT.md` — Полная документация по деплою
- ✅ `docs/QUICK_DEPLOY.md` — Быстрый старт за 15 минут
- ✅ Обновлен `README.md` с информацией о деплое

---

## 🚀 Пошаговая инструкция

### Этап 1: Подготовка сервера (10 минут)

#### 1.1. Подключитесь к серверу Ubuntu

```bash
ssh user@your-server-ip
```

#### 1.2. Запустите скрипт автоматической настройки

```bash
# Скачайте скрипт
wget https://raw.githubusercontent.com/dynamost/telegram-bot/main/deploy/setup-server.sh

# Дайте права на выполнение
chmod +x setup-server.sh

# Запустите с правами sudo
sudo ./setup-server.sh
```

#### 1.3. Сохраните вывод скрипта

Скрипт выведет:
- ✅ Строку подключения к БД (`DB_DSN`) — **сохраните её!**
- ✅ Инструкции по настройке SSH ключа

---

### Этап 2: Настройка SSH ключа (5 минут)

#### 2.1. На локальной машине сгенерируйте ключ

```bash
ssh-keygen -t ed25519 -C "github-deploy" -f ~/.ssh/github_deploy
```

#### 2.2. Скопируйте приватный ключ

```bash
cat ~/.ssh/github_deploy
```

**Скопируйте ВСЁ содержимое** (включая строки `-----BEGIN` и `-----END`).

#### 2.3. Скопируйте публичный ключ

```bash
cat ~/.ssh/github_deploy.pub
```

#### 2.4. На сервере добавьте публичный ключ

```bash
echo "ВАШ_ПУБЛИЧНЫЙ_КЛЮЧ_ЗДЕСЬ" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

#### 2.5. Проверьте подключение

```bash
ssh -i ~/.ssh/github_deploy user@your-server-ip
```

Если подключение успешно — переходите к следующему этапу.

---

### Этап 3: Получение данных (5 минут)

Соберите следующие данные:

#### 3.1. BOT_TOKEN

1. Откройте [@BotFather](https://t.me/BotFather) в Telegram
2. Отправьте `/newbot` (для нового бота) или `/token` (для существующего)
3. Скопируйте токен (формат: `1234567890:ABCdefGHIjklMNOpqrsTUVwxyz`)

#### 3.2. ADMIN_IDS

1. Откройте [@userinfobot](https://t.me/userinfobot) в Telegram
2. Отправьте любое сообщение
3. Скопируйте ваш `Id` (например: `123456789`)
4. Для нескольких админов перечислите через запятую: `123456789,987654321`

#### 3.3. DB_DSN

Из вывода скрипта `setup-server.sh` (этап 1.3)

Формат: `postgres://dynamost:password@localhost:5432/dynamost_bot?sslmode=disable`

#### 3.4. Остальные данные

| Параметр | Значение |
|----------|----------|
| `SSH_PRIVATE_KEY` | Содержимое `~/.ssh/github_deploy` |
| `SERVER_HOST` | IP адрес вашего сервера |
| `SERVER_USER` | Имя пользователя для SSH (например: `ubuntu`) |
| `CLUB_TZ` | Временная зона (например: `Europe/Moscow`) |

---

### Этап 4: Настройка GitHub Secrets (5 минут)

#### 4.1. Откройте настройки репозитория

На GitHub перейдите в:
```
Settings → Secrets and variables → Actions
```

#### 4.2. Добавьте секреты

Нажмите **New repository secret** и добавьте **все 7 секретов**:

1. **SSH_PRIVATE_KEY**
   - Значение: содержимое файла `~/.ssh/github_deploy`
   - Включая `-----BEGIN OPENSSH PRIVATE KEY-----` и `-----END OPENSSH PRIVATE KEY-----`

2. **SERVER_HOST**
   - Значение: IP адрес сервера (например: `123.45.67.89`)

3. **SERVER_USER**
   - Значение: имя пользователя SSH (например: `ubuntu`)

4. **BOT_TOKEN**
   - Значение: токен от @BotFather

5. **DB_DSN**
   - Значение: строка подключения к PostgreSQL

6. **ADMIN_IDS**
   - Значение: ID администраторов через запятую (например: `123456789,987654321`)

7. **CLUB_TZ**
   - Значение: временная зона (например: `Europe/Moscow`)

---

### Этап 5: Первый деплой (5 минут)

#### 5.1. Сделайте коммит

```bash
git add .
git commit -m "feat: setup deployment infrastructure"
git push origin main
```

#### 5.2. Отследите процесс деплоя

1. Перейдите на GitHub → **Actions**
2. Выберите workflow **Deploy to Ubuntu Server**
3. Дождитесь завершения (зеленая галочка ✅)

#### 5.3. Проверьте деплой

На сервере:

```bash
# Проверьте статус бота
sudo systemctl status dynamost-bot

# Посмотрите логи
sudo journalctl -u dynamost-bot -n 50
```

---

### Этап 6: Проверка работы (2 минуты)

#### 6.1. Откройте бота в Telegram

Найдите вашего бота по username

#### 6.2. Отправьте команду

```
/start
```

#### 6.3. Проверьте ответ

Бот должен ответить. Если нет — проверьте логи на сервере:

```bash
sudo journalctl -u dynamost-bot -f
```

---

## 🎉 Готово!

Теперь при каждом push в ветку `main` бот будет автоматически обновляться на сервере.

### Процесс автоматического деплоя:

1. ✅ Сборка бинарника
2. ✅ Копирование на сервер
3. ✅ Применение миграций БД
4. ✅ Перезапуск systemd сервиса
5. ✅ Проверка статуса

---

## 📚 Дополнительная документация

- [docs/QUICK_DEPLOY.md](docs/QUICK_DEPLOY.md) — краткая инструкция
- [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) — полная документация
  - Troubleshooting
  - Мониторинг и логи
  - Резервное копирование
  - Откат изменений
  - Безопасность

---

## 🔧 Полезные команды

### На сервере:

```bash
# Статус бота
sudo systemctl status dynamost-bot

# Перезапуск
sudo systemctl restart dynamost-bot

# Остановка
sudo systemctl stop dynamost-bot

# Логи в реальном времени
sudo journalctl -u dynamost-bot -f

# Последние 100 строк логов
sudo journalctl -u dynamost-bot -n 100

# Проверка версии миграций
cd /opt/dynamost-bot
source .env
goose -dir ./migrations postgres "$DB_DSN" status
```

### Для разработки:

```bash
# Локальный запуск
make run

# Применение миграций локально
make migrate-up

# Откат миграций
make migrate-down

# Тесты
go test ./...

# Линтер
make lint
```

---

## ❓ Проблемы?

### Бот не запускается

```bash
# Проверьте логи
sudo journalctl -u dynamost-bot -n 100 --no-pager

# Проверьте .env файл
cat /opt/dynamost-bot/.env

# Проверьте права
ls -la /opt/dynamost-bot/bot
```

### Ошибка подключения к БД

```bash
# Проверьте PostgreSQL
sudo systemctl status postgresql

# Попробуйте подключиться вручную
psql "ВАША_DB_DSN_СТРОКА" -c "SELECT 1;"
```

### Ошибка SSH при деплое

- Проверьте, что публичный ключ добавлен на сервер
- Убедитесь, что в GitHub Secret добавлен ПОЛНЫЙ приватный ключ
- Проверьте права: `chmod 600 ~/.ssh/authorized_keys`

### Миграции не применяются

```bash
# На сервере
cd /opt/dynamost-bot
source .env

# Проверьте статус
goose -dir ./migrations postgres "$DB_DSN" status

# Примените вручную
goose -dir ./migrations postgres "$DB_DSN" up
```

---

## 📞 Нужна помощь?

Если проблема не решена:
1. Изучите [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) — раздел Troubleshooting
2. Проверьте логи: `sudo journalctl -u dynamost-bot -n 200`
3. Создайте Issue в GitHub с описанием проблемы и логами

---

**Успешного деплоя! ⚽️🤖**


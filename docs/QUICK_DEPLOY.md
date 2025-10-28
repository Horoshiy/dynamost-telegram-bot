# Быстрый старт деплоя

Краткая инструкция для развертывания бота на Ubuntu сервере.

## Шаг 1: Подготовка сервера (5-10 минут)

На Ubuntu сервере выполните:

### Вариант А: Скачать скрипт из GitHub (если уже запушен)

```bash
# Скачайте и запустите скрипт настройки
wget https://raw.githubusercontent.com/YOUR_USERNAME/YOUR_REPO/main/deploy/setup-server.sh
chmod +x setup-server.sh
sudo ./setup-server.sh
```

**Замените** `YOUR_USERNAME` и `YOUR_REPO` на ваши данные!

### Вариант Б: Скопировать скрипт вручную (если еще не в GitHub)

```bash
# Скопируйте файл deploy/setup-server.sh с локальной машины на сервер
scp deploy/setup-server.sh user@your-server:/home/user/

# На сервере запустите
chmod +x setup-server.sh
sudo ./setup-server.sh
```

### Вариант В: Создать скрипт на сервере

```bash
# Создайте файл
nano setup-server.sh

# Вставьте содержимое из deploy/setup-server.sh
# Сохраните (Ctrl+O, Enter, Ctrl+X)

# Запустите
chmod +x setup-server.sh
sudo ./setup-server.sh
```

Скрипт:
- ✅ Установит PostgreSQL
- ✅ Создаст базу данных и пользователя
- ✅ Установит необходимые инструменты
- ✅ Выведет строку подключения `DB_DSN`

**Сохраните строку подключения** — она понадобится на следующем шаге!

## Шаг 2: Настройка SSH ключа (2-3 минуты)

### На локальной машине:

```bash
# Генерация SSH ключа для деплоя
ssh-keygen -t ed25519 -C "github-deploy" -f ~/.ssh/github_deploy

# Вывод приватного ключа (скопируйте ВСЕ, включая BEGIN/END)
cat ~/.ssh/github_deploy

# Вывод публичного ключа
cat ~/.ssh/github_deploy.pub
```

### На сервере:

```bash
# Добавьте публичный ключ
echo "ВАHASH_ПУБЛИЧНОГО_КЛЮЧА_ЗДЕСЬ" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

## Шаг 3: Получение данных для GitHub Secrets (5 минут)

Соберите следующую информацию:

| Параметр | Где получить | Пример |
|----------|--------------|--------|
| `SSH_PRIVATE_KEY` | `cat ~/.ssh/github_deploy` | `-----BEGIN OPENSSH...` |
| `SERVER_HOST` | IP вашего сервера | `123.45.67.89` |
| `SERVER_USER` | Имя пользователя SSH | `ubuntu` |
| `BOT_TOKEN` | [@BotFather](https://t.me/BotFather) → `/newbot` | `1234567890:ABC...` |
| `DB_DSN` | Из вывода скрипта setup-server.sh | `postgres://dynamost:...` |
| `ADMIN_IDS` | [@userinfobot](https://t.me/userinfobot) | `123456789,987654321` |
| `CLUB_TZ` | [Список зон](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) | `Europe/Moscow` |

## Шаг 4: Добавление GitHub Secrets (5 минут)

1. Откройте ваш репозиторий на GitHub
2. Перейдите: **Settings** → **Secrets and variables** → **Actions**
3. Нажмите **New repository secret**
4. Добавьте все 7 секретов из таблицы выше

## Шаг 5: Запуск деплоя (1 минута)

### Автоматический деплой при push:

```bash
git add .
git commit -m "feat: setup deployment"
git push origin main
```

### Или ручной запуск:

1. Перейдите в раздел **Actions** на GitHub
2. Выберите **Deploy to Ubuntu Server**
3. Нажмите **Run workflow**

## Шаг 6: Проверка (1 минута)

1. Дождитесь завершения деплоя в GitHub Actions (зеленая галочка ✅)
2. Напишите боту в Telegram: `/start`
3. Бот должен ответить!

### Если бот не отвечает:

На сервере проверьте логи:

```bash
sudo journalctl -u dynamost-bot -n 50
```

## Готово! 🎉

Теперь при каждом push в `main` бот будет автоматически обновляться на сервере.

---

### Полезные команды на сервере:

```bash
# Статус бота
sudo systemctl status dynamost-bot

# Перезапуск
sudo systemctl restart dynamost-bot

# Логи в реальном времени
sudo journalctl -u dynamost-bot -f

# Проверка БД
psql "$DB_DSN" -c "SELECT * FROM goose_db_version;"
```

---

📖 **Подробная документация:** [DEPLOYMENT.md](./DEPLOYMENT.md)


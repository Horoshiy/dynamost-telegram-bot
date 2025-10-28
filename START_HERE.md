# 🚀 Начните отсюда: Деплой Telegram бота

## ✅ Что готово

Инфраструктура для автоматического деплоя полностью настроена!

### Созданные файлы:

#### GitHub Actions
- ✅ `.github/workflows/deploy.yml` — автоматический деплой при push

#### Конфигурация сервера
- ✅ `deploy/dynamost-bot.service` — systemd сервис
- ✅ `deploy/setup-server.sh` — скрипт настройки сервера

#### Документация
- ✅ `docs/DEPLOYMENT.md` — полное руководство (400+ строк)
- ✅ `docs/QUICK_DEPLOY.md` — быстрый старт (15 минут)
- ✅ `docs/DEPLOYMENT_SUMMARY.md` — техническая сводка
- ✅ `DEPLOYMENT_CHECKLIST.md` — пошаговый чеклист
- ✅ `.github/DEPLOY_SECRETS.md` — справочник по секретам
- ✅ `README.md` — обновлен с информацией о деплое

---

## 🎯 Ваш план действий

### Вариант 1: Быстрый старт (15 минут)

**Для тех, кто хочет быстро развернуть бота:**

1. Откройте → **[docs/QUICK_DEPLOY.md](docs/QUICK_DEPLOY.md)**
2. Следуйте 6 шагам
3. Готово! ✅

### Вариант 2: Пошаговый чеклист (20 минут)

**Для тех, кто хочет убедиться, что всё сделано правильно:**

1. Откройте → **[DEPLOYMENT_CHECKLIST.md](DEPLOYMENT_CHECKLIST.md)**
2. Отмечайте выполненные пункты
3. Готово! ✅

### Вариант 3: Полное руководство (30+ минут)

**Для тех, кто хочет понять каждый шаг:**

1. Откройте → **[docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)**
2. Изучите все разделы
3. Готово! ✅

---

## 📚 Структура документации

```
START_HERE.md (вы здесь)
│
├─ DEPLOYMENT_CHECKLIST.md ............. Пошаговый чеклист с командами
│
├─ docs/
│  ├─ QUICK_DEPLOY.md .................. Быстрый старт за 15 минут
│  ├─ DEPLOYMENT.md .................... Полное руководство
│  ├─ DEPLOYMENT_SUMMARY.md ............ Техническая сводка
│  └─ POSTGRES_PASSWORD.md ............. Работа с паролями PostgreSQL
│
└─ .github/
   └─ DEPLOY_SECRETS.md ................ Справочник по GitHub Secrets
```

---

## 🔑 Что вам понадобится

Перед началом подготовьте:

### 1. Сервер Ubuntu
- Ubuntu 20.04 LTS или новее
- Минимум 1GB RAM
- SSH доступ с правами sudo

### 2. Учетные данные Telegram
- Токен бота от [@BotFather](https://t.me/BotFather)
- Ваш Telegram ID от [@userinfobot](https://t.me/userinfobot)

### 3. GitHub аккаунт
- Права администратора на этот репозиторий
- Возможность добавлять секреты

---

## ⚡ Самый быстрый путь

### Шаг 1: На сервере (5 минут)

```bash
ssh user@your-server-ip
wget https://raw.githubusercontent.com/dynamost/telegram-bot/main/deploy/setup-server.sh
chmod +x setup-server.sh
sudo ./setup-server.sh
```

**Сохраните** строку `DB_DSN` из вывода!

### Шаг 2: Настройте SSH (3 минуты)

**На локальной машине:**
```bash
ssh-keygen -t ed25519 -C "deploy" -f ~/.ssh/github_deploy
cat ~/.ssh/github_deploy      # Приватный ключ → GitHub Secret
cat ~/.ssh/github_deploy.pub  # Публичный ключ → На сервер
```

**На сервере:**
```bash
echo "ВАШ_ПУБЛИЧНЫЙ_КЛЮЧ" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

### Шаг 3: GitHub Secrets (5 минут)

На GitHub: **Settings → Secrets and variables → Actions**

Добавьте 7 секретов (см. [.github/DEPLOY_SECRETS.md](.github/DEPLOY_SECRETS.md)):
1. `SSH_PRIVATE_KEY`
2. `SERVER_HOST`
3. `SERVER_USER`
4. `BOT_TOKEN`
5. `DB_DSN`
6. `ADMIN_IDS`
7. `CLUB_TZ`

### Шаг 4: Запустите деплой (1 минута)

```bash
git push origin main
```

Или на GitHub: **Actions → Deploy to Ubuntu Server → Run workflow**

### Шаг 5: Проверьте (1 минута)

Напишите боту в Telegram: `/start`

**Готово! 🎉**

---

## 🆘 Нужна помощь?

### Если что-то пошло не так:

1. **Проверьте логи на сервере:**
   ```bash
   sudo journalctl -u dynamost-bot -n 100
   ```

2. **Проверьте GitHub Actions:**
   - Перейдите в раздел **Actions**
   - Откройте последний запуск
   - Изучите логи

3. **Изучите Troubleshooting:**
   - [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) — раздел "Troubleshooting"

4. **Создайте Issue:**
   - Опишите проблему
   - Приложите логи
   - Укажите версию Ubuntu

---

## 📋 Проверочный список

Перед началом убедитесь:

- [ ] У вас есть доступ к серверу Ubuntu
- [ ] У вас есть токен Telegram бота
- [ ] У вас есть права администратора на GitHub репозиторий
- [ ] Вы знаете свой Telegram ID
- [ ] У вас установлен SSH клиент

---

## 🔄 Как работает автоматический деплой

```
1. git push origin main
         ↓
2. GitHub Actions запускается
         ↓
3. Сборка бинарника для Linux
         ↓
4. Подключение к серверу по SSH
         ↓
5. Копирование файлов (bot, migrations, .env)
         ↓
6. Применение миграций БД (goose up)
         ↓
7. Перезапуск systemd сервиса
         ↓
8. Бот обновлен и работает! ✅
```

---

## 💡 Полезные команды после деплоя

### На сервере:

```bash
# Статус бота
sudo systemctl status dynamost-bot

# Логи в реальном времени
sudo journalctl -u dynamost-bot -f

# Перезапуск
sudo systemctl restart dynamost-bot

# Проверка БД
psql "$DB_DSN" -c "SELECT * FROM teams;"
```

### Локально:

```bash
# Автоматический деплой
git push origin main

# Ручной деплой
# GitHub → Actions → Deploy to Ubuntu Server → Run workflow
```

---

## 🎓 Что дальше?

После успешного деплоя:

1. ✅ Настройте мониторинг (uptimerobot.com)
2. ✅ Настройте автоматические бэкапы БД
3. ✅ Добавьте алерты при падении бота
4. ✅ Настройте SSL сертификат (если используете webhook)
5. ✅ Изучите логи: `sudo journalctl -u dynamost-bot -f`

---

## 📖 Рекомендуемый порядок чтения

1. **Сначала:** [docs/QUICK_DEPLOY.md](docs/QUICK_DEPLOY.md) ← **начните здесь**
2. **Потом:** [DEPLOYMENT_CHECKLIST.md](DEPLOYMENT_CHECKLIST.md)
3. **Для справки:** [.github/DEPLOY_SECRETS.md](.github/DEPLOY_SECRETS.md)
4. **Полное руководство:** [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)
5. **Техническая сводка:** [docs/DEPLOYMENT_SUMMARY.md](docs/DEPLOYMENT_SUMMARY.md)

---

## 🎯 Следующий шаг

### Выберите свой путь:

**A) Быстро развернуть бота прямо сейчас:**
→ Откройте [docs/QUICK_DEPLOY.md](docs/QUICK_DEPLOY.md)

**B) Понять каждый шаг подробно:**
→ Откройте [DEPLOYMENT_CHECKLIST.md](DEPLOYMENT_CHECKLIST.md)

**C) Изучить всю документацию:**
→ Откройте [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)

---

**Удачного деплоя! ⚽️🤖**

*Если возникнут вопросы — все ответы в документации выше.*


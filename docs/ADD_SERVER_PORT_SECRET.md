# Как добавить секрет SERVER_PORT

## Шаги:

1. **Откройте GitHub репозиторий**

2. **Перейдите в Settings**
   - Нажмите на вкладку **Settings** (справа вверху)

3. **Откройте Secrets**
   - В левом меню: **Secrets and variables** → **Actions**

4. **Добавьте новый секрет**
   - Нажмите кнопку **New repository secret**

5. **Заполните данные:**
   - **Name:** `SERVER_PORT`
   - **Secret:** `25178` (ваш SSH порт)

6. **Сохраните**
   - Нажмите **Add secret**

## Проверка:

После добавления вы должны увидеть список секретов:

```
✓ ADMIN_IDS
✓ BOT_TOKEN
✓ CLUB_TZ
✓ DB_DSN
✓ SERVER_HOST
✓ SERVER_PORT  ← новый секрет
✓ SERVER_USER
✓ SSH_PRIVATE_KEY
```

## Повторный запуск:

После добавления секрета:

1. Перейдите в **Actions**
2. Выберите неудавшийся workflow
3. Нажмите **Re-run jobs** → **Re-run failed jobs**

Или сделайте новый push:

```bash
git commit --allow-empty -m "trigger deployment"
git push origin main
```

---

**Готово!** Теперь деплой должен пройти успешно. ✅


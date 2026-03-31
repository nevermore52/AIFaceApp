# Исправление ошибок дублирования ключей в web_sessions

## Проблема

В логах PostgreSQL появляются две ошибки:

1. **Duplicate primary key**: `duplicate key value violates unique constraint "web_sessions_pkey"`
2. **Duplicate token**: `duplicate key value violates unique constraint "web_sessions_token_key"`

## Причины

1. **Sequence рассинхронизирована** - после импорта данных или прямой вставки значение `web_sessions_id_seq` не соответствует максимальному ID в таблице
2. **Дублирование JWT токенов** - при быстрых повторных логинах (в пределах одной секунды) генерируются одинаковые токены

## Решение

### Шаг 1: Применить SQL-скрипт для исправления базы данных

Запустите SQL-скрипт для сброса sequence и очистки дубликатов:

```bash
# Вариант 1: Через docker exec
docker exec -i aifacebot_postgres psql -U postgres -d aifacebot < fix_web_sessions.sql

# Вариант 2: Через docker-compose
docker-compose exec -T postgres psql -U postgres -d aifacebot < fix_web_sessions.sql

# Вариант 3: Вручную через psql
docker exec -it aifacebot_postgres psql -U postgres -d aifacebot
```

Затем выполните команды из `fix_web_sessions.sql`:

```sql
-- 1. Сброс sequence
SELECT setval('web_sessions_id_seq', (SELECT COALESCE(MAX(id), 1) FROM web_sessions));

-- 2. Удаление дубликатов токенов
DELETE FROM web_sessions a
USING web_sessions b
WHERE a.id < b.id AND a.token = b.token;

-- 3. Удаление истекших сессий
DELETE FROM web_sessions WHERE expires_at < NOW();
```

### Шаг 2: Обновить зависимости Go

Перейдите в директорию `web` и обновите зависимости:

```bash
cd web
go mod tidy
```

Это загрузит новую зависимость `github.com/google/uuid v1.6.0`.

### Шаг 3: Пересобрать и перезапустить контейнеры

```bash
# Остановить контейнеры
docker-compose down

# Пересобрать web-backend с новыми зависимостями
docker-compose build web-backend

# Запустить все контейнеры
docker-compose up -d

# Проверить логи
docker-compose logs -f web-backend
```

## Что было исправлено в коде

### 1. `auth_service.go` - Добавлен UUID в JWT токены

**Изменения:**
- Добавлен импорт `github.com/google/uuid`
- В функции `generateJWT()` добавлена генерация уникального JTI (JWT ID) через `uuid.New().String()`
- Это гарантирует, что каждый токен уникален, даже если создан в одну миллисекунду

### 2. `auth_service.go` - Retry механизм при создании сессии

**Изменения:**
- Функция `createSession()` теперь делает до 3 попыток создания сессии
- При ошибке дубликата токена (`duplicate key`) автоматически генерируется новый токен
- Добавлена небольшая задержка (10ms) между попытками

### 3. `go.mod` - Добавлена зависимость

**Изменения:**
- Добавлен пакет `github.com/google/uuid v1.6.0`

## Проверка результата

После применения исправлений проверьте:

```bash
# 1. Проверьте логи на отсутствие ошибок дубликатов
docker-compose logs -f postgres | grep "duplicate key"

# 2. Проверьте текущее значение sequence
docker exec -it aifacebot_postgres psql -U postgres -d aifacebot -c "SELECT last_value FROM web_sessions_id_seq;"

# 3. Проверьте количество сессий и уникальность токенов
docker exec -it aifacebot_postgres psql -U postgres -d aifacebot -c "
SELECT 
    COUNT(*) as total_sessions,
    COUNT(DISTINCT token) as unique_tokens,
    COUNT(DISTINCT user_id) as unique_users
FROM web_sessions;"
```

## Профилактика

Для предотвращения проблем в будущем:

1. **Не вставляйте данные напрямую с указанием ID** - используйте `DEFAULT` для автоинкрементных полей
2. **Регулярно очищайте истекшие сессии** - можно настроить cron-задачу:
   ```sql
   DELETE FROM web_sessions WHERE expires_at < NOW();
   ```
3. **Мониторьте логи PostgreSQL** на предмет ошибок дубликатов

## Дополнительная информация

- **Файл с SQL-скриптом**: `fix_web_sessions.sql`
- **Измененные файлы**:
  - `web/internal/services/auth_service.go`
  - `web/go.mod`

Если проблема повторится, проверьте:
- Не импортируются ли данные из дампа с явным указанием ID
- Не выполняются ли прямые INSERT с ID в обход sequence

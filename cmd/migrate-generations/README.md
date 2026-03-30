# Generation Requests Migration Tool

Этот скрипт мигрирует существующие генерации в БД, чтобы использовать правильный `user_id` из таблицы `users`.

## Проблема

Старые генерации могут содержать `user_id`, который равен `telegram_id` вместо `users.id`. Этот скрипт исправляет это.

## Использование

### 1. Сухой прогон (рекомендуется сначала)

```bash
cd cmd/migrate-generations
go run main.go \
  -host=localhost \
  -port=5432 \
  -user=aifacebot_user \
  -password=aifacebot_password \
  -db=aifacebot \
  -dry-run=true
```

Это покажет, что будет изменено, но не внесет изменения.

### 2. Реальная миграция

```bash
go run main.go \
  -host=localhost \
  -port=5432 \
  -user=aifacebot_user \
  -password=aifacebot_password \
  -db=aifacebot \
  -dry-run=false
```

## Что делает скрипт

1. **Проверяет** каждую генерацию в `generation_requests`
2. **Обновляет** генерации, где `user_id` равен `telegram_id` пользователя
3. **Удаляет** orphaned генерации (без соответствующего пользователя)
4. **Выводит** статистику изменений

## Параметры

- `-host` - хост БД (по умолчанию: localhost)
- `-port` - порт БД (по умолчанию: 5432)
- `-user` - пользователь БД (по умолчанию: aifacebot_user)
- `-password` - пароль БД (по умолчанию: aifacebot_password)
- `-db` - имя БД (по умолчанию: aifacebot)
- `-dry-run` - сухой прогон без изменений (по умолчанию: true)

## Пример вывода

```
Connected to database successfully
Found 150 generation requests

Gen 1: user_id 123456789 -> 5 (telegram_id match)
Gen 2: user_id 987654321 -> 8 (telegram_id match)
Gen 3: user_id 111111111 is orphaned (no matching user)

Summary:
- Valid: 147
- Need update: 2
- Orphaned: 1

[DRY RUN] No changes made. Run with -dry-run=false to apply changes.
```

## На сервере

Если БД в Docker контейнере:

```bash
# Скопировать скрипт на сервер
scp -r cmd/migrate-generations user@server:/path/to/app/

# Подключиться к серверу
ssh user@server

# Выполнить миграцию
cd /path/to/app/cmd/migrate-generations
go run main.go \
  -host=aifacebot_postgres \
  -port=5432 \
  -user=aifacebot_user \
  -password=aifacebot_password \
  -db=aifacebot \
  -dry-run=false
```

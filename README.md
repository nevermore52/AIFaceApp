# AI Face Bot

Telegram‑бот для генерации изображений/видео/музыки, с оплатой и квотами.

## Возможности

- Генерация фото/видео/музыки
- Платежи (YooKassa)
- Квоты и лимиты
- Webhook‑сервер для платежей и Suno

## Быстрый старт

1. Поднять Postgres и Redis (docker‑compose).
2. Заполнить `.env`.
3. Собрать и запустить.

```bash
go build -o bin/bot ./cmd/bot
./bin/bot
```

## Переменные окружения

| Переменная | Назначение |
|---|---|
| `TELEGRAM_BOT_TOKEN` | Токен Telegram бота |
| `DATABASE_URL` | URL PostgreSQL |
| `DEBUG_LOGGING` | Включить подробные логи |
| `REDIS_URL` | Адрес Redis |
| `REDIS_PASSWORD` | Пароль Redis |
| `REDIS_DB` | База Redis |
| `YOOKASSA_SHOP_ID` | ID магазина YooKassa |
| `YOOKASSA_SECRET_KEY` | Секретный ключ YooKassa |
| `YOOKASSA_RETURN_URL` | URL возврата после оплаты |
| `PIAPI_API_KEY` | Ключ PIAPI |
| `PIAPI_BASE_URL` | URL PIAPI |
| `SUNO_API_KEY` | Ключ Suno |
| `SUNO_CALLBACK_URL` | Callback для Suno |
| `PAYMENT_PROVIDER` | Провайдер платежей (youkassa) |
| `WHITELIST_TELEGRAM_IDS` | Админы для белого списка (через запятую) |
| `SERVER_PORT` | Порт веб‑сервера |
| `SERVER_HOST` | Хост веб‑сервера |

## Docker

```bash
docker build -t ai-face-bot .
docker-compose up
```

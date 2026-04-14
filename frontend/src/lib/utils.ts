import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDate(date: string | Date) {
  return new Intl.DateTimeFormat('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(date))
}

export function formatPrice(price: number) {
  return new Intl.NumberFormat('ru-RU', {
    style: 'currency',
    currency: 'RUB',
    minimumFractionDigits: 0,
  }).format(price)
}

const ERROR_MAP: Array<[RegExp | string, string]> = [
  [/insufficient quota/i,           'Недостаточно запросов. Пополните баланс в разделе «Покупка».'],
  [/subscription required/i,        'Для этой модели нужна подписка. Оформите её в разделе «Покупка».'],
  [/not available for your subscription/i, 'Эта модель недоступна на вашем тарифе.'],
  [/telegram.*(not linked|account)/i, 'Привяжите Telegram аккаунт в настройках профиля.'],
  [/not authenticated/i,            'Требуется авторизация. Войдите в аккаунт.'],
  [/generation service not available/i, 'Сервис генерации временно недоступен. Попробуйте позже.'],
  [/payment service not configured/i,   'Платёжный сервис временно недоступен.'],
  [/video resolution must be at least/i, 'Разрешение видео слишком маленькое. Минимум 340×340 пикселей.'],
  [/invalid request/i,              'Неверный запрос. Проверьте введённые данные.'],
  [/model is required/i,            'Выберите модель.'],
  [/prompt is required/i,           'Введите текст запроса.'],
  [/session expired/i,              'Сессия истекла. Войдите заново.'],
  [/access denied/i,                'Нет доступа.'],
  [/generation not found/i,         'Генерация не найдена.'],
  [/failed to get/i,                'Не удалось загрузить данные. Попробуйте обновить страницу.'],
  [/failed to create/i,             'Не удалось создать запрос. Попробуйте ещё раз.'],
  [/timeout|timed out/i,            'Время ожидания истекло. Попробуйте ещё раз.'],
  [/network|fetch/i,                'Ошибка сети. Проверьте подключение к интернету.'],
]

export function humanizeError(err: unknown, fallback = 'Что-то пошло не так. Попробуйте ещё раз.'): string {
  const raw = err instanceof Error ? err.message : String(err ?? '')
  for (const [pattern, message] of ERROR_MAP) {
    if (typeof pattern === 'string' ? raw.includes(pattern) : pattern.test(raw)) {
      return message
    }
  }
  return fallback
}

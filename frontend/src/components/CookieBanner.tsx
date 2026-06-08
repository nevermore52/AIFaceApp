import { useState, useEffect } from 'react'
import { X } from 'lucide-react'

export function CookieBanner() {
  const [isVisible, setIsVisible] = useState(false)
  const [showDetails, setShowDetails] = useState(false)

  useEffect(() => {
    const consent = localStorage.getItem('cookie-consent')
    if (!consent) {
      setIsVisible(true)
    } else {
      // Если есть согласие, загружаем трекеры
      loadTrackers()
    }
  }, [])

  const loadTrackers = () => {
    // Здесь можно добавить загрузку Яндекс.Метрики, Google Analytics и т.д.
    // Пока оставляем пустым, так как трекеры загружаются через Telegram Web App
  }

  const handleAccept = () => {
    localStorage.setItem('cookie-consent', 'accepted')
    localStorage.setItem('cookie-consent-date', new Date().toISOString())
    setIsVisible(false)
    loadTrackers()
  }

  const handleDecline = () => {
    localStorage.setItem('cookie-consent', 'declined')
    localStorage.setItem('cookie-consent-date', new Date().toISOString())
    setIsVisible(false)
  }

  if (!isVisible) return null

  return (
    <div className="fixed inset-0 z-[9999] flex items-end justify-center p-4 pointer-events-none">
      <div className="bg-zinc-900 border border-zinc-800 rounded-2xl shadow-2xl max-w-2xl w-full pointer-events-auto animate-slide-up">
        <div className="p-6">
          <div className="flex justify-between items-start mb-4">
            <h3 className="text-lg font-semibold text-white">
              Мы используем cookies
            </h3>
            <button
              onClick={handleDecline}
              className="text-zinc-400 hover:text-white transition-colors"
            >
              <X className="h-5 w-5" />
            </button>
          </div>

          <div className="space-y-4 text-sm text-zinc-300">
            <p>
              Мы используем cookies и другие технологии для улучшения работы сервиса, 
              персонализации контента и анализа использования.
            </p>

            {!showDetails ? (
              <button
                onClick={() => setShowDetails(true)}
                className="text-blue-400 hover:text-blue-300 underline"
              >
                Подробнее об обработке данных
              </button>
            ) : (
              <div className="space-y-3 bg-zinc-800/50 rounded-lg p-4">
                <div>
                  <h4 className="font-semibold text-white mb-2">Цели обработки персональных данных:</h4>
                  <ul className="list-disc list-inside space-y-1 text-xs">
                    <li>Авторизация и идентификация пользователей через Telegram</li>
                    <li>Обработка запросов на генерацию контента (изображения, видео, музыка, текст)</li>
                    <li>Хранение истории генераций и пользовательских настроек</li>
                    <li>Обработка платежей и управление балансом токенов</li>
                    <li>Улучшение качества сервиса и пользовательского опыта</li>
                    <li>Статистика и аналитика использования сервиса</li>
                  </ul>
                </div>

                <div>
                  <h4 className="font-semibold text-white mb-2">Какие данные мы собираем:</h4>
                  <ul className="list-disc list-inside space-y-1 text-xs">
                    <li>Telegram ID, имя пользователя и данные профиля</li>
                    <li>Загруженные изображения и видео (временно, для обработки)</li>
                    <li>Текстовые запросы (промпты) и результаты генераций</li>
                    <li>История транзакций и баланс токенов</li>
                    <li>Технические данные (IP-адрес, браузер, устройство)</li>
                    <li>Cookies для работы сессий и аналитики</li>
                  </ul>
                </div>

                <div>
                  <h4 className="font-semibold text-white mb-2">Оператор персональных данных:</h4>
                  <div className="text-xs space-y-1">
                    <p><strong>Название:</strong> AIFACEAPP</p>
                    <p><strong>Email:</strong> support@aifaceapp.ru</p>
                    <p><strong>Telegram:</strong> @aifaceapp_support</p>
                  </div>
                </div>

                <div className="text-xs text-zinc-400">
                  <p>
                    Используя сервис, вы соглашаетесь с обработкой ваших данных 
                    в соответствии с{' '}
                    <a href="/privacy" className="text-blue-400 hover:text-blue-300 underline">
                      Политикой конфиденциальности
                    </a>
                    {' '}и{' '}
                    <a href="/terms" className="text-blue-400 hover:text-blue-300 underline">
                      Пользовательским соглашением
                    </a>.
                  </p>
                </div>

                <button
                  onClick={() => setShowDetails(false)}
                  className="text-blue-400 hover:text-blue-300 underline text-xs"
                >
                  Свернуть
                </button>
              </div>
            )}
          </div>

          <div className="flex gap-3 mt-6">
            <button
              onClick={handleAccept}
              className="flex-1 bg-gradient-to-r from-blue-600 to-purple-600 text-white font-semibold py-3 px-6 rounded-xl hover:opacity-90 transition-opacity"
            >
              Принять все
            </button>
            <button
              onClick={handleDecline}
              className="flex-1 bg-zinc-800 text-white font-semibold py-3 px-6 rounded-xl hover:bg-zinc-700 transition-colors"
            >
              Отклонить
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

import { ArrowLeft } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

export function PrivacyPage() {
  const navigate = useNavigate()

  return (
    <div className="min-h-screen bg-[#030303] text-white">
      <div className="max-w-4xl mx-auto px-4 py-8">
        <button
          onClick={() => navigate(-1)}
          className="flex items-center gap-2 text-zinc-400 hover:text-white mb-6 transition-colors"
        >
          <ArrowLeft className="h-5 w-5" />
          Назад
        </button>

        <div className="prose prose-invert prose-zinc max-w-none">
          <h1 className="text-3xl font-bold mb-6">Политика конфиденциальности</h1>
          
          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">1. Введение</h2>
            <p className="text-zinc-300 leading-relaxed">
              Настоящая Политика конфиденциальности (далее — «Политика») регулирует порядок обработки данных 
              пользователей (далее — «Вы», «Пользователь») веб-сервиса AIFACEAPP и Telegram-бота @AIFaceAppBot 
              (далее — «Бот», «Сервис»), являющихся инструментами для генерации и редактирования изображений, 
              видео, музыки и текстового контента с использованием искусственного интеллекта.
            </p>
            <p className="text-zinc-300 leading-relaxed mt-4">
              Используя Сервис, Вы подтверждаете согласие с условиями данной Политики.
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">2. Оператор данных</h2>
            <p className="text-zinc-300 leading-relaxed">
              Оператором, определяющим цели и способы обработки предоставленных данных, выступает владелец 
              и разработчик сервиса AIFACEAPP (далее — «Оператор»).
            </p>
            <div className="mt-4 p-4 bg-zinc-900 rounded-lg">
              <p className="text-zinc-300"><strong>Telegram:</strong> @aifaceapps</p>
            </div>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">3. Какие данные мы обрабатываем</h2>
            <p className="text-zinc-300 leading-relaxed mb-4">
              При использовании Сервиса мы можем обрабатывать следующие данные:
            </p>
            <ul className="list-disc list-inside space-y-2 text-zinc-300">
              <li><strong>Данные аккаунта Telegram:</strong> Ваш идентификатор пользователя (User ID), имя пользователя (username), данные профиля.</li>
              <li><strong>Контент, предоставленный Вами:</strong> Фотографии, изображения, видео и текстовые запросы (промпты), которые Вы отправляете в Сервис для обработки.</li>
              <li><strong>Данные взаимодействия:</strong> История команд, запросов и генераций для обеспечения корректной работы Сервиса и анализа ошибок.</li>
              <li><strong>Платежные данные:</strong> История транзакций, баланс токенов, информация о подписках (без хранения данных банковских карт).</li>
              <li><strong>Технические данные:</strong> IP-адрес, тип устройства, браузер, операционная система для обеспечения безопасности и улучшения работы Сервиса.</li>
              <li><strong>Cookies:</strong> Файлы cookies для работы сессий, аутентификации и веб-аналитики.</li>
            </ul>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">4. Цели обработки данных</h2>
            <p className="text-zinc-300 leading-relaxed mb-4">
              Ваши данные обрабатываются исключительно для:
            </p>
            <ul className="list-disc list-inside space-y-2 text-zinc-300">
              <li>Авторизации и идентификации пользователей через Telegram.</li>
              <li>Предоставления основной функциональности Сервиса: приема контента, его обработки с помощью интегрированных моделей искусственного интеллекта согласно Вашему запросу и возврата Вам результата.</li>
              <li>Хранения истории генераций и пользовательских настроек.</li>
              <li>Обработки платежей, управления балансом токенов и подписками.</li>
              <li>Технической поддержки и устранения неисправностей.</li>
              <li>Улучшения качества сервиса и пользовательского опыта.</li>
              <li>Статистики и аналитики использования Сервиса.</li>
              <li>Соблюдения применимых законов.</li>
            </ul>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">5. Хранение и защита данных</h2>
            <div className="space-y-4 text-zinc-300">
              <div>
                <h3 className="font-semibold text-white mb-2">Хранение:</h3>
                <p className="leading-relaxed">
                  Обработанные изображения, видео, аудио и промежуточные данные хранятся на защищенных серверах 
                  ограниченное время, необходимое для функционирования Сервиса. Загруженные пользователем файлы 
                  для обработки хранятся временно и автоматически удаляются после завершения обработки. 
                  История генераций сохраняется в Вашем личном кабинете до момента её удаления Вами. 
                  Мы не создаем публичных архивов Ваших материалов без Вашего явного согласия.
                </p>
              </div>
              <div>
                <h3 className="font-semibold text-white mb-2">Передача третьим лицам:</h3>
                <p className="leading-relaxed">
                  Ваши персональные данные (User ID, username) и контент передаются сторонним API-сервисам 
                  искусственного интеллекта исключительно для обработки Ваших запросов. Мы не передаем данные 
                  третьим лицам в маркетинговых целях. Передача может осуществляться, если это требуется по закону 
                  или для защиты прав и безопасности.
                </p>
              </div>
              <div>
                <h3 className="font-semibold text-white mb-2">Безопасность:</h3>
                <p className="leading-relaxed">
                  Мы применяем современные организационные и технические меры для защиты Ваших данных 
                  от несанкционированного доступа, изменения, раскрытия или уничтожения, включая шифрование, 
                  контроль доступа и регулярный аудит безопасности.
                </p>
              </div>
            </div>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">6. Ваши права</h2>
            <p className="text-zinc-300 leading-relaxed mb-4">
              Вы имеете право:
            </p>
            <ul className="list-disc list-inside space-y-2 text-zinc-300">
              <li>Прекратить использование Сервиса в любой момент.</li>
              <li>Отозвать согласие на обработку данных, прекратив использование Сервиса.</li>
              <li>Запросить информацию об обработке Ваших данных.</li>
              <li>Запросить удаление Ваших данных, связавшись с Оператором по указанным контактам.</li>
              <li>Управлять настройками cookies в Вашем браузере.</li>
              <li>Экспортировать историю Ваших генераций.</li>
            </ul>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">7. Cookies и веб-аналитика</h2>
            <p className="text-zinc-300 leading-relaxed mb-4">
              Мы используем cookies для:
            </p>
            <ul className="list-disc list-inside space-y-2 text-zinc-300">
              <li>Аутентификации и управления сессиями пользователей.</li>
              <li>Сохранения пользовательских настроек и предпочтений.</li>
              <li>Статистики и анализа использования Сервиса.</li>
            </ul>
            <p className="text-zinc-300 leading-relaxed mt-4">
              Вы можете управлять cookies через настройки браузера. Отключение cookies может ограничить 
              функциональность Сервиса.
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">8. Заключительные положения</h2>
            <ul className="list-disc list-inside space-y-2 text-zinc-300">
              <li>Используя Сервис, Вы подтверждаете, что являетесь совершеннолетним или получили разрешение от родителей/опекунов.</li>
              <li>Вы несете ответственность за содержание материалов (изображения, видео, аудио и текстовые запросы), которые отправляете на обработку.</li>
              <li>Оператор оставляет за собой право вносить изменения в данную Политику. Актуальная версия всегда доступна на данной странице.</li>
              <li>Продолжение использования Сервиса после внесения изменений означает Ваше согласие с обновленной Политикой.</li>
            </ul>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">9. Контакты</h2>
            <p className="text-zinc-300 leading-relaxed mb-4">
              По всем вопросам, касающимся обработки Ваших данных, Вы можете связаться с Оператором:
            </p>
            <div className="p-4 bg-zinc-900 rounded-lg">
              <p className="text-zinc-300"><strong>Telegram:</strong> @aifaceapps</p>
              <p className="text-zinc-300"><strong>Telegram-бот:</strong> @AIFaceAppBot</p>
            </div>
          </section>

          <div className="mt-8 pt-8 border-t border-zinc-800">
            <p className="text-sm text-zinc-500">
              Последнее обновление: {new Date().toLocaleDateString('ru-RU', { year: 'numeric', month: 'long', day: 'numeric' })}
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}

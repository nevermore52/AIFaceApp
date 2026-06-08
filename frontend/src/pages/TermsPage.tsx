import { ArrowLeft } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

export function TermsPage() {
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
          <h1 className="text-3xl font-bold mb-6">Пользовательское соглашение</h1>
          
          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">1. Общие положения</h2>
            <div className="space-y-4 text-zinc-300">
              <p className="leading-relaxed">
                <strong>1.1.</strong> Настоящее Пользовательское соглашение (далее — «Соглашение») регулирует 
                отношения между вами (далее — «Пользователь») и сервисом AIFACEAPP, включая веб-приложение 
                и Telegram-бота @AIFaceAppBot (далее — «Бот», «Сервис»).
              </p>
              <p className="leading-relaxed">
                <strong>1.2.</strong> Сервис предоставляет услуги на платной основе. Условия и стоимость доступа 
                к различным функциям указываются непосредственно в интерфейсе Сервиса.
              </p>
              <p className="leading-relaxed">
                <strong>1.3.</strong> Используя платные функции Сервиса, вы подтверждаете, что ознакомились 
                с условиями настоящего Соглашения и принимаете их. Если вы не согласны с условиями, 
                вы должны немедленно прекратить использование Сервиса.
              </p>
            </div>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">2. Условия предоставления услуг</h2>
            <div className="space-y-4 text-zinc-300">
              <p className="leading-relaxed">
                <strong>2.1.</strong> AIFACEAPP является инструментом для автоматизированного редактирования 
                изображений, создания фото, видео, музыки и текстового контента с использованием технологий 
                искусственного интеллекта.
              </p>
              <p className="leading-relaxed">
                <strong>2.2.</strong> Сервис предоставляется «как есть». Мы прилагаем усилия для его стабильной 
                работы, но не гарантируем, что результаты генерации всегда будут соответствовать ожиданиям 
                Пользователя или что Сервис будет доступен непрерывно.
              </p>
              <p className="leading-relaxed">
                <strong>2.3.</strong> Доступ к Сервису предоставляется через веб-интерфейс (app.aifaceapp.ru) 
                и Telegram-бот (@AIFaceAppBot). Функциональность в обоих интерфейсах может различаться.
              </p>
            </div>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">3. Обязанности и ответственность Пользователя</h2>
            <div className="space-y-4 text-zinc-300">
              <p className="leading-relaxed">
                <strong>3.1.</strong> Пользователь обязуется:
              </p>
              <ul className="list-disc list-inside space-y-2 ml-4">
                <li>Использовать Сервис исключительно в законных целях.</li>
                <li>
                  Не загружать и не обрабатывать через Сервис материалы, которые:
                  <ul className="list-circle list-inside ml-6 mt-2 space-y-1">
                    <li>Содержат информацию, распространение которой запрещено законодательством РФ;</li>
                    <li>Нарушают авторские права или иные права третьих лиц;</li>
                    <li>Являются непристойными, оскорбительными, содержат призывы к насилию или дискриминации;</li>
                    <li>Содержат персональные данные третьих лиц без их согласия;</li>
                    <li>Направлены на обход технических ограничений Сервиса.</li>
                  </ul>
                </li>
              </ul>
              <p className="leading-relaxed mt-4">
                <strong>3.2.</strong> Пользователь самостоятельно несет всю ответственность за содержание 
                загружаемых изображений, видео, аудио и текстовых запросов.
              </p>
              <p className="leading-relaxed">
                <strong>3.3.</strong> Оператор оставляет за собой право заблокировать доступ к Сервису 
                без возврата средств в случае нарушения Пользователем условий настоящего Соглашения.
              </p>
            </div>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">4. Порядок оказания и оплаты услуг</h2>
            <div className="space-y-4 text-zinc-300">
              <p className="leading-relaxed">
                <strong>4.1.</strong> Условия доступа к функциям Сервиса, включая их стоимость, сроки и объем, 
                определяются Оператором и доводятся до сведения Пользователя в интерфейсе Сервиса перед 
                совершением оплаты.
              </p>
              <p className="leading-relaxed">
                <strong>4.2.</strong> Оплата услуг осуществляется через интегрированные платежные системы 
                (Yoomoney, банковские карты и др.). Оператор не хранит данные банковских карт.
              </p>
              <p className="leading-relaxed">
                <strong>4.3.</strong> Оплата услуг является невозвратной, если иное не предусмотрено 
                применимым законодательством или настоящим Соглашением.
              </p>
              <p className="leading-relaxed">
                <strong>4.4.</strong> Токены и баланс имеют срок действия и сгорают через 365 дней после их начисления.
              </p>
            </div>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">5. Условия автоматических списаний (подписки)</h2>
            <div className="space-y-4 text-zinc-300">
              <p className="leading-relaxed">
                <strong>5.1.</strong> Сервис может предоставлять доступ к функциям на основе автоматически 
                продлеваемой подписки (автоплатежа). Информация о периоде подписки (неделя, месяц, год), 
                цене и условиях предоставляется перед оформлением.
              </p>
              <p className="leading-relaxed">
                <strong>5.2.</strong> Оплата подписки списывается автоматически в конце каждого расчетного 
                периода, пока Пользователь не отменит ее.
              </p>
              <p className="leading-relaxed">
                <strong>5.3.</strong> Пользователь может управлять подпиской (отключить автопродление) 
                следующими способами:
              </p>
              <ul className="list-disc list-inside space-y-2 ml-4">
                <li>Через личный кабинет веб-приложения в разделе «Профиль» → «Подписки».</li>
                <li>Через интерфейс Telegram-бота: воспользовавшись соответствующими кнопками в меню бота.</li>
                <li>Для иных платежных систем: согласно инструкциям платежного агрегатора или эмитента карты.</li>
              </ul>
              <p className="leading-relaxed mt-4">
                <strong>5.4.</strong> Отмена подписки означает отказ от автоматического продления. Доступ 
                к платным функциям сохранится до конца уже оплаченного периода, после чего будет автоматически 
                прекращен.
              </p>
              <p className="leading-relaxed">
                <strong>5.5.</strong> Возврат средств за неиспользованную часть текущей подписки при ее отмене 
                не производится, если иное прямо не предусмотрено законодательством РФ.
              </p>
              <p className="leading-relaxed">
                <strong>5.6.</strong> Оператор оставляет за собой право изменять стоимость подписки. 
                Изменение цены для текущих подписчиков вступит в силу не ранее, чем с начала следующего 
                расчетного периода после уведомления Пользователя через интерфейс Сервиса или по электронной почте.
              </p>
            </div>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">6. Интеллектуальная собственность</h2>
            <div className="space-y-4 text-zinc-300">
              <p className="leading-relaxed">
                <strong>6.1.</strong> Результаты генерации (изображения, видео, музыка, текст), созданные 
                с помощью Сервиса на основе Ваших запросов, принадлежат Вам.
              </p>
              <p className="leading-relaxed">
                <strong>6.2.</strong> Интерфейс, программный код, дизайн, логотип и другие элементы Сервиса 
                являются интеллектуальной собственностью Оператора и защищены законодательством об авторском праве.
              </p>
              <p className="leading-relaxed">
                <strong>6.3.</strong> Вы не вправе копировать, модифицировать, распространять или 
                воспроизводить элементы Сервиса без письменного согласия Оператора.
              </p>
            </div>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">7. Ограничение ответственности</h2>
            <div className="space-y-4 text-zinc-300">
              <p className="leading-relaxed">
                <strong>7.1.</strong> Оператор Сервиса не несет ответственности за:
              </p>
              <ul className="list-disc list-inside space-y-2 ml-4">
                <li>Качество, точность или эстетическую ценность результатов, полученных с помощью технологий ИИ.</li>
                <li>Любой прямой или косвенный ущерб, возникший в результате использования или невозможности использования Сервиса.</li>
                <li>Содержание материалов, загруженных Пользователем, и последствия их распространения.</li>
                <li>Временную неработоспособность Сервиса, связанную с техническим обслуживанием или причинами, не зависящими от Оператора (в том числе технические работы на стороне API-провайдеров ИИ).</li>
                <li>Возможные технические сбои при списании средств в рамках автоплатежа, которые находятся в зоне ответственности платежных систем. В случае таких сбоев Пользователю следует обращаться напрямую в поддержку соответствующей платежной системы.</li>
                <li>Действия третьих лиц, в том числе несанкционированный доступ к данным Пользователя через его учетную запись Telegram.</li>
              </ul>
              <p className="leading-relaxed mt-4">
                <strong>7.2.</strong> Максимальная ответственность Оператора в любом случае ограничена 
                суммой, уплаченной Пользователем за Сервис за последние 30 дней.
              </p>
            </div>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">8. Возврат средств</h2>
            <div className="space-y-4 text-zinc-300">
              <p className="leading-relaxed">
                <strong>8.1.</strong> Возврат средств возможен в следующих случаях:
              </p>
              <ul className="list-disc list-inside space-y-2 ml-4">
                <li>Технический сбой привел к двойному списанию средств.</li>
                <li>Услуга не была предоставлена по вине Оператора.</li>
                <li>Иные случаи, предусмотренные законодательством РФ.</li>
              </ul>
              <p className="leading-relaxed mt-4">
                <strong>8.2.</strong> Для запроса возврата средств обратитесь в службу поддержки по адресу 
                support@aifaceapp.ru с указанием причины и приложением подтверждающих документов.
              </p>
              <p className="leading-relaxed">
                <strong>8.3.</strong> Возврат средств не производится, если услуга была частично или 
                полностью использована, за исключением случаев, прямо предусмотренных законом.
              </p>
            </div>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">9. Заключительные положения</h2>
            <div className="space-y-4 text-zinc-300">
              <p className="leading-relaxed">
                <strong>9.1.</strong> Настоящее Соглашение может быть изменено Оператором в одностороннем 
                порядке. Актуальная версия всегда доступна на данной странице. Продолжение использования 
                Сервиса после изменений означает согласие с новой редакцией.
              </p>
              <p className="leading-relaxed">
                <strong>9.2.</strong> Используя Сервис, вы также подтверждаете согласие с условиями{' '}
                <a href="/privacy" className="text-blue-400 hover:text-blue-300 underline">
                  Политики конфиденциальности
                </a>.
              </p>
              <p className="leading-relaxed">
                <strong>9.3.</strong> К настоящему Соглашению применяется законодательство Российской Федерации.
              </p>
              <p className="leading-relaxed">
                <strong>9.4.</strong> Все споры, возникающие из настоящего Соглашения, решаются путем 
                переговоров. При невозможности урегулирования спора в досудебном порядке, споры передаются 
                на рассмотрение в суд по месту нахождения Оператора.
              </p>
            </div>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">10. Контакты</h2>
            <p className="text-zinc-300 leading-relaxed mb-4">
              По всем вопросам, связанным с работой Сервиса, вы можете обратиться к Оператору:
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

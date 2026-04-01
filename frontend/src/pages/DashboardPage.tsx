import { useEffect, useState, useRef, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { userApi } from '../lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Image, Music, Video, MessageSquare, Sparkles, X, Send } from 'lucide-react'
import { Button } from '../components/ui/button'
import { cn } from '../lib/utils'

interface Quota {
  text_daily: number
  text_extra: number
  image_weekly: number
  image_extra: number
  music_weekly: number
  music_extra: number
  video_weekly: number
  video_extra: number
}

const CHANNEL_URL = 'https://t.me/aifaceapps'
const BANNER_DISMISS_KEY = 'channel_banner_dismissed_at'
const BANNER_REDISPLAY_MS = 5 * 60 * 1000 // 5 минут

function shouldShowBanner(alreadyClaimed: boolean): boolean {
  if (alreadyClaimed) return false
  const ts = localStorage.getItem(BANNER_DISMISS_KEY)
  if (!ts) return true
  return Date.now() - parseInt(ts) > BANNER_REDISPLAY_MS
}

export function DashboardPage() {
  const navigate = useNavigate()
  const { user, isAuthenticated, updateUser } = useAuthStore()
  const [quota, setQuota] = useState<Quota | null>(null)
  const [loading, setLoading] = useState(true)
  const [showBanner, setShowBanner] = useState(false)
  const [bonusClaimed, setBonusClaimed] = useState(false)
  const [bonusChecking, setBonusChecking] = useState(false)
  const [bonusSuccess, setBonusSuccess] = useState(false)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const refreshQuota = useCallback(() => {
    if (user?.telegram_id) {
      userApi.getQuota().then((data) => setQuota(data as Quota)).catch(console.error)
    }
  }, [user?.telegram_id])

  useEffect(() => {
    if (user?.telegram_id) {
      // Получаем актуальные данные пользователя с сервера (в т.ч. channel_trial_claimed)
      userApi.getMe()
        .then((data) => {
          updateUser(data as any)
          const claimed = !!(data as any)?.channel_trial_claimed
          setBonusClaimed(claimed)
          setShowBanner(shouldShowBanner(claimed))
        })
        .catch(() => {
          const claimed = !!user?.channel_trial_claimed
          setBonusClaimed(claimed)
          setShowBanner(shouldShowBanner(claimed))
        })
      userApi.getQuota()
        .then((data) => setQuota(data as Quota))
        .catch(console.error)
        .finally(() => setLoading(false))
    } else {
      setLoading(false)
      const claimed = !!user?.channel_trial_claimed
      setBonusClaimed(claimed)
      setShowBanner(shouldShowBanner(claimed))
    }
  }, [user?.id])

  const dismissBanner = () => {
    localStorage.setItem(BANNER_DISMISS_KEY, Date.now().toString())
    setShowBanner(false)
  }

  const startChannelCheck = () => {
    window.open(CHANNEL_URL, '_blank')
    setBonusChecking(true)

    // Начинаем поллинг каждые 20 секунд
    if (pollRef.current) clearInterval(pollRef.current)
    pollRef.current = setInterval(async () => {
      try {
        const res = await userApi.claimChannelBonus()
        if (res.already_claimed || res.subscribed) {
          clearInterval(pollRef.current!)
          setBonusChecking(false)
          setBonusClaimed(true)
          setBonusSuccess(true)
          setShowBanner(false)
          // Обновляем данные
          userApi.getMe().then((data) => updateUser(data as any))
          refreshQuota()
        }
      } catch {
        // продолжаем попытки
      }
    }, 20000)

    // Максимум 5 минут
    setTimeout(() => {
      if (pollRef.current) {
        clearInterval(pollRef.current)
        setBonusChecking(false)
      }
    }, 5 * 60 * 1000)
  }

  useEffect(() => () => { if (pollRef.current) clearInterval(pollRef.current) }, [])

  const quotaCards = [
    { title: 'Текст', icon: MessageSquare, daily: quota?.text_daily ?? 0, extra: quota?.text_extra ?? 0, color: 'text-blue-400', bgColor: 'bg-blue-500/10' },
    { title: 'Картинки', icon: Image, daily: quota?.image_weekly ?? 0, extra: quota?.image_extra ?? 0, color: 'text-green-400', bgColor: 'bg-green-500/10' },
    { title: 'Музыка', icon: Music, daily: quota?.music_weekly ?? 0, extra: quota?.music_extra ?? 0, color: 'text-purple-400', bgColor: 'bg-purple-500/10' },
    { title: 'Видео', icon: Video, daily: quota?.video_weekly ?? 0, extra: quota?.video_extra ?? 0, color: 'text-orange-400', bgColor: 'bg-orange-500/10' },
  ]

  const handleModelClick = () => {
    if (!isAuthenticated) {
      navigate('/login', { state: { from: '/' } })
      return
    }
    window.location.href = '/generate'
  }

  return (
    <div className="space-y-4 max-w-6xl mx-auto">

      {/* Баннер подписки на канал */}
      {isAuthenticated && user?.telegram_id && showBanner && !bonusClaimed && (
        <div className="relative rounded-xl border border-[#229ED9]/30 bg-[#229ED9]/10 px-4 py-3 flex items-center gap-3">
          <Send className="h-5 w-5 text-[#229ED9] flex-shrink-0" />
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-white/90">
              Подпишитесь на канал и получите <span className="text-[#229ED9] font-bold">+2 бонусных запроса</span> в каждой категории
            </p>
            {bonusChecking ? (
              <p className="text-xs text-white/40 mt-0.5 animate-pulse">Проверяем подписку...</p>
            ) : (
              <button
                onClick={startChannelCheck}
                className="text-xs text-[#229ED9] hover:underline mt-0.5"
              >
                Подписаться на @aifaceapps →
              </button>
            )}
          </div>
          <button onClick={dismissBanner} className="p-1 rounded-md hover:bg-white/5 flex-shrink-0">
            <X className="h-4 w-4 text-white/30" />
          </button>
        </div>
      )}

      {/* Успех */}
      {bonusSuccess && (
        <div className="rounded-xl border border-green-500/30 bg-green-500/10 px-4 py-3 text-sm text-green-300 font-medium">
          Бонус получен! +2 запроса в каждой категории добавлены.
        </div>
      )}

      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-bold tracking-tight bg-gradient-to-br from-white to-white/40 bg-clip-text text-transparent">
          {isAuthenticated ? `С возвращением, ${user?.first_name}!` : 'AI Face App'}
        </h1>
        <p className="text-white/40 text-sm">
          {isAuthenticated
            ? 'Личный кабинет и статистика генераций'
            : 'Войдите, чтобы запускать генерации'}
        </p>
      </div>

      {!isAuthenticated && (
        <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm">
          <CardContent className="pt-8 text-center space-y-4">
            <div className="w-16 h-16 mx-auto rounded-full bg-primary/10 flex items-center justify-center">
              <Sparkles className="w-8 h-8 text-primary" />
            </div>
            <div className="space-y-2">
              <p className="text-lg font-semibold text-white/90">Готовы создавать?</p>
              <p className="text-sm text-white/40 max-w-xs mx-auto">
                Авторизуйтесь через Google или Telegram для доступа к мощным инструментам генерации.
              </p>
            </div>
            <Button size="lg" onClick={() => navigate('/login', { state: { from: '/' } })} className="rounded-full px-8">
              Войти в аккаунт
            </Button>
          </CardContent>
        </Card>
      )}

      {isAuthenticated && !user?.telegram_id && (
        <Card className="border-yellow-500/20 bg-yellow-500/5 backdrop-blur-sm">
          <CardContent className="pt-4 pb-4">
            <p className="text-yellow-200/80 text-sm flex items-center gap-3">
              <span className="h-2 w-2 rounded-full bg-yellow-500 animate-pulse flex-shrink-0" />
              Привяжите Telegram аккаунт в настройках профиля для доступа к генерации.
            </p>
          </CardContent>
        </Card>
      )}

      {/* Компактные карточки квоты — всегда 4 в ряд */}
      <div className="grid grid-cols-4 gap-2">
        {quotaCards.map((card) => (
          <Card
            key={card.title}
            className="group cursor-pointer border-white/5 bg-white/[0.02] backdrop-blur-sm hover:bg-white/[0.04] transition-all duration-200"
            onClick={handleModelClick}
          >
            <CardContent className="p-3">
              <div className="flex items-center justify-between mb-2">
                <span className="text-[11px] font-medium text-white/50 leading-none">{card.title}</span>
                <div className={cn("p-1.5 rounded-lg", card.bgColor)}>
                  <card.icon className={cn("h-3.5 w-3.5", card.color)} />
                </div>
              </div>
              {loading ? (
                <div className="h-7 w-12 bg-white/5 animate-pulse rounded" />
              ) : (
                <>
                  <div className="text-2xl font-bold text-white leading-none">
                    {card.daily + card.extra}
                  </div>
                  <p className="text-[9px] uppercase tracking-wider text-white/20 font-medium mt-1 leading-none">
                    {card.daily} баз <span className="text-white/10">+</span> {card.extra} доп
                  </p>
                </>
              )}
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm">
          <CardHeader className="pb-3 pt-4 px-4">
            <CardTitle className="text-base flex items-center gap-2">
              <div className="h-5 w-1 bg-primary rounded-full" />
              Ваша подписка
            </CardTitle>
            <CardDescription className="text-white/40 text-xs">Статус и сроки действия</CardDescription>
          </CardHeader>
          <CardContent className="px-4 pb-4">
            <div className="space-y-2">
              <div className="flex justify-between items-center p-3 rounded-xl bg-white/[0.03] border border-white/5">
                <span className="text-xs text-white/40">Тарифный план</span>
                <span className="text-xs font-bold uppercase tracking-widest text-primary bg-primary/10 px-2.5 py-1 rounded-full border border-primary/20">
                  {user?.subscription_type || 'FREE'}
                </span>
              </div>
              {user?.subscription_end && (
                <div className="flex justify-between items-center p-3 rounded-xl bg-white/[0.03] border border-white/5">
                  <span className="text-xs text-white/40">Действует до</span>
                  <span className="text-xs font-semibold text-white/80">
                    {new Date(user.subscription_end).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })}
                  </span>
                </div>
              )}
              {!user?.subscription_end && (
                <Button variant="outline" className="w-full rounded-xl border-white/10 hover:bg-white/5 text-xs h-10" onClick={() => navigate('/payments')}>
                  Улучшить тариф
                </Button>
              )}
            </div>
          </CardContent>
        </Card>

        <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm relative overflow-hidden">
          <div className="absolute top-0 right-0 w-32 h-32 bg-primary/10 blur-[60px] -mr-16 -mt-16" />
          <CardHeader className="pb-3 pt-4 px-4 relative">
            <CardTitle className="text-base flex items-center gap-2">
              <div className="h-5 w-1 bg-primary rounded-full" />
              Быстрые действия
            </CardTitle>
            <CardDescription className="text-white/40 text-xs">Начните творить прямо сейчас</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 relative px-4 pb-4">
            <Button
              onClick={handleModelClick}
              className="w-full py-5 rounded-xl font-bold shadow-[0_10px_25px_-8px_rgba(139,92,246,0.3)] hover:shadow-[0_15px_30px_-6px_rgba(139,92,246,0.4)] transition-all"
            >
              Начать генерацию
              <Sparkles className="ml-2 h-4 w-4" />
            </Button>
            {/* Telegram канал */}
            <a
              href={CHANNEL_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-center gap-2 w-full py-2.5 rounded-xl font-semibold text-sm bg-[#229ED9] hover:bg-[#1a8bc2] text-white transition-colors"
            >
              <Send className="h-4 w-4" />
              Наш Telegram канал
            </a>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

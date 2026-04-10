import { useEffect, useState, useRef, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { userApi } from '../lib/api'
import { Button } from '../components/ui/button'
import { cn } from '../lib/utils'
import { Image, Film, Music, Type, Sparkles, X, Send } from 'lucide-react'

const CHANNEL_URL = 'https://t.me/aifaceapps'
const BANNER_DISMISS_KEY = 'channel_banner_dismissed_at'
const BANNER_REDISPLAY_MS = 5 * 60 * 1000

function shouldShowBanner(alreadyClaimed: boolean): boolean {
  if (alreadyClaimed) return false
  const ts = localStorage.getItem(BANNER_DISMISS_KEY)
  if (!ts) return true
  return Date.now() - parseInt(ts) > BANNER_REDISPLAY_MS
}

const categories = [
  { id: 'image', label: 'Картинка', icon: Image, color: 'from-amber-500 to-orange-600' },
  { id: 'video', label: 'Видео', icon: Film, color: 'from-purple-500 to-pink-600' },
  { id: 'music', label: 'Аудио', icon: Music, color: 'from-green-500 to-emerald-600' },
  { id: 'text', label: 'Текст', icon: Type, color: 'from-blue-500 to-cyan-600' },
]

export function DashboardPage() {
  const navigate = useNavigate()
  const { user, isAuthenticated, updateUser } = useAuthStore()
  const [showBanner, setShowBanner] = useState(false)
  const [bonusClaimed, setBonusClaimed] = useState(false)
  const [bonusChecking, setBonusChecking] = useState(false)
  const [bonusSuccess, setBonusSuccess] = useState(false)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const refreshQuota = useCallback(() => {
    if (isAuthenticated) {
      userApi.getQuota().catch(console.error)
    }
  }, [isAuthenticated])

  useEffect(() => {
    if (isAuthenticated) {
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
    } else {
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
          userApi.getMe().then((data) => updateUser(data as any))
          refreshQuota()
        }
      } catch {}
    }, 20000)
    setTimeout(() => {
      if (pollRef.current) { clearInterval(pollRef.current); setBonusChecking(false) }
    }, 5 * 60 * 1000)
  }

  useEffect(() => () => { if (pollRef.current) clearInterval(pollRef.current) }, [])

  const handleCategoryClick = (categoryId: string) => {
    if (!isAuthenticated) {
      navigate('/login', { state: { from: '/generate' } })
      return
    }
    navigate('/generate', { state: { category: categoryId } })
  }

  return (
    <div className="space-y-5 max-w-6xl mx-auto">
      {/* TG link banner */}
      {isAuthenticated && !user?.telegram_id && !user?.channel_trial_claimed && (
        <div className="rounded-xl border border-yellow-500/30 bg-yellow-500/10 px-4 py-3 flex items-center gap-3">
          <Send className="h-5 w-5 text-yellow-400 flex-shrink-0" />
          <p className="text-sm text-white/80">
            Пробные запросы доступны только после{' '}
            <a href="/profile" className="text-yellow-400 hover:underline font-medium">привязки Telegram</a>
          </p>
        </div>
      )}

      {/* Channel subscribe banner */}
      {isAuthenticated && user?.telegram_id && showBanner && !bonusClaimed && (
        <div className="relative rounded-xl border border-[#229ED9]/30 bg-[#229ED9]/10 px-4 py-3 flex items-center gap-3">
          <Send className="h-5 w-5 text-[#229ED9] flex-shrink-0" />
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-white/90">
              Подпишитесь на канал и получите <span className="text-[#229ED9] font-bold">+2 генерации фото</span>
            </p>
            {bonusChecking ? (
              <p className="text-xs text-white/40 mt-0.5 animate-pulse">Проверяем подписку...</p>
            ) : (
              <button onClick={startChannelCheck} className="text-xs text-[#229ED9] hover:underline mt-0.5">
                Подписаться на @aifaceapps →
              </button>
            )}
          </div>
          <button onClick={dismissBanner} className="p-1 rounded-md hover:bg-white/5 flex-shrink-0">
            <X className="h-4 w-4 text-white/30" />
          </button>
        </div>
      )}

      {bonusSuccess && (
        <div className="rounded-xl border border-green-500/30 bg-green-500/10 px-4 py-3 text-sm text-green-300 font-medium">
          Бонус получен! +2 запроса добавлены.
        </div>
      )}

      {/* Title */}
      <h1 className="text-2xl font-bold tracking-tight text-white">Создать</h1>

      {/* Category grid — like the reference screenshot */}
      <div className="flex gap-3 overflow-x-auto pb-1 -mx-1 px-1 scrollbar-hide">
        {categories.map((cat) => (
          <button
            key={cat.id}
            onClick={() => handleCategoryClick(cat.id)}
            className="flex flex-col items-center gap-2 flex-shrink-0 group"
          >
            <div className={cn(
              "w-16 h-16 rounded-2xl flex items-center justify-center bg-gradient-to-br transition-all duration-200 group-hover:scale-105 group-active:scale-95 border border-white/10",
              cat.color
            )}>
              <cat.icon className="w-7 h-7 text-white" />
            </div>
            <span className="text-[11px] font-medium text-white/60 group-hover:text-white transition-colors">
              {cat.label}
            </span>
          </button>
        ))}
      </div>

      {/* Not authenticated card */}
      {!isAuthenticated && (
        <div className="rounded-2xl border border-white/5 bg-white/[0.02] p-8 text-center space-y-4">
          <div className="w-14 h-14 mx-auto rounded-full bg-primary/10 flex items-center justify-center">
            <Sparkles className="w-7 h-7 text-primary" />
          </div>
          <div className="space-y-1">
            <p className="text-lg font-semibold text-white/90">Готовы создавать?</p>
            <p className="text-sm text-white/40 max-w-xs mx-auto">
              Авторизуйтесь для доступа к генерации
            </p>
          </div>
          <Button size="lg" onClick={() => navigate('/login', { state: { from: '/' } })} className="rounded-full px-8">
            Войти в аккаунт
          </Button>
        </div>
      )}

      {/* TG channel link */}
      <a
        href={CHANNEL_URL}
        target="_blank"
        rel="noopener noreferrer"
        className="flex items-center justify-center gap-2 w-full py-3 rounded-xl font-semibold text-sm bg-[#229ED9]/10 border border-[#229ED9]/20 text-[#229ED9] hover:bg-[#229ED9]/20 transition-colors"
      >
        <Send className="h-4 w-4" />
        Наш Telegram канал
      </a>
    </div>
  )
}

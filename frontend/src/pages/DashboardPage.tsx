import { useEffect, useState, useRef, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { userApi } from '../lib/api'
import { Button } from '../components/ui/button'
import { Image, Film, Music, Sparkles, Send, Banana, Wand2, ChevronRight } from 'lucide-react'

const CHANNEL_URL = 'https://t.me/aifaceapps'
const BANNER_DISMISS_KEY = 'channel_banner_dismissed_at'
const BANNER_REDISPLAY_MS = 5 * 60 * 1000

function shouldShowBanner(alreadyClaimed: boolean): boolean {
  if (alreadyClaimed) return false
  const ts = localStorage.getItem(BANNER_DISMISS_KEY)
  if (!ts) return true
  return Date.now() - parseInt(ts) > BANNER_REDISPLAY_MS
}

interface CategoryItem {
  id: string
  label: string
  icon: any
  action?: { model?: string; prompt?: string; category?: string }
}

const categories: CategoryItem[] = [
  { id: 'image', label: 'Картинка', icon: Image, action: { category: 'image' } },
  { id: 'video', label: 'Видео', icon: Film, action: { category: 'video' } },
  { id: 'nbpro', label: 'Nano Banana Pro', icon: Banana, action: { category: 'image', model: 'google/nano-banana-pro' } },
  { id: 'music', label: 'Аудио', icon: Music, action: { category: 'music' } },
  { id: 'animate', label: 'Оживить фото', icon: Wand2, action: { model: 'veo3_fast', prompt: 'Оживи фото', category: 'video' } },
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

  const handleCategoryClick = (cat: CategoryItem) => {
    if (!isAuthenticated) {
      navigate('/login', { state: { from: '/generate' } })
      return
    }
    navigate('/generate', { state: cat.action })
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
            onClick={() => handleCategoryClick(cat)}
            className="flex flex-col items-center gap-2 flex-shrink-0 group"
          >
            <div className="w-16 h-16 rounded-2xl flex items-center justify-center bg-[#1a1a1f] transition-all duration-200 group-hover:scale-105 group-active:scale-95 border border-white/10">
              <cat.icon className="w-7 h-7 text-[#FFB700]" />
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

      {/* TG channel card */}
      <a
        href={CHANNEL_URL}
        target="_blank"
        rel="noopener noreferrer"
        className="block relative w-full overflow-hidden rounded-2xl group"
        style={{ height: '140px' }}
      >
        {/* Blurred background */}
        <div className="absolute inset-0" style={{
          background: 'linear-gradient(135deg, #1a3a5c 0%, #229ED9 50%, #1a3a5c 100%)',
          filter: 'blur(0px)',
        }} />
        <div className="absolute inset-0 bg-black/20" />

        {/* Content */}
        <div className="relative h-full flex flex-col items-center justify-center gap-3">
          {/* Telegram icon */}
          <div className="w-16 h-16 rounded-2xl bg-white/10 backdrop-blur-sm flex items-center justify-center border border-white/10">
            <svg className="w-9 h-9 text-white" fill="currentColor" viewBox="0 0 24 24">
              <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm4.64 6.8c-.15 1.58-.8 5.42-1.13 7.19-.14.75-.42 1-.68 1.03-.58.05-1.02-.38-1.58-.75-.88-.58-1.38-.94-2.23-1.5-.99-.65-.35-1.01.22-1.59.15-.15 2.71-2.48 2.76-2.69a.2.2 0 00-.05-.18c-.06-.05-.14-.03-.21-.02-.09.02-1.49.95-4.22 2.79-.4.27-.76.41-1.08.4-.36-.01-1.04-.2-1.55-.37-.63-.2-1.12-.31-1.08-.66.02-.18.27-.36.74-.55 2.92-1.27 4.86-2.11 5.83-2.51 2.78-1.16 3.35-1.36 3.73-1.36.08 0 .27.02.39.12.1.08.13.19.14.27-.01.06.01.24 0 .38z"/>
            </svg>
          </div>
          <div className="flex items-center gap-1.5 text-white/90 text-sm font-semibold">
            <span>Перейти в Telegram-канал</span>
            <ChevronRight className="w-4 h-4 text-white/50" />
          </div>
        </div>
      </a>
    </div>
  )
}

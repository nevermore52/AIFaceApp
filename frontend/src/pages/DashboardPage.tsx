import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { userApi } from '../lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Image, Music, Video, MessageSquare, Sparkles } from 'lucide-react'
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

export function DashboardPage() {
  const navigate = useNavigate()
  const { user, isAuthenticated } = useAuthStore()
  const [quota, setQuota] = useState<Quota | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (user?.telegram_id) {
      userApi.getQuota()
        .then((data) => setQuota(data as Quota))
        .catch(console.error)
        .finally(() => setLoading(false))
    } else {
      setLoading(false)
    }
  }, [user])

  const quotaCards = [
    {
      title: 'Текст',
      icon: MessageSquare,
      daily: quota?.text_daily ?? 0,
      extra: quota?.text_extra ?? 0,
      color: 'text-blue-500',
      bgColor: 'bg-blue-50 dark:bg-blue-950',
    },
    {
      title: 'Изображения',
      icon: Image,
      daily: quota?.image_weekly ?? 0,
      extra: quota?.image_extra ?? 0,
      color: 'text-green-500',
      bgColor: 'bg-green-50 dark:bg-green-950',
    },
    {
      title: 'Музыка',
      icon: Music,
      daily: quota?.music_weekly ?? 0,
      extra: quota?.music_extra ?? 0,
      color: 'text-purple-500',
      bgColor: 'bg-purple-50 dark:bg-purple-950',
    },
    {
      title: 'Видео',
      icon: Video,
      daily: quota?.video_weekly ?? 0,
      extra: quota?.video_extra ?? 0,
      color: 'text-orange-500',
      bgColor: 'bg-orange-50 dark:bg-orange-950',
    },
  ]

  const handleModelClick = () => {
    if (!isAuthenticated) {
      navigate('/login', { state: { from: '/' } })
      return
    }

    window.location.href = '/generate'
  }

  return (
    <div className="space-y-8 max-w-6xl mx-auto">
      <div className="flex flex-col gap-1">
        <h1 className="text-4xl font-bold tracking-tight bg-gradient-to-br from-white to-white/40 bg-clip-text text-transparent">
          {isAuthenticated ? `С возвращением, ${user?.first_name}! 👋` : 'AI Face App'}
        </h1>
        <p className="text-white/40 text-sm">
          {isAuthenticated
            ? 'Ваш личный кабинет и статистика генераций'
            : 'Войдите, чтобы запускать генерации и сохранять историю'}
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
          <CardContent className="pt-6">
            <p className="text-yellow-200/80 text-sm flex items-center gap-3">
              <span className="h-2 w-2 rounded-full bg-yellow-500 animate-pulse" />
              Для доступа к генерации контента привяжите Telegram аккаунт в настройках профиля.
            </p>
          </CardContent>
        </Card>
      )}

      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
        {quotaCards.map((card) => (
          <Card
            key={card.title}
            className="group cursor-pointer border-white/5 bg-white/[0.02] backdrop-blur-sm hover:bg-white/[0.04] transition-all duration-300"
            onClick={handleModelClick}
          >
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
              <CardTitle className="text-sm font-medium text-white/60 group-hover:text-white/90 transition-colors">
                {card.title}
              </CardTitle>
              <div className={cn("p-2.5 rounded-xl transition-all group-hover:scale-110", card.bgColor)}>
                <card.icon className={cn("h-5 w-5", card.color)} />
              </div>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="h-10 w-24 bg-white/5 animate-pulse rounded-lg" />
              ) : (
                <div className="space-y-1">
                  <div className="text-3xl font-bold text-white group-hover:text-primary transition-colors">
                    {card.daily + card.extra}
                  </div>
                  <p className="text-[10px] uppercase tracking-wider text-white/20 font-medium">
                    {card.daily} базовых <span className="text-white/10 px-1">+</span> {card.extra} доп.
                  </p>
                </div>
              )}
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="grid gap-8 md:grid-cols-2">
        <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm group overflow-hidden">
          <CardHeader className="pb-4">
            <CardTitle className="text-lg flex items-center gap-2">
              <div className="h-6 w-1 bg-primary rounded-full" />
              Ваша подписка
            </CardTitle>
            <CardDescription className="text-white/40">Статус и сроки действия</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex justify-between items-center p-4 rounded-2xl bg-white/[0.03] border border-white/5">
                <span className="text-sm text-white/40">Тарифный план</span>
                <span className="text-sm font-bold uppercase tracking-widest text-primary bg-primary/10 px-3 py-1 rounded-full border border-primary/20">
                  {user?.subscription_type || 'FREE'}
                </span>
              </div>
              {user?.subscription_end && (
                <div className="flex justify-between items-center p-4 rounded-2xl bg-white/[0.03] border border-white/5">
                  <span className="text-sm text-white/40">Действует до</span>
                  <span className="text-sm font-semibold text-white/80">
                    {new Date(user.subscription_end).toLocaleDateString('ru-RU', {
                      day: 'numeric',
                      month: 'long',
                      year: 'numeric'
                    })}
                  </span>
                </div>
              )}
              {!user?.subscription_end && (
                <Button variant="outline" className="w-full rounded-xl border-white/10 hover:bg-white/5 text-xs h-12" onClick={() => navigate('/payments')}>
                  Улучшить тариф
                </Button>
              )}
            </div>
          </CardContent>
        </Card>

        <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm group overflow-hidden relative">
          <div className="absolute top-0 right-0 w-32 h-32 bg-primary/10 blur-[60px] -mr-16 -mt-16 group-hover:bg-primary/20 transition-all duration-700" />
          <CardHeader className="pb-4 relative">
            <CardTitle className="text-lg flex items-center gap-2">
              <div className="h-6 w-1 bg-primary rounded-full" />
              Быстрые действия
            </CardTitle>
            <CardDescription className="text-white/40">Начните творить прямо сейчас</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6 relative">
            <p className="text-sm text-white/40 leading-relaxed">
              Используйте всю мощь искусственного интеллекта для создания уникальных изображений, видео, музыки и текстов на одной платформе.
            </p>
            <Button 
              onClick={handleModelClick} 
              className="w-full py-6 rounded-xl font-bold shadow-[0_15px_30px_-10px_rgba(139,92,246,0.3)] hover:shadow-[0_20px_40px_-8px_rgba(139,92,246,0.4)] transition-all"
            >
              Начать генерацию
              <Sparkles className="ml-2 h-4 w-4" />
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

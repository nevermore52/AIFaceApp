import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { userApi } from '../lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Image, Music, Video, MessageSquare } from 'lucide-react'
import { Button } from '../components/ui/button'

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

    const botName = (import.meta.env.VITE_TELEGRAM_BOT_NAME || 'aifaceappbot').replace('@', '')
    window.open(`https://t.me/${botName}?startapp=web_generate`, '_blank', 'noopener,noreferrer')
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">
          {isAuthenticated ? `Привет, ${user?.first_name}! 👋` : 'AI Face App'}
        </h1>
        <p className="text-muted-foreground mt-1">
          {isAuthenticated
            ? 'Добро пожаловать в AI Face App'
            : 'Войдите, чтобы запускать генерации и сохранять историю'}
        </p>
      </div>

      {!isAuthenticated && (
        <Card className="border-amber-300 bg-amber-50 dark:bg-amber-950/40">
          <CardContent className="pt-6 text-center space-y-3">
            <p className="font-medium">Войдите, чтобы создавать генерации</p>
            <p className="text-sm text-muted-foreground">
              Авторизуйтесь через Google или Telegram для доступа к генерации.
            </p>
            <Button size="sm" onClick={() => navigate('/login', { state: { from: '/' } })}>
              Войти
            </Button>
          </CardContent>
        </Card>
      )}

      {isAuthenticated && !user?.telegram_id && (
        <Card className="border-yellow-200 bg-yellow-50 dark:bg-yellow-950 dark:border-yellow-800">
          <CardContent className="pt-6">
            <p className="text-yellow-800 dark:text-yellow-200">
              Для доступа к генерации контента привяжите Telegram аккаунт в настройках профиля.
            </p>
          </CardContent>
        </Card>
      )}

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {quotaCards.map((card) => (
          <Card
            key={card.title}
            className="cursor-pointer hover:border-primary/40 transition-colors"
            onClick={handleModelClick}
          >
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{card.title}</CardTitle>
              <div className={`p-2 rounded-lg ${card.bgColor}`}>
                <card.icon className={`h-4 w-4 ${card.color}`} />
              </div>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="h-8 w-20 bg-muted animate-pulse rounded" />
              ) : (
                <>
                  <div className="text-2xl font-bold">
                    {card.daily + card.extra}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {card.daily} основных + {card.extra} дополнительных
                  </p>
                </>
              )}
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Подписка</CardTitle>
            <CardDescription>Информация о вашей подписке</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Тип:</span>
                <span className="font-medium capitalize">
                  {user?.subscription_type || 'Бесплатная'}
                </span>
              </div>
              {user?.subscription_end && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Действует до:</span>
                  <span className="font-medium">
                    {new Date(user.subscription_end).toLocaleDateString('ru-RU')}
                  </span>
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Быстрые действия</CardTitle>
            <CardDescription>Начните создавать контент</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            <p className="text-sm text-muted-foreground">
              Генерация контента доступна через Telegram бота.
              Откройте бота для создания изображений, видео и музыки.
            </p>
            <a
              href={`https://t.me/${import.meta.env.VITE_TELEGRAM_BOT_NAME || 'aifaceappbot'}`}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center text-primary hover:underline"
            >
              Открыть бота →
            </a>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

import { useEffect, useState } from 'react'
import { useAuthStore } from '../store/auth'
import { paymentApi } from '../lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { formatDate, formatPrice, humanizeError } from '../lib/utils'
import { Check, MessageSquare, Image, Music, Video, Cpu, Zap } from 'lucide-react'

interface Package {
  category: string
  qty: number
  price: number
}

interface Subscription {
  name: string
  price: number
  text_daily: number
  image_weekly: number
  music_weekly: number
  video_weekly: number
  discount: number
  text_models: string[]
  features: string[]
}

interface Payment {
  id: number
  category: string
  qty: number
  amount: number
  created_at: string
}

export function PaymentsPage() {
  const { user, isAuthenticated } = useAuthStore()
  const [packages, setPackages] = useState<Package[]>([])
  const [subscriptions, setSubscriptions] = useState<any[]>([])
  const [payments, setPayments] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [purchasing, setPurchasing] = useState<string | null>(null)
  const [purchasingSubscription, setPurchasingSubscription] = useState<string | null>(null)
  const [photoDiscount, setPhotoDiscount] = useState<{ percent: number; end_time: number } | null>(null)

  useEffect(() => {
    Promise.all([
      paymentApi.getPackages(),
      paymentApi.getSubscriptions(),
      paymentApi.getHistory(10, 0),
      paymentApi.getPhotoDiscount(),
    ])
      .then(([pkgs, subs, hist, pd]) => {
        setPackages(pkgs as Package[])
        setSubscriptions(subs as Subscription[])
        setPayments((hist as { data: Payment[] }).data || [])
        const d = pd as { percent: number; end_time: number }
        if (d.percent > 0) setPhotoDiscount(d)
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  const getCategoryName = (category: string) => {
    switch (category) {
      case 'image':
        return 'Изображения'
      case 'text':
        return 'Текст'
      case 'music':
        return 'Музыка'
      case 'video':
        return 'Видео'
      default:
        return category
    }
  }

  const handlePurchase = async (category: string, qty: number) => {
    if (!isAuthenticated) {
      alert('Для покупки необходимо авторизоваться')
      return
    }

    const key = `${category}-${qty}`
    setPurchasing(key)

    try {
      const result = await paymentApi.create(category, qty) as { checkout_url?: string }
      if (result.checkout_url) {
        window.open(result.checkout_url, '_blank')
      }
    } catch (error: unknown) {
      alert(humanizeError(error, 'Ошибка при создании платежа. Попробуйте ещё раз.'))
    } finally {
      setPurchasing(null)
    }
  }

  const handleSubscriptionPurchase = async (subscriptionName: string) => {
    if (!isAuthenticated) {
      alert('Для покупки необходимо авторизоваться.')
      return
    }

    setPurchasingSubscription(subscriptionName)

    try {
      const result = await paymentApi.createSubscription(subscriptionName) as { checkout_url?: string }
      if (result.checkout_url) {
        window.open(result.checkout_url, '_blank')
      }
    } catch (error: unknown) {
      alert(humanizeError(error, 'Ошибка при создании платежа. Попробуйте ещё раз.'))
    } finally {
      setPurchasingSubscription(null)
    }
  }

  const discountByPlan: Record<string, number> = { mini: 10, start: 15, pro: 20 }
  const userDiscount = discountByPlan[user?.subscription_type?.toLowerCase() ?? ''] ?? 0

  // Итоговая скидка для пакета с учётом photo_discount и скидки подписки
  const getEffectivePrice = (pkg: Package): { final: number; hasDiscount: boolean } => {
    if (pkg.category !== 'image') return { final: pkg.price, hasDiscount: false }
    let price = pkg.price
    let discounted = false
    if (photoDiscount && photoDiscount.percent > 0) {
      price = Math.round(price * (100 - photoDiscount.percent) / 100)
      discounted = true
    }
    if (userDiscount > 0) {
      price = Math.round(price * (100 - userDiscount) / 100)
      discounted = true
    }
    return { final: price, hasDiscount: discounted }
  }

  const groupedPackages = packages.reduce((acc, pkg) => {
    if (!acc[pkg.category]) {
      acc[pkg.category] = []
    }
    acc[pkg.category].push(pkg)
    return acc
  }, {} as Record<string, Package[]>)

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }


  return (
    <div className="space-y-10 max-w-6xl mx-auto">
      <div className="flex flex-col gap-1">
        <h1 className="text-4xl font-bold tracking-tight bg-gradient-to-br from-white to-white/40 bg-clip-text text-transparent">
          Тарифы и Пакеты
        </h1>
        <p className="text-white/40 text-sm">
          Выберите подходящий план подписки или купите дополнительные генерации
        </p>
      </div>

      <div>
        <h2 className="text-xl font-semibold mb-6 flex items-center gap-2 text-white/90">
          <div className="h-8 w-1 bg-primary rounded-full" />
          Подписки
        </h2>
        <div className="grid gap-6 md:grid-cols-3">
          {subscriptions.map((sub) => {
            const isCurrentPlan = user?.subscription_type?.toLowerCase() === sub.name.toLowerCase()
            const isPro = sub.name.toLowerCase() === 'pro'
            return (
              <Card key={sub.name} className={`relative overflow-hidden backdrop-blur-sm transition-all duration-300 group ${isPro ? 'border-primary/40 bg-primary/5 shadow-[0_0_30px_-10px_rgba(139,92,246,0.3)]' : 'border-white/5 bg-white/[0.02] hover:bg-white/[0.04]'}`}>
                {isPro && (
                  <div className="absolute top-0 left-0 right-0 h-0.5 bg-gradient-to-r from-transparent via-primary to-transparent" />
                )}
                {sub.discount > 0 && (
                  <div className="absolute top-3 right-3 bg-primary text-white text-[10px] font-bold px-2 py-1 rounded-md shadow-[0_0_15px_rgba(139,92,246,0.5)]">
                    -{sub.discount}%
                  </div>
                )}
                {isCurrentPlan && (
                  <div className="absolute top-3 left-3 bg-green-500/20 text-green-400 text-[10px] font-bold px-2 py-1 rounded-md border border-green-500/20">
                    Активен
                  </div>
                )}
                <CardHeader className="pb-3 pt-5">
                  <CardTitle className="capitalize text-xl tracking-tight text-white/90">{sub.name}</CardTitle>
                  <CardDescription className="pt-1">
                    <span className="text-3xl font-bold text-white">{formatPrice(sub.price)}</span>
                    <span className="text-white/40 ml-1 text-sm">/нед</span>
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  {/* Квоты */}
                  <div className="grid grid-cols-2 gap-2">
                    <div className="flex items-center gap-2 rounded-lg bg-white/[0.03] px-3 py-2">
                      <MessageSquare className="h-3.5 w-3.5 text-blue-400 flex-shrink-0" />
                      <div>
                        <p className="text-[10px] text-white/30 leading-none">Текст/24ч</p>
                        <p className="text-sm font-bold text-white/80">{sub.text_daily}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2 rounded-lg bg-white/[0.03] px-3 py-2">
                      <Image className="h-3.5 w-3.5 text-green-400 flex-shrink-0" />
                      <div>
                        <p className="text-[10px] text-white/30 leading-none">Фото/нед</p>
                        <p className="text-sm font-bold text-white/80">{sub.image_weekly}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2 rounded-lg bg-white/[0.03] px-3 py-2">
                      <Music className="h-3.5 w-3.5 text-purple-400 flex-shrink-0" />
                      <div>
                        <p className="text-[10px] text-white/30 leading-none">Музыка/нед</p>
                        <p className="text-sm font-bold text-white/80">{sub.music_weekly}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2 rounded-lg bg-white/[0.03] px-3 py-2">
                      <Video className="h-3.5 w-3.5 text-orange-400 flex-shrink-0" />
                      <div>
                        <p className="text-[10px] text-white/30 leading-none">Видео/нед</p>
                        <p className="text-sm font-bold text-white/80">{sub.video_weekly || '—'}</p>
                      </div>
                    </div>
                  </div>

                  {/* Текстовые модели */}
                  {sub.text_models?.length > 0 && (
                    <div className="space-y-1.5">
                      <p className="text-[10px] uppercase tracking-wider text-white/25 flex items-center gap-1.5">
                        <Cpu className="h-3 w-3" /> Модели
                      </p>
                      {sub.text_models.map((m: string) => (
                        <div key={m} className="flex items-center gap-2 text-xs text-white/60">
                          <Zap className="h-3 w-3 text-yellow-400 flex-shrink-0" />
                          {m}
                        </div>
                      ))}
                    </div>
                  )}

                  {/* Доп. фичи */}
                  {sub.features?.length > 0 && (
                    <div className="space-y-1.5">
                      {sub.features.map((f: string) => (
                        <div key={f} className="flex items-center gap-2 text-xs text-white/60">
                          <Check className="h-3 w-3 text-primary flex-shrink-0" />
                          {f}
                        </div>
                      ))}
                    </div>
                  )}

                  <Button
                    className={`w-full py-5 rounded-xl font-bold transition-all duration-300 ${isPro ? 'shadow-[0_10px_30px_-10px_rgba(139,92,246,0.5)] hover:shadow-[0_15px_35px_-8px_rgba(139,92,246,0.6)]' : ''}`}
                    onClick={() => handleSubscriptionPurchase(sub.name)}
                    disabled={purchasingSubscription === sub.name || isCurrentPlan}
                  >
                    {purchasingSubscription === sub.name ? (
                      <div className="flex items-center gap-2">
                        <span className="h-4 w-4 animate-spin rounded-full border-2 border-white/20 border-t-white" />
                        Обработка...
                      </div>
                    ) : isCurrentPlan ? 'Текущий план' : 'Оформить подписку'}
                  </Button>
                </CardContent>
              </Card>
            )
          })}
        </div>
      </div>

      <div>
        <h2 className="text-xl font-semibold mb-1 flex items-center gap-2 text-white/90">
          <div className="h-8 w-1 bg-primary rounded-full" />
          Пакеты генераций
        </h2>
        <div className="flex flex-wrap gap-2 mb-6 pl-3">
          {photoDiscount && photoDiscount.percent > 0 && (
            <span className="text-xs text-yellow-400 bg-yellow-400/10 border border-yellow-400/20 rounded-full px-3 py-1">
              🔥 Акция на фото: -{photoDiscount.percent}% до {new Date(photoDiscount.end_time * 1000).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}
            </span>
          )}
          {userDiscount > 0 && (
            <span className="text-xs text-primary bg-primary/10 border border-primary/20 rounded-full px-3 py-1">
              Скидка по подписке: -{userDiscount}%
            </span>
          )}
        </div>
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
          {(['image', 'video', 'music', 'text'] as const).map((category) => {
            const pkgs = groupedPackages[category]
            if (!pkgs) return null
            return (
              <Card key={category} className="border-white/5 bg-white/[0.02] backdrop-blur-sm">
                <CardHeader className="pb-4">
                  <CardTitle className="text-lg text-white/80">{getCategoryName(category)}</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-2">
                    {pkgs
                      .sort((a, b) => a.qty - b.qty)
                      .map((pkg) => {
                        const key = `${pkg.category}-${pkg.qty}`
                        const isPurchasing = purchasing === key
                        const { final: effectivePrice, hasDiscount } = getEffectivePrice(pkg)
                        return (
                          <div
                            key={key}
                            className="flex justify-between items-center p-3 rounded-xl hover:bg-white/[0.03] transition-all group/item"
                          >
                            <span className="text-sm text-white/60 group-hover/item:text-white/90 transition-colors">{pkg.qty} шт.</span>
                            <Button
                              size="sm"
                              variant="secondary"
                              className="h-8 min-w-[90px] rounded-lg bg-white/5 border border-white/5 hover:bg-primary hover:text-white transition-all text-xs font-bold"
                              onClick={() => handlePurchase(pkg.category, pkg.qty)}
                              disabled={isPurchasing || !isAuthenticated}
                            >
                              {isPurchasing ? (
                                <span className="h-3 w-3 animate-spin rounded-full border-2 border-white/20 border-t-white" />
                              ) : hasDiscount ? (
                                <span className="flex flex-col items-end leading-none gap-0.5">
                                  <span className="line-through text-[9px] text-white/30">{formatPrice(pkg.price)}</span>
                                  <span className="text-primary">{formatPrice(effectivePrice)}</span>
                                </span>
                              ) : (
                                formatPrice(pkg.price)
                              )}
                            </Button>
                          </div>
                        )
                      })}
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>
      </div>

      <div className="space-y-6 pt-4">
        <h2 className="text-xl font-semibold flex items-center gap-2 text-white/90">
          <div className="h-8 w-1 bg-primary rounded-full" />
          История платежей
        </h2>
        {payments.length === 0 ? (
          <Card className="border-white/5 bg-white/[0.01] border-dashed">
            <CardContent className="p-12 text-center">
              <p className="text-white/20 text-sm italic">У вас пока нет совершенных платежей</p>
            </CardContent>
          </Card>
        ) : (
          <Card className="border-white/5 bg-white/[0.01] overflow-hidden">
            <CardContent className="p-0">
              <div className="divide-y divide-white/5">
                {payments.map((payment) => (
                  <div key={payment.id} className="flex justify-between items-center p-5 hover:bg-white/[0.02] transition-colors group">
                    <div className="space-y-1">
                      <p className="font-medium text-white/80 group-hover:text-white transition-colors">
                        {getCategoryName(payment.category)} <span className="text-white/20 px-2">/</span> {payment.qty} ед.
                      </p>
                      <p className="text-[10px] uppercase tracking-wider text-white/20">
                        {formatDate(payment.at || payment.created_at)}
                      </p>
                    </div>
                    <span className="font-bold text-primary group-hover:scale-105 transition-transform">
                      {formatPrice(payment.amount)}
                    </span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}

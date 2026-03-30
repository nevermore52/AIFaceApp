import { useEffect, useState } from 'react'
import { useAuthStore } from '../store/auth'
import { paymentApi } from '../lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { formatDate, formatPrice } from '../lib/utils'

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

  useEffect(() => {
    Promise.all([
      paymentApi.getPackages(),
      paymentApi.getSubscriptions(),
      paymentApi.getHistory(10, 0),
    ])
      .then(([pkgs, subs, hist]) => {
        setPackages(pkgs as Package[])
        setSubscriptions(subs as Subscription[])
        setPayments((hist as { data: Payment[] }).data || [])
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
    if (!isAuthenticated || !user?.telegram_id) {
      alert('Для покупки необходимо авторизоваться и привязать Telegram аккаунт')
      return
    }

    const key = `${category}-${qty}`
    setPurchasing(key)

    try {
      const result = await paymentApi.create(category, qty) as { checkout_url?: string }
      if (result.checkout_url) {
        window.open(result.checkout_url, '_blank')
      }
    } catch (error: any) {
      alert(error.message || 'Ошибка при создании платежа')
    } finally {
      setPurchasing(null)
    }
  }

  const handleSubscriptionPurchase = async (subscriptionName: string) => {
    if (!isAuthenticated || !user?.telegram_id) {
      alert('Для покупки необходимо авторизоваться и привязать Telegram аккаунт')
      return
    }

    setPurchasingSubscription(subscriptionName)

    try {
      const result = await paymentApi.createSubscription(subscriptionName) as { checkout_url?: string }
      if (result.checkout_url) {
        window.open(result.checkout_url, '_blank')
      }
    } catch (error: any) {
      alert(error.message || 'Ошибка при создании платежа')
    } finally {
      setPurchasingSubscription(null)
    }
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

  if (!user?.telegram_id) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold">Платежи</h1>
          <p className="text-muted-foreground mt-1">
            Управляйте подписками и покупайте дополнительные запросы
          </p>
        </div>
        <Card className="border-yellow-200 bg-yellow-50 dark:bg-yellow-950 dark:border-yellow-800">
          <CardContent className="pt-6">
            <p className="text-yellow-800 dark:text-yellow-200">
              Для покупки пакетов необходимо привязать Telegram аккаунт.
            </p>
          </CardContent>
        </Card>
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
          {subscriptions.map((sub) => (
            <Card key={sub.name} className="relative overflow-hidden border-white/5 bg-white/[0.02] backdrop-blur-sm transition-all duration-300 hover:bg-white/[0.04] group">
              {sub.discount > 0 && (
                <div className="absolute top-4 right-4 bg-primary text-white text-[10px] font-bold px-2 py-1 rounded-md shadow-[0_0_15px_rgba(139,92,246,0.5)]">
                  -{sub.discount}%
                </div>
              )}
              <CardHeader className="pb-4">
                <CardTitle className="capitalize text-xl tracking-tight text-white/90">{sub.name}</CardTitle>
                <CardDescription className="pt-2">
                  <span className="text-3xl font-bold text-white">
                    {formatPrice(sub.price)}
                  </span>
                  <span className="text-white/40 ml-1">/нед</span>
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="space-y-3">
                  <div className="flex justify-between items-center text-sm border-b border-white/5 pb-2">
                    <span className="text-white/40">Текст (24ч)</span>
                    <span className="font-semibold text-white/80">{sub.text_daily}</span>
                  </div>
                  <div className="flex justify-between items-center text-sm border-b border-white/5 pb-2">
                    <span className="text-white/40">Изображения</span>
                    <span className="font-semibold text-white/80">{sub.image_weekly}</span>
                  </div>
                  <div className="flex justify-between items-center text-sm border-b border-white/5 pb-2">
                    <span className="text-white/40">Музыка</span>
                    <span className="font-semibold text-white/80">{sub.music_weekly}</span>
                  </div>
                  <div className="flex justify-between items-center text-sm border-b border-white/5 pb-2">
                    <span className="text-white/40">Видео</span>
                    <span className="font-semibold text-white/80">{sub.video_weekly}</span>
                  </div>
                </div>
                <Button 
                  className="w-full py-6 rounded-xl font-bold transition-all duration-300 group-hover:shadow-[0_10px_30px_-10px_rgba(139,92,246,0.4)]"
                  onClick={() => handleSubscriptionPurchase(sub.name)}
                  disabled={purchasingSubscription === sub.name}
                >
                  {purchasingSubscription === sub.name ? (
                    <div className="flex items-center gap-2">
                      <span className="h-4 w-4 animate-spin rounded-full border-2 border-white/20 border-t-white" />
                      Обработка...
                    </div>
                  ) : 'Оформить подписку'}
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>

      <div>
        <h2 className="text-xl font-semibold mb-6 flex items-center gap-2 text-white/90">
          <div className="h-8 w-1 bg-primary rounded-full" />
          Пакеты генераций
        </h2>
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
                        return (
                          <div
                            key={key}
                            className="flex justify-between items-center p-3 rounded-xl hover:bg-white/[0.03] transition-all group/item"
                          >
                            <span className="text-sm text-white/60 group-hover/item:text-white/90 transition-colors">{pkg.qty} шт.</span>
                            <Button
                              size="sm"
                              variant="secondary"
                              className="h-8 min-w-[80px] rounded-lg bg-white/5 border border-white/5 hover:bg-primary hover:text-white transition-all text-xs font-bold"
                              onClick={() => handlePurchase(pkg.category, pkg.qty)}
                              disabled={isPurchasing || !user?.telegram_id}
                            >
                              {isPurchasing ? (
                                <span className="h-3 w-3 animate-spin rounded-full border-2 border-white/20 border-t-white" />
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

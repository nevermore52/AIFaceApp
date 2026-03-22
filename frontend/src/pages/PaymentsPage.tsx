import { useEffect, useState } from 'react'
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
  const [packages, setPackages] = useState<Package[]>([])
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([])
  const [payments, setPayments] = useState<Payment[]>([])
  const [loading, setLoading] = useState(true)

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

  const groupedPackages = packages.reduce((acc, pkg) => {
    if (!acc[pkg.category]) {
      acc[pkg.category] = []
    }
    acc[pkg.category].push(pkg)
    return acc
  }, {} as Record<string, Package[]>)

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="h-8 w-48 bg-muted animate-pulse rounded" />
        <div className="grid gap-4 md:grid-cols-3">
          {[...Array(3)].map((_, i) => (
            <Card key={i}>
              <CardContent className="p-6">
                <div className="h-32 bg-muted animate-pulse rounded" />
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold">Оплата</h1>
        <p className="text-muted-foreground mt-1">
          Пополните баланс или оформите подписку
        </p>
      </div>

      <div>
        <h2 className="text-xl font-semibold mb-4">Подписки</h2>
        <div className="grid gap-4 md:grid-cols-3">
          {subscriptions.map((sub) => (
            <Card key={sub.name} className="relative overflow-hidden">
              {sub.discount > 0 && (
                <div className="absolute top-2 right-2 bg-primary text-primary-foreground text-xs px-2 py-1 rounded-full">
                  -{sub.discount}%
                </div>
              )}
              <CardHeader>
                <CardTitle className="capitalize">{sub.name}</CardTitle>
                <CardDescription>
                  <span className="text-2xl font-bold text-foreground">
                    {formatPrice(sub.price)}
                  </span>
                  <span className="text-muted-foreground">/мес</span>
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="text-sm">
                  <div className="flex justify-between">
                    <span>Текст (в день):</span>
                    <span className="font-medium">{sub.text_daily}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Изображения (в неделю):</span>
                    <span className="font-medium">{sub.image_weekly}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Музыка (в неделю):</span>
                    <span className="font-medium">{sub.music_weekly}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Видео (в неделю):</span>
                    <span className="font-medium">{sub.video_weekly}</span>
                  </div>
                </div>
                <Button className="w-full mt-4" variant="outline">
                  Оформить в боте
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>

      <div>
        <h2 className="text-xl font-semibold mb-4">Пакеты</h2>
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
          {Object.entries(groupedPackages).map(([category, pkgs]) => (
            <Card key={category}>
              <CardHeader>
                <CardTitle>{getCategoryName(category)}</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {pkgs
                    .sort((a, b) => a.qty - b.qty)
                    .map((pkg) => (
                      <div
                        key={`${pkg.category}-${pkg.qty}`}
                        className="flex justify-between items-center p-2 rounded-lg hover:bg-muted transition-colors"
                      >
                        <span>{pkg.qty} шт.</span>
                        <span className="font-medium">{formatPrice(pkg.price)}</span>
                      </div>
                    ))}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>

      <div>
        <h2 className="text-xl font-semibold mb-4">История платежей</h2>
        {payments.length === 0 ? (
          <Card>
            <CardContent className="p-8 text-center">
              <p className="text-muted-foreground">У вас пока нет платежей</p>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardContent className="p-0">
              <div className="divide-y">
                {payments.map((payment) => (
                  <div key={payment.id} className="flex justify-between items-center p-4">
                    <div>
                      <p className="font-medium">
                        {getCategoryName(payment.category)} x{payment.qty}
                      </p>
                      <p className="text-sm text-muted-foreground">
                        {formatDate(payment.created_at)}
                      </p>
                    </div>
                    <span className="font-medium text-green-600">
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

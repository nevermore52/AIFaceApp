import { useEffect, useState } from 'react'
import { generationApi } from '../lib/api'
import { Card, CardContent } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { formatDate } from '../lib/utils'
import { Image, Music, Video, MessageSquare, ChevronLeft, ChevronRight } from 'lucide-react'

interface Generation {
  id: number
  model_type: string
  model: string
  status: string
  prompt?: string
  output?: string
  created_at: string
  completed_at?: string
}

export function HistoryPage() {
  const [generations, setGenerations] = useState<Generation[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [loading, setLoading] = useState(true)
  const limit = 10

  useEffect(() => {
    loadGenerations()
  }, [page])

  const loadGenerations = async () => {
    setLoading(true)
    try {
      const response = await generationApi.getAll(limit, page * limit) as {
        data: Generation[]
        total: number
      }
      setGenerations(response.data || [])
      setTotal(response.total)
    } catch (error) {
      console.error('Failed to load generations:', error)
    } finally {
      setLoading(false)
    }
  }

  const getIcon = (modelType: string) => {
    switch (modelType) {
      case 'image':
        return Image
      case 'music':
        return Music
      case 'video':
        return Video
      default:
        return MessageSquare
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'text-green-500 bg-green-50 dark:bg-green-950'
      case 'processing':
        return 'text-yellow-500 bg-yellow-50 dark:bg-yellow-950'
      case 'failed':
        return 'text-red-500 bg-red-50 dark:bg-red-950'
      default:
        return 'text-gray-500 bg-gray-50 dark:bg-gray-950'
    }
  }

  const getStatusText = (status: string) => {
    switch (status) {
      case 'completed':
        return 'Завершено'
      case 'processing':
        return 'В процессе'
      case 'failed':
        return 'Ошибка'
      default:
        return status
    }
  }

  const totalPages = Math.ceil(total / limit)

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">История генераций</h1>
        <p className="text-muted-foreground mt-1">
          Все ваши запросы на генерацию контента
        </p>
      </div>

      {loading ? (
        <div className="space-y-4">
          {[...Array(3)].map((_, i) => (
            <Card key={i}>
              <CardContent className="p-6">
                <div className="h-20 bg-muted animate-pulse rounded" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : generations.length === 0 ? (
        <Card>
          <CardContent className="p-12 text-center">
            <p className="text-muted-foreground">
              У вас пока нет генераций. Начните создавать контент через Telegram бота!
            </p>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="space-y-4">
            {generations.map((gen: Generation) => {
              const Icon = getIcon(gen.model_type)
              return (
                <Card key={gen.id}>
                  <CardContent className="p-6">
                    <div className="flex items-start gap-4">
                      <div className="p-3 rounded-lg bg-muted">
                        <Icon className="h-6 w-6" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="font-medium">{gen.model}</span>
                          <span className={`text-xs px-2 py-0.5 rounded-full ${getStatusColor(gen.status)}`}>
                            {getStatusText(gen.status)}
                          </span>
                        </div>
                        {gen.prompt && (
                          <p className="text-sm text-muted-foreground truncate">
                            {gen.prompt}
                          </p>
                        )}
                        <p className="text-xs text-muted-foreground mt-2">
                          {formatDate(gen.created_at)}
                        </p>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              )
            })}
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p: number) => Math.max(0, p - 1))}
                disabled={page === 0}
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <span className="text-sm text-muted-foreground">
                Страница {page + 1} из {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p: number) => Math.min(totalPages - 1, p + 1))}
                disabled={page >= totalPages - 1}
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}

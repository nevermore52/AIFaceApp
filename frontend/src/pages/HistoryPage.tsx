import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { generationApi } from '../lib/api'
import { Card, CardContent } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { formatDate } from '../lib/utils'
import { cn } from '../lib/utils'
import { Image, Music, Video, MessageSquare, ChevronLeft, ChevronRight, History } from 'lucide-react'

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
  const navigate = useNavigate()
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
    <div className="space-y-8 max-w-5xl mx-auto">
      <div className="flex flex-col gap-1">
        <h1 className="text-4xl font-bold tracking-tight bg-gradient-to-br from-white to-white/40 bg-clip-text text-transparent">
          История генераций
        </h1>
        <p className="text-white/40 text-sm">
          Архив всех ваших творческих запросов и результатов
        </p>
      </div>

      {loading ? (
        <div className="grid gap-4">
          {[...Array(4)].map((_, i) => (
            <Card key={i} className="border-white/5 bg-white/[0.01]">
              <CardContent className="p-6">
                <div className="flex items-center gap-4">
                  <div className="h-12 w-12 rounded-xl bg-white/5 animate-pulse" />
                  <div className="flex-1 space-y-2">
                    <div className="h-4 w-1/4 bg-white/5 animate-pulse rounded" />
                    <div className="h-3 w-1/2 bg-white/5 animate-pulse rounded" />
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : generations.length === 0 ? (
        <Card className="border-white/5 bg-white/[0.01] border-dashed">
          <CardContent className="p-20 text-center space-y-4">
            <div className="w-16 h-16 mx-auto rounded-full bg-white/5 flex items-center justify-center text-white/20">
              <History className="w-8 h-8" />
            </div>
            <div className="space-y-1">
              <p className="text-lg font-medium text-white/80">История пуста</p>
              <p className="text-sm text-white/40 max-w-xs mx-auto">
                Вы еще ничего не создали. Самое время начать генерацию контента!
              </p>
            </div>
            <Button size="sm" variant="secondary" onClick={() => navigate('/generate')} className="rounded-full px-6 bg-white/5 border-white/10 hover:bg-white/10 transition-all">
              Перейти к генерации
            </Button>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="grid gap-4">
            {generations.map((gen: Generation) => {
              const Icon = getIcon(gen.model_type)
              return (
                <Card key={gen.id} className="group border-white/5 bg-white/[0.02] backdrop-blur-sm hover:bg-white/[0.04] transition-all duration-300">
                  <CardContent className="p-5">
                    <div className="flex items-center gap-5">
                      <div className="p-3.5 rounded-2xl bg-white/5 group-hover:bg-primary/10 transition-colors">
                        <Icon className="h-6 w-6 text-white/60 group-hover:text-primary transition-colors" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center justify-between gap-4 mb-1.5">
                          <div className="flex items-center gap-3">
                            <span className="font-semibold text-white/90 group-hover:text-white transition-colors">{gen.model}</span>
                            <span className={cn(
                              "text-[10px] px-2.5 py-0.5 rounded-full font-bold uppercase tracking-wider",
                              getStatusColor(gen.status)
                            )}>
                              {getStatusText(gen.status)}
                            </span>
                          </div>
                          <p className="text-[10px] uppercase tracking-widest text-white/20 font-medium">
                            {formatDate(gen.created_at)}
                          </p>
                        </div>
                        {gen.prompt && (
                          <p className="text-sm text-white/40 line-clamp-1 group-hover:text-white/60 transition-colors italic">
                            &ldquo;{gen.prompt}&rdquo;
                          </p>
                        )}
                      </div>
                      <Button variant="ghost" size="icon" className="rounded-full hover:bg-white/5 text-white/20 hover:text-white" asChild>
                        <Link to={`/generations/${gen.id}`}>
                          <ChevronRight className="h-5 w-5" />
                        </Link>
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              )
            })}
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-6 pt-4">
              <Button
                variant="outline"
                size="sm"
                className="rounded-xl border-white/5 bg-white/[0.02] hover:bg-white/10"
                onClick={() => setPage((p: number) => Math.max(0, p - 1))}
                disabled={page === 0}
              >
                <ChevronLeft className="h-4 w-4 mr-2" />
                Назад
              </Button>
              <span className="text-xs font-semibold uppercase tracking-widest text-white/30">
                Стр. {page + 1} <span className="text-white/10 mx-1">/</span> {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                className="rounded-xl border-white/5 bg-white/[0.02] hover:bg-white/10"
                onClick={() => setPage((p: number) => Math.min(totalPages - 1, p + 1))}
                disabled={page >= totalPages - 1}
              >
                Вперед
                <ChevronRight className="h-4 w-4 ml-2" />
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}

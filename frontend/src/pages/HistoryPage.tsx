import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { userApi } from '../lib/api'
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
      const response = await userApi.getHistory(limit, page * limit) as {
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
    <div className="space-y-6 max-w-2xl mx-auto w-full overflow-hidden">
      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-bold tracking-tight bg-gradient-to-br from-white to-white/40 bg-clip-text text-transparent">
          История генераций
        </h1>
        <p className="text-white/40 text-xs">
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
                <Link key={gen.id} to={`/generations/${gen.id}`} style={{ display: 'block', width: '100%', minWidth: 0 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '14px', background: 'rgba(255,255,255,0.02)', border: '1px solid rgba(255,255,255,0.05)', borderRadius: '12px', width: '100%', boxSizing: 'border-box', overflow: 'hidden' }}>
                    <div style={{ padding: '10px', borderRadius: '10px', background: 'rgba(255,255,255,0.05)', flexShrink: 0 }}>
                      <Icon style={{ width: 20, height: 20, color: 'rgba(255,255,255,0.6)' }} />
                    </div>
                    <div style={{ flex: 1, minWidth: 0, overflow: 'hidden' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '2px' }}>
                        <span style={{ fontWeight: 600, fontSize: '14px', color: 'rgba(255,255,255,0.9)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0, flex: 1 }}>{gen.model}</span>
                        <span className={cn("text-[10px] px-2 py-0.5 rounded-full font-bold uppercase tracking-wider", getStatusColor(gen.status))} style={{ flexShrink: 0, whiteSpace: 'nowrap' }}>
                          {getStatusText(gen.status)}
                        </span>
                      </div>
                      {gen.prompt && (
                        <p style={{ fontSize: '12px', color: 'rgba(255,255,255,0.4)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontStyle: 'italic', margin: 0 }}>&ldquo;{gen.prompt}&rdquo;</p>
                      )}
                      <p style={{ fontSize: '10px', color: 'rgba(255,255,255,0.2)', marginTop: '2px', margin: 0 }}>{formatDate(gen.created_at)}</p>
                    </div>
                    <ChevronRight style={{ width: 16, height: 16, color: 'rgba(255,255,255,0.2)', flexShrink: 0 }} />
                  </div>
                </Link>
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

import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { generationApi } from '../lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { formatDate } from '../lib/utils'
import { cn } from '../lib/utils'
import { ChevronLeft, Image, Music, Video, MessageSquare, Download } from 'lucide-react'

interface Generation {
  id: number
  model_type: string
  model: string
  status: string
  prompt?: string
  output?: string
  error_msg?: string
  created_at: string
  completed_at?: string
}

export function GenerationDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [generation, setGeneration] = useState<Generation | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return

    const loadGeneration = async () => {
      setLoading(true)
      try {
        const response = await generationApi.getById(parseInt(id))
        setGeneration(response as Generation)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Не удалось загрузить генерацию')
        console.error('Failed to load generation:', err)
      } finally {
        setLoading(false)
      }
    }

    loadGeneration()
  }, [id])

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

  if (loading) {
    return (
      <div className="space-y-8 max-w-4xl mx-auto">
        <Button variant="ghost" size="sm" onClick={() => navigate('/history')} className="text-white/40 hover:text-white">
          <ChevronLeft className="h-4 w-4 mr-2" />
          Вернуться в историю
        </Button>
        <div className="flex items-center justify-center min-h-[400px]">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
        </div>
      </div>
    )
  }

  if (error || !generation) {
    return (
      <div className="space-y-8 max-w-4xl mx-auto">
        <Button variant="ghost" size="sm" onClick={() => navigate('/history')} className="text-white/40 hover:text-white">
          <ChevronLeft className="h-4 w-4 mr-2" />
          Вернуться в историю
        </Button>
        <Card className="border-destructive/20 bg-destructive/5">
          <CardContent className="p-8 text-center">
            <p className="text-destructive font-medium">{error || 'Генерация не найдена'}</p>
          </CardContent>
        </Card>
      </div>
    )
  }

  const Icon = getIcon(generation.model_type)

  return (
    <div className="space-y-8 max-w-4xl mx-auto">
      <Button variant="ghost" size="sm" onClick={() => navigate('/history')} className="text-white/40 hover:text-white">
        <ChevronLeft className="h-4 w-4 mr-2" />
        Вернуться в историю
      </Button>

      <div className="flex flex-col gap-1">
        <h1 className="text-4xl font-bold tracking-tight bg-gradient-to-br from-white to-white/40 bg-clip-text text-transparent">
          Детали генерации
        </h1>
        <p className="text-white/40 text-sm">
          Просмотр результата и параметров генерации
        </p>
      </div>

      <div className="grid gap-6">
        <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-4">
                <div className="p-3.5 rounded-2xl bg-white/5">
                  <Icon className="h-6 w-6 text-white/60" />
                </div>
                <div>
                  <CardTitle className="text-2xl">{generation.model}</CardTitle>
                  <p className="text-sm text-white/40 mt-1">{formatDate(generation.created_at)}</p>
                </div>
              </div>
              <span className={cn(
                "text-xs px-3 py-1.5 rounded-full font-bold uppercase tracking-wider",
                getStatusColor(generation.status)
              )}>
                {getStatusText(generation.status)}
              </span>
            </div>
          </CardHeader>
        </Card>

        {generation.prompt && (
          <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm">
            <CardHeader>
              <CardTitle className="text-lg">Промпт</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-white/80 leading-relaxed italic">
                &ldquo;{generation.prompt}&rdquo;
              </p>
            </CardContent>
          </Card>
        )}

        {generation.status === 'completed' && generation.output ? (
          <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm overflow-hidden">
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-lg">Результат</CardTitle>
                <Button asChild variant="secondary" size="sm" className="rounded-full">
                  <a href={generation.output} target="_blank" rel="noopener noreferrer" download>
                    <Download className="h-4 w-4 mr-2" />
                    Скачать
                  </a>
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              <div className="rounded-2xl overflow-hidden border border-white/10">
                {generation.output.includes('.mp4') || generation.output.includes('video') ? (
                  <video
                    src={generation.output}
                    controls
                    className="w-full h-auto max-h-[600px]"
                  />
                ) : (
                  <img
                    src={generation.output}
                    alt="Generated content"
                    className="w-full h-auto max-h-[600px] object-contain"
                  />
                )}
              </div>
            </CardContent>
          </Card>
        ) : generation.status === 'failed' ? (
          <Card className="border-destructive/20 bg-destructive/5">
            <CardContent className="p-8">
              <div className="space-y-2">
                <p className="font-semibold text-destructive">Ошибка при генерации</p>
                {generation.error_msg && (
                  <p className="text-sm text-white/60">{generation.error_msg}</p>
                )}
              </div>
            </CardContent>
          </Card>
        ) : (
          <Card className="border-yellow-500/20 bg-yellow-500/5">
            <CardContent className="p-8 text-center">
              <div className="space-y-4">
                <div className="flex justify-center">
                  <div className="h-8 w-8 animate-spin rounded-full border-4 border-yellow-500/20 border-t-yellow-500" />
                </div>
                <p className="text-yellow-200/80 font-medium">Генерация в процессе</p>
                <p className="text-sm text-white/40">Обновите страницу через несколько секунд</p>
              </div>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}

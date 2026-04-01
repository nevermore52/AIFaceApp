import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { generationApi } from '../lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { formatDate } from '../lib/utils'
import { cn } from '../lib/utils'
import { ChevronLeft, Image, Music, Video, MessageSquare, Download } from 'lucide-react'

interface MediaOutput {
  url: string
  urls?: string[]
  type: string
  title?: string
  mime_type?: string
}

interface Generation {
  id: number
  model_type: string
  model: string
  status: string
  prompt?: string
  output?: string
  media_output?: MediaOutput
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
  const modelType = generation.model_type
  const isMusic = modelType === 'music'
  const isVideo = modelType === 'video'
  const isText = modelType === 'text' || modelType === 'chat'
  const resultTitle = isMusic ? 'Песня' : isVideo ? 'Видео' : isText ? 'Ответ' : 'Изображение'
  const audioUrls = generation.media_output?.urls ?? (generation.output ? [generation.output] : [])

  return (
    <div className="space-y-3 max-w-4xl mx-auto h-[calc(100vh-8rem)] flex flex-col">
      <Button variant="ghost" size="sm" onClick={() => navigate('/history')} className="text-white/40 hover:text-white self-start">
        <ChevronLeft className="h-4 w-4 mr-2" />
        Назад
      </Button>

      <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm flex-shrink-0">
        <CardContent className="p-4">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-3 min-w-0">
              <div className="p-2 rounded-xl bg-white/5 flex-shrink-0">
                <Icon className="h-5 w-5 text-white/60" />
              </div>
              <div className="min-w-0">
                <h2 className="text-base font-semibold truncate">{generation.model}</h2>
                <p className="text-xs text-white/40">{formatDate(generation.created_at)}</p>
              </div>
            </div>
            <span className={cn(
              "text-[10px] px-2 py-1 rounded-full font-bold uppercase tracking-wider flex-shrink-0",
              getStatusColor(generation.status)
            )}>
              {getStatusText(generation.status)}
            </span>
          </div>
        </CardContent>
      </Card>

      {generation.status === 'completed' && (generation.output || generation.media_output) ? (
        <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm overflow-hidden flex-1 min-h-0 flex flex-col">
          <CardHeader className="p-3 flex-shrink-0">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm">{resultTitle}</CardTitle>
              {!isText && generation.output && (
                <Button asChild variant="secondary" size="sm" className="rounded-full h-7 text-xs">
                  <a href={generation.output} target="_blank" rel="noopener noreferrer" download>
                    <Download className="h-3 w-3 mr-1" />
                    Скачать
                  </a>
                </Button>
              )}
            </div>
          </CardHeader>
          <CardContent className="p-3 flex-1 min-h-0 overflow-auto">
            {isText ? (
              <div className="text-sm text-white/80 leading-relaxed whitespace-pre-wrap">
                {generation.output}
              </div>
            ) : isMusic ? (
              <div className="space-y-3 w-full">
                {audioUrls.map((url, i) => (
                  <div key={i} className="space-y-1">
                    {audioUrls.length > 1 && (
                      <p className="text-xs text-white/40">Вариант {i + 1}</p>
                    )}
                    <audio controls className="w-full" src={url} />
                  </div>
                ))}
              </div>
            ) : isVideo ? (
              <div className="flex items-center justify-center max-h-full">
                <div className="rounded-xl overflow-hidden border border-white/10 max-h-full max-w-full">
                  <video
                    src={generation.output!}
                    controls
                    className="w-full h-full object-contain max-h-[50vh]"
                  />
                </div>
              </div>
            ) : (
              <div className="flex items-center justify-center max-h-full">
                <div className="rounded-xl overflow-hidden border border-white/10 max-h-full max-w-full">
                  <img
                    src={generation.output!}
                    alt="Generated content"
                    className="w-full h-full object-contain max-h-[50vh]"
                  />
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      ) : generation.status === 'failed' ? (
        <Card className="border-destructive/20 bg-destructive/5 flex-1">
          <CardContent className="p-6 flex items-center justify-center">
            <div className="space-y-2 text-center">
              <p className="font-semibold text-destructive">Ошибка при генерации</p>
              {generation.error_msg && (
                <p className="text-sm text-white/60">{generation.error_msg}</p>
              )}
            </div>
          </CardContent>
        </Card>
      ) : (
        <Card className="border-yellow-500/20 bg-yellow-500/5 flex-1">
          <CardContent className="p-6 flex items-center justify-center">
            <div className="space-y-3 text-center">
              <div className="flex justify-center">
                <div className="h-6 w-6 animate-spin rounded-full border-4 border-yellow-500/20 border-t-yellow-500" />
              </div>
              <p className="text-yellow-200/80 font-medium text-sm">Генерация в процессе</p>
              <p className="text-xs text-white/40">Обновите страницу</p>
            </div>
          </CardContent>
        </Card>
      )}

      {generation.prompt && (
        <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm flex-shrink-0">
          <CardContent className="p-3">
            <p className="text-xs font-semibold text-white/40 mb-1">Запрос</p>
            <p className="text-sm text-white/80 line-clamp-2 italic">
              &ldquo;{generation.prompt}&rdquo;
            </p>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

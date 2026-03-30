import { useEffect, useState, useRef, ChangeEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { generationApi } from '../lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'

interface Model {
  id: string
  name: string
  type: string
  description: string
  token_cost: number
}

interface Generation {
  id: number
  status: string
  output?: string
  error_msg?: string
}

export function GeneratePage() {
  const navigate = useNavigate()
  const { isAuthenticated, user } = useAuthStore()
  const [models, setModels] = useState<Model[]>([])
  const [selectedModel, setSelectedModel] = useState<string>('')
  const [prompt, setPrompt] = useState('')
  const [imageFile, setImageFile] = useState<File | null>(null)
  const [imagePreview, setImagePreview] = useState<string>('')
  const [uploadingImage, setUploadingImage] = useState(false)
  const [loading, setLoading] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [currentGeneration, setCurrentGeneration] = useState<Generation | null>(null)
  const [error, setError] = useState<string | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/login', { state: { from: '/generate' } })
      return
    }

    setLoading(true)
    generationApi.getModels()
      .then((data) => {
        setModels(data)
        if (data.length > 0) {
          setSelectedModel(data[0].id)
        }
      })
      .catch((err) => {
        setError('Не удалось загрузить модели')
        console.error(err)
      })
      .finally(() => setLoading(false))

    return () => {
      if (pollRef.current) clearInterval(pollRef.current)
    }
  }, [isAuthenticated, navigate])

  const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    if (!file.type.startsWith('image/')) {
      setError('Пожалуйста, выберите изображение')
      return
    }

    setImageFile(file)
    const reader = new FileReader()
    reader.onloadend = () => {
      setImagePreview(reader.result as string)
    }
    reader.readAsDataURL(file)
  }

  const uploadImageToImgur = async (file: File): Promise<string> => {
    const formData = new FormData()
    formData.append('image', file)

    const response = await fetch('https://api.imgur.com/3/image', {
      method: 'POST',
      headers: {
        Authorization: 'Client-ID 546c25a59c58ad7',
      },
      body: formData,
    })

    if (!response.ok) {
      throw new Error('Не удалось загрузить изображение')
    }

    const data = await response.json()
    return data.data.link
  }

  const handleGenerate = async () => {
    if (!selectedModel || !prompt.trim()) {
      setError('Выберите модель и введите промпт')
      return
    }

    setError(null)
    setGenerating(true)
    setCurrentGeneration(null)

    try {
      let imageUrls: string[] | undefined

      if (imageFile) {
        setUploadingImage(true)
        try {
          const uploadedUrl = await uploadImageToImgur(imageFile)
          imageUrls = [uploadedUrl]
        } catch (uploadErr) {
          setError('Ошибка загрузки изображения')
          setGenerating(false)
          setUploadingImage(false)
          return
        }
        setUploadingImage(false)
      }

      const result = await generationApi.create({
        model: selectedModel,
        prompt: prompt.trim(),
        image_urls: imageUrls,
      })

      setCurrentGeneration({ id: result.id, status: result.status })

      pollRef.current = setInterval(async () => {
        try {
          const status = await generationApi.getStatus(result.id) as Generation
          setCurrentGeneration(status)

          if (status.status === 'completed' || status.status === 'failed') {
            if (pollRef.current) clearInterval(pollRef.current)
            setGenerating(false)
          }
        } catch (err) {
          console.error('Poll error:', err)
        }
      }, 3000)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Ошибка при создании генерации'
      setError(message)
      setGenerating(false)
    }
  }

  const selectedModelInfo = models.find(m => m.id === selectedModel)

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">✨ Генерация</h1>
        <p className="text-muted-foreground mt-1">
          Создавайте изображения и видео с помощью ИИ
        </p>
      </div>

      {!user?.telegram_id && (
        <Card className="border-yellow-200 bg-yellow-50 dark:bg-yellow-950 dark:border-yellow-800">
          <CardContent className="pt-6">
            <p className="text-yellow-800 dark:text-yellow-200">
              Для генерации контента необходимо привязать Telegram аккаунт.
            </p>
          </CardContent>
        </Card>
      )}

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Настройки генерации</CardTitle>
            <CardDescription>Выберите модель и опишите, что хотите создать</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Модель</label>
              <select
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={selectedModel}
                onChange={(e: ChangeEvent<HTMLSelectElement>) => setSelectedModel(e.target.value)}
              >
                {models.map((model) => (
                  <option key={model.id} value={model.id}>
                    {model.type === 'video' ? '🎬' : '🖼️'} {model.name} ({model.token_cost} токен)
                  </option>
                ))}
              </select>
              {selectedModelInfo && (
                <p className="text-xs text-muted-foreground">{selectedModelInfo.description}</p>
              )}
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Промпт</label>
              <textarea
                className="flex min-h-[100px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                placeholder="Опишите, что хотите создать..."
                value={prompt}
                onChange={(e: ChangeEvent<HTMLTextAreaElement>) => setPrompt(e.target.value)}
                rows={4}
              />
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">Изображение (опционально)</label>
              <input
                type="file"
                accept="image/*"
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm file:border-0 file:bg-transparent file:text-sm file:font-medium"
                onChange={handleFileChange}
              />
              {imagePreview && (
                <div className="mt-2">
                  <img
                    src={imagePreview}
                    alt="Preview"
                    className="max-h-32 rounded-md border"
                  />
                  <button
                    onClick={() => {
                      setImageFile(null)
                      setImagePreview('')
                    }}
                    className="mt-1 text-xs text-destructive hover:underline"
                  >
                    Удалить
                  </button>
                </div>
              )}
              <p className="text-xs text-muted-foreground">
                Для редактирования или генерации на основе изображения
              </p>
            </div>

            {error && (
              <p className="text-sm text-destructive">{error}</p>
            )}

            <Button
              className="w-full"
              onClick={handleGenerate}
              disabled={generating || uploadingImage || !user?.telegram_id}
            >
              {uploadingImage ? (
                <>
                  <span className="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent inline-block" />
                  Загрузка изображения...
                </>
              ) : generating ? (
                <>
                  <span className="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent inline-block" />
                  Генерация...
                </>
              ) : (
                '✨ Сгенерировать'
              )}
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Результат</CardTitle>
            <CardDescription>Здесь появится результат генерации</CardDescription>
          </CardHeader>
          <CardContent>
            {!currentGeneration && !generating && (
              <div className="flex items-center justify-center h-64 bg-muted rounded-lg">
                <p className="text-muted-foreground">Результат появится здесь</p>
              </div>
            )}

            {generating && currentGeneration?.status !== 'completed' && (
              <div className="flex flex-col items-center justify-center h-64 bg-muted rounded-lg gap-4">
                <div className="h-12 w-12 animate-spin rounded-full border-4 border-primary border-t-transparent" />
                <p className="text-muted-foreground">
                  {currentGeneration?.status === 'processing' ? 'Обработка...' : 'Создание задачи...'}
                </p>
              </div>
            )}

            {currentGeneration?.status === 'completed' && currentGeneration.output && (
              <div className="space-y-4">
                {currentGeneration.output.includes('.mp4') || currentGeneration.output.includes('video') ? (
                  <video
                    src={currentGeneration.output}
                    controls
                    className="w-full rounded-lg"
                  />
                ) : (
                  <img
                    src={currentGeneration.output}
                    alt="Generated"
                    className="w-full rounded-lg"
                  />
                )}
                <a
                  href={currentGeneration.output}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-primary hover:underline"
                >
                  Открыть в новой вкладке
                </a>
              </div>
            )}

            {currentGeneration?.status === 'failed' && (
              <div className="flex flex-col items-center justify-center h-64 bg-destructive/10 rounded-lg gap-2">
                <p className="text-destructive font-medium">Ошибка генерации</p>
                <p className="text-sm text-muted-foreground">
                  {currentGeneration.error_msg || 'Неизвестная ошибка'}
                </p>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

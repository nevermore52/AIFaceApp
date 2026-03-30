import { useEffect, useState, useRef, ChangeEvent, DragEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { generationApi } from '../lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { cn } from '../lib/utils'
import { Sparkles } from 'lucide-react'

type Category = 'image' | 'video' | 'music' | 'text'

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

const CATEGORY_LABELS: Record<Category, string> = {
  image: 'Изображения',
  video: 'Видео',
  music: 'Музыка',
  text: 'Текст',
}

const MAX_IMAGES_PER_MODEL: Record<string, number> = {
  'google/nano-banana': 1,
  'google/nano-banana-pro': 4,
  'nano-banana-2': 4,
  'seedream/4.5-edit': 4,
}

export function GeneratePage() {
  const navigate = useNavigate()
  const { isAuthenticated, user } = useAuthStore()
  const [allModels, setAllModels] = useState<Model[]>([])
  const [selectedCategory, setSelectedCategory] = useState<Category>('image')
  const [selectedModel, setSelectedModel] = useState<string>('')
  const [prompt, setPrompt] = useState('')
  const [imageFiles, setImageFiles] = useState<File[]>([])
  const [imagePreviews, setImagePreviews] = useState<string[]>([])
  const [uploadingImage, setUploadingImage] = useState(false)
  const [loading, setLoading] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [currentGeneration, setCurrentGeneration] = useState<Generation | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [dragActive, setDragActive] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/login', { state: { from: '/generate' } })
      return
    }

    setLoading(true)
    generationApi.getModels()
      .then((data) => {
        setAllModels(data)
        const imageModels = data.filter((m: Model) => m.type === 'image')
        if (imageModels.length > 0) {
          setSelectedModel(imageModels[0].id)
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

  const filteredModels = allModels.filter((m) => m.type === selectedCategory)

  useEffect(() => {
    if (filteredModels.length > 0 && !filteredModels.find(m => m.id === selectedModel)) {
      setSelectedModel(filteredModels[0].id)
    }
  }, [selectedCategory, filteredModels, selectedModel])

  // Проверяем лимит фото при смене модели
  useEffect(() => {
    if (!selectedModel) return
    
    const maxImages = MAX_IMAGES_PER_MODEL[selectedModel] || 4
    if (imageFiles.length > maxImages) {
      const allowedFiles = imageFiles.slice(0, maxImages)
      const allowedPreviews = imagePreviews.slice(0, maxImages)
      setImageFiles(allowedFiles)
      setImagePreviews(allowedPreviews)
      const modelInfo = allModels.find(m => m.id === selectedModel)
      setError(`Для модели ${modelInfo?.name || selectedModel} максимум ${maxImages} изображений. Лишние удалены.`)
    }
  }, [selectedModel])

  const getMaxImages = (): number => {
    return MAX_IMAGES_PER_MODEL[selectedModel] || 4
  }

  const handleFilesChange = (files: FileList | null) => {
    if (!files) return

    const maxImages = getMaxImages()
    const newFiles = Array.from(files).filter(f => f.type.startsWith('image/'))
    
    if (newFiles.length === 0) {
      setError('Пожалуйста, выберите изображения')
      return
    }

    const remainingSlots = maxImages - imageFiles.length
    if (remainingSlots <= 0) {
      setError(`Максимум ${maxImages} изображений для этой модели`)
      return
    }

    // Обрезаем файлы до оставшихся слотов
    const filesToAdd = newFiles.slice(0, remainingSlots)
    if (filesToAdd.length < newFiles.length) {
      setError(`Можно добавить только ${filesToAdd.length} изображений. Максимум ${maxImages} для этой модели`)
    } else {
      setError(null)
    }

    const newPreviews: string[] = []
    let loadedCount = 0

    filesToAdd.forEach((file) => {
      const reader = new FileReader()
      reader.onloadend = () => {
        newPreviews.push(reader.result as string)
        loadedCount++
        if (loadedCount === filesToAdd.length) {
          setImageFiles([...imageFiles, ...filesToAdd])
          setImagePreviews([...imagePreviews, ...newPreviews])
        }
      }
      reader.readAsDataURL(file)
    })
  }

  const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    handleFilesChange(e.target.files)
  }

  const handleDrag = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    e.stopPropagation()
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true)
    } else if (e.type === 'dragleave') {
      setDragActive(false)
    }
  }

  const handleDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    e.stopPropagation()
    setDragActive(false)
    handleFilesChange(e.dataTransfer.files)
  }

  const removeImage = (index: number) => {
    setImageFiles(imageFiles.filter((_, i) => i !== index))
    setImagePreviews(imagePreviews.filter((_, i) => i !== index))
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

      if (imageFiles.length > 0) {
        setUploadingImage(true)
        try {
          const uploadedUrls = await Promise.all(
            imageFiles.map(file => uploadImageToImgur(file))
          )
          imageUrls = uploadedUrls
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

  const selectedModelInfo = filteredModels.find(m => m.id === selectedModel)

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  return (
    <div className="space-y-8 max-w-6xl mx-auto">
      <div className="flex flex-col gap-1">
        <h1 className="text-4xl font-bold tracking-tight bg-gradient-to-br from-white to-white/40 bg-clip-text text-transparent">
          Генерация
        </h1>
        <p className="text-white/40 text-sm">
          Создавайте шедевры с помощью передовых моделей искусственного интеллекта
        </p>
      </div>

      {!user?.telegram_id && (
        <Card className="border-yellow-500/20 bg-yellow-500/5 backdrop-blur-sm">
          <CardContent className="pt-6">
            <p className="text-yellow-200/80 text-sm flex items-center gap-2">
              <span className="h-2 w-2 rounded-full bg-yellow-500 animate-pulse" />
              Для генерации контента необходимо привязать Telegram аккаунт в профиле.
            </p>
          </CardContent>
        </Card>
      )}

      <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm overflow-hidden">
        <CardContent className="p-1">
          <div className="flex flex-wrap gap-1">
            {(Object.entries(CATEGORY_LABELS) as [Category, string][]).map(([cat, label]) => (
              <button
                key={cat}
                onClick={() => setSelectedCategory(cat)}
                className={cn(
                  'flex-1 min-w-[120px] py-3 px-4 rounded-xl text-sm font-medium transition-all duration-300',
                  selectedCategory === cat
                    ? 'bg-white/10 text-white shadow-[0_0_20px_rgba(255,255,255,0.05)]'
                    : 'text-white/40 hover:text-white/60 hover:bg-white/[0.02]'
                )}
              >
                {label}
              </button>
            ))}
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-8 lg:grid-cols-12">
        <div className="lg:col-span-5 space-y-6">
          <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm h-full">
            <CardHeader>
              <CardTitle className="text-lg">Медиа файлы</CardTitle>
              <CardDescription className="text-white/40">
                Загрузите до {getMaxImages()} изображений для обработки
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div
                onDragEnter={handleDrag}
                onDragLeave={handleDrag}
                onDragOver={handleDrag}
                onDrop={handleDrop}
                className={cn(
                  'relative group border-2 border-dashed rounded-2xl p-12 text-center transition-all duration-500',
                  dragActive 
                    ? 'border-primary bg-primary/5 scale-[0.99]' 
                    : 'border-white/5 bg-white/[0.01] hover:border-white/10 hover:bg-white/[0.02]'
                )}
              >
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/*"
                  multiple
                  className="hidden"
                  onChange={handleFileChange}
                />
                <div className="flex flex-col items-center gap-4">
                  <div className="p-4 rounded-full bg-white/5 group-hover:scale-110 transition-transform duration-500">
                    <svg className="w-8 h-8 text-white/40" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                    </svg>
                  </div>
                  <div className="space-y-1">
                    <p className="text-sm font-medium text-white/80">Перетащите файлы</p>
                    <p className="text-xs text-white/40">PNG, JPG до 10MB</p>
                  </div>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => fileInputRef.current?.click()}
                    className="rounded-full px-6 bg-white/5 hover:bg-white/10 border-white/5 text-xs"
                  >
                    Выбрать на устройстве
                  </Button>
                </div>
              </div>

              {imagePreviews.length > 0 && (
                <div className="grid grid-cols-2 gap-3">
                  {imagePreviews.map((preview, idx) => (
                    <div key={idx} className="relative group aspect-square rounded-xl overflow-hidden border border-white/10">
                      <img src={preview} alt={`Preview ${idx}`} className="w-full h-full object-cover transition-transform duration-500 group-hover:scale-110" />
                      <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity duration-300 flex items-center justify-center">
                        <button
                          onClick={() => removeImage(idx)}
                          className="bg-destructive/80 hover:bg-destructive text-white rounded-full p-2 backdrop-blur-md transition-all scale-75 group-hover:scale-100"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                          </svg>
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        <div className="lg:col-span-7 space-y-6">
          <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm">
            <CardHeader>
              <CardTitle className="text-lg">Конфигурация</CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-3">
                <label className="text-xs font-semibold uppercase tracking-wider text-white/40 ml-1">Модель</label>
                <div className="relative group">
                  <select
                    className="w-full appearance-none rounded-2xl border border-white/5 bg-white/[0.03] px-4 py-3.5 text-sm transition-all focus:border-primary/50 focus:ring-0 hover:bg-white/[0.05]"
                    value={selectedModel}
                    onChange={(e: ChangeEvent<HTMLSelectElement>) => setSelectedModel(e.target.value)}
                  >
                    {filteredModels.map((model) => (
                      <option key={model.id} value={model.id} className="bg-[#0a0a0a]">
                        {model.name} — {model.token_cost} {model.token_cost === 1 ? 'токен' : 'токена'}
                      </option>
                    ))}
                  </select>
                  <div className="absolute right-4 top-1/2 -translate-y-1/2 pointer-events-none text-white/20 group-hover:text-white/40 transition-colors">
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                  </div>
                </div>
                {selectedModelInfo && (
                  <p className="text-xs text-white/30 ml-1 italic">{selectedModelInfo.description}</p>
                )}
              </div>

              <div className="space-y-3">
                <label className="text-xs font-semibold uppercase tracking-wider text-white/40 ml-1">Промпт <span className="text-primary">*</span></label>
                <textarea
                  className="w-full rounded-2xl border border-white/5 bg-white/[0.03] px-4 py-4 text-sm transition-all focus:border-primary/50 focus:ring-0 hover:bg-white/[0.05] min-h-[160px] resize-none"
                  placeholder="Опишите детально, что вы хотите получить в результате..."
                  value={prompt}
                  onChange={(e: ChangeEvent<HTMLTextAreaElement>) => setPrompt(e.target.value)}
                />
              </div>

              {error && (
                <div className="p-4 rounded-xl bg-destructive/10 border border-destructive/20 text-destructive text-xs animate-in fade-in slide-in-from-top-2">
                  {error}
                </div>
              )}

              <Button
                className="w-full py-7 rounded-2xl text-base font-bold shadow-[0_20px_40px_-15px_rgba(139,92,246,0.3)] transition-all hover:shadow-[0_25px_50px_-12px_rgba(139,92,246,0.4)] active:scale-[0.98]"
                onClick={handleGenerate}
                disabled={generating || uploadingImage || !user?.telegram_id}
              >
                {uploadingImage ? (
                  <div className="flex items-center gap-3">
                    <span className="h-5 w-5 animate-spin rounded-full border-2 border-white/20 border-t-white" />
                    Загрузка медиа...
                  </div>
                ) : generating ? (
                  <div className="flex items-center gap-3">
                    <span className="h-5 w-5 animate-spin rounded-full border-2 border-white/20 border-t-white" />
                    Создание магии...
                  </div>
                ) : (
                  <div className="flex items-center gap-2">
                    <Sparkles className="h-5 w-5" />
                    Сгенерировать
                  </div>
                )}
              </Button>
            </CardContent>
          </Card>
        </div>
      </div>

      <Card className="border-white/5 bg-white/[0.01] overflow-hidden">
        <CardHeader className="border-b border-white/5">
          <CardTitle className="text-lg">Визуализация</CardTitle>
          <CardDescription className="text-white/40">Результат вашей генерации появится здесь</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <div className="min-h-[400px] flex flex-col items-center justify-center p-8">
            {!currentGeneration && !generating && (
              <div className="text-center space-y-4 max-w-xs opacity-20 group">
                <div className="w-20 h-20 mx-auto rounded-full bg-white/5 flex items-center justify-center group-hover:scale-110 transition-transform duration-500">
                  <Sparkles className="w-8 h-8" />
                </div>
                <p className="text-sm font-medium">Готов к работе</p>
              </div>
            )}

            {generating && currentGeneration?.status !== 'completed' && (
              <div className="flex flex-col items-center justify-center gap-6 text-center animate-in fade-in duration-500">
                <div className="relative">
                  <div className="h-20 w-20 animate-spin rounded-full border-4 border-white/5 border-t-primary" />
                  <div className="absolute inset-0 flex items-center justify-center">
                    <div className="h-10 w-10 animate-pulse rounded-full bg-primary/20" />
                  </div>
                </div>
                <div className="space-y-2">
                  <p className="text-lg font-semibold text-white/90">
                    {currentGeneration?.status === 'processing' ? 'Нейросеть рисует...' : 'Подготовка запроса...'}
                  </p>
                  <p className="text-sm text-white/30">Обычно это занимает от 30 до 90 секунд</p>
                </div>
              </div>
            )}

            {currentGeneration?.status === 'completed' && currentGeneration.output && (
              <div className="w-full max-w-3xl space-y-6 animate-in zoom-in-95 fade-in duration-700">
                <div className="relative group rounded-2xl overflow-hidden border border-white/10 shadow-2xl">
                  {currentGeneration.output.includes('.mp4') || currentGeneration.output.includes('video') ? (
                    <video
                      src={currentGeneration.output}
                      controls
                      className="w-full h-auto"
                    />
                  ) : (
                    <img
                      src={currentGeneration.output}
                      alt="Generated"
                      className="w-full h-auto transition-transform duration-700 group-hover:scale-[1.02]"
                    />
                  )}
                  <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity duration-500 flex items-center justify-center gap-4">
                    <Button asChild variant="secondary" className="rounded-full px-6">
                      <a href={currentGeneration.output} target="_blank" rel="noopener noreferrer">
                        Просмотреть оригинал
                      </a>
                    </Button>
                  </div>
                </div>
              </div>
            )}

            {currentGeneration?.status === 'failed' && (
              <div className="flex flex-col items-center justify-center gap-4 text-center p-12 rounded-3xl bg-destructive/5 border border-destructive/10 max-w-md animate-in shake-1">
                <div className="p-4 rounded-full bg-destructive/10 text-destructive">
                  <svg className="w-10 h-10" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                </div>
                <div className="space-y-1">
                  <p className="text-lg font-bold text-destructive">Ошибка генерации</p>
                  <p className="text-sm text-white/40">
                    {currentGeneration.error_msg || 'Произошла непредвиденная ошибка при обработке запроса.'}
                  </p>
                </div>
                <Button variant="outline" size="sm" onClick={handleGenerate} className="mt-4 border-white/10 hover:bg-white/5">
                  Попробовать снова
                </Button>
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

import { useEffect, useState, useRef, ChangeEvent, DragEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { generationApi } from '../lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { cn } from '../lib/utils'
import { Sparkles, Image as ImageIcon, Video, Music, Type, ChevronDown, Plus, Minus, Info, X, ChevronRight, Mic } from 'lucide-react'

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
  image: 'Картинка',
  video: 'Видео',
  music: 'Музыка',
  text: 'Текст',
}

const CATEGORY_ICONS: Record<Category, any> = {
  image: ImageIcon,
  video: Video,
  music: Music,
  text: Type,
}

const ASPECT_RATIOS = [
  { id: 'auto', label: 'Автоматически', value: 'auto' },
  { id: '1:1', label: '1:1 (Квадрат)', value: '1:1' },
  { id: '16:9', label: '16:9 (Горизонтальный)', value: '16:9' },
  { id: '9:16', label: '9:16 (Вертикальный)', value: '9:16' },
  { id: '4:3', label: '4:3', value: '4:3' },
  { id: '3:4', label: '3:4', value: '3:4' },
]

const MAX_IMAGES_PER_MODEL: Record<string, number> = {
  'google/nano-banana': 1,
  'google/nano-banana-pro': 4,
  'nano-banana-2': 4,
  'seedream/4.5-edit': 4,
}

// Модели, требующие подписку
const SUBSCRIPTION_REQUIRED_MODELS: Record<string, string[]> = {
  'google/gemini-3-flash': ['start', 'pro'],
  'openai/gpt-5-mini': ['mini', 'start', 'pro'],
  'openai/gpt-5-nano': ['mini', 'start', 'pro'],
  'chat-gpt-4.1mini': ['start', 'pro'],
}

export function GeneratePage() {
  const navigate = useNavigate()
  const { isAuthenticated, user } = useAuthStore()
  const [allModels, setAllModels] = useState<Model[]>([])
  const [selectedCategory, setSelectedCategory] = useState<Category>('image')
  const [selectedModel, setSelectedModel] = useState<string>('')
  const [selectedAspectRatio, setSelectedAspectRatio] = useState('auto')
  const [numOutputs, setNumOutputs] = useState(1)
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

  const canAccessModel = (modelId: string): boolean => {
    const requiredSubscriptions = SUBSCRIPTION_REQUIRED_MODELS[modelId]
    if (!requiredSubscriptions) return true // Модель доступна всем
    if (!user?.subscription_type) return false // Нет подписки
    return requiredSubscriptions.includes(user.subscription_type)
  }

  const filteredModels = allModels.filter((m) => {
    if (m.type !== selectedCategory) return false
    return canAccessModel(m.id)
  })

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
        aspect_ratio: selectedAspectRatio !== 'auto' ? selectedAspectRatio : undefined,
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
    <div className="max-w-2xl mx-auto pb-20 lg:pb-8">
      {/* Category Header */}
      <div className="flex items-center justify-between mb-4 px-2">
        <div className="relative group">
          <button className="flex items-center gap-2 text-white/90 hover:text-white transition-colors py-1">
            {(() => {
              const Icon = CATEGORY_ICONS[selectedCategory]
              return <Icon className="w-4 h-4" />
            })()}
            <span className="text-sm font-medium">{CATEGORY_LABELS[selectedCategory]}</span>
            <ChevronDown className="w-3 h-3 opacity-50" />
          </button>
          
          <div className="absolute top-full left-0 mt-1 w-48 bg-[#0a0a0a] border border-white/10 rounded-xl shadow-2xl opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all z-50 overflow-hidden">
            {(Object.entries(CATEGORY_LABELS) as [Category, string][]).map(([cat, label]) => {
              const Icon = CATEGORY_ICONS[cat]
              return (
                <button
                  key={cat}
                  onClick={() => setSelectedCategory(cat)}
                  className={cn(
                    "w-full flex items-center gap-3 px-4 py-3 text-xs transition-colors hover:bg-white/5",
                    selectedCategory === cat ? "text-primary bg-primary/5" : "text-white/60"
                  )}
                >
                  <Icon className="w-3.5 h-3.5" />
                  {label}
                </button>
              )
            })}
          </div>
        </div>
        
        <button 
          onClick={() => navigate('/')}
          className="p-2 hover:bg-white/5 rounded-full transition-colors text-white/40 hover:text-white"
        >
          <X className="w-5 h-5" />
        </button>
      </div>

      <div className="space-y-2 px-2">
        {/* Model Selection */}
        <div className="space-y-1">
          <label className="text-[13px] font-medium text-white/90 ml-1">Модель</label>
          <div className="relative">
            <div className="w-full flex items-center rounded-xl border border-white/10 bg-white/[0.03] px-3 py-2.5 text-sm transition-all hover:bg-white/[0.05] cursor-pointer group">
              <div className="w-6 h-6 rounded-lg bg-white/10 flex items-center justify-center mr-2.5">
                <span className="text-[10px] font-bold text-white/40">G</span>
              </div>
              <Sparkles className="w-3.5 h-3.5 text-orange-400 mr-2 shrink-0" />
              <span className="text-[13px] text-white/80 truncate flex-1">{selectedModelInfo?.name || 'Выберите модель'}</span>
              <ChevronRight className="w-4 h-4 text-white/20 ml-2 shrink-0 group-hover:text-white/40 transition-colors" />
            </div>
            <select
              className="absolute inset-0 w-full opacity-0 cursor-pointer"
              value={selectedModel}
              onChange={(e: ChangeEvent<HTMLSelectElement>) => setSelectedModel(e.target.value)}
            >
              {filteredModels.map((model) => (
                <option key={model.id} value={model.id} className="bg-[#0a0a0a]">
                  {model.name}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Media Upload */}
        <div className="space-y-1">
          <label className="text-[13px] font-medium text-white/90 ml-1">Изображения</label>
          <div className="flex flex-wrap gap-2">
            <div
              onDragEnter={handleDrag}
              onDragLeave={handleDrag}
              onDragOver={handleDrag}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
              className={cn(
                "relative flex flex-col items-center justify-center w-24 h-32 rounded-xl border border-white/10 transition-all cursor-pointer group",
                dragActive 
                  ? "border-primary bg-primary/5" 
                  : "bg-white/[0.03] hover:bg-white/[0.05]"
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
              <ImageIcon className="w-5 h-5 text-white/20 mb-2 group-hover:scale-110 transition-transform" />
              <p className="text-[9px] text-white/25 text-center px-2 leading-tight">
                Загрузите одно или несколько изображений для редактирования.
              </p>
            </div>

            {imagePreviews.map((preview, idx) => (
              <div key={idx} className="relative w-24 h-32 rounded-xl overflow-hidden border border-white/10 shadow-lg">
                <img src={preview} alt={`Preview ${idx}`} className="w-full h-full object-cover" />
                <button
                  onClick={(e) => { e.stopPropagation(); removeImage(idx); }}
                  className="absolute top-1 right-1 p-1 bg-black/60 hover:bg-black/80 text-white rounded-lg backdrop-blur-md transition-all border border-white/5"
                >
                  <X className="w-2.5 h-2.5" />
                </button>
              </div>
            ))}
          </div>
        </div>

        {/* Prompt Section */}
        <div className="space-y-1">
          <div className="flex items-center gap-1 ml-1">
            <label className="text-[13px] font-medium text-white/90">Запрос</label>
            <span className="text-orange-400 font-bold">*</span>
          </div>
          <div className="relative">
            <textarea
              className="w-full rounded-xl border border-white/10 bg-white/[0.03] px-3.5 py-3 text-sm transition-all focus:border-primary/50 focus:ring-0 hover:bg-white/[0.05] min-h-[80px] max-h-[120px] resize-none"
              placeholder="Опишите, что должно быть на изображении."
              value={prompt}
              onChange={(e: ChangeEvent<HTMLTextAreaElement>) => setPrompt(e.target.value)}
            />
            <div className="absolute bottom-3 right-3">
              <button className="p-1.5 hover:bg-white/5 rounded-lg text-white/40 hover:text-white transition-colors">
                <Mic className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        {/* Aspect Ratio */}
        <div className="space-y-1">
          <label className="text-[13px] font-medium text-white/90 ml-1">Соотношение сторон</label>
          <div className="relative">
            <div className="w-full flex items-center rounded-xl border border-white/10 bg-white/[0.03] px-3.5 py-3 text-sm transition-all hover:bg-white/[0.05] cursor-pointer group">
              <span className="text-[13px] text-white/80 flex-1">{ASPECT_RATIOS.find(r => r.value === selectedAspectRatio)?.label}</span>
              <div className="p-1 rounded-md bg-[#2a2218] border border-orange-500/20 mr-2.5">
                <Info className="w-3.5 h-3.5 text-orange-400" />
              </div>
              <ChevronDown className="w-4 h-4 text-white/20 group-hover:text-white/40 transition-colors shrink-0" />
            </div>
            <select
              className="absolute inset-0 w-full opacity-0 cursor-pointer"
              value={selectedAspectRatio}
              onChange={(e) => setSelectedAspectRatio(e.target.value)}
            >
              {ASPECT_RATIOS.map((ratio) => (
                <option key={ratio.id} value={ratio.value} className="bg-[#0a0a0a]">
                  {ratio.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        {error && (
          <div className="p-3 rounded-xl bg-destructive/10 border border-destructive/20 text-destructive text-xs animate-in fade-in slide-in-from-top-2">
            {error}
          </div>
        )}

        {/* Sticky Footer */}
        <div className="fixed bottom-0 left-0 right-0 p-3 lg:relative lg:p-0 z-40 bg-black/80 backdrop-blur-xl border-t border-white/5 lg:border-none lg:bg-transparent">
          <div className="max-w-2xl mx-auto flex items-center gap-2">
            {/* Quantity Selector */}
            <div className="flex items-center bg-white/[0.05] border border-white/10 rounded-xl h-12 px-0.5">
              <button 
                onClick={() => setNumOutputs(Math.max(1, numOutputs - 1))}
                className="p-2.5 text-white/30 hover:text-white transition-colors"
              >
                <Minus className="w-3.5 h-3.5" />
              </button>
              <span className="w-8 text-center text-xs font-bold text-white/60">{numOutputs}/4</span>
              <button 
                onClick={() => setNumOutputs(Math.min(4, numOutputs + 1))}
                className="p-2.5 text-white/30 hover:text-white transition-colors"
              >
                <Plus className="w-3.5 h-3.5" />
              </button>
            </div>

            {/* Generate Button */}
            <Button
              className="flex-1 h-12 rounded-xl text-sm font-bold bg-gradient-to-r from-[#f7d570] via-[#f7b733] to-[#f7d570] text-black hover:opacity-95 transition-all shadow-[0_4px_15px_rgba(247,183,51,0.15)] active:scale-[0.98]"
              onClick={handleGenerate}
              disabled={generating || uploadingImage || !user?.telegram_id}
            >
              {uploadingImage ? (
                <div className="flex items-center gap-2">
                  <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-black/20 border-t-black" />
                  Загрузка...
                </div>
              ) : generating ? (
                <div className="flex items-center gap-2">
                  <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-black/20 border-t-black" />
                  Создание...
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <Sparkles className="h-3.5 w-3.5" />
                  Сгенерировать — {selectedModelInfo?.token_cost || 0}
                </div>
              )}
            </Button>
          </div>
        </div>
      </div>

      {/* Result Visualization - compact or modal style */}
      {(currentGeneration || generating) && (
        <Card className="mt-8 border-white/5 bg-white/[0.01] overflow-hidden">
          <CardHeader className="p-4 border-b border-white/5 flex flex-row items-center justify-between">
            <div>
              <CardTitle className="text-sm">Результат</CardTitle>
            </div>
            {currentGeneration?.status === 'completed' && (
              <button 
                onClick={() => setCurrentGeneration(null)}
                className="p-1 hover:bg-white/5 rounded-md transition-colors"
              >
                <X className="w-4 h-4 text-white/40" />
              </button>
            )}
          </CardHeader>
          <CardContent className="p-4">
            <div className="min-h-[200px] flex flex-col items-center justify-center">
              {generating && currentGeneration?.status !== 'completed' && (
                <div className="flex flex-col items-center gap-4 text-center">
                  <div className="relative">
                    <div className="h-12 w-12 animate-spin rounded-full border-2 border-white/5 border-t-primary" />
                  </div>
                  <p className="text-sm font-medium text-white/60">Нейросеть рисует...</p>
                </div>
              )}

              {currentGeneration?.status === 'completed' && currentGeneration.output && (
                <div className="w-full space-y-4">
                  <div className="relative group rounded-xl overflow-hidden border border-white/10">
                    {currentGeneration.output.includes('.mp4') || currentGeneration.output.includes('video') ? (
                      <video src={currentGeneration.output} controls className="w-full h-auto" />
                    ) : (
                      <img src={currentGeneration.output} alt="Generated" className="w-full h-auto" />
                    )}
                    <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center gap-3">
                      <Button asChild variant="secondary" size="sm" className="rounded-full">
                        <a href={currentGeneration.output} target="_blank" rel="noopener noreferrer">
                          Открыть
                        </a>
                      </Button>
                    </div>
                  </div>
                </div>
              )}

              {currentGeneration?.status === 'failed' && (
                <div className="flex flex-col items-center gap-3 text-center p-4">
                  <div className="p-3 rounded-full bg-destructive/10 text-destructive">
                    <Info className="w-6 h-6" />
                  </div>
                  <p className="text-sm font-bold text-destructive">Ошибка генерации</p>
                  <p className="text-xs text-white/40 max-w-xs">
                    {currentGeneration.error_msg || 'Попробуйте изменить запрос или выбрать другую модель.'}
                  </p>
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

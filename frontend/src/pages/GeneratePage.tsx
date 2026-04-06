import { useEffect, useState, useRef, ChangeEvent, DragEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { generationApi, GenerationCreateParams } from '../lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { cn, humanizeError } from '../lib/utils'
import { Sparkles, Image as ImageIcon, Video, Music, Type, ChevronDown, Info, X, ChevronRight, Mic, Download } from 'lucide-react'

async function downloadFile(url: string, filename: string) {
  try {
    const res = await fetch(url)
    const blob = await res.blob()
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = filename
    a.click()
    URL.revokeObjectURL(a.href)
  } catch {
    window.open(url, '_blank')
  }
}

type Category = 'image' | 'video' | 'music' | 'text'

interface Model {
  id: string
  name: string
  type: string
  description: string
  token_cost: number
}

interface MediaOutput {
  url: string
  urls?: string[] // Несколько файлов (2 песни Suno)
  type: string    // "audio" | "video" | "image"
  title?: string
  duration?: string
  thumbnail?: string
}

interface Generation {
  id: number
  status: string
  model_type?: string
  output?: string
  media_output?: MediaOutput
  error_msg?: string
}

const CATEGORY_LABELS: Record<Category, string> = {
  image: 'Картинка',
  video: 'Видео',
  music: 'Музыка',
  text: 'Текст',
}

const CATEGORY_PLACEHOLDERS: Record<Category, string> = {
  image: 'Опишите, что должно быть на изображении.',
  video: 'Опишите сцену, которую должна создать нейросеть.',
  music: 'Опишите, что должно быть в песне: жанр, настроение, тему.',
  text: 'Введите ваш запрос.',
}

const CATEGORY_GENERATING_TEXT: Record<Category, string> = {
  image: 'Нейросеть рисует...',
  video: 'Нейросеть снимает видео...',
  music: 'Нейросеть сочиняет...',
  text: 'Нейросеть думает...',
}

const CATEGORY_ICONS: Record<Category, any> = {
  image: ImageIcon,
  video: Video,
  music: Music,
  text: Type,
}

// Форматы для фото моделей (как в боте)
const PHOTO_ASPECT_RATIOS = [
  { id: '16:9', label: '16:9 Альбомный', value: '16:9' },
  { id: '9:16', label: '9:16 Портретный', value: '9:16' },
  { id: '1:1', label: '1:1 Квадрат', value: '1:1' },
]

// Форматы для Veo 3.1 Fast
const VEO_ASPECT_RATIOS = [
  { id: '16:9', label: '16:9 Альбомный', value: '16:9' },
  { id: '9:16', label: '9:16 Портретный', value: '9:16' },
  { id: 'auto', label: 'Авто', value: 'auto' },
]

// Разрешения для Nano Banana 2
const NANO_BANANA_2_RESOLUTIONS = [
  { id: '1K', label: '1K (2 генерации)', value: '1K', cost: 2 },
  { id: '2K', label: '2K (3 генерации)', value: '2K', cost: 3 },
  { id: '4K', label: '4K (4 генерации)', value: '4K', cost: 4 },
]

// Разрешения для Nano Banana Pro
const NANO_BANANA_PRO_RESOLUTIONS = [
  { id: '2K', label: '2K (4 генерации)', value: '2K', cost: 4 },
  { id: '5K', label: '5K (5 генераций)', value: '5K', cost: 5 },
]

// Длительность для Wan 2.6
const WAN_DURATIONS = [
  { id: '5', label: '5 сек (2 генерации)', value: '5', cost: 2 },
  { id: '10', label: '10 сек (4 генерации)', value: '10', cost: 4 },
  { id: '15', label: '15 сек (6 генераций)', value: '15', cost: 6 },
]

// Длительность для Kling 2.6
const KLING_DURATIONS = [
  { id: '5', label: '5 сек (1 генерация)', value: '5', cost: 1 },
  { id: '10', label: '10 сек (2 генерации)', value: '10', cost: 2 },
]

// Режимы для Suno Music
const SUNO_MODES = [
  { id: 'instrumental', label: 'Без голоса (инструментал)', value: 'instrumental' },
  { id: 'vocal', label: 'С голосом', value: 'vocal' },
]

// Голоса для Suno Music
const SUNO_VOICES = [
  { id: 'm', label: 'Мужской', value: 'm' },
  { id: 'f', label: 'Женский', value: 'f' },
]

const MAX_IMAGES_PER_MODEL: Record<string, number> = {
  'google/nano-banana': 1,
  'google/nano-banana-pro': 4,
  'nano-banana-2': 4,
  'seedream/4.5-edit': 4,
  'veo3_fast': 2,
}

// Модели, требующие подписку
const SUBSCRIPTION_REQUIRED_MODELS: Record<string, string[]> = {
  'google/gemini-3-flash': ['start', 'pro'],
  'openai/gpt-5-nano': ['mini', 'start', 'pro'],
}

export function GeneratePage() {
  const navigate = useNavigate()
  const { isAuthenticated, user, accessToken } = useAuthStore()
  const [allModels, setAllModels] = useState<Model[]>([])
  const [selectedCategory, setSelectedCategory] = useState<Category>('image')
  const [selectedModel, setSelectedModel] = useState<string>('')
  const [selectedAspectRatio, setSelectedAspectRatio] = useState('1:1')
  // Новые параметры для разных моделей
  const [selectedResolution, setSelectedResolution] = useState('2K')
  const [googleSearch, setGoogleSearch] = useState(false)
  const [videoDuration, setVideoDuration] = useState('5')
  const [withSound, setWithSound] = useState(false)
  const [sunoMode, setSunoMode] = useState('instrumental')
  const [sunoVoice, setSunoVoice] = useState('m')
  const [prompt, setPrompt] = useState('')
  const [chatHistory, setChatHistory] = useState<{ role: string; content: string }[]>([])
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
        setError(humanizeError(err, 'Не удалось загрузить модели. Попробуйте обновить страницу.'))
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
    setChatHistory([])
  }, [selectedCategory, filteredModels, selectedModel])

  // Устанавливаем дефолтные параметры при смене модели
  useEffect(() => {
    if (!selectedModel) return

    // Nano Banana Pro: дефолт 2K
    if (selectedModel === 'google/nano-banana-pro') {
      setSelectedResolution('2K')
    }
    // Nano Banana 2: дефолт 1K
    else if (selectedModel === 'nano-banana-2') {
      setSelectedResolution('1K')
    }
    // Kling 2.6: дефолт 5 сек
    else if (selectedModel === 'kling-2.6/image-to-video') {
      setVideoDuration('5')
    }
    // Wan 2.6: дефолт 5 сек
    else if (selectedModel === 'wan/2-6-image-to-video') {
      setVideoDuration('5')
    }
    // Veo 3.1 Fast: дефолт авто
    if (selectedModel === 'veo3_fast') {
      setSelectedAspectRatio('auto')
    }
  }, [selectedModel])

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

  const handleFilesChange = async (files: FileList | null) => {
    if (!files) return
    const maxImages = getMaxImages()
    const IMAGE_EXTS = /\.(jpe?g|png|webp|gif|heic|heif|avif|bmp)$/i
    const newFiles = Array.from(files).filter(f =>
      f.type.startsWith('image/') || (!f.type && IMAGE_EXTS.test(f.name))
    )
    
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

    try {
      // Используем Promise.all для корректной работы во всех браузерах
      const newPreviews = await Promise.all(
        filesToAdd.map((file) => {
          return new Promise<string>((resolve, reject) => {
            const reader = new FileReader()
            reader.onload = (e) => {
              if (e.target?.result) {
                resolve(e.target.result as string)
              } else {
                reject(new Error('Не удалось прочитать файл'))
              }
            }
            reader.onerror = () => {
              reject(new Error('Ошибка чтения файла'))
            }
            reader.readAsDataURL(file)
          })
        })
      )

      setImageFiles([...imageFiles, ...filesToAdd])
      setImagePreviews([...imagePreviews, ...newPreviews])
    } catch (err: any) {
      console.error('Error reading files:', err)
      setError(err?.message || 'Ошибка загрузки изображений. Попробуйте снова.')
    }
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

    if (!accessToken) {
      throw new Error('Не авторизованы')
    }

    // Upload through our backend instead of direct imgur
    // This avoids IP blocking issues
    const response = await fetch('/api/upload-image', {
      method: 'POST',
      body: formData,
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
    })

    if (!response.ok) {
      if (response.status === 413) {
        throw new Error('Файл слишком большой (макс. 15 МБ)')
      }
      const text = await response.text()
      let msg = 'Не удалось загрузить изображение'
      try { msg = JSON.parse(text).error || msg } catch {}
      throw new Error(msg)
    }

    const data = await response.json()
    return data.url
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
        } catch (uploadErr: any) {
          console.error('Upload error:', uploadErr)
          setError(uploadErr?.message || 'Ошибка загрузки изображения')
          setGenerating(false)
          setUploadingImage(false)
          return
        }
        setUploadingImage(false)
      }

      // Собираем параметры в зависимости от модели
      const params: GenerationCreateParams = {
        model: selectedModel,
        prompt: prompt.trim(),
        image_urls: imageUrls,
      }

      // Формат для фото моделей и Veo
      if (selectedCategory === 'image' || selectedModel === 'veo3_fast') {
        if (selectedAspectRatio !== 'auto') {
          params.aspect_ratio = selectedAspectRatio
        }
      }

      // Nano Banana 2: разрешение и Google поиск
      if (selectedModel === 'nano-banana-2') {
        params.resolution = selectedResolution
        params.google_search = googleSearch ? 'true' : 'false'
      }

      // Nano Banana Pro: разрешение
      if (selectedModel === 'google/nano-banana-pro') {
        params.resolution = selectedResolution
      }

      // Wan 2.6: длительность (разрешение всегда 1080p)
      if (selectedModel === 'wan/2-6-image-to-video') {
        params.duration = videoDuration
        params.resolution = '1080p'
      }

      // Kling 2.6: длительность и звук
      if (selectedModel === 'kling-2.6/image-to-video') {
        params.duration = videoDuration
        params.sound = withSound ? 'true' : 'false'
      }

      // Suno Music: режим и голос
      if (selectedModel === 'music-suno') {
        params.instrumental = sunoMode === 'instrumental'
        if (sunoMode === 'vocal') {
          params.vocal_gender = sunoVoice
        }
      }

      // Текстовые модели: передаём историю чата
      const currentPrompt = prompt.trim()
      if (selectedCategory === 'text') {
        const newHistory = [...chatHistory, { role: 'user', content: currentPrompt }]
        params.messages = newHistory
      }

      const result = await generationApi.create(params)

      setCurrentGeneration({ id: result.id, status: result.status })

      pollRef.current = setInterval(async () => {
        try {
          const status = await generationApi.getStatus(result.id) as Generation
          setCurrentGeneration(status)

          if (status.status === 'completed' || status.status === 'failed') {
            if (pollRef.current) clearInterval(pollRef.current)
            setGenerating(false)
            // Добавляем сообщения в историю чата
            if (selectedCategory === 'text' && status.status === 'completed' && status.output) {
              setChatHistory(prev => [
                ...prev,
                { role: 'user', content: currentPrompt },
                { role: 'assistant', content: status.output! },
              ])
              setPrompt('')
            }
          }
        } catch (err) {
          console.error('Poll error:', err)
        }
      }, 3000)
    } catch (err: unknown) {
      setError(humanizeError(err, 'Ошибка при создании генерации. Попробуйте ещё раз.'))
      setGenerating(false)
    }
  }

  const selectedModelInfo = filteredModels.find(m => m.id === selectedModel)

  // Расчет динамической стоимости в зависимости от параметров
  const calculateCost = (): number => {
    let baseCost = selectedModelInfo?.token_cost || 0

    // Nano Banana 2: стоимость зависит от разрешения
    if (selectedModel === 'nano-banana-2') {
      const res = NANO_BANANA_2_RESOLUTIONS.find(r => r.value === selectedResolution)
      if (res) baseCost = res.cost
    }

    // Nano Banana Pro: стоимость зависит от разрешения
    if (selectedModel === 'google/nano-banana-pro') {
      const res = NANO_BANANA_PRO_RESOLUTIONS.find(r => r.value === selectedResolution)
      if (res) baseCost = res.cost
    }

    // Wan 2.6: стоимость зависит от длительности
    if (selectedModel === 'wan/2-6-image-to-video') {
      const dur = WAN_DURATIONS.find(d => d.value === videoDuration)
      if (dur) baseCost = dur.cost
    }

    // Kling 2.6: стоимость зависит от длительности и звука
    if (selectedModel === 'kling-2.6/image-to-video') {
      const dur = KLING_DURATIONS.find(d => d.value === videoDuration)
      if (dur) baseCost = dur.cost
      if (withSound) baseCost *= 2 // Со звуком цена x2
    }

    // Suno Music: всегда 1 генерация
    if (selectedModel === 'music-suno') {
      baseCost = 1
    }

    return baseCost
  }

  const totalCost = calculateCost()

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
          <label className="text-[13px] lg:text-sm font-medium text-white/90 ml-1">Модель</label>
          <div className="relative">
            <div className="w-full flex items-center rounded-xl border border-white/10 bg-white/[0.03] px-3 lg:px-4 py-2.5 lg:py-3.5 text-sm lg:text-base transition-all hover:bg-white/[0.05] cursor-pointer group">
              <div className="w-6 h-6 lg:w-7 lg:h-7 rounded-lg bg-white/10 flex items-center justify-center mr-2.5">
                <span className="text-[10px] lg:text-xs font-bold text-white/40">G</span>
              </div>
              <Sparkles className="w-3.5 h-3.5 lg:w-4 lg:h-4 text-orange-400 mr-2 shrink-0" />
              <span className="text-[13px] lg:text-base text-white/80 truncate flex-1">{selectedModelInfo?.name || 'Выберите модель'}</span>
              <ChevronRight className="w-4 h-4 lg:w-5 lg:h-5 text-white/20 ml-2 shrink-0 group-hover:text-white/40 transition-colors" />
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

        {/* Media Upload - скрыто для музыки и текста */}
        {selectedCategory !== 'music' && selectedCategory !== 'text' && (
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
        )}

        {/* Prompt Section */}
        <div className="space-y-1">
          <div className="flex items-center gap-1 ml-1">
            <label className="text-[13px] lg:text-sm font-medium text-white/90">Запрос</label>
            <span className="text-orange-400 font-bold">*</span>
          </div>
          <div className="relative">
            <textarea
              className="w-full rounded-xl border border-white/10 bg-white/[0.03] px-3.5 lg:px-4 py-3 lg:py-4 text-sm lg:text-base transition-all focus:border-primary/50 focus:ring-0 hover:bg-white/[0.05] min-h-[80px] lg:min-h-[100px] max-h-[120px] lg:max-h-[150px] resize-none"
              placeholder={CATEGORY_PLACEHOLDERS[selectedCategory]}
              value={prompt}
              onChange={(e: ChangeEvent<HTMLTextAreaElement>) => setPrompt(e.target.value)}
            />
            <div className="absolute bottom-3 lg:bottom-4 right-3 lg:right-4">
              <button className="p-1.5 lg:p-2 hover:bg-white/5 rounded-lg text-white/40 hover:text-white transition-colors">
                <Mic className="w-4 h-4 lg:w-5 lg:h-5" />
              </button>
            </div>
          </div>
        </div>

        {/* Aspect Ratio - для фото моделей */}
        {selectedCategory === 'image' && (
          <div className="space-y-1">
            <label className="text-[13px] lg:text-sm font-medium text-white/90 ml-1">Формат</label>
            <div className="flex gap-2">
              {PHOTO_ASPECT_RATIOS.map((ratio) => (
                <button
                  key={ratio.id}
                  onClick={() => setSelectedAspectRatio(ratio.value)}
                  className={cn(
                    "flex-1 py-2.5 lg:py-3.5 px-3 lg:px-4 rounded-xl border text-xs lg:text-sm font-medium transition-all",
                    selectedAspectRatio === ratio.value
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-white/10 bg-white/[0.03] text-white/60 hover:bg-white/[0.05]"
                  )}
                >
                  {ratio.label}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Разрешение для Nano Banana 2 */}
        {selectedModel === 'nano-banana-2' && (
          <>
            <div className="space-y-1">
              <label className="text-[13px] lg:text-sm font-medium text-white/90 ml-1">Разрешение</label>
              <div className="flex gap-2">
                {NANO_BANANA_2_RESOLUTIONS.map((res) => (
                  <button
                    key={res.id}
                    onClick={() => setSelectedResolution(res.value)}
                    className={cn(
                      "flex-1 py-2.5 lg:py-3.5 px-3 lg:px-4 rounded-xl border text-xs lg:text-sm font-medium transition-all",
                      selectedResolution === res.value
                        ? "border-primary bg-primary/10 text-primary"
                        : "border-white/10 bg-white/[0.03] text-white/60 hover:bg-white/[0.05]"
                    )}
                  >
                    {res.label}
                  </button>
                ))}
              </div>
            </div>
            <div className="flex items-center gap-3 p-3 lg:p-4 rounded-xl border border-white/10 bg-white/[0.03]">
              <input
                type="checkbox"
                id="googleSearch"
                checked={googleSearch}
                onChange={(e) => setGoogleSearch(e.target.checked)}
                className="w-4 h-4 lg:w-5 lg:h-5 rounded border-white/20 bg-white/5 text-primary focus:ring-primary/50"
              />
              <label htmlFor="googleSearch" className="text-[13px] lg:text-sm text-white/80 cursor-pointer">
                Google поиск (улучшает качество)
              </label>
            </div>
          </>
        )}

        {/* Разрешение для Nano Banana Pro */}
        {selectedModel === 'google/nano-banana-pro' && (
          <div className="space-y-1">
            <label className="text-[13px] lg:text-sm font-medium text-white/90 ml-1">Разрешение</label>
            <div className="flex gap-2">
              {NANO_BANANA_PRO_RESOLUTIONS.map((res) => (
                <button
                  key={res.id}
                  onClick={() => setSelectedResolution(res.value)}
                  className={cn(
                    "flex-1 py-2.5 lg:py-3.5 px-3 lg:px-4 rounded-xl border text-xs lg:text-sm font-medium transition-all",
                    selectedResolution === res.value
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-white/10 bg-white/[0.03] text-white/60 hover:bg-white/[0.05]"
                  )}
                >
                  {res.label}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Формат для Veo 3.1 Fast */}
        {selectedModel === 'veo3_fast' && (
          <div className="space-y-1">
            <label className="text-[13px] lg:text-sm font-medium text-white/90 ml-1">Формат видео</label>
            <div className="flex gap-2">
              {VEO_ASPECT_RATIOS.map((ratio) => (
                <button
                  key={ratio.id}
                  onClick={() => setSelectedAspectRatio(ratio.value)}
                  className={cn(
                    "flex-1 py-2.5 lg:py-3.5 px-3 lg:px-4 rounded-xl border text-xs lg:text-sm font-medium transition-all",
                    selectedAspectRatio === ratio.value
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-white/10 bg-white/[0.03] text-white/60 hover:bg-white/[0.05]"
                  )}
                >
                  {ratio.label}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Длительность для Wan 2.6 */}
        {selectedModel === 'wan/2-6-image-to-video' && (
          <div className="space-y-1">
            <label className="text-[13px] lg:text-sm font-medium text-white/90 ml-1">Длительность (разрешение 1080p)</label>
            <div className="flex gap-2">
              {WAN_DURATIONS.map((dur) => (
                <button
                  key={dur.id}
                  onClick={() => setVideoDuration(dur.value)}
                  className={cn(
                    "flex-1 py-2.5 lg:py-3.5 px-3 lg:px-4 rounded-xl border text-xs lg:text-sm font-medium transition-all",
                    videoDuration === dur.value
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-white/10 bg-white/[0.03] text-white/60 hover:bg-white/[0.05]"
                  )}
                >
                  {dur.label}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Длительность и звук для Kling 2.6 */}
        {selectedModel === 'kling-2.6/image-to-video' && (
          <>
            <div className="space-y-1">
              <label className="text-[13px] lg:text-sm font-medium text-white/90 ml-1">Длительность</label>
              <div className="flex gap-2">
                {KLING_DURATIONS.map((dur) => (
                  <button
                    key={dur.id}
                    onClick={() => setVideoDuration(dur.value)}
                    className={cn(
                      "flex-1 py-2.5 lg:py-3.5 px-3 lg:px-4 rounded-xl border text-xs lg:text-sm font-medium transition-all",
                      videoDuration === dur.value
                        ? "border-primary bg-primary/10 text-primary"
                        : "border-white/10 bg-white/[0.03] text-white/60 hover:bg-white/[0.05]"
                    )}
                  >
                    {dur.label}
                  </button>
                ))}
              </div>
            </div>
            <div className="flex items-center gap-3 p-3 lg:p-4 rounded-xl border border-white/10 bg-white/[0.03]">
              <input
                type="checkbox"
                id="withSound"
                checked={withSound}
                onChange={(e) => setWithSound(e.target.checked)}
                className="w-4 h-4 lg:w-5 lg:h-5 rounded border-white/20 bg-white/5 text-primary focus:ring-primary/50"
              />
              <label htmlFor="withSound" className="text-[13px] lg:text-sm text-white/80 cursor-pointer">
                Со звуком (цена ×2)
              </label>
            </div>
          </>
        )}

        {/* Режимы для Suno Music */}
        {selectedModel === 'music-suno' && (
          <>
            <div className="space-y-1">
              <label className="text-[13px] lg:text-sm font-medium text-white/90 ml-1">Режим</label>
              <div className="flex gap-2">
                {SUNO_MODES.map((mode) => (
                  <button
                    key={mode.id}
                    onClick={() => setSunoMode(mode.value)}
                    className={cn(
                      "flex-1 py-2.5 lg:py-3.5 px-3 lg:px-4 rounded-xl border text-xs lg:text-sm font-medium transition-all",
                      sunoMode === mode.value
                        ? "border-primary bg-primary/10 text-primary"
                        : "border-white/10 bg-white/[0.03] text-white/60 hover:bg-white/[0.05]"
                    )}
                  >
                    {mode.label}
                  </button>
                ))}
              </div>
            </div>
            {sunoMode === 'vocal' && (
              <div className="space-y-1">
                <label className="text-[13px] lg:text-sm font-medium text-white/90 ml-1">Голос</label>
                <div className="flex gap-2">
                  {SUNO_VOICES.map((voice) => (
                    <button
                      key={voice.id}
                      onClick={() => setSunoVoice(voice.value)}
                      className={cn(
                        "flex-1 py-2.5 lg:py-3.5 px-3 lg:px-4 rounded-xl border text-xs lg:text-sm font-medium transition-all",
                        sunoVoice === voice.value
                          ? "border-primary bg-primary/10 text-primary"
                          : "border-white/10 bg-white/[0.03] text-white/60 hover:bg-white/[0.05]"
                      )}
                    >
                      {voice.label}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </>
        )}

        {error && (
          <div className="p-3 rounded-xl bg-destructive/10 border border-destructive/20 text-destructive text-xs animate-in fade-in slide-in-from-top-2">
            {error}
          </div>
        )}

        {/* Generate Button Section */}
        <div className="mt-10 mb-4 px-2">
          <div className="max-w-2xl mx-auto">
            <Button
              className="w-full h-16 rounded-2xl text-lg font-black bg-gradient-to-r from-[#FFD700] via-[#FFB700] to-[#FF8C00] text-black hover:opacity-90 transition-all shadow-[0_8px_30px_rgba(255,183,0,0.3)] active:scale-[0.95] flex items-center justify-center gap-3"
              onClick={handleGenerate}
              disabled={generating || uploadingImage}
            >
              {uploadingImage ? (
                <>
                  <span className="h-5 w-5 animate-spin rounded-full border-2 border-black/20 border-t-black" />
                  <span>Загрузка...</span>
                </>
              ) : generating ? (
                <>
                  <span className="h-5 w-5 animate-spin rounded-full border-2 border-black/20 border-t-black" />
                  <span>Создание...</span>
                </>
              ) : (
                <>
                  <Sparkles className="h-6 w-6 fill-black" />
                  <span>Сгенерировать — {totalCost}</span>
                </>
              )}
            </Button>
          </div>
        </div>
      </div>

      {/* История чата для текстовых моделей */}
      {selectedCategory === 'text' && chatHistory.length > 0 && (
        <Card className="mt-8 border-white/5 bg-white/[0.01] overflow-hidden">
          <CardHeader className="p-4 border-b border-white/5 flex flex-row items-center justify-between">
            <CardTitle className="text-sm">История чата</CardTitle>
            <button onClick={() => setChatHistory([])} className="p-1 hover:bg-white/5 rounded-md transition-colors">
              <X className="w-4 h-4 text-white/40" />
            </button>
          </CardHeader>
          <CardContent className="p-4 space-y-3 max-h-[400px] overflow-y-auto">
            {chatHistory.map((msg, i) => (
              <div key={i} className={cn('flex', msg.role === 'user' ? 'justify-end' : 'justify-start')}>
                <div className={cn(
                  'max-w-[85%] rounded-xl px-4 py-2.5 text-sm whitespace-pre-wrap leading-relaxed',
                  msg.role === 'user'
                    ? 'bg-primary/20 text-white border border-primary/20'
                    : 'bg-white/[0.04] text-white/80 border border-white/5'
                )}>
                  {msg.content}
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {/* Result Visualization - compact or modal style */}
      {(currentGeneration || generating) && (
        <Card className="mt-4 border-white/5 bg-white/[0.01] overflow-hidden">
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
                  <p className="text-sm font-medium text-white/60">{CATEGORY_GENERATING_TEXT[selectedCategory]}</p>
                </div>
              )}

              {currentGeneration?.status === 'completed' && (currentGeneration.output || currentGeneration.media_output) && (() => {
                const modelType = currentGeneration.model_type || selectedCategory
                const mediaType = currentGeneration.media_output?.type
                const outputUrl = currentGeneration.media_output?.url || currentGeneration.output || ''

                // Текстовый результат
                if (modelType === 'text') {
                  return (
                    <div className="w-full">
                      <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4 text-sm text-white/80 whitespace-pre-wrap leading-relaxed">
                        {currentGeneration.output}
                      </div>
                    </div>
                  )
                }

                // Аудио (музыка)
                if (modelType === 'music' || mediaType === 'audio' || outputUrl.includes('.mp3') || outputUrl.includes('audio')) {
                  const audioUrls = currentGeneration.media_output?.urls && currentGeneration.media_output.urls.length > 0
                    ? currentGeneration.media_output.urls
                    : [outputUrl]
                  return (
                    <div className="w-full space-y-4">
                      {currentGeneration.media_output?.title && (
                        <p className="text-sm font-medium text-white/70 px-1">{currentGeneration.media_output.title}</p>
                      )}
                      {audioUrls.map((url, i) => (
                        <div key={i} className="space-y-1">
                          {audioUrls.length > 1 && (
                            <p className="text-xs text-white/40 px-1">Вариант {i + 1}</p>
                          )}
                          <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
                            <audio controls className="w-full" src={url}>
                              Ваш браузер не поддерживает аудио.
                            </audio>
                          </div>
                          <button
                            onClick={() => downloadFile(url, `audio_${i + 1}.mp3`)}
                            className="inline-flex items-center gap-1.5 text-xs bg-white/5 hover:bg-white/10 border border-white/10 rounded-lg px-3 py-1.5 text-white/70 hover:text-white transition-colors mt-1">
                            <Download className="h-3 w-3" />
                            Скачать вариант {i + 1}
                          </button>
                        </div>
                      ))}
                    </div>
                  )
                }

                // Видео
                if (modelType === 'video' || mediaType === 'video' || outputUrl.includes('.mp4') || outputUrl.includes('video')) {
                  return (
                    <div className="w-full space-y-2">
                      <div className="rounded-xl overflow-hidden border border-white/10">
                        <video src={outputUrl} controls className="w-full h-auto" />
                      </div>
                      <button
                        onClick={() => downloadFile(outputUrl, 'video.mp4')}
                        className="inline-flex items-center gap-1.5 text-xs bg-white/5 hover:bg-white/10 border border-white/10 rounded-lg px-3 py-1.5 text-white/70 hover:text-white transition-colors">
                        <Download className="h-3 w-3" />
                        Скачать видео
                      </button>
                    </div>
                  )
                }

                // Изображение (default)
                return (
                  <div className="w-full space-y-2">
                    <div className="flex items-center justify-between gap-2">
                      <a href={outputUrl} target="_blank" rel="noopener noreferrer" className="text-xs text-white/40 hover:text-white/70 transition-colors">
                        Открыть в полном размере ↗
                      </a>
                      <Button
                        size="sm"
                        variant="secondary"
                        className="rounded-full h-7 text-xs bg-white/5 border border-white/10 hover:bg-white/10"
                        onClick={() => downloadFile(outputUrl, 'generated.jpg')}
                      >
                        <Download className="h-3.5 w-3.5 mr-1" />
                        Скачать
                      </Button>
                    </div>
                    <div className="rounded-xl overflow-hidden border border-white/10">
                      <img src={outputUrl} alt="Generated" className="w-full h-auto" />
                    </div>
                  </div>
                )
              })()}

              {currentGeneration?.status === 'failed' && (
                <div className="flex flex-col items-center gap-3 text-center p-4">
                  <div className="p-3 rounded-full bg-destructive/10 text-destructive">
                    <Info className="w-6 h-6" />
                  </div>
                  <p className="text-sm font-bold text-destructive">Ошибка генерации</p>
                  <p className="text-xs text-white/40 max-w-xs">
                    {currentGeneration.error_msg
                      ? humanizeError(new Error(currentGeneration.error_msg), currentGeneration.error_msg)
                      : 'Попробуйте изменить запрос или выбрать другую модель.'}
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

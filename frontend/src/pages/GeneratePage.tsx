import { useEffect, useState, useRef, ChangeEvent, DragEvent } from 'react'
import { useLocation } from 'react-router-dom'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { generationApi, userApi, GenerationCreateParams } from '../lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { cn, humanizeError } from '../lib/utils'
import { Sparkles, Image as ImageIcon, Video, Music, Type, ChevronDown, Info, X, ChevronRight, Download, ShoppingCart, Zap, Crown } from 'lucide-react'
import { ImageLibraryPicker } from '../components/ImageLibraryPicker'

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

const MOTION_CONTROL_PLACEHOLDER = 'Опишите желаемый результат. Например, стиль, настроение или детали движения.'

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
  { id: '5', label: '5 сек (20 токенов)', value: '5', cost: 20 },
  { id: '10', label: '10 сек (40 токенов)', value: '10', cost: 40 },
  { id: '15', label: '15 сек (60 токенов)', value: '15', cost: 60 },
]

// Длительность для Kling 2.6
const KLING_DURATIONS = [
  { id: '5', label: '5 сек (10 токенов)', value: '5', cost: 10 },
  { id: '10', label: '10 сек (20 токенов)', value: '10', cost: 20 },
]

// Режимы для Kling 2.6 Motion Control
const KLING_MOTION_MODES = [
  { id: '720p', label: '720p', value: '720p' },
  { id: '1080p', label: '1080p', value: '1080p' },
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
  'kling-2.6/motion-control': 1,
}

// Модели, требующие подписку
const SUBSCRIPTION_REQUIRED_MODELS: Record<string, string[]> = {
  'google/gemini-3-flash': ['start', 'pro'],
  'openai/gpt-5-nano': ['mini', 'start', 'pro'],
}

export function GeneratePage() {
  const navigate = useNavigate()
  const location = useLocation()
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
  const [motionVideoFile, setMotionVideoFile] = useState<File | null>(null)
  const [motionVideoPreview, setMotionVideoPreview] = useState<string | null>(null)
  const [motionVideoUrl, setMotionVideoUrl] = useState<string | null>(null) // pre-filled URL from trend (no upload needed)
  const [motionDuration, setMotionDuration] = useState(5)
  const [prompt, setPrompt] = useState('')
  const [chatHistory, setChatHistory] = useState<{ role: string; content: string }[]>([])
  const [imageFiles, setImageFiles] = useState<File[]>([])
  const [imagePreviews, setImagePreviews] = useState<string[]>([])
  const [uploadingImage, setUploadingImage] = useState(false)
  const [loading, setLoading] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [quota, setQuota] = useState<{ text_daily: number; text_extra: number; image_weekly: number; image_extra: number; music_weekly: number; music_extra: number; video_weekly: number; video_extra: number } | null>(null)
  const [quotaError, setQuotaError] = useState(false)
  const [currentGeneration, setCurrentGeneration] = useState<Generation | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [dragActive, setDragActive] = useState(false)
  const [showModelPicker, setShowModelPicker] = useState(false)
  const [showLibraryPicker, setShowLibraryPicker] = useState(false)
  const [libraryUrls, setLibraryUrls] = useState<string[]>([]) // URLs выбранных из библиотеки
  const fileInputRef = useRef<HTMLInputElement>(null)
  const motionVideoInputRef = useRef<HTMLInputElement>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/login', { state: { from: '/generate' } })
      return
    }

    const state = location.state as { prompt?: string; model?: string; category?: string; imageUrl?: string; motionVideoUrl?: string } | null
    if (state?.prompt) setPrompt(state.prompt)
    if (state?.category && ['image', 'video', 'music', 'text'].includes(state.category)) {
      setSelectedCategory(state.category as Category)
    }
    // If imageUrl is provided, add it to libraryUrls for video models
    if (state?.imageUrl) {
      setLibraryUrls([state.imageUrl])
    }
    // If motionVideoUrl is provided (from trend), pre-fill the reference video
    if (state?.motionVideoUrl) {
      setMotionVideoPreview(state.motionVideoUrl)
      setMotionVideoUrl(state.motionVideoUrl)
    }

    setLoading(true)
    generationApi.getModels()
      .then((data) => {
        setAllModels(data)
        if (state?.model) {
          setSelectedModel(state.model)
        } else {
          const cat = state?.category || 'image'
          const catModels = data.filter((m: Model) => m.type === cat)
          if (catModels.length > 0) setSelectedModel(catModels[0].id)
        }
      })
      .catch((err) => {
        setError(humanizeError(err, 'Не удалось загрузить модели. Попробуйте обновить страницу.'))
        console.error(err)
      })
      .finally(() => setLoading(false))

    userApi.getQuota()
      .then((q: any) => setQuota(q))
      .catch(() => {})

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

  // Сброс медиа и промта при смене категории
  const prevCategoryRef = useRef<Category | null>(null)
  useEffect(() => {
    if (prevCategoryRef.current !== null && prevCategoryRef.current !== selectedCategory) {
      setPrompt('')
      setImageFiles([])
      setImagePreviews([])
      setLibraryUrls([])
      setChatHistory([])
      setMotionVideoFile(null)
      setMotionVideoPreview(null)
      setMotionVideoUrl(null)
      setMotionDuration(0)
      setError(null)
    }
    prevCategoryRef.current = selectedCategory
  }, [selectedCategory])

  // Корректировка выбранной модели при смене категории или недоступности модели
  useEffect(() => {
    if (filteredModels.length > 0 && !filteredModels.find(m => m.id === selectedModel)) {
      setSelectedModel(filteredModels[0].id)
    }
  }, [selectedCategory, selectedModel])

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
    // Kling 2.6 Motion Control: дефолт 720p
    else if (selectedModel === 'kling-2.6/motion-control') {
      setSelectedResolution('720p')
      setMotionDuration(0)
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

  const uploadVideoToServer = async (file: File): Promise<string> => {
    const formData = new FormData()
    formData.append('video', file)
    if (!accessToken) throw new Error('Не авторизованы')
    const response = await fetch('/api/upload-video', {
      method: 'POST',
      body: formData,
      headers: { Authorization: `Bearer ${accessToken}` },
    })
    if (!response.ok) {
      const text = await response.text()
      let msg = 'Не удалось загрузить видео'
      try { msg = JSON.parse(text).error || msg } catch {}
      throw new Error(msg)
    }
    const data = await response.json()
    return data.url
  }

  const handleGenerate = async () => {
    const isPromptOptional = selectedModel === 'kling-2.6/motion-control'
    if (!selectedModel || (!prompt.trim() && !isPromptOptional)) {
      setError('Выберите модель и введите промпт')
      return
    }

    // Safety guard: never allow generation with 0 cost
    if (totalCost === 0) {
      setError('Не удалось определить стоимость генерации. Попробуйте выбрать модель заново.')
      return
    }

    if ((selectedCategory === 'image' || selectedCategory === 'video') && imageFiles.length === 0 && libraryUrls.length === 0) {
      setError('Для генерации необходимо загрузить входное фото')
      return
    }

    setError(null)
    setQuotaError(false)
    setGenerating(true)
    setCurrentGeneration(null)

    try {
      let imageUrls: string[] | undefined

      if (imageFiles.length > 0 || libraryUrls.length > 0) {
        let uploadedUrls: string[] = [...libraryUrls]
        if (imageFiles.length > 0) {
          setUploadingImage(true)
          try {
            const newUrls = await Promise.all(
              imageFiles.map(file => uploadImageToImgur(file))
            )
            uploadedUrls = [...uploadedUrls, ...newUrls]
          } catch (uploadErr: any) {
            console.error('Upload error:', uploadErr)
            setError(uploadErr?.message || 'Ошибка загрузки изображения')
            setGenerating(false)
            setUploadingImage(false)
            return
          }
          setUploadingImage(false)
        }
        imageUrls = uploadedUrls
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

      // Kling 2.6 Motion Control: длительность, режим, опорное видео
      if (selectedModel === 'kling-2.6/motion-control') {
        params.duration = String(motionDuration > 0 ? motionDuration : 5)
        params.resolution = selectedResolution
        if (motionVideoFile) {
          setUploadingImage(true)
          try {
            const videoUrl = await uploadVideoToServer(motionVideoFile)
            params.video_urls = [videoUrl]
          } catch (uploadErr: any) {
            setError(uploadErr?.message || 'Ошибка загрузки опорного видео')
            setGenerating(false)
            setUploadingImage(false)
            return
          }
          setUploadingImage(false)
        } else if (motionVideoUrl) {
          params.video_urls = [motionVideoUrl]
        }
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
            setQuotaError(false)
            refreshQuota()
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
      const msg = err instanceof Error ? err.message : String(err ?? '')
      if (/insufficient quota/i.test(msg)) {
        setQuotaError(true)
        refreshQuota()
      }
      setError(humanizeError(err, 'Ошибка при создании генерации. Попробуйте ещё раз.'))
      setGenerating(false)
    }
  }

  // Обновляем квоту после каждой генерации
  const refreshQuota = () => {
    userApi.getQuota().then((q: any) => setQuota(q)).catch(() => {})
  }

  // Считаем оставшуюся квоту для текущей категории
  const getCategoryQuota = (): number => {
    if (!quota) return -1
    switch (selectedCategory) {
      case 'text':  return quota.text_daily + quota.text_extra
      case 'image': return quota.image_weekly + quota.image_extra
      case 'music': return quota.music_weekly + quota.music_extra
      case 'video': return quota.video_weekly + quota.video_extra
      default:      return -1
    }
  }
  const currentQuota = getCategoryQuota()

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

    // Kling 2.6 Motion Control: 720p=1 токен/сек, 1080p=ceil(1.5 токена/сек)
    // Используем тот же fallback 5 сек что и в params.duration при отправке
    if (selectedModel === 'kling-2.6/motion-control') {
      const dur = motionDuration > 0 ? motionDuration : 5
      if (selectedResolution === '1080p') {
        baseCost = Math.ceil(dur * 1.5)
      } else {
        baseCost = dur
      }
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

  const handleLibrarySelect = (urls: string[]) => {
    if (!urls.length) return
    const maxImages = MAX_IMAGES_PER_MODEL[selectedModel] || 4
    setLibraryUrls(prev => {
      const remaining = maxImages - imageFiles.length - prev.length
      if (remaining <= 0) return prev
      return [...prev, ...urls.slice(0, remaining)]
    })
    setShowLibraryPicker(false)
  }

  return (
    <>
    {/* Model Picker — портал к body, чтобы fixed работал корректно поверх transform-анимаций */}
    {showModelPicker && createPortal(
      <div
        className="fixed inset-0 z-50 flex items-end justify-center"
        style={{ background: 'rgba(0,0,0,0.6)', animation: 'fadeIn 220ms ease-out' }}
        onClick={(e) => { if (e.target === e.currentTarget) setShowModelPicker(false) }}
      >
        <div
          className="w-full max-w-lg rounded-t-2xl bg-[#1a1a1f] overflow-hidden"
          style={{ animation: 'slideUp 380ms cubic-bezier(0.16,1,0.3,1)' }}
        >
          <div className="flex justify-center pt-3 pb-1">
            <div className="w-10 h-1 rounded-full bg-white/20" />
          </div>
          <div className="px-4 pb-2">
            <p className="text-white font-semibold text-base">Выберите модель</p>
            <p className="text-white/40 text-xs mt-0.5">
              {selectedCategory === 'image' ? 'Генерация и редактирование изображений' :
               selectedCategory === 'video' ? 'Генерация видео' :
               selectedCategory === 'music' ? 'Генерация музыки' : 'Текстовые модели'}
            </p>
          </div>
          <div className="px-3 pb-4 space-y-1 max-h-[60vh] overflow-y-auto">
            {filteredModels.map((model, idx) => {
              const isSelected = model.id === selectedModel
              return (
                <button
                  key={model.id}
                  onClick={() => { setSelectedModel(model.id); setShowModelPicker(false) }}
                  className={cn(
                    "w-full flex items-center gap-3 px-4 py-3.5 rounded-xl transition-all active:scale-[0.97]",
                    isSelected
                      ? "bg-gradient-to-r from-yellow-500/15 to-orange-500/10 border border-yellow-500/30"
                      : "bg-white/[0.03] border border-transparent hover:bg-white/[0.06]"
                  )}
                  style={{ animation: `fadeSlideIn ${220 + idx * 60}ms cubic-bezier(0.16,1,0.3,1) both` }}
                >
                  <div className={cn(
                    "w-8 h-8 rounded-xl flex items-center justify-center shrink-0 transition-colors",
                    isSelected ? "bg-yellow-500/20" : "bg-white/5"
                  )}>
                    <Sparkles className={cn("w-4 h-4", isSelected ? "text-yellow-400" : "text-white/30")} />
                  </div>
                  <div className="flex-1 text-left min-w-0">
                    <p className={cn("text-sm font-medium", isSelected ? "text-white" : "text-white/70")}>{model.name}</p>
                    <p className="text-[11px] text-white/30 truncate">{model.description}</p>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <span className={cn(
                      "text-[11px] font-semibold px-2 py-0.5 rounded-full whitespace-nowrap",
                      isSelected ? "bg-yellow-500/20 text-yellow-300" : "bg-white/5 text-white/30"
                    )}>
                      {['nano-banana-2', 'google/nano-banana-pro', 'wan/2-6-image-to-video', 'kling-2.6/image-to-video', 'kling-2.6/motion-control'].includes(model.id)
                        ? `от ${model.token_cost}`
                        : model.token_cost}
                    </span>
                    {isSelected && (
                      <div className="w-5 h-5 rounded-full bg-yellow-400 flex items-center justify-center">
                        <svg className="w-3 h-3 text-black" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                        </svg>
                      </div>
                    )}
                  </div>
                </button>
              )
            })}
          </div>
        </div>
      </div>,
      document.body
    )}

    {showLibraryPicker && createPortal(
      <ImageLibraryPicker
        maxSelect={getMaxImages()}
        alreadySelected={imageFiles.length + libraryUrls.length}
        onSelect={handleLibrarySelect}
        onUploadNew={() => { setShowLibraryPicker(false); fileInputRef.current?.click() }}
        onClose={() => setShowLibraryPicker(false)}
      />,
      document.body
    )}
    <div className="max-w-2xl mx-auto pb-20 lg:pb-8">
      {/* Category Header */}
      <div className="flex items-center justify-between mb-4 px-2">
        <div className="flex items-center gap-3">
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

          {/* Quota badge */}
          {currentQuota >= 0 && (
            <div className={cn(
              "px-2.5 py-1 rounded-full text-xs font-bold tabular-nums",
              currentQuota < totalCost
                ? "bg-red-500/15 text-red-400 border border-red-500/20"
                : currentQuota <= totalCost * 3
                  ? "bg-orange-500/15 text-orange-400 border border-orange-500/20"
                  : "bg-white/5 text-white/50 border border-white/10"
            )}>
              {currentQuota}
            </div>
          )}
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
          <button
            onClick={() => setShowModelPicker(true)}
            className="w-full flex items-center rounded-xl border border-white/10 bg-white/[0.03] px-3 lg:px-4 py-2.5 lg:py-3.5 text-sm lg:text-base transition-all hover:bg-white/[0.05] hover:border-white/20 active:scale-[0.98] cursor-pointer group"
          >
            <div className="w-6 h-6 lg:w-7 lg:h-7 rounded-lg bg-white/10 flex items-center justify-center mr-2.5">
              <span className="text-[10px] lg:text-xs font-bold text-white/40">G</span>
            </div>
            <Sparkles className="w-3.5 h-3.5 lg:w-4 lg:h-4 text-orange-400 mr-2 shrink-0" />
            <span className="text-[13px] lg:text-base text-white/80 truncate flex-1 text-left">{selectedModelInfo?.name || 'Выберите модель'}</span>
            <ChevronRight className="w-4 h-4 lg:w-5 lg:h-5 text-white/20 ml-2 shrink-0 group-hover:text-white/40 transition-colors" />
          </button>
        </div>

        {/* Media Upload - скрыто для музыки и текста */}
        {selectedCategory !== 'music' && selectedCategory !== 'text' && (
          <div className="space-y-1">
            <div className="flex items-center gap-1 ml-1">
              <label className="text-[13px] font-medium text-white/90">Изображения</label>
              {(selectedCategory === 'image' || selectedCategory === 'video') && (
                <span className="text-orange-400 font-bold">*</span>
              )}
            </div>
            <div className="flex flex-wrap gap-2">
              <div
                onDragEnter={handleDrag}
                onDragLeave={handleDrag}
                onDragOver={handleDrag}
                onDrop={handleDrop}
                onClick={(e) => { e.stopPropagation(); setShowLibraryPicker(true) }}
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
                <div key={`file-${idx}`} className="relative w-24 h-32 rounded-xl overflow-hidden border border-white/10 shadow-lg">
                  <img src={preview} alt={`Preview ${idx}`} className="w-full h-full object-cover" />
                  <button
                    onClick={(e) => { e.stopPropagation(); removeImage(idx); }}
                    className="absolute top-1 right-1 p-1 bg-black/60 hover:bg-black/80 text-white rounded-lg backdrop-blur-md transition-all border border-white/5"
                  >
                    <X className="w-2.5 h-2.5" />
                  </button>
                </div>
              ))}

              {libraryUrls.map((url, idx) => (
                <div key={`lib-${idx}`} className="relative w-24 h-32 rounded-xl overflow-hidden border border-yellow-400/40 shadow-lg">
                  <img src={url} alt={`Library ${idx}`} className="w-full h-full object-cover" />
                  <button
                    onClick={(e) => { e.stopPropagation(); setLibraryUrls(libraryUrls.filter((_, i) => i !== idx)); }}
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
            {selectedModel !== 'kling-2.6/motion-control' && <span className="text-orange-400 font-bold">*</span>}
            {selectedModel === 'kling-2.6/motion-control' && <span className="text-white/30 text-[11px]">(необязательно)</span>}
          </div>
          <textarea
            className="w-full rounded-xl border border-white/10 bg-white/[0.03] px-3.5 lg:px-4 py-3 lg:py-4 text-sm lg:text-base transition-all focus:border-primary/50 focus:ring-0 hover:bg-white/[0.05] min-h-[80px] lg:min-h-[100px] max-h-[120px] lg:max-h-[150px] resize-none"
            placeholder={selectedModel === 'kling-2.6/motion-control' ? MOTION_CONTROL_PLACEHOLDER : CATEGORY_PLACEHOLDERS[selectedCategory]}
            value={prompt}
            onChange={(e: ChangeEvent<HTMLTextAreaElement>) => setPrompt(e.target.value)}
          />
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

        {/* Kling 2.6 Motion Control */}
        {selectedModel === 'kling-2.6/motion-control' && (
          <>
            {/* Опорное видео */}
            <div className="space-y-1">
              <div className="flex items-center gap-1 ml-1">
                <label className="text-[13px] font-medium text-white/90">Опорное видео движения</label>
                <span className="text-orange-400 font-bold">*</span>
              </div>
              <input
                id="motion-video-input"
                ref={motionVideoInputRef}
                type="file"
                accept="video/*"
                className="hidden"
                onChange={(e) => {
                  const file = e.target.files?.[0]
                  if (!file) return
                  const objectUrl = URL.createObjectURL(file)
                  const vid = document.createElement('video')
                  vid.preload = 'metadata'
                  vid.onloadedmetadata = () => {
                    const secs = Math.ceil(vid.duration)
                    const w = vid.videoWidth
                    const h = vid.videoHeight
                    URL.revokeObjectURL(vid.src)
                    if (w < 340 || h < 340) {
                      setError(`Разрешение видео ${w}×${h} слишком маленькое. Минимум 340×340 пикселей.`)
                      if (motionVideoInputRef.current) motionVideoInputRef.current.value = ''
                      return
                    }
                    setMotionVideoFile(file)
                    setMotionVideoPreview(objectUrl)
                    setMotionDuration(secs > 0 ? secs : 0)
                  }
                  vid.src = objectUrl
                }}
              />
              {motionVideoPreview ? (
                <div className="relative flex items-center gap-3 px-4 w-full h-24 rounded-xl border border-white/10 bg-white/[0.03]">
                  <Video className="w-5 h-5 text-primary shrink-0" />
                  <span className="text-sm text-white/70 truncate">
                    {motionVideoFile?.name ?? 'Видео из тренда'}
                  </span>
                  <button
                    onClick={(e) => { e.stopPropagation(); e.preventDefault(); setMotionVideoFile(null); setMotionVideoPreview(null); setMotionVideoUrl(null); setMotionDuration(0) }}
                    className="ml-auto p-1 bg-white/10 hover:bg-white/20 rounded-lg transition-all"
                  >
                    <X className="w-3.5 h-3.5 text-white/60" />
                  </button>
                </div>
              ) : (
                <label
                  htmlFor="motion-video-input"
                  className="relative flex flex-col items-center justify-center w-full h-24 rounded-xl border border-white/10 bg-white/[0.03] hover:bg-white/[0.05] cursor-pointer transition-all"
                >
                  <Video className="w-5 h-5 text-white/20 mb-1" />
                  <p className="text-[11px] text-white/30 text-center px-4">Нажмите, чтобы загрузить опорное видео</p>
                  <p className="text-[10px] text-white/20 text-center px-4">мин. разрешение 340×340</p>
                </label>
              )}
            </div>

            {/* Режим 720p / 1080p */}
            <div className="space-y-1">
              <label className="text-[13px] lg:text-sm font-medium text-white/90 ml-1">Качество</label>
              <div className="flex gap-2">
                {KLING_MOTION_MODES.map((m) => (
                  <button
                    key={m.id}
                    onClick={() => setSelectedResolution(m.value)}
                    className={cn(
                      "flex-1 py-2.5 lg:py-3.5 px-3 lg:px-4 rounded-xl border text-xs lg:text-sm font-medium transition-all",
                      selectedResolution === m.value
                        ? "border-primary bg-primary/10 text-primary"
                        : "border-white/10 bg-white/[0.03] text-white/60 hover:bg-white/[0.05]"
                    )}
                  >
                    {m.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Длительность — определяется автоматически из видео */}
            {motionDuration > 0 && (
              <div className="flex items-center gap-2 px-3 py-2 rounded-xl bg-white/[0.03] border border-white/10">
                <span className="text-[12px] text-white/50">⏱ Длительность видео:</span>
                <span className="text-[13px] font-semibold text-white/90">{motionDuration} сек</span>
              </div>
            )}
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

        {error && !quotaError && (
          <div className="p-3 rounded-xl bg-destructive/10 border border-destructive/20 text-destructive text-xs animate-in fade-in slide-in-from-top-2">
            {error}
          </div>
        )}

        {/* Quota banner — нехватка запросов */}
        {(quotaError || (currentQuota >= 0 && currentQuota < totalCost)) && (
          <div className="relative overflow-hidden rounded-2xl border border-yellow-500/20 bg-gradient-to-br from-yellow-500/10 via-orange-500/5 to-transparent p-4 animate-in fade-in slide-in-from-top-2">
            <div className="absolute -top-6 -right-6 w-24 h-24 bg-yellow-500/10 blur-[40px] rounded-full" />
            <div className="flex items-start gap-3 relative">
              <div className="p-2 rounded-xl bg-yellow-500/10 border border-yellow-500/20 shrink-0">
                <Zap className="w-5 h-5 text-yellow-400" />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-semibold text-white">Запросы закончились</p>
                <p className="text-xs text-white/50 mt-0.5">
                  {!user?.subscription_type
                    ? 'Оформите подписку — больше запросов, лучшие модели и приоритет'
                    : 'Пополните запросы или перейдите на более мощный тариф'}
                </p>
              </div>
            </div>
            <div className="flex gap-2 mt-3">
              <Button
                className="flex-1 h-11 rounded-xl text-sm font-bold bg-gradient-to-r from-yellow-400 to-orange-400 text-black hover:opacity-90"
                onClick={() => navigate('/payments')}
              >
                <Crown className="w-4 h-4 mr-1.5" />
                {!user?.subscription_type ? 'Оформить подписку' : 'Пополнить запросы'}
              </Button>
            </div>
          </div>
        )}

        {/* Generate Button Section */}
        <div className="mt-10 mb-4 px-2">
          <div className="max-w-2xl mx-auto">
            {currentQuota >= 0 && currentQuota < totalCost && !generating ? (
              <Button
                className="w-full h-16 rounded-2xl text-lg font-black bg-gradient-to-r from-yellow-400 via-orange-400 to-red-400 text-black hover:opacity-90 transition-all shadow-[0_8px_30px_rgba(255,140,0,0.3)] active:scale-[0.95] flex items-center justify-center gap-3"
                onClick={() => navigate('/payments')}
              >
                <ShoppingCart className="h-6 w-6" />
                <span>Купить запросы</span>
              </Button>
            ) : (
              <Button
                className="w-full h-16 rounded-2xl text-lg font-black bg-gradient-to-r from-[#FFD700] via-[#FFB700] to-[#FF8C00] text-black hover:opacity-90 transition-all shadow-[0_8px_30px_rgba(255,183,0,0.3)] active:scale-[0.95] flex items-center justify-center gap-3"
                onClick={handleGenerate}
                disabled={generating || uploadingImage || totalCost === 0 || ((selectedCategory === 'image' || selectedCategory === 'video') && imageFiles.length === 0 && libraryUrls.length === 0) || (selectedModel === 'kling-2.6/motion-control' && !motionVideoFile && !motionVideoUrl)}
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
            )}
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
    </>
  )
}

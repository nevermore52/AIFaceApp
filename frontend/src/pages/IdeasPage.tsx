import { useEffect, useState, useRef, useCallback } from 'react'
import { publicApi, type GalleryItem } from '../lib/api'
import { X, Copy, Check, Sparkles } from 'lucide-react'
import { Button } from '../components/ui/button'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useAuthStore } from '../store/auth'

export function IdeasPage() {
  const navigate = useNavigate()
  const { isAuthenticated } = useAuthStore()
  const [searchParams] = useSearchParams()
  const [items, setItems] = useState<GalleryItem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [selected, setSelected] = useState<GalleryItem | null>(null)
  const [copied, setCopied] = useState(false)
  const loaderRef = useRef<HTMLDivElement | null>(null)

  const limit = 30

  useEffect(() => {
    publicApi.getGallery(limit, 0).then((res) => {
      setItems(res.data || [])
      setTotal(res.total)

      // Open by URL param or Telegram startapp
      let itemId: number | null = null
      const idParam = searchParams.get('id')
      if (idParam) itemId = parseInt(idParam, 10)

      const tg = window.Telegram?.WebApp
      if (tg?.initDataUnsafe) {
        const startParam = (tg.initDataUnsafe as any).start_param
        if (startParam && typeof startParam === 'string' && startParam.startsWith('g-')) {
          itemId = parseInt(startParam.substring(2), 10)
        }
      }

      if (itemId) {
        const item = res.data?.find((i: GalleryItem) => i.id === itemId)
        if (item) setSelected(item)
      }
    }).catch(console.error).finally(() => setLoading(false))
  }, [searchParams])

  const loadMore = useCallback(() => {
    if (loadingMore || items.length >= total) return
    setLoadingMore(true)
    publicApi.getGallery(limit, items.length).then((res) => {
      setItems(prev => [...prev, ...(res.data || [])])
      setTotal(res.total)
    }).catch(console.error).finally(() => setLoadingMore(false))
  }, [items.length, total, loadingMore])

  useEffect(() => {
    const el = loaderRef.current
    if (!el) return
    const obs = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) loadMore()
    }, { rootMargin: '200px' })
    obs.observe(el)
    return () => obs.disconnect()
  }, [loadMore])

  const getShareLink = (item: GalleryItem) => {
    const tg = window.Telegram?.WebApp
    if (tg?.initData && tg.initData.length > 0) {
      const botName = (import.meta.env.VITE_TELEGRAM_BOT_NAME || 'aifaceappbot').replace('@', '')
      return `https://t.me/${botName}/app?startapp=g-${item.id}`
    }
    return `${window.location.origin}/ideas?id=${item.id}`
  }

  const handleCopy = async (item: GalleryItem) => {
    const link = getShareLink(item)
    try {
      await navigator.clipboard.writeText(link)
    } catch {
      const el = document.createElement('textarea')
      el.value = link
      document.body.appendChild(el)
      el.select()
      document.execCommand('copy')
      document.body.removeChild(el)
    }
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleRepeat = (item: GalleryItem) => {
    if (!isAuthenticated) {
      navigate('/login', { state: { from: '/generate' } })
      return
    }
    navigate('/generate', { state: { prompt: item.prompt, model: item.model } })
  }

  return (
    <div className="max-w-6xl mx-auto space-y-4">
      <h1 className="text-2xl font-bold tracking-tight bg-gradient-to-br from-white to-white/40 bg-clip-text text-transparent">
        Идеи
      </h1>

      {loading ? (
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-2">
          {[...Array(12)].map((_, i) => (
            <div key={i} className="aspect-square rounded-xl bg-white/5 animate-pulse" />
          ))}
        </div>
      ) : items.length === 0 ? (
        <div className="text-center py-20 text-white/40">Галерея пока пуста</div>
      ) : (
        <>
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-2">
            {items.map((item) => (
              <button
                key={item.id}
                onClick={() => setSelected(item)}
                className="relative aspect-square rounded-xl overflow-hidden group cursor-pointer bg-white/[0.03]"
              >
                <img
                  src={item.output}
                  alt=""
                  className="w-full h-full object-contain transition-transform duration-300 group-hover:scale-105"
                  loading="lazy"
                />
              </button>
            ))}
          </div>

          <div ref={loaderRef} className="py-4 flex justify-center">
            {loadingMore && <div className="h-6 w-6 animate-spin rounded-full border-2 border-white/10 border-t-white/60" />}
          </div>
        </>
      )}

      {/* Full-screen detail overlay */}
      {selected && (
        <div
          className="fixed inset-0 z-[60] bg-black/90 backdrop-blur-sm overflow-y-auto"
          onClick={(e) => { if (e.target === e.currentTarget) setSelected(null) }}
        >
          <div className="min-h-full flex flex-col items-center justify-center p-4 md:py-8">
            {/* Card container */}
            <div className="relative w-full max-w-2xl">
              {/* Close */}
              <button
                onClick={() => setSelected(null)}
                className="absolute -top-2 -right-2 z-20 p-2 rounded-full bg-black/70 hover:bg-black/90 transition-colors border border-white/10"
              >
                <X className="w-5 h-5 text-white" />
              </button>

              {/* Image */}
              <img
                src={selected.output}
                alt=""
                className="w-full rounded-t-2xl md:rounded-t-3xl block"
                style={{ maxHeight: '70vh', objectFit: 'contain', background: '#000' }}
              />

              {/* Info panel */}
              <div className="bg-[#111] rounded-b-2xl md:rounded-b-3xl border-t border-white/5 p-4 space-y-3">
                <span className="inline-block text-[10px] font-bold uppercase tracking-widest text-white/50 bg-white/10 px-3 py-1 rounded-full">
                  {selected.model}
                </span>

                {selected.prompt && (
                  <div>
                    <p className="text-[11px] font-semibold text-white/40 mb-1 uppercase tracking-wider">Запрос</p>
                    <div className="max-h-32 overflow-y-auto">
                      <p className="text-sm text-white/80 leading-relaxed">{selected.prompt}</p>
                    </div>
                  </div>
                )}

                <div className="flex gap-2 pt-1">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={(e) => { e.stopPropagation(); handleCopy(selected) }}
                    className="flex-1 rounded-xl border-white/15 bg-white/5 hover:bg-white/10 gap-2 h-11"
                  >
                    {copied ? <Check className="h-4 w-4 text-green-400" /> : <Copy className="h-4 w-4" />}
                    {copied ? 'Скопировано' : 'Поделиться'}
                  </Button>
                  <Button
                    size="sm"
                    onClick={(e) => { e.stopPropagation(); setSelected(null); handleRepeat(selected) }}
                    className="flex-1 rounded-xl bg-gradient-to-r from-[#FFD700] via-[#FFB700] to-[#FF8C00] text-black font-bold hover:opacity-90 gap-2 h-11"
                  >
                    <Sparkles className="h-4 w-4" />
                    Повторить
                  </Button>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

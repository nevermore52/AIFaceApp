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
      
      // Check for id parameter from URL or Telegram startapp
      let itemId: number | null = null
      
      // Check URL parameter
      const idParam = searchParams.get('id')
      if (idParam) {
        itemId = parseInt(idParam, 10)
      }
      
      // Check Telegram startapp parameter (format: g-123)
      const tg = window.Telegram?.WebApp
      if (tg?.initDataUnsafe) {
        const startParam = (tg.initDataUnsafe as any).start_param
        if (startParam && typeof startParam === 'string' && startParam.startsWith('g-')) {
          itemId = parseInt(startParam.substring(2), 10)
        }
      }
      
      // Open the item if found
      if (itemId) {
        const item = res.data?.find((i: GalleryItem) => i.id === itemId)
        if (item) {
          setSelected(item)
        }
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
                className="relative aspect-square rounded-xl overflow-hidden group cursor-pointer border border-white/5 hover:border-white/20 transition-all"
              >
                <img
                  src={item.output}
                  alt=""
                  className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
                  loading="lazy"
                />
                <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
              </button>
            ))}
          </div>

          <div ref={loaderRef} className="py-4 flex justify-center">
            {loadingMore && <div className="h-6 w-6 animate-spin rounded-full border-2 border-white/10 border-t-white/60" />}
          </div>
        </>
      )}

      {/* Detail overlay */}
      {selected && (
        <div
          className="fixed inset-0 z-[60] bg-black/95"
          onClick={(e) => { if (e.target === e.currentTarget) setSelected(null) }}
        >
          <button onClick={() => setSelected(null)} className="absolute top-4 right-4 z-20 p-2 rounded-full bg-white/10 hover:bg-white/20 transition-colors">
            <X className="w-6 h-6 text-white" />
          </button>

          <div className="h-full flex flex-col">
            {/* Image container - takes most space */}
            <div className="flex-1 flex items-center justify-center p-4 pb-2">
              <img 
                src={selected.output} 
                alt="" 
                className="max-w-full max-h-full object-contain"
                style={{ maxHeight: 'calc(100vh - 200px)' }}
              />
            </div>

            {/* Info panel - fixed at bottom */}
            <div className="flex-shrink-0 bg-gradient-to-t from-black via-black/90 to-transparent p-4 pt-8">
              <div className="max-w-4xl mx-auto space-y-3">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-bold uppercase tracking-widest text-white/50 bg-white/10 px-3 py-1 rounded-full">{selected.model}</span>
                </div>
                {selected.prompt && (
                  <p className="text-sm text-white/80 italic">&ldquo;{selected.prompt}&rdquo;</p>
                )}
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleCopy(selected)}
                    className="flex-1 rounded-xl border-white/20 bg-white/5 hover:bg-white/10 gap-2"
                  >
                    {copied ? <Check className="h-4 w-4 text-green-400" /> : <Copy className="h-4 w-4" />}
                    {copied ? 'Скопировано' : 'Поделиться'}
                  </Button>
                  <Button
                    size="sm"
                    onClick={() => { setSelected(null); handleRepeat(selected) }}
                    className="flex-1 rounded-xl bg-gradient-to-r from-[#FFD700] via-[#FFB700] to-[#FF8C00] text-black font-bold hover:opacity-90 gap-2"
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

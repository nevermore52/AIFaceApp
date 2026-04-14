import { useEffect, useState, useRef, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { publicApi, type GalleryItem } from '../lib/api'
import { X, Copy, Check, Sparkles } from 'lucide-react'
import { Button } from '../components/ui/button'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useAuthStore } from '../store/auth'

type SortMode = 'all' | 'new'

export function IdeasPage() {
  const navigate = useNavigate()
  const { isAuthenticated } = useAuthStore()
  const [searchParams] = useSearchParams()
  const [sort, setSort] = useState<SortMode>('all')
  const [items, setItems] = useState<GalleryItem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [selected, setSelected] = useState<GalleryItem | null>(null)
  const [copied, setCopied] = useState(false)
  const loaderRef = useRef<HTMLDivElement | null>(null)

  const limit = 30

  const loadPage = useCallback((sortMode: SortMode, offset: number, replace: boolean) => {
    if (offset === 0) setLoading(true)
    else setLoadingMore(true)

    publicApi.getGallery(limit, offset, sortMode).then((res) => {
      const data = res.data || []
      setItems(prev => replace ? data : [...prev, ...data])
      setTotal(res.total)

      if (offset === 0) {
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
          const item = data.find((i: GalleryItem) => i.id === itemId)
          if (item) setSelected(item)
        }
      }
    }).catch(console.error).finally(() => {
      setLoading(false)
      setLoadingMore(false)
    })
  }, [searchParams])

  // Reload when sort changes
  useEffect(() => {
    setItems([])
    loadPage(sort, 0, true)
  }, [sort])

  const loadMore = useCallback(() => {
    if (loadingMore || items.length >= total) return
    loadPage(sort, items.length, false)
  }, [items.length, total, loadingMore, sort, loadPage])

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
    // Pass image URL for video models that need input image
    navigate('/generate', { state: { prompt: item.prompt, model: item.model, imageUrl: item.output } })
  }

  return (
    <div className="max-w-6xl mx-auto space-y-4">
      <h1 className="text-2xl font-bold tracking-tight bg-gradient-to-br from-white to-white/40 bg-clip-text text-transparent">
        Идеи
      </h1>

      {/* Sort tabs */}
      <div className="flex gap-2">
        {(['all', 'new'] as SortMode[]).map((s) => (
          <button
            key={s}
            onClick={() => setSort(s)}
            className={`px-4 py-1.5 rounded-full text-sm font-medium transition-colors ${
              sort === s
                ? 'bg-white text-black'
                : 'bg-white/10 text-white/60 hover:bg-white/15'
            }`}
          >
            {s === 'all' ? 'Все' : 'Новые'}
          </button>
        ))}
      </div>

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
                className="relative aspect-square rounded-xl overflow-hidden group cursor-pointer bg-white/[0.03] idea-card"
              >
                <img
                  src={item.output}
                  alt=""
                  className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
                  loading="lazy"
                  onError={(e) => {
                    // hide cards with broken images
                    const card = (e.currentTarget as HTMLElement).closest('.idea-card') as HTMLElement | null
                    if (card) card.style.display = 'none'
                  }}
                />
              </button>
            ))}
          </div>

          <div ref={loaderRef} className="py-4 flex justify-center">
            {loadingMore && <div className="h-6 w-6 animate-spin rounded-full border-2 border-white/10 border-t-white/60" />}
          </div>
        </>
      )}

      {/* Detail overlay — portal to body */}
      {selected && createPortal(
        <div
          style={{ position: 'fixed', inset: 0, zIndex: 9999, background: '#000', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '16px' }}
          onClick={(e) => { if (e.target === e.currentTarget) setSelected(null) }}
        >
          {/* Close — always visible at top-right of screen */}
          <button
            onClick={() => setSelected(null)}
            style={{ position: 'fixed', top: '12px', right: '12px', zIndex: 10001, padding: '8px', borderRadius: '50%', background: 'rgba(30,30,30,0.85)', border: '1px solid rgba(255,255,255,0.15)', cursor: 'pointer', backdropFilter: 'blur(4px)' }}
          >
            <X className="w-5 h-5 text-white" />
          </button>

          <div style={{ position: 'relative', width: '100%', maxWidth: '640px' }} onClick={(e) => e.stopPropagation()}>
            {/* Image */}
            <img
              src={selected.output}
              alt=""
              style={{ width: '100%', maxHeight: 'calc(100vh - 220px)', objectFit: 'contain', display: 'block', borderRadius: '16px 16px 0 0', background: '#000' }}
            />

            {/* Info panel */}
            <div style={{ background: '#111', borderRadius: '0 0 16px 16px', borderTop: '1px solid rgba(255,255,255,0.05)', padding: '16px' }}>
              <span style={{ display: 'inline-block', fontSize: '10px', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.1em', color: 'rgba(255,255,255,0.5)', background: 'rgba(255,255,255,0.1)', padding: '4px 12px', borderRadius: '9999px' }}>
                {selected.model}
              </span>

              {selected.prompt && (
                <div style={{ marginTop: '12px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '6px' }}>
                    <p style={{ fontSize: '11px', fontWeight: 600, color: 'rgba(255,255,255,0.4)', textTransform: 'uppercase', letterSpacing: '0.1em', margin: 0 }}>Запрос</p>
                    <button
                      onClick={async () => {
                        try { await navigator.clipboard.writeText(selected.prompt) }
                        catch { const el = document.createElement('textarea'); el.value = selected.prompt; document.body.appendChild(el); el.select(); document.execCommand('copy'); document.body.removeChild(el) }
                        setCopied(true); setTimeout(() => setCopied(false), 2000)
                      }}
                      style={{ padding: '4px 10px', borderRadius: '6px', background: 'rgba(255,255,255,0.07)', border: '1px solid rgba(255,255,255,0.1)', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '5px', fontSize: '11px', color: copied ? '#4ade80' : 'rgba(255,255,255,0.5)' }}
                    >
                      {copied
                        ? <><Check style={{ width: 12, height: 12 }} /> Скопировано</>
                        : <><Copy style={{ width: 12, height: 12 }} /> Копировать</>
                      }
                    </button>
                  </div>
                  {/* Scrollable prompt with visible scrollbar */}
                  <div style={{ maxHeight: '100px', overflowY: 'auto', paddingRight: '4px', scrollbarWidth: 'thin', scrollbarColor: 'rgba(255,255,255,0.2) transparent' }}>
                    <p style={{ fontSize: '14px', color: 'rgba(255,255,255,0.8)', lineHeight: 1.6, margin: 0 }}>{selected.prompt}</p>
                  </div>
                </div>
              )}

              <div style={{ display: 'flex', gap: '8px', marginTop: '12px' }}>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleCopy(selected)}
                  className="flex-1 rounded-xl border-white/15 bg-white/5 hover:bg-white/10 gap-2 h-11"
                >
                  {copied ? <Check className="h-4 w-4 text-green-400" /> : <Copy className="h-4 w-4" />}
                  {copied ? 'Скопировано' : 'Поделиться'}
                </Button>
                <Button
                  size="sm"
                  onClick={() => { setSelected(null); handleRepeat(selected) }}
                  className="flex-1 rounded-xl bg-gradient-to-r from-[#FFD700] via-[#FFB700] to-[#FF8C00] text-black font-bold hover:opacity-90 gap-2 h-11"
                >
                  <Sparkles className="h-4 w-4" />
                  Повторить
                </Button>
              </div>
            </div>
          </div>
        </div>,
        document.body
      )}
    </div>
  )
}

import { useEffect, useRef, useState } from 'react'
import { X, Plus, Check } from 'lucide-react'
import { generationApi } from '../lib/api'

interface UploadItem {
  url: string
  filename: string
}

interface Props {
  maxSelect: number
  alreadySelected: number
  onSelect: (urls: string[]) => void
  onUploadNew: () => void
  onClose: () => void
}

export function ImageLibraryPicker({ maxSelect, alreadySelected, onSelect, onUploadNew, onClose }: Props) {
  const [uploads, setUploads] = useState<UploadItem[]>([])
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const overlayRef = useRef<HTMLDivElement>(null)

  const remaining = maxSelect - alreadySelected
  const canSelectMore = selected.size < remaining

  useEffect(() => {
    generationApi.getUserUploads()
      .then(data => setUploads(data.uploads || []))
      .catch(() => setUploads([]))
      .finally(() => setLoading(false))
  }, [])

  const toggle = (url: string) => {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(url)) {
        next.delete(url)
      } else if (canSelectMore || next.has(url)) {
        next.add(url)
      }
      return next
    })
  }

  const handleConfirm = () => {
    onSelect(Array.from(selected))
  }

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-end justify-center"
      style={{ background: 'rgba(0,0,0,0.6)' }}
      onClick={e => { if (e.target === overlayRef.current) onClose() }}
    >
      <div className="w-full max-w-lg rounded-t-2xl bg-[#1a1a1f] flex flex-col"
        style={{ maxHeight: '80vh' }}>
        {/* Handle */}
        <div className="flex justify-center pt-3 pb-1">
          <div className="w-10 h-1 rounded-full bg-white/20" />
        </div>

        {/* Header */}
        <div className="px-4 pb-3 flex items-center justify-between">
          <div>
            <p className="text-white font-semibold text-base">Выбрать из библиотеки</p>
            <p className="text-white/40 text-xs mt-0.5">Выберите ранее загруженные файлы</p>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-full bg-white/10 text-white/60 hover:bg-white/20">
            <X size={16} />
          </button>
        </div>

        {/* Grid */}
        <div className="flex-1 overflow-y-auto px-4 pb-2">
          {loading ? (
            <div className="flex items-center justify-center h-40 text-white/30 text-sm">Загрузка...</div>
          ) : (
            <div className="grid grid-cols-3 gap-2 pb-2">
              {/* Upload new button */}
              <button
                onClick={onUploadNew}
                className="aspect-square rounded-xl border-2 border-dashed border-white/20 flex items-center justify-center hover:border-white/40 transition-colors"
              >
                <Plus size={24} className="text-white/40" />
              </button>

              {uploads.length === 0 && (
                <div className="col-span-2 flex items-center text-white/30 text-xs pl-2">
                  Нет загруженных изображений
                </div>
              )}

              {uploads.map(item => {
                const isSelected = selected.has(item.url)
                const isDisabled = !isSelected && !canSelectMore
                return (
                  <button
                    key={item.url}
                    onClick={() => toggle(item.url)}
                    disabled={isDisabled}
                    className={`aspect-square rounded-xl overflow-hidden relative border-2 transition-all ${
                      isSelected ? 'border-yellow-400' : 'border-transparent'
                    } ${isDisabled ? 'opacity-40' : ''}`}
                  >
                    <img
                      src={item.url}
                      alt=""
                      className="w-full h-full object-cover"
                      loading="lazy"
                    />
                    {isSelected && (
                      <div className="absolute inset-0 bg-yellow-400/20 flex items-end justify-end p-1">
                        <div className="bg-yellow-400 rounded-full p-0.5">
                          <Check size={12} className="text-black" />
                        </div>
                      </div>
                    )}
                  </button>
                )
              })}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="px-4 py-4 flex gap-3 border-t border-white/5">
          <button
            onClick={onClose}
            className="flex-1 py-3 rounded-xl border border-white/10 text-white/70 text-sm font-medium"
          >
            Отмена
          </button>
          <button
            onClick={handleConfirm}
            disabled={selected.size === 0}
            className="flex-1 py-3 rounded-xl text-sm font-bold transition-opacity disabled:opacity-40"
            style={{ background: selected.size > 0 ? 'linear-gradient(135deg, #d4a017, #f0c040)' : undefined, color: selected.size > 0 ? '#000' : '#fff' }}
          >
            {selected.size > 0 ? `Выбрать (${selected.size})` : 'Выбрать'}
          </button>
        </div>
      </div>
    </div>
  )
}

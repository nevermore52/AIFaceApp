import { useState, useEffect, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { adminApi, generationApi } from '../lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'

type Tab = 'stats' | 'users' | 'top_users' | 'generations' | 'payments' | 'gallery_ideas' | 'trends' | 'promo_codes'

const TABS: { id: Tab; label: string }[] = [
  { id: 'stats', label: '📊 Статистика' },
  { id: 'users', label: '👥 Пользователи' },
  { id: 'top_users', label: '📈 Топ-24ч' },
  { id: 'generations', label: '🎨 Генерации' },
  { id: 'payments', label: '💳 Платежи' },
  { id: 'gallery_ideas', label: '💡 Идеи' },
  { id: 'trends', label: '🔥 Тренды' },
  { id: 'promo_codes', label: '🎁 Промокоды' },
]

function StatCard({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="p-4 rounded-xl border border-white/5 bg-white/[0.03]">
      <p className="text-xs text-white/40 uppercase tracking-wider mb-1">{label}</p>
      <p className="text-2xl font-bold text-white">{value ?? '—'}</p>
    </div>
  )
}

// ─── Stats Tab ────────────────────────────────────────────────────────────────
function StatsTab() {
  const [period, setPeriod] = useState('all')
  const [data, setData] = useState<any>(null)
  const [loading, setLoading] = useState(false)

  const load = useCallback(async (p: string) => {
    setLoading(true)
    try {
      const res = await adminApi.getStats(p)
      setData(res)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load(period) }, [period, load])

  const g = data?.generations ?? {}
  const u = data?.users ?? {}
  const pay = data?.payments ?? {}

  return (
    <div className="space-y-6">
      <div className="flex gap-2 flex-wrap">
        {[['day', 'День'], ['week', 'Неделя'], ['month', 'Месяц'], ['all', 'Всё время']].map(([p, l]) => (
          <button
            key={p}
            onClick={() => setPeriod(p)}
            className={`px-4 py-1.5 rounded-full text-sm font-semibold transition-all ${
              period === p ? 'bg-primary text-white' : 'bg-white/5 text-white/50 hover:bg-white/10'
            }`}
          >
            {l}
          </button>
        ))}
      </div>

      {loading ? <div className="text-white/40 text-sm">Загрузка...</div> : (
        <>
          <div>
            <p className="text-xs text-white/30 uppercase tracking-wider mb-3">Генерации</p>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              <StatCard label="Всего запросов" value={g.total_requests ?? 0} />
              <StatCard label="Успешных" value={g.completed_requests ?? 0} />
              <StatCard label="Ошибок" value={g.failed_requests ?? 0} />
              <StatCard label="В процессе" value={g.processing_requests ?? 0} />
              <StatCard label="Успешность" value={`${g.success_rate ?? 0}%`} />
              <StatCard label="Среднее время" value={g.avg_processing_time_seconds ? `${Number(g.avg_processing_time_seconds).toFixed(1)}с` : '—'} />
            </div>
          </div>

          <div>
            <p className="text-xs text-white/30 uppercase tracking-wider mb-3">Пользователи</p>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              <StatCard label="Всего" value={u.total_users ?? 0} />
              <StatCard label="Новых сегодня" value={u.today_users ?? 0} />
              <StatCard label="Активных подписок" value={u.active_subscriptions ?? 0} />
            </div>
          </div>

          <div>
            <p className="text-xs text-white/30 uppercase tracking-wider mb-3">Платежи</p>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              <StatCard label="За сутки (кол-во)" value={pay.day_count ?? 0} />
              <StatCard label="За сутки (сумма)" value={pay.day_amount ? `${pay.day_amount} ₽` : '0 ₽'} />
              <StatCard label="За неделю" value={pay.week_amount ? `${pay.week_amount} ₽` : '0 ₽'} />
              <StatCard label="Всего (кол-во)" value={pay.total_count ?? 0} />
              <StatCard label="Всего (сумма)" value={pay.total_amount ? `${pay.total_amount} ₽` : '0 ₽'} />
            </div>
          </div>
        </>
      )}
    </div>
  )
}

// ─── Users Tab ────────────────────────────────────────────────────────────────
function UsersTab() {
  const [users, setUsers] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(false)
  const [editUser, setEditUser] = useState<any>(null)
  const [subPlan, setSubPlan] = useState('pro')
  const [subDays, setSubDays] = useState(30)
  const [saving, setSaving] = useState(false)
  const [msg, setMsg] = useState('')
  const limit = 20

  const load = useCallback(async (off: number) => {
    setLoading(true)
    try {
      const res = await adminApi.getUsers(limit, off)
      setUsers(res.data ?? [])
      setTotal(res.total ?? 0)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load(offset) }, [offset, load])

  const toggleAdmin = async (u: any) => {
    await adminApi.updateUser(u.id, { is_admin: !u.is_admin })
    load(offset)
  }
  const toggleBlocked = async (u: any) => {
    await adminApi.updateUser(u.id, { is_blocked: !u.is_blocked })
    load(offset)
  }
  const setSub = async () => {
    if (!editUser) return
    setSaving(true)
    try {
      await adminApi.setSubscription(editUser.id, subPlan, subDays)
      setMsg(`✅ Подписка ${subPlan} на ${subDays} дней выдана`)
      load(offset)
    } catch (e: any) {
      setMsg(`❌ ${e.message}`)
    } finally {
      setSaving(false)
    }
  }
  const removeSub = async (u: any) => {
    await adminApi.removeSubscription(u.id)
    load(offset)
  }

  return (
    <div className="space-y-4">
      <p className="text-white/40 text-sm">Всего: {total}</p>

      {loading ? <div className="text-white/40 text-sm">Загрузка...</div> : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-white/30 text-xs uppercase tracking-wider border-b border-white/5">
                <th className="pb-2 pr-4">ID</th>
                <th className="pb-2 pr-4">Имя / Username</th>
                <th className="pb-2 pr-4">Подписка</th>
                <th className="pb-2 pr-4">Статус</th>
                <th className="pb-2">Действия</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} className="border-b border-white/[0.03] hover:bg-white/[0.02]">
                  <td className="py-2 pr-4 text-white/40">{u.id}</td>
                  <td className="py-2 pr-4">
                    <div className="font-medium text-white/80">{u.first_name} {u.last_name}</div>
                    <div className="text-white/30 text-xs">@{u.username || '—'}</div>
                  </td>
                  <td className="py-2 pr-4">
                    {u.subscription_type ? (
                      <span className="px-2 py-0.5 rounded-full bg-primary/20 text-primary text-xs font-bold uppercase">
                        {u.subscription_type}
                      </span>
                    ) : <span className="text-white/20 text-xs">—</span>}
                  </td>
                  <td className="py-2 pr-4">
                    <div className="flex gap-1 flex-wrap">
                      {u.is_admin && <span className="px-1.5 py-0.5 rounded bg-yellow-500/20 text-yellow-400 text-[10px] font-bold">ADMIN</span>}
                      {u.is_blocked && <span className="px-1.5 py-0.5 rounded bg-red-500/20 text-red-400 text-[10px] font-bold">BLOCKED</span>}
                      {u.telegram_id && <span className="px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 text-[10px]">TG</span>}
                      {u.email && <span className="px-1.5 py-0.5 rounded bg-green-500/10 text-green-400 text-[10px]">G</span>}
                    </div>
                  </td>
                  <td className="py-2">
                    <div className="flex gap-1 flex-wrap">
                      <button
                        onClick={() => toggleAdmin(u)}
                        className={`px-2 py-1 rounded text-[10px] font-bold transition-all ${u.is_admin ? 'bg-yellow-500/20 text-yellow-400 hover:bg-yellow-500/30' : 'bg-white/5 text-white/40 hover:bg-white/10'}`}
                      >
                        {u.is_admin ? '- Админ' : '+ Админ'}
                      </button>
                      <button
                        onClick={() => toggleBlocked(u)}
                        className={`px-2 py-1 rounded text-[10px] font-bold transition-all ${u.is_blocked ? 'bg-red-500/20 text-red-400 hover:bg-red-500/30' : 'bg-white/5 text-white/40 hover:bg-white/10'}`}
                      >
                        {u.is_blocked ? 'Разблок' : 'Блок'}
                      </button>
                      <button
                        onClick={() => { setEditUser(u); setMsg('') }}
                        className="px-2 py-1 rounded text-[10px] font-bold bg-primary/20 text-primary hover:bg-primary/30 transition-all"
                      >
                        Подписка
                      </button>
                      {u.subscription_type && (
                        <button
                          onClick={() => removeSub(u)}
                          className="px-2 py-1 rounded text-[10px] font-bold bg-white/5 text-white/40 hover:bg-red-500/20 hover:text-red-400 transition-all"
                        >
                          - Подп
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="flex gap-2">
        <Button size="sm" variant="outline" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}
          className="border-white/10">← Назад</Button>
        <Button size="sm" variant="outline" disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}
          className="border-white/10">Вперёд →</Button>
      </div>

      {/* Subscription modal */}
      {editUser && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
          <div className="bg-[#0e0e0e] border border-white/10 rounded-2xl p-6 w-full max-w-sm space-y-4">
            <h3 className="text-lg font-bold text-white">Подписка для {editUser.first_name}</h3>
            {msg && <p className="text-sm text-white/60">{msg}</p>}
            <div className="space-y-3">
              <div>
                <label className="text-xs text-white/40 uppercase tracking-wider">Тариф</label>
                <select
                  value={subPlan}
                  onChange={(e) => setSubPlan(e.target.value)}
                  className="w-full mt-1 px-3 py-2 rounded-xl bg-white/5 border border-white/10 text-white text-sm"
                >
                  <option value="mini">Mini</option>
                  <option value="start">Start</option>
                  <option value="pro">Pro</option>
                </select>
              </div>
              <div>
                <label className="text-xs text-white/40 uppercase tracking-wider">Дней</label>
                <input
                  type="number"
                  value={subDays}
                  onChange={(e) => setSubDays(Number(e.target.value))}
                  min={1}
                  className="w-full mt-1 px-3 py-2 rounded-xl bg-white/5 border border-white/10 text-white text-sm"
                />
              </div>
            </div>
            <div className="flex gap-2">
              <Button onClick={setSub} disabled={saving} className="flex-1">
                {saving ? 'Выдаём...' : 'Выдать'}
              </Button>
              <Button variant="outline" onClick={() => setEditUser(null)} className="flex-1 border-white/10">
                Закрыть
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// ─── Top Users Tab ─────────────────────────────────────────────────────────────
function TopUsersTab() {
  const [users, setUsers] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    adminApi.getTopUsers().then((res) => {
      setUsers(res.data ?? [])
    }).finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="text-white/40 text-sm">Загрузка...</div>
  if (!users.length) return <div className="text-white/40 text-sm">Нет генераций за последние 24 часа</div>

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-white/30 text-xs uppercase tracking-wider border-b border-white/5">
            <th className="pb-2 pr-3">#</th>
            <th className="pb-2 pr-3">Username</th>
            <th className="pb-2 pr-3">Всего</th>
            <th className="pb-2 pr-3">🖼️</th>
            <th className="pb-2 pr-3">🎬</th>
            <th className="pb-2 pr-3">🎵</th>
            <th className="pb-2 pr-3">💬</th>
            <th className="pb-2">Токены</th>
          </tr>
        </thead>
        <tbody>
          {users.map((u, i) => (
            <tr key={u.user_id} className="border-b border-white/[0.03]">
              <td className="py-2 pr-3 text-white/40">{i + 1}</td>
              <td className="py-2 pr-3 text-white/80">@{u.username || u.user_id}</td>
              <td className="py-2 pr-3 font-bold text-white">{u.total_generations}</td>
              <td className="py-2 pr-3 text-white/60">{u.photo_generations}</td>
              <td className="py-2 pr-3 text-white/60">{u.video_generations}</td>
              <td className="py-2 pr-3 text-white/60">{u.music_generations}</td>
              <td className="py-2 pr-3 text-white/60">{u.text_generations}</td>
              <td className="py-2 text-white/40">{u.tokens_spent}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ─── Generations Tab ──────────────────────────────────────────────────────────
function GenerationsTab() {
  const [items, setItems] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(false)
  const limit = 20

  const load = useCallback(async (off: number) => {
    setLoading(true)
    try {
      const res = await adminApi.getGenerations(limit, off)
      setItems(res.data ?? [])
      setTotal(res.total ?? 0)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load(offset) }, [offset, load])

  const statusColor = (s: string) =>
    s === 'completed' ? 'text-green-400' : s === 'failed' ? 'text-red-400' : 'text-yellow-400'

  return (
    <div className="space-y-4">
      <p className="text-white/40 text-sm">Всего: {total}</p>
      {loading ? <div className="text-white/40 text-sm">Загрузка...</div> : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-white/30 text-xs uppercase tracking-wider border-b border-white/5">
                <th className="pb-2 pr-4">ID</th>
                <th className="pb-2 pr-4">User</th>
                <th className="pb-2 pr-4">Модель</th>
                <th className="pb-2 pr-4">Статус</th>
                <th className="pb-2">Время</th>
              </tr>
            </thead>
            <tbody>
              {items.map((g) => (
                <tr key={g.id} className="border-b border-white/[0.03]">
                  <td className="py-2 pr-4 text-white/40">{g.id}</td>
                  <td className="py-2 pr-4 text-white/60">@{g.username || g.user_id}</td>
                  <td className="py-2 pr-4 text-white/60 text-xs">{g.model}</td>
                  <td className={`py-2 pr-4 text-xs font-semibold ${statusColor(g.status)}`}>{g.status}</td>
                  <td className="py-2 text-white/30 text-xs">{new Date(g.created_at).toLocaleString('ru')}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <div className="flex gap-2">
        <Button size="sm" variant="outline" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}
          className="border-white/10">← Назад</Button>
        <Button size="sm" variant="outline" disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}
          className="border-white/10">Вперёд →</Button>
      </div>
    </div>
  )
}

// ─── Payments Tab ─────────────────────────────────────────────────────────────
function PaymentsTab() {
  const [items, setItems] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(false)
  const limit = 20

  const load = useCallback(async (off: number) => {
    setLoading(true)
    try {
      const res = await adminApi.getPayments(limit, off)
      setItems(res.data ?? [])
      setTotal(res.total ?? 0)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load(offset) }, [offset, load])

  return (
    <div className="space-y-4">
      <p className="text-white/40 text-sm">Всего: {total}</p>
      {loading ? <div className="text-white/40 text-sm">Загрузка...</div> : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-white/30 text-xs uppercase tracking-wider border-b border-white/5">
                <th className="pb-2 pr-4">ID</th>
                <th className="pb-2 pr-4">User</th>
                <th className="pb-2 pr-4">Категория</th>
                <th className="pb-2 pr-4">Кол-во</th>
                <th className="pb-2 pr-4">Сумма</th>
                <th className="pb-2">Время</th>
              </tr>
            </thead>
            <tbody>
              {items.map((p) => (
                <tr key={p.id} className="border-b border-white/[0.03]">
                  <td className="py-2 pr-4 text-white/40">{p.id}</td>
                  <td className="py-2 pr-4 text-white/60">@{p.username || p.telegram_id}</td>
                  <td className="py-2 pr-4 text-white/60 text-xs">{p.category}</td>
                  <td className="py-2 pr-4 text-white/60">{p.qty}</td>
                  <td className="py-2 pr-4 font-semibold text-white">{p.amount} ₽</td>
                  <td className="py-2 text-white/30 text-xs">{new Date(p.created_at).toLocaleString('ru')}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <div className="flex gap-2">
        <Button size="sm" variant="outline" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}
          className="border-white/10">← Назад</Button>
        <Button size="sm" variant="outline" disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}
          className="border-white/10">Вперёд →</Button>
      </div>
    </div>
  )
}

// ─── Gallery Ideas Tab ────────────────────────────────────────────────────────

type IdeaFormData = { model: string; output: string; prompt: string; priority: number | null }

const EMPTY_FORM: IdeaFormData = { model: '', output: '', prompt: '', priority: null }

function IdeaForm({
  data,
  onChange,
  models,
  takenPriorities,
  uploading,
  onSubmit,
  onCancel,
  submitLabel,
}: {
  data: IdeaFormData
  onChange: (d: IdeaFormData) => void
  models: any[]
  takenPriorities: number[]
  uploading: boolean
  // receives resolved output URL so parent doesn't depend on async state update
  onSubmit: (resolvedOutput: string) => void
  onCancel?: () => void
  submitLabel: string
}) {
  const [imageFile, setImageFile] = useState<File | null>(null)
  const [imagePreview, setImagePreview] = useState<string>(data.output || '')
  const [localUploading, setLocalUploading] = useState(false)

  const uploadImage = async (file: File): Promise<string> => {
    const fd = new FormData()
    fd.append('image', file)
    const storage = localStorage.getItem('auth-storage')
    let token = null
    if (storage) {
      try { token = JSON.parse(storage).state?.accessToken } catch {}
    }
    const resp = await fetch('/api/admin/upload', {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: fd,
    })
    if (!resp.ok) throw new Error('Upload failed: ' + await resp.text())
    return (await resp.json()).url
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setImageFile(file)
    const reader = new FileReader()
    reader.onloadend = () => setImagePreview(reader.result as string)
    reader.readAsDataURL(file)
  }

  const handleSubmit = async () => {
    if (!data.model || !data.prompt) { alert('Заполните модель и промпт'); return }
    if (!imageFile && !data.output) { alert('Загрузите изображение или укажите URL'); return }

    let resolvedOutput = data.output
    if (imageFile) {
      setLocalUploading(true)
      try {
        resolvedOutput = await uploadImage(imageFile)
        onChange({ ...data, output: resolvedOutput })
      } catch (err) {
        alert('Ошибка загрузки: ' + (err instanceof Error ? err.message : String(err)))
        setLocalUploading(false)
        return
      }
      setLocalUploading(false)
    }
    // Pass resolved URL directly — no async state race
    onSubmit(resolvedOutput)
  }

  const isDisabled = uploading || localUploading

  // Available positions: 1..50 minus taken (excluding current idea's own slot)
  const availablePositions = Array.from({ length: 50 }, (_, i) => i + 1)
    .filter(n => !takenPriorities.includes(n))

  return (
    <div className="space-y-3">
      {/* Model */}
      <div>
        <label className="text-xs text-white/50 mb-1 block">Модель</label>
        <select
          value={data.model}
          onChange={(e) => onChange({ ...data, model: e.target.value })}
          className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm [&>option]:bg-gray-900 [&>option]:text-white"
        >
          <option value="">Выберите модель</option>
          {models.map((m) => (
            <option key={m.id} value={m.id}>{m.name} ({m.type})</option>
          ))}
        </select>
      </div>

      {/* Image */}
      <div>
        <label className="text-xs text-white/50 mb-1 block">Изображение</label>
        <input
          type="file"
          accept="image/*"
          onChange={handleFileChange}
          className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm file:mr-4 file:py-1 file:px-3 file:rounded file:border-0 file:text-sm file:bg-white/10 file:text-white hover:file:bg-white/20"
        />
        {imagePreview && (
          <img src={imagePreview} alt="Preview" className="mt-2 w-24 h-24 object-cover rounded-lg" />
        )}
      </div>

      {/* Prompt */}
      <textarea
        placeholder="Промпт"
        value={data.prompt}
        onChange={(e) => onChange({ ...data, prompt: e.target.value })}
        className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm min-h-[72px]"
      />

      {/* Priority / место */}
      <div>
        <label className="text-xs text-white/50 mb-1 block">Место (приоритет)</label>
        <select
          value={data.priority ?? ''}
          onChange={(e) => onChange({ ...data, priority: e.target.value ? parseInt(e.target.value) : null })}
          className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm [&>option]:bg-gray-900 [&>option]:text-white"
        >
          <option value="">— без приоритета —</option>
          {/* Always include current value even if "taken" by self */}
          {data.priority !== null && !availablePositions.includes(data.priority) && (
            <option value={data.priority}>{data.priority}</option>
          )}
          {availablePositions.map(n => (
            <option key={n} value={n}>{n}</option>
          ))}
        </select>
      </div>

      <div className="flex gap-2">
        <Button onClick={handleSubmit} size="sm" className="flex-1" disabled={isDisabled}>
          {isDisabled ? 'Загрузка...' : submitLabel}
        </Button>
        {onCancel && (
          <Button onClick={onCancel} size="sm" variant="outline" disabled={isDisabled}>Отмена</Button>
        )}
      </div>
    </div>
  )
}

function GalleryIdeasTab() {
  const [ideas, setIdeas] = useState<any[]>([])
  const [models, setModels] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [limit] = useState(50)
  const [offset, setOffset] = useState(0)

  // Selected idea for detail panel
  const [selected, setSelected] = useState<any | null>(null)
  const [editing, setEditing] = useState(false)
  const [editForm, setEditForm] = useState<IdeaFormData>(EMPTY_FORM)

  // Create panel
  const [showCreate, setShowCreate] = useState(false)
  const [createForm, setCreateForm] = useState<IdeaFormData>(EMPTY_FORM)

  // Taken priorities for current context
  const [takenForCreate, setTakenForCreate] = useState<number[]>([])
  const [takenForEdit, setTakenForEdit] = useState<number[]>([])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await adminApi.getGalleryIdeas(limit, offset)
      setIdeas(res.data || [])
      setTotal(res.total)
    } finally {
      setLoading(false)
    }
  }, [limit, offset])

  useEffect(() => {
    load()
    generationApi.getModels().then((res) => {
      setModels(res.filter((m: any) => m.type === 'image' || m.type === 'video'))
    }).catch(console.error)
  }, [load])

  // Load taken priorities when opening create panel
  useEffect(() => {
    if (showCreate) {
      adminApi.getGalleryIdeaPriorities(0).then(r => setTakenForCreate(r.taken)).catch(console.error)
    }
  }, [showCreate])

  // Load taken priorities (excluding self) when opening edit
  useEffect(() => {
    if (editing && selected) {
      adminApi.getGalleryIdeaPriorities(selected.id).then(r => setTakenForEdit(r.taken)).catch(console.error)
    }
  }, [editing, selected])

  const handleCreate = async (resolvedOutput: string) => {
    setUploading(true)
    try {
      await adminApi.createGalleryIdea({
        model: createForm.model,
        output: resolvedOutput,
        prompt: createForm.prompt,
        priority: createForm.priority,
      })
      setCreateForm(EMPTY_FORM)
      setShowCreate(false)
      load()
    } catch (err) {
      alert('Ошибка: ' + (err instanceof Error ? err.message : String(err)))
    } finally {
      setUploading(false)
    }
  }

  const handleUpdate = async (resolvedOutput: string) => {
    if (!selected) return
    setUploading(true)
    try {
      await adminApi.updateGalleryIdea(selected.id, {
        model: editForm.model,
        output: resolvedOutput,
        prompt: editForm.prompt,
        priority: editForm.priority,
      })
      setEditing(false)
      setSelected(null)
      load()
    } catch (err) {
      alert('Ошибка: ' + (err instanceof Error ? err.message : String(err)))
    } finally {
      setUploading(false)
    }
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Удалить эту идею?')) return
    try {
      await adminApi.deleteGalleryIdea(id)
      setSelected(null)
      load()
    } catch (err) {
      console.error(err)
    }
  }

  const openDetail = (idea: any) => {
    setSelected(idea)
    setEditing(false)
  }

  const startEdit = () => {
    if (!selected) return
    setEditForm({
      model: selected.model,
      output: selected.output,
      prompt: selected.prompt,
      priority: selected.priority ?? null,
    })
    setEditing(true)
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <span className="text-xs text-white/40">Всего: {total}</span>
        <Button size="sm" onClick={() => { setShowCreate(v => !v); setCreateForm(EMPTY_FORM) }}>
          {showCreate ? 'Отмена' : '+ Добавить'}
        </Button>
      </div>

      {/* Create form */}
      {showCreate && (
        <div className="p-4 rounded-xl border border-white/10 bg-white/5">
          <p className="text-sm font-semibold text-white/70 mb-3">Новая идея</p>
          <IdeaForm
            data={createForm}
            onChange={setCreateForm}
            models={models}
            takenPriorities={takenForCreate}
            uploading={uploading}
            onSubmit={handleCreate}
            onCancel={() => setShowCreate(false)}
            submitLabel="Добавить"
          />
        </div>
      )}

      {/* Photo grid */}
      {loading ? (
        <div className="grid grid-cols-3 gap-2">
          {[...Array(9)].map((_, i) => <div key={i} className="aspect-square rounded-xl bg-white/5 animate-pulse" />)}
        </div>
      ) : (
        <div className="grid grid-cols-3 gap-2">
          {ideas.map((idea) => (
            <button
              key={idea.id}
              onClick={() => openDetail(idea)}
              className="relative aspect-square rounded-xl overflow-hidden bg-white/[0.03] group"
            >
              <img src={idea.output} alt="" className="w-full h-full object-cover" loading="lazy" />
              {idea.priority != null && (
                <span className="absolute top-1 left-1 text-[10px] font-bold bg-[#FFB700] text-black rounded px-1.5 py-0.5">
                  #{idea.priority}
                </span>
              )}
            </button>
          ))}
        </div>
      )}

      {/* Pagination */}
      <div className="flex justify-end gap-2 pt-1">
        <Button size="sm" variant="outline" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))} className="border-white/10">← Назад</Button>
        <Button size="sm" variant="outline" disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)} className="border-white/10">Вперёд →</Button>
      </div>

      {/* Detail drawer — portaled to body so fixed positioning works */}
      {selected && createPortal(
        <div
          style={{ position: 'fixed', inset: 0, zIndex: 9000, background: 'rgba(0,0,0,0.7)' }}
          onClick={(e) => { if (e.target === e.currentTarget) { setSelected(null); setEditing(false) } }}
        >
          <div
            style={{ position: 'fixed', bottom: 0, left: 0, right: 0, maxHeight: '85vh', overflowY: 'auto', background: '#111', borderRadius: '20px 20px 0 0', padding: '20px' }}
            onClick={(e) => e.stopPropagation()}
          >
            {/* Handle */}
            <div style={{ width: 40, height: 4, background: 'rgba(255,255,255,0.2)', borderRadius: 2, margin: '0 auto 16px' }} />

            {editing ? (
              <>
                <p className="text-sm font-semibold text-white/70 mb-3">Редактировать идею</p>
                <IdeaForm
                  data={editForm}
                  onChange={setEditForm}
                  models={models}
                  takenPriorities={takenForEdit}
                  uploading={uploading}
                  onSubmit={handleUpdate}
                  onCancel={() => setEditing(false)}
                  submitLabel="Сохранить"
                />
              </>
            ) : (
              <>
                <img src={selected.output} alt="" style={{ width: '100%', maxHeight: 320, objectFit: 'contain', borderRadius: 12, background: '#000', display: 'block' }} />
                <div style={{ marginTop: 12 }}>
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-xs text-white/40 bg-white/10 px-2 py-1 rounded-full">{selected.model}</span>
                    {selected.priority != null && (
                      <span className="text-xs font-bold bg-[#FFB700] text-black px-2 py-1 rounded-full">
                        Место #{selected.priority}
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-white/70 mb-4">{selected.prompt}</p>
                  <div className="flex gap-2">
                    <Button size="sm" variant="outline" onClick={startEdit} className="flex-1">Редактировать</Button>
                    <Button size="sm" variant="outline" onClick={() => handleDelete(selected.id)} className="flex-1 text-red-400 hover:text-red-300">Удалить</Button>
                  </div>
                </div>
              </>
            )}
          </div>
        </div>,
        document.body
      )}
    </div>
  )
}

// ─── Trends Tab ──────────────────────────────────────────────────────────────

type TrendFormData = { title: string; output: string; inputVideo: string; prompt: string; model: string; isPopular: boolean; priority: number | null }
const EMPTY_TREND_FORM: TrendFormData = { title: '', output: '', inputVideo: '', prompt: '', model: '', isPopular: false, priority: null }

function TrendForm({
  data,
  onChange,
  models,
  takenPriorities,
  uploading,
  onSubmit,
  onCancel,
  submitLabel,
}: {
  data: TrendFormData
  onChange: (d: TrendFormData) => void
  models: any[]
  takenPriorities: number[]
  uploading: boolean
  onSubmit: (resolvedOutput: string) => void
  onCancel?: () => void
  submitLabel: string
}) {
  const [mediaFile, setMediaFile] = useState<File | null>(null)
  const [mediaPreview, setMediaPreview] = useState<string>(data.output || '')
  const [isVideoFile, setIsVideoFile] = useState<boolean>(() => /\.mp4(\?|$)/i.test(data.output || ''))
  const [localUploading, setLocalUploading] = useState(false)

  const getToken = () => {
    try { return JSON.parse(localStorage.getItem('auth-storage') || '{}').state?.accessToken ?? null } catch { return null }
  }

  const uploadMedia = async (file: File): Promise<string> => {
    const isVideo = file.type.startsWith('video/')
    const fd = new FormData()
    fd.append(isVideo ? 'video' : 'image', file)
    const token = getToken()
    const resp = await fetch(isVideo ? '/api/admin/upload-video' : '/api/admin/upload', {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: fd,
    })
    if (!resp.ok) throw new Error('Upload failed: ' + await resp.text())
    return (await resp.json()).url
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setMediaFile(file)
    const isVid = file.type.startsWith('video/')
    setIsVideoFile(isVid)
    if (isVid) {
      setMediaPreview(URL.createObjectURL(file))
    } else {
      const reader = new FileReader()
      reader.onloadend = () => setMediaPreview(reader.result as string)
      reader.readAsDataURL(file)
    }
  }

  const handleSubmit = async () => {
    if (!data.output && !mediaFile) { alert('Загрузите изображение или видео'); return }
    let resolvedOutput = data.output
    if (mediaFile) {
      setLocalUploading(true)
      try {
        resolvedOutput = await uploadMedia(mediaFile)
        onChange({ ...data, output: resolvedOutput })
      } catch (err) {
        alert('Ошибка загрузки: ' + (err instanceof Error ? err.message : String(err)))
        setLocalUploading(false)
        return
      }
      setLocalUploading(false)
    }
    onSubmit(resolvedOutput)
  }

  const isDisabled = uploading || localUploading
  const availablePositions = Array.from({ length: 50 }, (_, i) => i + 1).filter(n => !takenPriorities.includes(n))

  return (
    <div className="space-y-3">
      <div>
        <label className="text-xs text-white/50 mb-1 block">Название</label>
        <input
          type="text"
          placeholder="Название тренда"
          value={data.title}
          onChange={(e) => onChange({ ...data, title: e.target.value })}
          className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm"
        />
      </div>
      <div>
        <label className="text-xs text-white/50 mb-1 block">Модель</label>
        <select
          value={data.model}
          onChange={(e) => onChange({ ...data, model: e.target.value })}
          className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm [&>option]:bg-gray-900 [&>option]:text-white"
        >
          <option value="">Выберите модель</option>
          {models.map((m) => (
            <option key={m.id} value={m.id}>{m.name} ({m.type})</option>
          ))}
        </select>
      </div>
      <div>
        <label className="text-xs text-white/50 mb-1 block">Результат (изображение или видео)</label>
        <input
          type="file"
          accept="image/*,video/*"
          onChange={handleFileChange}
          className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm file:mr-4 file:py-1 file:px-3 file:rounded file:border-0 file:text-sm file:bg-white/10 file:text-white hover:file:bg-white/20"
        />
        {mediaPreview && (
          isVideoFile
            ? <video src={mediaPreview} muted autoPlay loop playsInline className="mt-2 w-24 h-24 object-cover rounded-lg" />
            : <img src={mediaPreview} alt="Preview" className="mt-2 w-24 h-24 object-cover rounded-lg" />
        )}
      </div>
      <textarea
        placeholder="Промпт"
        value={data.prompt}
        onChange={(e) => onChange({ ...data, prompt: e.target.value })}
        className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm min-h-[72px]"
      />
      <div className="flex items-center gap-3">
        <label className="text-xs text-white/50">Популярное</label>
        <button
          type="button"
          onClick={() => onChange({ ...data, isPopular: !data.isPopular })}
          className={`w-10 h-5 rounded-full transition-colors relative ${data.isPopular ? 'bg-[#FFB700]' : 'bg-white/20'}`}
        >
          <span className={`absolute top-0.5 w-4 h-4 rounded-full bg-white transition-all ${data.isPopular ? 'left-5' : 'left-0.5'}`} />
        </button>
      </div>
      <div>
        <label className="text-xs text-white/50 mb-1 block">Место (приоритет)</label>
        <select
          value={data.priority ?? ''}
          onChange={(e) => onChange({ ...data, priority: e.target.value ? parseInt(e.target.value) : null })}
          className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm [&>option]:bg-gray-900 [&>option]:text-white"
        >
          <option value="">— без приоритета —</option>
          {data.priority !== null && !availablePositions.includes(data.priority) && (
            <option value={data.priority}>{data.priority}</option>
          )}
          {availablePositions.map(n => (
            <option key={n} value={n}>{n}</option>
          ))}
        </select>
      </div>
      <div className="flex gap-2">
        <Button onClick={handleSubmit} size="sm" className="flex-1" disabled={isDisabled}>
          {isDisabled ? 'Загрузка...' : submitLabel}
        </Button>
        {onCancel && (
          <Button onClick={onCancel} size="sm" variant="outline" disabled={isDisabled}>Отмена</Button>
        )}
      </div>
    </div>
  )
}

function TrendsTab() {
  const [trends, setTrends] = useState<any[]>([])
  const [models, setModels] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [limit] = useState(50)
  const [offset, setOffset] = useState(0)

  const [selected, setSelected] = useState<any | null>(null)
  const [editing, setEditing] = useState(false)
  const [editForm, setEditForm] = useState<TrendFormData>(EMPTY_TREND_FORM)

  const [showCreate, setShowCreate] = useState(false)
  const [createForm, setCreateForm] = useState<TrendFormData>(EMPTY_TREND_FORM)

  const [takenForCreate, setTakenForCreate] = useState<number[]>([])
  const [takenForEdit, setTakenForEdit] = useState<number[]>([])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await adminApi.getTrends(limit, offset)
      setTrends(res.data || [])
      setTotal(res.total)
    } finally {
      setLoading(false)
    }
  }, [limit, offset])

  useEffect(() => {
    load()
    generationApi.getModels().then(res => {
      setModels(res.filter((m: any) => m.type === 'image' || m.type === 'video'))
    }).catch(console.error)
  }, [load])

  useEffect(() => {
    if (showCreate) adminApi.getTrendPriorities(0).then(r => setTakenForCreate(r.taken)).catch(console.error)
  }, [showCreate])

  useEffect(() => {
    if (editing && selected) adminApi.getTrendPriorities(selected.id).then(r => setTakenForEdit(r.taken)).catch(console.error)
  }, [editing, selected])

  const handleCreate = async (resolvedOutput: string) => {
    setUploading(true)
    try {
      await adminApi.createTrend({ 
        title: createForm.title, 
        output: resolvedOutput, 
        input_video: createForm.inputVideo,
        prompt: createForm.prompt, 
        model: createForm.model, 
        is_popular: createForm.isPopular, 
        priority: createForm.priority 
      })
      setCreateForm(EMPTY_TREND_FORM)
      setShowCreate(false)
      load()
    } catch (err) {
      alert('Ошибка: ' + (err instanceof Error ? err.message : String(err)))
    } finally {
      setUploading(false)
    }
  }

  const handleUpdate = async (resolvedOutput: string) => {
    if (!selected) return
    setUploading(true)
    try {
      await adminApi.updateTrend(selected.id, { 
        title: editForm.title, 
        output: resolvedOutput, 
        input_video: editForm.inputVideo,
        prompt: editForm.prompt, 
        model: editForm.model, 
        is_popular: editForm.isPopular, 
        priority: editForm.priority 
      })
      setEditing(false)
      setSelected(null)
      load()
    } catch (err) {
      alert('Ошибка: ' + (err instanceof Error ? err.message : String(err)))
    } finally {
      setUploading(false)
    }
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Удалить этот тренд?')) return
    try {
      await adminApi.deleteTrend(id)
      setSelected(null)
      load()
    } catch (err) {
      console.error(err)
    }
  }

  const startEdit = () => {
    if (!selected) return
    setEditForm({
      title: selected.title || '',
      output: selected.output,
      inputVideo: selected.input_video || '',
      prompt: selected.prompt || '',
      model: selected.model || '',
      isPopular: selected.is_popular || false,
      priority: selected.priority ?? null,
    })
    setEditing(true)
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-xs text-white/40">Всего: {total}</span>
        <Button size="sm" onClick={() => { setShowCreate(v => !v); setCreateForm(EMPTY_TREND_FORM) }}>
          {showCreate ? 'Отмена' : '+ Добавить'}
        </Button>
      </div>

      {showCreate && (
        <div className="p-4 rounded-xl border border-white/10 bg-white/5">
          <p className="text-sm font-semibold text-white/70 mb-3">Новый тренд</p>
          <TrendForm
            data={createForm}
            onChange={setCreateForm}
            models={models}
            takenPriorities={takenForCreate}
            uploading={uploading}
            onSubmit={handleCreate}
            onCancel={() => setShowCreate(false)}
            submitLabel="Добавить"
          />
        </div>
      )}

      {loading ? (
        <div className="grid grid-cols-3 gap-2">
          {[...Array(9)].map((_, i) => <div key={i} className="aspect-square rounded-xl bg-white/5 animate-pulse" />)}
        </div>
      ) : (
        <div className="grid grid-cols-3 gap-2">
          {trends.map((t) => {
            const isVideo = /\.mp4(\?|$)/i.test(t.output || '')
            return (
              <button
                key={t.id}
                onClick={() => { setSelected(t); setEditing(false) }}
                className="relative aspect-square rounded-xl overflow-hidden bg-white/10 group"
              >
                {isVideo ? (
                  <video src={t.output} muted loop autoPlay playsInline className="w-full h-full object-cover" />
                ) : (
                  <img
                    src={t.output} alt=""
                    className="w-full h-full object-cover"
                    loading="lazy"
                    onError={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }}
                  />
                )}
                {/* fallback label when media fails */}
                <span className="absolute inset-0 flex items-center justify-center text-[10px] text-white/30 pointer-events-none">
                  {t.title || t.model || '—'}
                </span>
                {t.is_popular && (
                  <span className="absolute top-1 left-1 text-[10px] font-bold bg-[#FFB700] text-black rounded px-1.5 py-0.5">★</span>
                )}
                {t.priority != null && (
                  <span className="absolute top-1 right-1 text-[10px] font-bold bg-white/20 text-white rounded px-1.5 py-0.5">#{t.priority}</span>
                )}
                {t.title && (
                  <div className="absolute bottom-0 left-0 right-0 px-1.5 pb-1.5 pt-4" style={{ background: 'linear-gradient(to top, rgba(0,0,0,0.8), transparent)' }}>
                    <p className="text-[10px] text-white font-medium leading-tight line-clamp-2">{t.title}</p>
                  </div>
                )}
              </button>
            )
          })}
        </div>
      )}

      <div className="flex justify-end gap-2 pt-1">
        <Button size="sm" variant="outline" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))} className="border-white/10">← Назад</Button>
        <Button size="sm" variant="outline" disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)} className="border-white/10">Вперёд →</Button>
      </div>

      {selected && createPortal(
        <div
          style={{ position: 'fixed', inset: 0, zIndex: 9000, background: 'rgba(0,0,0,0.7)' }}
          onClick={(e) => { if (e.target === e.currentTarget) { setSelected(null); setEditing(false) } }}
        >
          <div
            style={{ position: 'fixed', bottom: 0, left: 0, right: 0, maxHeight: '85vh', overflowY: 'auto', background: '#111', borderRadius: '20px 20px 0 0', padding: '20px' }}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ width: 40, height: 4, background: 'rgba(255,255,255,0.2)', borderRadius: 2, margin: '0 auto 16px' }} />
            {editing ? (
              <>
                <p className="text-sm font-semibold text-white/70 mb-3">Редактировать тренд</p>
                <TrendForm
                  data={editForm}
                  onChange={setEditForm}
                  models={models}
                  takenPriorities={takenForEdit}
                  uploading={uploading}
                  onSubmit={handleUpdate}
                  onCancel={() => setEditing(false)}
                  submitLabel="Сохранить"
                />
              </>
            ) : (
              <>
                {/\.mp4(\?|$)/i.test(selected.output || '') ? (
                  <video src={selected.output} controls autoPlay muted loop style={{ width: '100%', maxHeight: 320, objectFit: 'contain', borderRadius: 12, background: '#000', display: 'block' }} />
                ) : (
                  <img src={selected.output} alt="" style={{ width: '100%', maxHeight: 320, objectFit: 'contain', borderRadius: 12, background: '#000', display: 'block' }} />
                )}
                <div style={{ marginTop: 12 }}>
                  {selected.title && <p style={{ margin: '0 0 8px', fontSize: '16px', fontWeight: 700, color: 'white' }}>{selected.title}</p>}
                  <div className="flex items-center gap-2 mb-2 flex-wrap">
                    {selected.model && <span className="text-xs text-white/40 bg-white/10 px-2 py-1 rounded-full">{selected.model}</span>}
                    {selected.is_popular && <span className="text-xs font-bold bg-[#FFB700] text-black px-2 py-1 rounded-full">Популярное</span>}
                    {selected.priority != null && <span className="text-xs font-bold bg-white/10 text-white/60 px-2 py-1 rounded-full">Место #{selected.priority}</span>}
                  </div>
                  {selected.prompt && <p className="text-sm text-white/70 mb-4">{selected.prompt}</p>}
                  <div className="flex gap-2">
                    <Button size="sm" variant="outline" onClick={startEdit} className="flex-1">Редактировать</Button>
                    <Button size="sm" variant="outline" onClick={() => handleDelete(selected.id)} className="flex-1 text-red-400 hover:text-red-300">Удалить</Button>
                  </div>
                </div>
              </>
            )}
          </div>
        </div>,
        document.body
      )}
    </div>
  )
}

// ─── Promo Codes Tab ──────────────────────────────────────────────────────────

interface PromoCode {
  id: number; code: string; description: string
  image_tokens: number; video_tokens: number; text_tokens: number; music_tokens: number
  max_activations: number | null; activations_count: number
  expires_at: string | null; is_active: boolean
}

const EMPTY_PROMO = (): Omit<PromoCode, 'id' | 'activations_count'> => ({
  code: '', description: '', image_tokens: 0, video_tokens: 0, text_tokens: 0, music_tokens: 0,
  max_activations: null, expires_at: null, is_active: true,
})

function PromoCodesTab() {
  const [list, setList] = useState<PromoCode[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [editItem, setEditItem] = useState<PromoCode | null>(null)
  const [form, setForm] = useState(EMPTY_PROMO())
  const [saving, setSaving] = useState(false)

  const load = () => {
    setLoading(true)
    adminApi.getPromoCodes().then((r: any) => setList(r.data || [])).catch(() => {}).finally(() => setLoading(false))
  }

  useEffect(() => { load() }, [])

  const openCreate = () => { setForm(EMPTY_PROMO()); setEditItem(null); setShowCreate(true) }
  const openEdit = (p: PromoCode) => {
    setForm({ code: p.code, description: p.description, image_tokens: p.image_tokens, video_tokens: p.video_tokens, text_tokens: p.text_tokens, music_tokens: p.music_tokens, max_activations: p.max_activations, expires_at: p.expires_at ? p.expires_at.slice(0, 10) : null, is_active: p.is_active })
    setEditItem(p); setShowCreate(true)
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const payload = { ...form, expires_at: form.expires_at || null }
      if (editItem) {
        await adminApi.updatePromoCode(editItem.id, payload)
      } else {
        await adminApi.createPromoCode(payload as any)
      }
      setShowCreate(false); load()
    } catch (e: any) { alert(e?.message || 'Ошибка') } finally { setSaving(false) }
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Удалить промокод?')) return
    await adminApi.deletePromoCode(id).catch(() => {})
    load()
  }

  const tokenField = (label: string, key: 'image_tokens' | 'video_tokens' | 'text_tokens' | 'music_tokens') => (
    <div>
      <label className="text-xs text-white/50 mb-1 block">{label}</label>
      <input type="number" min={0} value={form[key]} onChange={(e) => setForm({ ...form, [key]: Number(e.target.value) })}
        className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm" />
    </div>
  )

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <span className="text-sm text-white/50">Всего: {list.length}</span>
        <Button size="sm" onClick={openCreate} className="rounded-xl bg-primary text-black font-bold">+ Создать</Button>
      </div>

      {loading ? <p className="text-white/30 text-sm">Загрузка...</p> : list.length === 0 ? (
        <p className="text-white/30 text-sm text-center py-8">Промокодов нет</p>
      ) : (
        <div className="space-y-2">
          {list.map((p) => (
            <div key={p.id} className="flex items-center gap-3 px-4 py-3 rounded-xl bg-white/[0.03] border border-white/5">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="font-mono font-bold text-sm text-white">{p.code}</span>
                  <span className={`text-[10px] px-2 py-0.5 rounded-full font-bold ${p.is_active ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                    {p.is_active ? 'Активен' : 'Отключён'}
                  </span>
                  {p.expires_at && <span className="text-[10px] text-white/30">до {new Date(p.expires_at).toLocaleDateString('ru-RU')}</span>}
                </div>
                {p.description && <p className="text-xs text-white/40 mt-0.5 truncate">{p.description}</p>}
                <div className="flex gap-3 mt-1 flex-wrap">
                  {p.image_tokens > 0 && <span className="text-[11px] text-green-400">📷 +{p.image_tokens}</span>}
                  {p.video_tokens > 0 && <span className="text-[11px] text-orange-400">🎬 +{p.video_tokens}</span>}
                  {p.text_tokens > 0 && <span className="text-[11px] text-blue-400">💬 +{p.text_tokens}</span>}
                  {p.music_tokens > 0 && <span className="text-[11px] text-purple-400">🎵 +{p.music_tokens}</span>}
                  <span className="text-[11px] text-white/30">
                    {p.activations_count}{p.max_activations !== null ? `/${p.max_activations}` : ''} активаций
                  </span>
                </div>
              </div>
              <button onClick={() => openEdit(p)} className="text-xs text-white/40 hover:text-white px-2 py-1 rounded-lg hover:bg-white/5">✏️</button>
              <button onClick={() => handleDelete(p.id)} className="text-xs text-red-400/60 hover:text-red-400 px-2 py-1 rounded-lg hover:bg-red-500/10">🗑</button>
            </div>
          ))}
        </div>
      )}

      {showCreate && createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
          <div className="w-full max-w-md bg-[#111] border border-white/10 rounded-2xl p-6 space-y-4 max-h-[90vh] overflow-y-auto">
            <h3 className="text-lg font-bold text-white">{editItem ? 'Редактировать промокод' : 'Новый промокод'}</h3>
            <div>
              <label className="text-xs text-white/50 mb-1 block">Код *</label>
              <input value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value.toUpperCase() })} disabled={!!editItem}
                placeholder="SUMMER2025" className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm font-mono tracking-widest uppercase" />
            </div>
            <div>
              <label className="text-xs text-white/50 mb-1 block">Описание</label>
              <input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })}
                placeholder="Летняя акция" className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              {tokenField('Фото токены', 'image_tokens')}
              {tokenField('Видео токены', 'video_tokens')}
              {tokenField('Текст токены', 'text_tokens')}
              {tokenField('Муз. токены', 'music_tokens')}
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs text-white/50 mb-1 block">Макс. активаций (пусто = ∞)</label>
                <input type="number" min={1} value={form.max_activations ?? ''} onChange={(e) => setForm({ ...form, max_activations: e.target.value ? Number(e.target.value) : null })}
                  placeholder="∞" className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm" />
              </div>
              <div>
                <label className="text-xs text-white/50 mb-1 block">Действует до (пусто = бессрочно)</label>
                <input type="date" value={form.expires_at ?? ''} onChange={(e) => setForm({ ...form, expires_at: e.target.value || null })}
                  className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm" />
              </div>
            </div>
            <label className="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" checked={form.is_active} onChange={(e) => setForm({ ...form, is_active: e.target.checked })} className="w-4 h-4 accent-primary" />
              <span className="text-sm text-white/70">Активен</span>
            </label>
            <div className="flex gap-2 pt-2">
              <Button onClick={handleSave} disabled={saving || !form.code.trim()} className="flex-1 rounded-xl font-bold bg-gradient-to-r from-[#FFD700] via-[#FFB700] to-[#FF8C00] text-black">
                {saving ? 'Сохранение...' : 'Сохранить'}
              </Button>
              <Button variant="outline" onClick={() => setShowCreate(false)} className="rounded-xl border-white/10">Отмена</Button>
            </div>
          </div>
        </div>,
        document.body
      )}
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────
export function AdminPage() {
  const { user } = useAuthStore()
  const navigate = useNavigate()
  const [tab, setTab] = useState<Tab>('stats')

  useEffect(() => {
    if (!user?.is_admin) navigate('/', { replace: true })
  }, [user, navigate])

  if (!user?.is_admin) return null

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <div className="flex flex-col gap-1">
        <h1 className="text-4xl font-bold tracking-tight bg-gradient-to-br from-white to-white/40 bg-clip-text text-transparent">
          👑 Админ-панель
        </h1>
        <p className="text-white/40 text-sm">Управление пользователями, статистика и рассылки</p>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 flex-wrap border-b border-white/5 pb-0">
        {TABS.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`px-4 py-2.5 text-sm font-medium rounded-t-lg transition-all ${
              tab === t.id
                ? 'bg-white/[0.06] text-white border-b-2 border-primary -mb-px'
                : 'text-white/40 hover:text-white/70 hover:bg-white/[0.03]'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm">
        <CardHeader className="pb-2">
          <CardTitle className="text-base text-white/70">{TABS.find(t => t.id === tab)?.label}</CardTitle>
        </CardHeader>
        <CardContent>
          {tab === 'stats' && <StatsTab />}
          {tab === 'users' && <UsersTab />}
          {tab === 'top_users' && <TopUsersTab />}
          {tab === 'generations' && <GenerationsTab />}
          {tab === 'payments' && <PaymentsTab />}
          {tab === 'gallery_ideas' && <GalleryIdeasTab />}
          {tab === 'trends' && <TrendsTab />}
          {tab === 'promo_codes' && <PromoCodesTab />}
        </CardContent>
      </Card>
    </div>
  )
}

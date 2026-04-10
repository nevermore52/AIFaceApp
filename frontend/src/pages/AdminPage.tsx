import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { adminApi } from '../lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'

type Tab = 'stats' | 'users' | 'top_users' | 'generations' | 'payments' | 'gallery_ideas'

const TABS: { id: Tab; label: string }[] = [
  { id: 'stats', label: '📊 Статистика' },
  { id: 'users', label: '👥 Пользователи' },
  { id: 'top_users', label: '📈 Топ-24ч' },
  { id: 'generations', label: '🎨 Генерации' },
  { id: 'payments', label: '💳 Платежи' },
  { id: 'gallery_ideas', label: '💡 Идеи' },
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
function GalleryIdeasTab() {
  const [ideas, setIdeas] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [limit] = useState(20)
  const [offset, setOffset] = useState(0)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [formData, setFormData] = useState({ model: '', output: '', prompt: '' })

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

  useEffect(() => { load() }, [load])

  const handleCreate = async () => {
    if (!formData.model || !formData.output || !formData.prompt) return
    try {
      await adminApi.createGalleryIdea(formData)
      setFormData({ model: '', output: '', prompt: '' })
      load()
    } catch (err) {
      console.error('Failed to create idea:', err)
    }
  }

  const handleUpdate = async (id: number) => {
    if (!formData.model || !formData.output || !formData.prompt) return
    try {
      await adminApi.updateGalleryIdea(id, formData)
      setEditingId(null)
      setFormData({ model: '', output: '', prompt: '' })
      load()
    } catch (err) {
      console.error('Failed to update idea:', err)
    }
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Удалить эту идею?')) return
    try {
      await adminApi.deleteGalleryIdea(id)
      load()
    } catch (err) {
      console.error('Failed to delete idea:', err)
    }
  }

  const startEdit = (idea: any) => {
    setEditingId(idea.id)
    setFormData({ model: idea.model, output: idea.output, prompt: idea.prompt })
  }

  const cancelEdit = () => {
    setEditingId(null)
    setFormData({ model: '', output: '', prompt: '' })
  }

  return (
    <div className="space-y-4">
      <div className="p-4 rounded-xl border border-white/10 bg-white/5 space-y-3">
        <p className="text-sm font-semibold text-white/70">Добавить новую идею</p>
        <input
          type="text"
          placeholder="Модель (например: flux-pro)"
          value={formData.model}
          onChange={(e) => setFormData({ ...formData, model: e.target.value })}
          className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm"
        />
        <input
          type="text"
          placeholder="URL изображения"
          value={formData.output}
          onChange={(e) => setFormData({ ...formData, output: e.target.value })}
          className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm"
        />
        <textarea
          placeholder="Промпт"
          value={formData.prompt}
          onChange={(e) => setFormData({ ...formData, prompt: e.target.value })}
          className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm min-h-[80px]"
        />
        <Button onClick={handleCreate} size="sm" className="w-full">Добавить</Button>
      </div>

      {loading ? <div className="text-white/40 text-sm">Загрузка...</div> : (
        <div className="space-y-2">
          {ideas.map((idea) => (
            <div key={idea.id} className="p-4 rounded-xl border border-white/5 bg-white/[0.02] space-y-3">
              {editingId === idea.id ? (
                <>
                  <input
                    type="text"
                    value={formData.model}
                    onChange={(e) => setFormData({ ...formData, model: e.target.value })}
                    className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm"
                  />
                  <input
                    type="text"
                    value={formData.output}
                    onChange={(e) => setFormData({ ...formData, output: e.target.value })}
                    className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm"
                  />
                  <textarea
                    value={formData.prompt}
                    onChange={(e) => setFormData({ ...formData, prompt: e.target.value })}
                    className="w-full px-3 py-2 rounded-lg bg-white/5 border border-white/10 text-white text-sm min-h-[80px]"
                  />
                  <div className="flex gap-2">
                    <Button onClick={() => handleUpdate(idea.id)} size="sm" variant="default">Сохранить</Button>
                    <Button onClick={cancelEdit} size="sm" variant="outline">Отмена</Button>
                  </div>
                </>
              ) : (
                <>
                  <div className="flex items-start gap-3">
                    <img src={idea.output} alt="" className="w-20 h-20 rounded-lg object-cover" />
                    <div className="flex-1 min-w-0">
                      <p className="text-xs text-white/40 mb-1">{idea.model}</p>
                      <p className="text-sm text-white/70 italic">&ldquo;{idea.prompt}&rdquo;</p>
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <Button onClick={() => startEdit(idea)} size="sm" variant="outline">Редактировать</Button>
                    <Button onClick={() => handleDelete(idea.id)} size="sm" variant="outline" className="text-red-400 hover:text-red-300">Удалить</Button>
                  </div>
                </>
              )}
            </div>
          ))}
        </div>
      )}

      <div className="flex justify-between items-center pt-2">
        <span className="text-xs text-white/40">Всего: {total}</span>
        <div className="flex gap-2">
          <Button size="sm" variant="outline" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}
            className="border-white/10">← Назад</Button>
          <Button size="sm" variant="outline" disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}
            className="border-white/10">Вперёд →</Button>
        </div>
      </div>
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
        </CardContent>
      </Card>
    </div>
  )
}

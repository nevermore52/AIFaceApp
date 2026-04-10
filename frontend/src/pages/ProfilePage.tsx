import { useState, useEffect, useRef, useCallback } from 'react'
import { useAuthStore } from '../store/auth'
import { userApi, authApi, API_BASE_URL } from '../lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'
import { Copy, Check, Users, Shield, Image, Music, Video, MessageSquare, ChevronRight, ChevronLeft, History } from 'lucide-react'
import { useSearchParams, useNavigate, Link } from 'react-router-dom'
import { formatDate, cn } from '../lib/utils'

interface Generation {
  id: number
  model_type: string
  model: string
  status: string
  prompt?: string
  output?: string
  created_at: string
  completed_at?: string
}

type Tab = 'profile' | 'history' | 'partnership'

export function ProfilePage() {
  const { user, updateUser, accessToken } = useAuthStore()
  const [searchParams, setSearchParams] = useSearchParams()
  const navigate = useNavigate()

  const initialTab = (searchParams.get('tab') as Tab) || 'profile'
  const [activeTab, setActiveTab] = useState<Tab>(initialTab)

  const [saving, setSaving] = useState(false)
  const [copied, setCopied] = useState(false)
  const [linkNotice, setLinkNotice] = useState<{ type: 'success' | 'error'; msg: string } | null>(null)
  const [tgLinkStatus, setTgLinkStatus] = useState<'idle' | 'waiting' | 'linked' | 'error'>('idle')
  const tgPollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // History state
  const [generations, setGenerations] = useState<Generation[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [historyLoading, setHistoryLoading] = useState(false)
  const limit = 10

  const botName = (import.meta.env.VITE_TELEGRAM_BOT_NAME || 'aifaceappbot').replace('@', '')
  const referralLink = user?.referral_code ? `https://t.me/${botName}?start=${user.referral_code}` : ''

  const [formData, setFormData] = useState({
    first_name: user?.first_name || '',
    last_name: user?.last_name || '',
    username: user?.username || '',
  })

  useEffect(() => {
    const linked = searchParams.get('linked')
    const linkError = searchParams.get('link_error')
    if (linked === 'google') {
      setLinkNotice({ type: 'success', msg: 'Google аккаунт успешно привязан!' })
      userApi.getMe().then((data) => updateUser(data as Parameters<typeof updateUser>[0])).catch(() => {})
    } else if (linkError) {
      setLinkNotice({ type: 'error', msg: 'Не удалось привязать Google аккаунт. Возможно, он уже используется другим профилем.' })
    }
  }, [])

  useEffect(() => () => { if (tgPollRef.current) clearInterval(tgPollRef.current) }, [])

  useEffect(() => {
    if (activeTab === 'history') loadGenerations()
  }, [activeTab, page])

  const loadGenerations = async () => {
    setHistoryLoading(true)
    try {
      const response = await userApi.getHistory(limit, page * limit) as { data: Generation[]; total: number }
      setGenerations(response.data || [])
      setTotal(response.total)
    } catch (e) {
      console.error(e)
    } finally {
      setHistoryLoading(false)
    }
  }

  const switchTab = (tab: Tab) => {
    setActiveTab(tab)
    setSearchParams(tab === 'profile' ? {} : { tab })
    if (tab === 'history') setPage(0)
  }

  const stopTgPoll = useCallback(() => {
    if (tgPollRef.current) { clearInterval(tgPollRef.current); tgPollRef.current = null }
  }, [])

  const handleCopyLink = async () => {
    if (!referralLink) return
    try {
      await navigator.clipboard.writeText(referralLink)
    } catch {
      const el = document.createElement('textarea')
      el.value = referralLink
      document.body.appendChild(el)
      el.select()
      document.execCommand('copy')
      document.body.removeChild(el)
    }
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleLinkTelegram = async () => {
    try {
      const { token } = await authApi.createLinkToken()
      window.open(`https://t.me/${botName}?start=link-${token}`, '_blank', 'noopener,noreferrer')
      setTgLinkStatus('waiting')
      stopTgPoll()
      tgPollRef.current = setInterval(async () => {
        try {
          const result = await authApi.getWebTokenStatus(token)
          if (result.status === 'linked') {
            stopTgPoll()
            setTgLinkStatus('linked')
            userApi.getMe().then((data) => updateUser(data as Parameters<typeof updateUser>[0])).catch(() => {})
          }
        } catch { /* keep polling */ }
      }, 3000)
      setTimeout(() => { stopTgPoll(); setTgLinkStatus((s) => s === 'waiting' ? 'error' : s) }, 5 * 60 * 1000)
    } catch {
      setTgLinkStatus('error')
    }
  }

  const handleLinkGoogle = () => {
    window.location.href = `${API_BASE_URL}/auth/google?link_token=${accessToken}`
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    try {
      await userApi.updateProfile(formData)
      updateUser(formData)
    } catch (error) {
      console.error('Failed to update profile:', error)
    } finally {
      setSaving(false)
    }
  }

  const getIcon = (modelType: string) => {
    switch (modelType) {
      case 'image': return Image
      case 'music': return Music
      case 'video': return Video
      default: return MessageSquare
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed': return 'text-green-500 bg-green-950'
      case 'processing': return 'text-yellow-500 bg-yellow-950'
      case 'failed': return 'text-red-500 bg-red-950'
      default: return 'text-gray-500 bg-gray-950'
    }
  }

  const getStatusText = (status: string) => {
    switch (status) {
      case 'completed': return 'Завершено'
      case 'processing': return 'В процессе'
      case 'failed': return 'Ошибка'
      default: return status
    }
  }

  const totalPages = Math.ceil(total / limit)

  const tabs = [
    { id: 'profile' as Tab, label: 'Профиль' },
    { id: 'history' as Tab, label: 'История' },
    { id: 'partnership' as Tab, label: 'Партнерство' },
  ]

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      {/* Tab bar */}
      <div className="flex gap-2 p-1 rounded-2xl bg-white/[0.03] border border-white/5 w-fit">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => switchTab(tab.id)}
            className={cn(
              'px-5 py-2 rounded-xl text-sm font-semibold transition-all duration-200',
              activeTab === tab.id
                ? 'bg-white/10 text-white shadow-sm'
                : 'text-white/40 hover:text-white/70'
            )}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Notification */}
      {linkNotice && (
        <div className={`rounded-xl border px-4 py-3 text-sm font-medium ${
          linkNotice.type === 'success'
            ? 'border-green-500/30 bg-green-500/10 text-green-300'
            : 'border-red-500/30 bg-red-500/10 text-red-300'
        }`}>
          {linkNotice.msg}
        </div>
      )}

      {/* ── Профиль ── */}
      {activeTab === 'profile' && (
        <div className="space-y-6">
          <div className="grid gap-6 md:grid-cols-12">
            <div className="md:col-span-7 space-y-6">
              <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm">
                <CardHeader className="pb-4">
                  <CardTitle className="text-lg flex items-center gap-2 text-white/90">
                    <div className="h-6 w-1 bg-primary rounded-full" />
                    Личные данные
                  </CardTitle>
                  <CardDescription className="text-white/40 font-medium">Обновите информацию о себе</CardDescription>
                </CardHeader>
                <CardContent>
                  <form onSubmit={handleSubmit} className="space-y-4">
                    <div className="grid gap-4 md:grid-cols-2">
                      <div className="space-y-2">
                        <label className="text-xs font-semibold uppercase tracking-wider text-white/40 ml-1" htmlFor="first_name">Имя</label>
                        <input
                          id="first_name"
                          type="text"
                          value={formData.first_name}
                          onChange={(e) => setFormData({ ...formData, first_name: e.target.value })}
                          className="w-full px-4 py-3 rounded-xl border border-white/5 bg-white/[0.03] text-sm text-white/80 transition-all focus:border-primary/50 focus:ring-0 hover:bg-white/[0.05]"
                        />
                      </div>
                      <div className="space-y-2">
                        <label className="text-xs font-semibold uppercase tracking-wider text-white/40 ml-1" htmlFor="last_name">Фамилия</label>
                        <input
                          id="last_name"
                          type="text"
                          value={formData.last_name}
                          onChange={(e) => setFormData({ ...formData, last_name: e.target.value })}
                          className="w-full px-4 py-3 rounded-xl border border-white/5 bg-white/[0.03] text-sm text-white/80 transition-all focus:border-primary/50 focus:ring-0 hover:bg-white/[0.05]"
                        />
                      </div>
                    </div>
                    <div className="space-y-2">
                      <label className="text-xs font-semibold uppercase tracking-wider text-white/40 ml-1" htmlFor="username">Username</label>
                      <div className="relative">
                        <span className="absolute left-4 top-1/2 -translate-y-1/2 text-white/20 text-sm">@</span>
                        <input
                          id="username"
                          type="text"
                          value={formData.username}
                          readOnly
                          className="w-full pl-8 pr-4 py-3 rounded-xl border border-white/5 bg-white/[0.03] text-sm text-white/40 cursor-not-allowed opacity-60"
                        />
                      </div>
                      <p className="text-[10px] text-white/30 ml-1">Username не может быть изменен</p>
                    </div>
                    <Button type="submit" disabled={saving} className="w-full py-5 rounded-xl font-bold">
                      {saving ? (
                        <div className="flex items-center gap-2">
                          <span className="h-4 w-4 animate-spin rounded-full border-2 border-white/20 border-t-white" />
                          Сохранение...
                        </div>
                      ) : 'Обновить профиль'}
                    </Button>
                  </form>
                </CardContent>
              </Card>
            </div>

            <div className="md:col-span-5">
              <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm h-full">
                <CardHeader className="pb-4">
                  <CardTitle className="text-lg flex items-center gap-2 text-white/90">
                    <div className="h-6 w-1 bg-primary rounded-full" />
                    Безопасность
                  </CardTitle>
                  <CardDescription className="text-white/40 font-medium">Способы входа в ваш аккаунт</CardDescription>
                </CardHeader>
                <CardContent className="space-y-3">
                  {/* Telegram */}
                  <div className="p-4 rounded-2xl border border-white/5 bg-white/[0.03] hover:bg-white/[0.05] transition-all group">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <div className="w-10 h-10 rounded-xl bg-blue-500/10 flex items-center justify-center">
                          <svg className="w-5 h-5 text-blue-500" fill="currentColor" viewBox="0 0 24 24">
                            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm4.64 6.8c-.15 1.58-.8 5.42-1.13 7.19-.14.75-.42 1-.68 1.03-.58.05-1.02-.38-1.58-.75-.88-.58-1.38-.94-2.23-1.5-.99-.65-.35-1.01.22-1.59.15-.15 2.71-2.48 2.76-2.69a.2.2 0 00-.05-.18c-.06-.05-.14-.03-.21-.02-.09.02-1.49.95-4.22 2.79-.4.27-.76.41-1.08.4-.36-.01-1.04-.2-1.55-.37-.63-.2-1.12-.31-1.08-.66.02-.18.27-.36.74-.55 2.92-1.27 4.86-2.11 5.83-2.51 2.78-1.16 3.35-1.36 3.73-1.36.08 0 .27.02.39.12.1.08.13.19.14.27-.01.06.01.24 0 .38z"/>
                          </svg>
                        </div>
                        <div>
                          <p className="text-sm font-semibold text-white/90">Telegram</p>
                          <p className="text-[10px] text-white/20 mt-0.5">
                            {user?.telegram_id ? `ID: ${user.telegram_id}` : tgLinkStatus === 'waiting' ? 'Ожидание...' : tgLinkStatus === 'linked' ? 'Привязан!' : 'Не привязан'}
                          </p>
                        </div>
                      </div>
                      {user?.telegram_id || tgLinkStatus === 'linked' ? (
                        <div className="px-3 py-1 rounded-full bg-green-500/10 border border-green-500/20 text-green-500 text-[10px] font-bold uppercase tracking-widest">Активен</div>
                      ) : tgLinkStatus === 'waiting' ? (
                        <div className="h-4 w-4 animate-spin rounded-full border-2 border-white/10 border-t-primary" />
                      ) : (
                        <Button variant="outline" size="sm" className="rounded-full h-8 text-[10px] font-bold uppercase tracking-widest border-white/10 hover:bg-white/10" onClick={handleLinkTelegram}>Связать</Button>
                      )}
                    </div>
                    {tgLinkStatus === 'waiting' && <p className="text-[10px] text-white/30 mt-2 ml-13 animate-pulse">Нажмите «Войти на сайт» в боте...</p>}
                    {tgLinkStatus === 'error' && <p className="text-[10px] text-red-400/70 mt-2">Время вышло. Попробуйте ещё раз.</p>}
                  </div>

                  {/* Google */}
                  <div className="p-4 rounded-2xl border border-white/5 bg-white/[0.03] hover:bg-white/[0.05] transition-all group">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <div className="w-10 h-10 rounded-xl bg-white/5 flex items-center justify-center">
                          <svg className="w-5 h-5" viewBox="0 0 24 24">
                            <path fill="#EA4335" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                            <path fill="#FBBC05" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                            <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                            <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                          </svg>
                        </div>
                        <div>
                          <p className="text-sm font-semibold text-white/90">Google</p>
                          <p className="text-[10px] text-white/20 mt-0.5">{user?.email || 'Не привязан'}</p>
                        </div>
                      </div>
                      {user?.email ? (
                        <div className="px-3 py-1 rounded-full bg-green-500/10 border border-green-500/20 text-green-500 text-[10px] font-bold uppercase tracking-widest">Активен</div>
                      ) : (
                        <Button variant="outline" size="sm" className="rounded-full h-8 text-[10px] font-bold uppercase tracking-widest border-white/10 hover:bg-white/10" onClick={handleLinkGoogle}>Связать</Button>
                      )}
                    </div>
                  </div>
                </CardContent>
              </Card>
            </div>
          </div>

          {user?.is_admin && (
            <Card className="border-yellow-500/20 bg-yellow-500/5 cursor-pointer hover:bg-yellow-500/10 transition-all" onClick={() => navigate('/admin')}>
              <CardContent className="flex items-center gap-4 py-4">
                <div className="w-10 h-10 rounded-xl bg-yellow-500/20 flex items-center justify-center flex-shrink-0">
                  <Shield className="w-5 h-5 text-yellow-400" />
                </div>
                <div className="flex-1">
                  <p className="font-semibold text-yellow-300">Панель администратора</p>
                  <p className="text-xs text-yellow-400/60">Статистика, пользователи, рассылки</p>
                </div>
                <span className="text-yellow-400/40 text-lg">→</span>
              </CardContent>
            </Card>
          )}
        </div>
      )}

      {/* ── История ── */}
      {activeTab === 'history' && (
        <div className="space-y-4 w-full overflow-hidden">
          {historyLoading ? (
            <div className="grid gap-3">
              {[...Array(4)].map((_, i) => (
                <Card key={i} className="border-white/5 bg-white/[0.01]">
                  <CardContent className="p-4">
                    <div className="flex items-center gap-4">
                      <div className="h-10 w-10 rounded-xl bg-white/5 animate-pulse" />
                      <div className="flex-1 space-y-2">
                        <div className="h-3 w-1/4 bg-white/5 animate-pulse rounded" />
                        <div className="h-2 w-1/2 bg-white/5 animate-pulse rounded" />
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          ) : generations.length === 0 ? (
            <Card className="border-white/5 bg-white/[0.01] border-dashed">
              <CardContent className="p-16 text-center space-y-4">
                <div className="w-14 h-14 mx-auto rounded-full bg-white/5 flex items-center justify-center text-white/20">
                  <History className="w-7 h-7" />
                </div>
                <div className="space-y-1">
                  <p className="text-base font-medium text-white/80">История пуста</p>
                  <p className="text-sm text-white/40">Вы ещё ничего не создали</p>
                </div>
                <Button size="sm" variant="secondary" onClick={() => navigate('/generate')} className="rounded-full px-6 bg-white/5 border-white/10 hover:bg-white/10">
                  Перейти к генерации
                </Button>
              </CardContent>
            </Card>
          ) : (
            <>
              <div className="grid gap-3">
                {generations.map((gen) => {
                  const Icon = getIcon(gen.model_type)
                  return (
                    <Link key={gen.id} to={`/generations/${gen.id}`} style={{ display: 'block', width: '100%', minWidth: 0 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '12px', background: 'rgba(255,255,255,0.02)', border: '1px solid rgba(255,255,255,0.05)', borderRadius: '12px', width: '100%', boxSizing: 'border-box', overflow: 'hidden' }}>
                        <div style={{ padding: '8px', borderRadius: '10px', background: 'rgba(255,255,255,0.05)', flexShrink: 0 }}>
                          <Icon style={{ width: 18, height: 18, color: 'rgba(255,255,255,0.6)' }} />
                        </div>
                        <div style={{ flex: 1, minWidth: 0, overflow: 'hidden' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '2px' }}>
                            <span style={{ fontWeight: 600, fontSize: '13px', color: 'rgba(255,255,255,0.9)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0, flex: 1 }}>{gen.model}</span>
                            <span className={cn('text-[10px] px-2 py-0.5 rounded-full font-bold uppercase tracking-wider', getStatusColor(gen.status))} style={{ flexShrink: 0, whiteSpace: 'nowrap' }}>
                              {getStatusText(gen.status)}
                            </span>
                          </div>
                          {gen.prompt && (
                            <p style={{ fontSize: '11px', color: 'rgba(255,255,255,0.4)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontStyle: 'italic', margin: 0 }}>&ldquo;{gen.prompt}&rdquo;</p>
                          )}
                          <p style={{ fontSize: '10px', color: 'rgba(255,255,255,0.2)', marginTop: '2px', margin: 0 }}>{formatDate(gen.created_at)}</p>
                        </div>
                        <ChevronRight style={{ width: 15, height: 15, color: 'rgba(255,255,255,0.2)', flexShrink: 0 }} />
                      </div>
                    </Link>
                  )
                })}
              </div>

              {totalPages > 1 && (
                <div className="flex items-center justify-center gap-6 pt-2">
                  <Button variant="outline" size="sm" className="rounded-xl border-white/5 bg-white/[0.02] hover:bg-white/10"
                    onClick={() => setPage((p) => Math.max(0, p - 1))} disabled={page === 0}>
                    <ChevronLeft className="h-4 w-4 mr-2" />Назад
                  </Button>
                  <span className="text-xs font-semibold uppercase tracking-widest text-white/30">
                    {page + 1} <span className="text-white/10 mx-1">/</span> {totalPages}
                  </span>
                  <Button variant="outline" size="sm" className="rounded-xl border-white/5 bg-white/[0.02] hover:bg-white/10"
                    onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))} disabled={page >= totalPages - 1}>
                    Вперед<ChevronRight className="h-4 w-4 ml-2" />
                  </Button>
                </div>
              )}
            </>
          )}
        </div>
      )}

      {/* ── Партнерство ── */}
      {activeTab === 'partnership' && (
        <div className="space-y-6">
          <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm">
            <CardHeader className="pb-4">
              <CardTitle className="text-lg flex items-center gap-2 text-white/90">
                <div className="h-6 w-1 bg-primary rounded-full" />
                Реферальная программа
              </CardTitle>
              <CardDescription className="text-white/40 font-medium">Получайте 20% от покупок ваших рефералов</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <p className="text-sm text-white/50">
                🎁 Вы будете получать <span className="text-white/80 font-semibold">20% от покупок</span> ваших рефералов!
                Например, пользователь купил 10 генераций — вы получите 2 генерации бесплатно.
                <span className="text-white/30"> Не действует на подписки.</span>
              </p>

              <div className="flex items-center gap-2 p-3 rounded-xl bg-white/[0.03] border border-white/5">
                <Users className="h-4 w-4 text-primary flex-shrink-0" />
                <span className="text-sm text-white/50">Приглашено пользователей:</span>
                <span className="text-sm font-bold text-white ml-auto">{user?.referrals_count ?? 0}</span>
              </div>

              {referralLink && (
                <div className="space-y-2">
                  <label className="text-xs font-semibold uppercase tracking-wider text-white/40 ml-1">Ваша реферальная ссылка</label>
                  <div className="flex gap-2">
                    <input
                      readOnly
                      value={referralLink}
                      className="flex-1 min-w-0 px-3 py-2.5 rounded-xl border border-white/5 bg-white/[0.03] text-xs text-white/50 truncate"
                    />
                    <Button variant="outline" size="sm" onClick={handleCopyLink} className="rounded-xl border-white/10 hover:bg-white/5 flex-shrink-0 gap-1.5">
                      {copied ? <Check className="h-3.5 w-3.5 text-green-400" /> : <Copy className="h-3.5 w-3.5" />}
                      {copied ? 'Скопировано' : 'Копировать'}
                    </Button>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  )
}

import { useState } from 'react'
import { useAuthStore } from '../store/auth'
import { userApi } from '../lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Button } from '../components/ui/button'

export function ProfilePage() {
  const { user, updateUser } = useAuthStore()
  const [saving, setSaving] = useState(false)
  const [formData, setFormData] = useState({
    first_name: user?.first_name || '',
    last_name: user?.last_name || '',
    username: user?.username || '',
  })

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

  return (
    <div className="space-y-10 max-w-4xl mx-auto">
      <div className="flex flex-col gap-1">
        <h1 className="text-4xl font-bold tracking-tight bg-gradient-to-br from-white to-white/40 bg-clip-text text-transparent">
          Профиль
        </h1>
        <p className="text-white/40 text-sm">
          Управление вашими личными данными и подключенными аккаунтами
        </p>
      </div>

      <div className="grid gap-8 md:grid-cols-12">
        <div className="md:col-span-7 space-y-8">
          <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm overflow-hidden">
            <CardHeader className="pb-6">
              <CardTitle className="text-lg flex items-center gap-2 text-white/90">
                <div className="h-6 w-1 bg-primary rounded-full" />
                Личные данные
              </CardTitle>
              <CardDescription className="text-white/40 font-medium">Обновите информацию о себе</CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleSubmit} className="space-y-6">
                <div className="grid gap-6 md:grid-cols-2">
                  <div className="space-y-2.5">
                    <label className="text-xs font-semibold uppercase tracking-wider text-white/40 ml-1" htmlFor="first_name">
                      Имя
                    </label>
                    <input
                      id="first_name"
                      type="text"
                      value={formData.first_name}
                      onChange={(e) => setFormData({ ...formData, first_name: e.target.value })}
                      className="w-full px-4 py-3 rounded-xl border border-white/5 bg-white/[0.03] text-sm text-white/80 transition-all focus:border-primary/50 focus:ring-0 hover:bg-white/[0.05]"
                    />
                  </div>
                  <div className="space-y-2.5">
                    <label className="text-xs font-semibold uppercase tracking-wider text-white/40 ml-1" htmlFor="last_name">
                      Фамилия
                    </label>
                    <input
                      id="last_name"
                      type="text"
                      value={formData.last_name}
                      onChange={(e) => setFormData({ ...formData, last_name: e.target.value })}
                      className="w-full px-4 py-3 rounded-xl border border-white/5 bg-white/[0.03] text-sm text-white/80 transition-all focus:border-primary/50 focus:ring-0 hover:bg-white/[0.05]"
                    />
                  </div>
                </div>
                <div className="space-y-2.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-white/40 ml-1" htmlFor="username">
                    Username
                  </label>
                  <div className="relative">
                    <span className="absolute left-4 top-1/2 -translate-y-1/2 text-white/20 text-sm">@</span>
                    <input
                      id="username"
                      type="text"
                      value={formData.username}
                      onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                      className="w-full pl-8 pr-4 py-3 rounded-xl border border-white/5 bg-white/[0.03] text-sm text-white/80 transition-all focus:border-primary/50 focus:ring-0 hover:bg-white/[0.05]"
                    />
                  </div>
                </div>
                <Button 
                  type="submit" 
                  disabled={saving}
                  className="w-full py-6 rounded-xl font-bold transition-all shadow-[0_10px_30px_-10px_rgba(139,92,246,0.3)] hover:shadow-[0_15px_40px_-8px_rgba(139,92,246,0.4)]"
                >
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

        <div className="md:col-span-5 space-y-8">
          <Card className="border-white/5 bg-white/[0.02] backdrop-blur-sm h-full">
            <CardHeader className="pb-6">
              <CardTitle className="text-lg flex items-center gap-2 text-white/90">
                <div className="h-6 w-1 bg-primary rounded-full" />
                Безопасность
              </CardTitle>
              <CardDescription className="text-white/40 font-medium">Способы входа в ваш аккаунт</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="p-4 rounded-2xl border border-white/5 bg-white/[0.03] transition-all hover:bg-white/[0.05] group">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <div className="w-12 h-12 rounded-xl bg-blue-500/10 flex items-center justify-center transition-transform group-hover:scale-110 duration-300">
                      <svg className="w-6 h-6 text-blue-500" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm4.64 6.8c-.15 1.58-.8 5.42-1.13 7.19-.14.75-.42 1-.68 1.03-.58.05-1.02-.38-1.58-.75-.88-.58-1.38-.94-2.23-1.5-.99-.65-.35-1.01.22-1.59.15-.15 2.71-2.48 2.76-2.69a.2.2 0 00-.05-.18c-.06-.05-.14-.03-.21-.02-.09.02-1.49.95-4.22 2.79-.4.27-.76.41-1.08.4-.36-.01-1.04-.2-1.55-.37-.63-.2-1.12-.31-1.08-.66.02-.18.27-.36.74-.55 2.92-1.27 4.86-2.11 5.83-2.51 2.78-1.16 3.35-1.36 3.73-1.36.08 0 .27.02.39.12.1.08.13.19.14.27-.01.06.01.24 0 .38z"/>
                      </svg>
                    </div>
                    <div>
                      <p className="text-sm font-semibold text-white/90">Telegram</p>
                      <p className="text-[10px] uppercase tracking-wider text-white/20 font-medium mt-0.5">
                        {user?.telegram_id ? `Linked (ID: ${user.telegram_id})` : 'Not linked'}
                      </p>
                    </div>
                  </div>
                  {user?.telegram_id ? (
                    <div className="px-3 py-1 rounded-full bg-green-500/10 border border-green-500/20 text-green-500 text-[10px] font-bold uppercase tracking-widest">
                      Активен
                    </div>
                  ) : (
                    <Button variant="outline" size="sm" className="rounded-full h-8 text-[10px] font-bold uppercase tracking-widest border-white/10 hover:bg-white/10">
                      Связать
                    </Button>
                  )}
                </div>
              </div>

              <div className="p-4 rounded-2xl border border-white/5 bg-white/[0.03] transition-all hover:bg-white/[0.05] group">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <div className="w-12 h-12 rounded-xl bg-white/5 flex items-center justify-center transition-transform group-hover:scale-110 duration-300">
                      <svg className="w-6 h-6" viewBox="0 0 24 24">
                        <path fill="#EA4335" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                        <path fill="#FBBC05" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                        <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                        <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                      </svg>
                    </div>
                    <div>
                      <p className="text-sm font-semibold text-white/90">Google</p>
                      <p className="text-[10px] uppercase tracking-wider text-white/20 font-medium mt-0.5">
                        {user?.email || 'Not linked'}
                      </p>
                    </div>
                  </div>
                  {user?.email ? (
                    <div className="px-3 py-1 rounded-full bg-green-500/10 border border-green-500/20 text-green-500 text-[10px] font-bold uppercase tracking-widest">
                      Активен
                    </div>
                  ) : (
                    <Button variant="outline" size="sm" className="rounded-full h-8 text-[10px] font-bold uppercase tracking-widest border-white/10 hover:bg-white/10">
                      Связать
                    </Button>
                  )}
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}

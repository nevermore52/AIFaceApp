import { useEffect, useState, useRef, useCallback } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { authApi, API_BASE_URL } from '../lib/api'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Sparkles, Copy, Check } from 'lucide-react'

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { isAuthenticated, setAuth } = useAuthStore()
  const redirectTo = (location.state as { from?: string } | null)?.from || '/'

  const [tgStatus, setTgStatus] = useState<'idle' | 'waiting' | 'error' | 'expired'>('idle')
  const [cmdCopied, setCmdCopied] = useState(false)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const tokenRef = useRef<string | null>(null)

  useEffect(() => {
    if (isAuthenticated) {
      navigate(redirectTo)
    }
  }, [isAuthenticated, navigate, redirectTo])

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current)
    }
  }, [])

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [])

  const handleTelegramLogin = async () => {
    try {
      setTgStatus('waiting')
      stopPolling()

      // 1. Create auth token
      const { token } = await authApi.createWebToken()
      tokenRef.current = token

      // 2. Open bot link with auth token
      const botName = (import.meta.env.VITE_TELEGRAM_BOT_NAME || 'aifaceappbot').replace('@', '')
      window.open(`https://t.me/${botName}?start=auth-${token}`, '_blank', 'noopener,noreferrer')

      // 3. Poll for confirmation
      let elapsed = 0
      pollRef.current = setInterval(async () => {
        elapsed += 2
        if (elapsed > 300) { // 5 min timeout
          stopPolling()
          setTgStatus('expired')
          return
        }

        try {
          const result = await authApi.getWebTokenStatus(token)

          if (result.status === 'confirmed' && result.access_token && result.refresh_token) {
            stopPolling()
            // Fetch user profile with the new token, then set auth
            try {
              const resp = await fetch(`${API_BASE_URL}/me`, {
                headers: { Authorization: `Bearer ${result.access_token}` },
              })
              if (resp.ok) {
                const user = await resp.json()
                setAuth(user, result.access_token, result.refresh_token)
              } else {
                // Fallback: set minimal user object so auth works
                setAuth(
                  { id: 0, username: '', first_name: 'User', last_name: '', is_admin: false, subscription_type: '' },
                  result.access_token,
                  result.refresh_token,
                )
              }
            } catch {
              setAuth(
                { id: 0, username: '', first_name: 'User', last_name: '', is_admin: false, subscription_type: '' },
                result.access_token,
                result.refresh_token,
              )
            }
            navigate(redirectTo)
          } else if (result.status === 'expired') {
            stopPolling()
            setTgStatus('expired')
          }
        } catch {
          // Network error — keep polling
        }
      }, 2000)
    } catch (error) {
      console.error('Telegram web token creation failed:', error)
      setTgStatus('error')
    }
  }

  // Google OAuth временно отключен (новый закон РФ)
  // const handleGoogleLogin = () => {
  //   sessionStorage.setItem('post_login_redirect', redirectTo)
  //   window.location.href = `${API_BASE_URL}/auth/google`
  // }

  return (
    <div className="min-h-screen flex items-center justify-center bg-[#030303] relative overflow-hidden p-4">
      {/* Background Glows */}
      <div className="absolute top-0 left-0 w-full h-full overflow-hidden pointer-events-none">
        <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full" />
        <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-primary/5 blur-[120px] rounded-full" />
      </div>

      <Card className="w-full max-w-md border-white/5 bg-white/[0.02] backdrop-blur-xl relative z-10 overflow-hidden">
        <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-transparent via-primary to-transparent opacity-50" />
        <CardHeader className="text-center pt-10 pb-8">
          <div className="w-16 h-16 mx-auto mb-6 rounded-2xl bg-white/5 flex items-center justify-center shadow-[0_0_30px_rgba(255,255,255,0.05)] transition-transform hover:scale-105 duration-500">
            <Sparkles className="w-8 h-8 text-primary" />
          </div>
          <CardTitle className="text-4xl font-bold tracking-tighter text-white mb-2">
            AI Face App
          </CardTitle>
          <CardDescription className="text-white/40 text-sm font-medium">
            Раскройте свой творческий потенциал <br />с помощью искусственного интеллекта
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6 pb-10">
          {tgStatus === 'waiting' ? (
            <div className="space-y-4 py-4 animate-in fade-in zoom-in-95 duration-500">
              <div className="text-center space-y-4">
                <div className="relative mx-auto w-14 h-14">
                  <div className="absolute inset-0 rounded-full border-2 border-white/5" />
                  <div className="absolute inset-0 rounded-full border-2 border-primary border-t-transparent animate-spin" />
                </div>
                <div className="space-y-1">
                  <p className="text-sm font-semibold text-white/90">Ожидание подтверждения...</p>
                  <p className="text-[11px] text-white/30">Нажмите кнопку <span className="text-primary font-bold">Войти на сайт</span> в боте.</p>
                </div>
              </div>

              {tokenRef.current && (
                <div className="space-y-1.5">
                  <p className="text-[10px] text-white/30 text-center uppercase tracking-wider">Или введите команду вручную в боте</p>
                  <div className="flex gap-2 items-center p-2.5 rounded-xl border border-white/5 bg-white/[0.02]">
                    <code className="flex-1 text-[11px] text-primary/80 truncate font-mono">
                      /start auth-{tokenRef.current}
                    </code>
                    <button
                      onClick={async () => {
                        await navigator.clipboard.writeText(`/start auth-${tokenRef.current}`)
                        setCmdCopied(true)
                        setTimeout(() => setCmdCopied(false), 2000)
                      }}
                      className="flex-shrink-0 p-1.5 rounded-lg hover:bg-white/10 transition-colors"
                    >
                      {cmdCopied ? <Check className="h-3.5 w-3.5 text-green-400" /> : <Copy className="h-3.5 w-3.5 text-white/30" />}
                    </button>
                  </div>
                </div>
              )}

              <div className="text-center">
                <Button
                  variant="ghost"
                  size="sm"
                  className="rounded-full text-[10px] uppercase tracking-widest font-bold text-white/20 hover:text-white hover:bg-white/5"
                  onClick={() => { stopPolling(); setTgStatus('idle') }}
                >
                  Отменить вход
                </Button>
              </div>
            </div>
          ) : (
            <Button
              variant="secondary"
              className="w-full py-7 rounded-2xl font-bold bg-white/5 border border-white/5 hover:bg-white/10 text-white transition-all duration-300"
              onClick={handleTelegramLogin}
            >
              <svg className="mr-3 h-5 w-5" fill="currentColor" viewBox="0 0 24 24">
                <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm4.64 6.8c-.15 1.58-.8 5.42-1.13 7.19-.14.75-.42 1-.68 1.03-.58.05-1.02-.38-1.58-.75-.88-.58-1.38-.94-2.23-1.5-.99-.65-.35-1.01.22-1.59.15-.15 2.71-2.48 2.76-2.69a.2.2 0 00-.05-.18c-.06-.05-.14-.03-.21-.02-.09.02-1.49.95-4.22 2.79-.4.27-.76.41-1.08.4-.36-.01-1.04-.2-1.55-.37-.63-.2-1.12-.31-1.08-.66.02-.18.27-.36.74-.55 2.92-1.27 4.86-2.11 5.83-2.51 2.78-1.16 3.35-1.36 3.73-1.36.08 0 .27.02.39.12.1.08.13.19.14.27-.01.06.01.24 0 .38z"/>
              </svg>
              Войти через Telegram
            </Button>
          )}

          {tgStatus === 'expired' && (
            <p className="text-center text-xs font-medium text-destructive animate-in slide-in-from-top-1">
              Время ожидания истекло. Попробуйте ещё раз.
            </p>
          )}

          {tgStatus === 'error' && (
            <p className="text-center text-xs font-medium text-destructive animate-in slide-in-from-top-1">
              Ошибка создания ссылки. Попробуйте ещё раз.
            </p>
          )}
          
          {/* Google OAuth временно отключен (новый закон РФ) */}
          {/* 
          <div className="relative py-2">
            <div className="absolute inset-0 flex items-center">
              <span className="w-full border-t border-white/5" />
            </div>
            <div className="relative flex justify-center text-[10px] uppercase tracking-widest font-bold">
              <span className="bg-[#0a0a0a] px-4 text-white/20">или</span>
            </div>
          </div>

          <Button
            variant="outline"
            className="w-full py-7 rounded-2xl font-bold border-white/5 bg-white/[0.01] hover:bg-white/[0.03] text-white/80 transition-all duration-300"
            onClick={handleGoogleLogin}
          >
            <svg className="mr-3 h-5 w-5" viewBox="0 0 24 24">
              <path
                d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
                fill="#4285F4"
              />
              <path
                d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
                fill="#34A853"
              />
              <path
                d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
                fill="#FBBC05"
              />
              <path
                d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
                fill="#EA4335"
              />
            </svg>
            Продолжить с Google
          </Button>
          */}

          <p className="text-center text-[10px] text-white/20 px-6 leading-relaxed">
            Авторизуясь в сервисе, вы подтверждаете свое согласие с нашими 
            <a 
              href="https://telegra.ph/Polzovatelskoe-soglashenie-Usloviya-EHkspluatacii-i-Obsluzhivaniya-01-14" 
              target="_blank" 
              rel="noopener noreferrer"
              className="text-white/40 hover:text-primary cursor-pointer transition-colors px-1 underline"
            >
              Правилами пользования
            </a> 
            и <a 
              href="https://telegra.ph/Politika-Konfidencialnosti-01-14-87" 
              target="_blank" 
              rel="noopener noreferrer"
              className="text-white/40 hover:text-primary cursor-pointer transition-colors underline"
            >
              Политикой конфиденциальности
            </a>.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}

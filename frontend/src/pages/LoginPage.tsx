import { useEffect, useState, useRef, useCallback } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { authApi, API_BASE_URL } from '../lib/api'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { isAuthenticated, setAuth } = useAuthStore()
  const redirectTo = (location.state as { from?: string } | null)?.from || '/'

  const [tgStatus, setTgStatus] = useState<'idle' | 'waiting' | 'error' | 'expired'>('idle')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const tokenRef = useRef<string | null>(null)

  useEffect(() => {
    if (isAuthenticated) {
      navigate(redirectTo)
    }
  }, [isAuthenticated, navigate, redirectTo])

  useEffect(() => {
    const tg = window.Telegram?.WebApp
    if (tg?.initData) {
      tg.ready()
      tg.expand()
      handleMiniAppLogin(tg.initData)
    }
  }, [])

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current)
    }
  }, [])

  const handleMiniAppLogin = async (initData: string) => {
    try {
      const response = await authApi.miniAppLogin(initData) as {
        user: Parameters<typeof setAuth>[0]
        access_token: string
        refresh_token: string
      }
      setAuth(response.user, response.access_token, response.refresh_token)
      navigate(redirectTo)
    } catch (error) {
      console.error('Mini App login failed:', error)
    }
  }

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

  const handleGoogleLogin = () => {
    sessionStorage.setItem('post_login_redirect', redirectTo)
    window.location.href = `${API_BASE_URL}/auth/google`
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-blue-50 to-purple-50 dark:from-gray-900 dark:to-gray-800 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <CardTitle className="text-3xl font-bold bg-gradient-to-r from-blue-600 to-purple-600 bg-clip-text text-transparent">
            AI Face App
          </CardTitle>
          <CardDescription className="text-lg">
            Генерация изображений, видео и музыки с помощью ИИ
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {tgStatus === 'waiting' ? (
            <div className="text-center space-y-3 py-2">
              <div className="flex items-center justify-center gap-2">
                <div className="h-4 w-4 animate-spin rounded-full border-2 border-primary border-t-transparent" />
                <span className="text-sm font-medium">Ожидание подтверждения...</span>
              </div>
              <p className="text-xs text-muted-foreground">
                Нажмите /start в боте Telegram, затем вернитесь сюда.
                <br />Страница обновится автоматически.
              </p>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => { stopPolling(); setTgStatus('idle') }}
              >
                Отмена
              </Button>
            </div>
          ) : (
            <Button
              variant="outline"
              className="w-full"
              onClick={handleTelegramLogin}
            >
              Войти через Telegram
            </Button>
          )}

          {tgStatus === 'expired' && (
            <p className="text-center text-sm text-destructive">
              Время ожидания истекло. Попробуйте ещё раз.
            </p>
          )}

          {tgStatus === 'error' && (
            <p className="text-center text-sm text-destructive">
              Ошибка создания ссылки. Попробуйте ещё раз.
            </p>
          )}
          
          <div className="relative">
            <div className="absolute inset-0 flex items-center">
              <span className="w-full border-t" />
            </div>
            <div className="relative flex justify-center text-xs uppercase">
              <span className="bg-background px-2 text-muted-foreground">или</span>
            </div>
          </div>

          <Button
            variant="outline"
            className="w-full"
            onClick={handleGoogleLogin}
          >
            <svg className="mr-2 h-4 w-4" viewBox="0 0 24 24">
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
            Войти через Google
          </Button>

          <p className="text-center text-sm text-muted-foreground">
            Авторизуясь, вы соглашаетесь с условиями использования сервиса.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}

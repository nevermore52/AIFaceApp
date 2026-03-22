import { useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { userApi } from '../lib/api'

export function AuthCallbackPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { setAuth } = useAuthStore()

  useEffect(() => {
    const token = searchParams.get('token')
    const refresh = searchParams.get('refresh')
    const redirectTo = sessionStorage.getItem('post_login_redirect') || '/'

    if (token && refresh) {
      localStorage.setItem('auth-storage', JSON.stringify({
        state: {
          accessToken: token,
          refreshToken: refresh,
          isAuthenticated: true,
        }
      }))

      userApi.getMe()
        .then((user) => {
          setAuth(user as Parameters<typeof setAuth>[0], token, refresh)
          sessionStorage.removeItem('post_login_redirect')
          navigate(redirectTo)
        })
        .catch(() => {
          sessionStorage.removeItem('post_login_redirect')
          navigate('/login')
        })
    } else {
      sessionStorage.removeItem('post_login_redirect')
      navigate('/login')
    }
  }, [searchParams, setAuth, navigate])

  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="text-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mx-auto" />
        <p className="mt-4 text-muted-foreground">Авторизация...</p>
      </div>
    </div>
  )
}

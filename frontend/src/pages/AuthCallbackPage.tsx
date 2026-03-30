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
    <div className="min-h-screen flex items-center justify-center bg-[#030303]">
      <div className="text-center space-y-6">
        <div className="relative mx-auto w-16 h-16">
          <div className="absolute inset-0 rounded-full border-2 border-white/5" />
          <div className="absolute inset-0 rounded-full border-2 border-primary border-t-transparent animate-spin" />
        </div>
        <div className="space-y-2">
          <p className="text-lg font-semibold text-white/90">Авторизация...</p>
          <p className="text-sm text-white/30">Пожалуйста, подождите, мы настраиваем ваш профиль</p>
        </div>
      </div>
    </div>
  )
}

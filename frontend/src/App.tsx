import { useEffect, useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { useAuthStore } from './store/auth'
import { authApi, userApi } from './lib/api'
import { Layout } from './components/Layout'
import { LoginPage } from './pages/LoginPage'
import { DashboardPage } from './pages/DashboardPage'
import { GeneratePage } from './pages/GeneratePage'
import { HistoryPage } from './pages/HistoryPage'
import { IdeasPage } from './pages/IdeasPage'
import { GenerationDetailPage } from './pages/GenerationDetailPage'
import { PaymentsPage } from './pages/PaymentsPage'
import { ProfilePage } from './pages/ProfilePage'
import { AdminPage } from './pages/AdminPage'
import { AuthCallbackPage } from './pages/AuthCallbackPage'
import { Toaster } from './components/ui/toaster'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated)
  const location = useLocation()
  
  if (!isAuthenticated) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  
  return <>{children}</>
}

function TelegramWebAppAuth({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, setAuth, updateUser, logout } = useAuthStore()
  const [isChecking, setIsChecking] = useState(true)

  useEffect(() => {
    const checkTelegramAuth = async () => {
      const tg = window.Telegram?.WebApp
      const hasMiniAppData = !!(tg?.initData && tg.initData.length > 0)

      if (isAuthenticated) {
        try {
          // Проверяем что токен актуален и пользователь существует в БД
          const data = await userApi.getMe()
          updateUser(data as Parameters<typeof updateUser>[0])
          setIsChecking(false)
          return
        } catch {
          // Токен протух или пользователя нет в БД — сбрасываем сессию
          logout()
          // Если открыто в мини-апп — перелогиниваем ниже
          if (!hasMiniAppData) {
            setIsChecking(false)
            return
          }
        }
      }

      if (hasMiniAppData) {
        try {
          tg!.ready()
          tg!.expand()

          const response = await authApi.miniAppLogin(tg!.initData) as {
            user: Parameters<typeof setAuth>[0]
            access_token: string
            refresh_token: string
          }

          setAuth(response.user, response.access_token, response.refresh_token)
        } catch (error) {
          console.error('Mini App auto-login failed:', error)
        }
      }

      setIsChecking(false)
    }

    checkTelegramAuth()
  }, [])

  if (isChecking) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#030303]">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  return <>{children}</>
}

function TelegramDeepLinkRedirect({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate()
  const [handled, setHandled] = useState(false)

  useEffect(() => {
    const tg = window.Telegram?.WebApp
    if (tg?.initDataUnsafe) {
      const startParam = (tg.initDataUnsafe as any).start_param
      if (startParam && typeof startParam === 'string') {
        if (startParam.startsWith('g-')) {
          navigate(`/ideas?id=${startParam.substring(2)}`, { replace: true })
        } else if (startParam.startsWith('h-')) {
          navigate(`/ideas?id=${startParam.substring(2)}`, { replace: true })
        }
      }
    }
    setHandled(true)
  }, [])

  if (!handled) return null
  return <>{children}</>
}

function App() {
  return (
    <BrowserRouter>
      <TelegramWebAppAuth>
        <TelegramDeepLinkRedirect>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/auth/callback" element={<AuthCallbackPage />} />
          <Route path="/" element={<Layout />}>
            <Route index element={<DashboardPage />} />
            <Route path="ideas" element={<IdeasPage />} />
            <Route
              path="generate"
              element={
                <ProtectedRoute>
                  <GeneratePage />
                </ProtectedRoute>
              }
            />
            <Route
              path="history"
              element={
                <ProtectedRoute>
                  <HistoryPage />
                </ProtectedRoute>
              }
            />
            <Route
              path="generations/:id"
              element={
                <ProtectedRoute>
                  <GenerationDetailPage />
                </ProtectedRoute>
              }
            />
            <Route
              path="payments"
              element={
                <ProtectedRoute>
                  <PaymentsPage />
                </ProtectedRoute>
              }
            />
            <Route
              path="profile"
              element={
                <ProtectedRoute>
                  <ProfilePage />
                </ProtectedRoute>
              }
            />
            <Route
              path="admin"
              element={
                <ProtectedRoute>
                  <AdminPage />
                </ProtectedRoute>
              }
            />
          </Route>
        </Routes>
        <Toaster />
        </TelegramDeepLinkRedirect>
      </TelegramWebAppAuth>
    </BrowserRouter>
  )
}

export default App

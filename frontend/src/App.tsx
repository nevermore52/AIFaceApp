import { useEffect, useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { useAuthStore } from './store/auth'
import { authApi, userApi } from './lib/api'
import { Layout } from './components/Layout'
import { LoginPage } from './pages/LoginPage'
import { DashboardPage } from './pages/DashboardPage'
import { GeneratePage } from './pages/GeneratePage'
import { HistoryPage } from './pages/HistoryPage'
import { GenerationDetailPage } from './pages/GenerationDetailPage'
import { PaymentsPage } from './pages/PaymentsPage'
import { ProfilePage } from './pages/ProfilePage'
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
  const { isAuthenticated, setAuth, updateUser } = useAuthStore()
  const [isChecking, setIsChecking] = useState(true)

  useEffect(() => {
    const checkTelegramAuth = async () => {
      // If already authenticated — refresh user data to get actual subscription
      if (isAuthenticated) {
        userApi.getMe().then((data) => updateUser(data as Parameters<typeof updateUser>[0])).catch(() => {})
        setIsChecking(false)
        return
      }

      const tg = window.Telegram?.WebApp
      if (tg?.initData && tg.initData.length > 0) {
        try {
          tg.ready()
          tg.expand()
          
          const response = await authApi.miniAppLogin(tg.initData) as {
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
  }, [isAuthenticated, setAuth])

  if (isChecking) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#030303]">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  return <>{children}</>
}

function App() {
  return (
    <BrowserRouter>
      <TelegramWebAppAuth>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/auth/callback" element={<AuthCallbackPage />} />
          <Route path="/" element={<Layout />}>
            <Route index element={<DashboardPage />} />
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
          </Route>
        </Routes>
        <Toaster />
      </TelegramWebAppAuth>
    </BrowserRouter>
  )
}

export default App

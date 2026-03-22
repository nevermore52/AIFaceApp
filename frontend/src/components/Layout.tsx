import { Outlet, Link, useLocation } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { Home, Sparkles, History, CreditCard, User, LogOut } from 'lucide-react'
import { cn } from '../lib/utils'
import { Button } from './ui/button'

export function Layout() {
  const location = useLocation()
  const { user, isAuthenticated, logout } = useAuthStore()

  const navigation = [
    { name: 'Главная', href: '/', icon: Home },
    { name: 'Генерация', href: '/generate', icon: Sparkles },
    { name: 'История', href: '/history', icon: History },
    { name: 'Оплата', href: '/payments', icon: CreditCard },
    { name: 'Профиль', href: '/profile', icon: User },
  ]

  return (
    <div className="min-h-screen bg-background">
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="container flex h-14 items-center">
          <div className="mr-4 flex">
            <Link to="/" className="mr-6 flex items-center space-x-2">
              <span className="font-bold text-xl bg-gradient-to-r from-blue-600 to-purple-600 bg-clip-text text-transparent">
                AI Face App
              </span>
            </Link>
          </div>
          <nav className="flex items-center space-x-6 text-sm font-medium flex-1">
            {navigation.map((item) => (
              <Link
                key={item.href}
                to={item.href}
                className={cn(
                  'flex items-center gap-2 transition-colors hover:text-foreground/80',
                  location.pathname === item.href
                    ? 'text-foreground'
                    : 'text-foreground/60'
                )}
              >
                <item.icon className="h-4 w-4" />
                <span className="hidden sm:inline">{item.name}</span>
              </Link>
            ))}
          </nav>
          <div className="flex items-center gap-4">
            {isAuthenticated && user && (
              <div className="flex items-center gap-2">
                {user.avatar_url ? (
                  <img
                    src={user.avatar_url}
                    alt={user.first_name}
                    className="h-8 w-8 rounded-full"
                  />
                ) : (
                  <div className="h-8 w-8 rounded-full bg-primary flex items-center justify-center text-primary-foreground text-sm font-medium">
                    {user.first_name[0]}
                  </div>
                )}
                <span className="hidden md:inline text-sm font-medium">
                  {user.first_name}
                </span>
              </div>
            )}
            {isAuthenticated ? (
              <button
                onClick={logout}
                className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
              >
                <LogOut className="h-4 w-4" />
                <span className="hidden sm:inline">Выйти</span>
              </button>
            ) : (
              <Button asChild size="sm">
                <Link to="/login">Войти</Link>
              </Button>
            )}
          </div>
        </div>
      </header>
      <main className="container py-6">
        <Outlet />
      </main>
    </div>
  )
}

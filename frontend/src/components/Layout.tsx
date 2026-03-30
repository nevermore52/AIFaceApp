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
    <div className="min-h-screen bg-[#030303] text-foreground selection:bg-primary/30">
      <header className="sticky top-0 z-50 w-full border-b border-white/5 bg-[#030303]/80 backdrop-blur-xl">
        <div className="container flex h-16 items-center justify-between">
          <div className="flex items-center gap-8">
            <Link to="/" className="flex items-center space-x-2">
              <span className="font-bold text-2xl tracking-tight bg-gradient-to-br from-white via-white to-white/50 bg-clip-text text-transparent">
                AIFACEAPP
              </span>
            </Link>
            <nav className="hidden md:flex items-center space-x-1">
              {navigation.map((item) => (
                <Link
                  key={item.href}
                  to={item.href}
                  className={cn(
                    'flex items-center gap-2 px-4 py-2 rounded-full text-sm font-medium transition-all duration-200',
                    location.pathname === item.href
                      ? 'bg-white/10 text-white'
                      : 'text-white/60 hover:text-white hover:bg-white/5'
                  )}
                >
                  <item.icon className="h-4 w-4" />
                  <span>{item.name}</span>
                </Link>
              ))}
            </nav>
          </div>
          
          <div className="flex items-center gap-4">
            {isAuthenticated && user && (
              <div className="flex items-center gap-3 px-3 py-1.5 rounded-full bg-white/5 border border-white/10">
                {user.avatar_url ? (
                  <img
                    src={user.avatar_url}
                    alt={user.first_name}
                    className="h-6 w-6 rounded-full object-cover"
                  />
                ) : (
                  <div className="h-6 w-6 rounded-full bg-primary/20 flex items-center justify-center text-primary text-[10px] font-bold">
                    {user.first_name[0]}
                  </div>
                )}
                <span className="hidden md:inline text-xs font-medium text-white/90">
                  {user.first_name}
                </span>
              </div>
            )}
            {isAuthenticated ? (
              <button
                onClick={logout}
                className="p-2 text-white/40 hover:text-white hover:bg-white/5 rounded-full transition-all"
                title="Выйти"
              >
                <LogOut className="h-5 w-5" />
              </button>
            ) : (
              <Button asChild size="sm" className="rounded-full px-6">
                <Link to="/login">Войти</Link>
              </Button>
            )}
          </div>
        </div>
      </header>
      <main className="container py-8">
        <Outlet />
      </main>
    </div>
  )
}

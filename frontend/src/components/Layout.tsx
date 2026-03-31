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
    { name: 'Создать', href: '/generate', icon: Sparkles, isPrimary: true },
    { name: 'История', href: '/history', icon: History },
    { name: 'Профиль', href: '/profile', icon: User },
  ]

  return (
    <div className="min-h-screen bg-[#030303] text-foreground selection:bg-primary/30">
      <header className="sticky top-0 z-50 w-full border-b border-white/10 bg-[#030303] shadow-lg">
        <div className="container flex h-16 items-center justify-between px-4">
          <div className="flex items-center gap-8">
            <Link to="/" className="flex items-center space-x-2">
              <span className="font-bold text-xl md:text-2xl tracking-tight text-white">
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
              <Link
                to="/payments"
                className={cn(
                  'flex items-center gap-2 px-4 py-2 rounded-full text-sm font-medium transition-all duration-200',
                  location.pathname === '/payments'
                    ? 'bg-white/10 text-white'
                    : 'text-white/60 hover:text-white hover:bg-white/5'
                )}
              >
                <CreditCard className="h-4 w-4" />
                <span>Оплата</span>
              </Link>
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

      {/* Bottom Navigation for Mobile */}
      <nav className="fixed bottom-0 left-0 right-0 z-50 md:hidden bg-[#0A0A0A]/95 backdrop-blur-lg border-t border-white/10 px-6 py-3">
        <div className="flex items-center justify-between max-w-lg mx-auto">
          {navigation.map((item) => {
            const isActive = location.pathname === item.href
            if (item.isPrimary) {
              return (
                <Link
                  key={item.href}
                  to={item.href}
                  className="relative -top-3 flex flex-col items-center gap-1 group"
                >
                  <div className={cn(
                    "w-[84px] h-12 rounded-[24px] flex items-center justify-center transition-all duration-300 group-hover:scale-105 group-active:scale-95 shadow-[0_8px_30px_rgba(255,183,0,0.4)]",
                    "bg-gradient-to-br from-[#FFD700] via-[#FFB700] to-[#FF9000] border border-white/30"
                  )}>
                    <item.icon className="h-7 w-7 text-black stroke-[2.5]" />
                  </div>
                  <span className="text-[10px] font-black text-white uppercase tracking-[0.2em] mt-1 drop-shadow-md">
                    {item.name}
                  </span>
                </Link>
              )
            }
            return (
              <Link
                key={item.href}
                to={item.href}
                className="flex flex-col items-center gap-1.5 group"
              >
                <item.icon className={cn(
                  "h-6 w-6 transition-colors duration-200",
                  isActive ? "text-white" : "text-white/40 group-hover:text-white/60"
                )} />
                <span className={cn(
                  "text-[10px] font-medium transition-colors duration-200",
                  isActive ? "text-white" : "text-white/40"
                )}>
                  {item.name}
                </span>
              </Link>
            )
          })}
        </div>
      </nav>

      <main className="container py-8 px-4 pb-32 md:pb-8">
        <Outlet />
      </main>
    </div>
  )
}

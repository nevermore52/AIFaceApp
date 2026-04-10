export const API_BASE_URL = (import.meta.env.VITE_API_URL || '/api').replace(/\/$/, '')

interface ApiOptions {
  method?: string
  body?: unknown
  headers?: Record<string, string>
}

class ApiClient {
  private baseUrl: string

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl
  }

  private getToken(): string | null {
    const storage = localStorage.getItem('auth-storage')
    if (storage) {
      const parsed = JSON.parse(storage)
      return parsed.state?.accessToken
    }
    return null
  }

  async request<T>(endpoint: string, options: ApiOptions = {}): Promise<T> {
    const { method = 'GET', body, headers = {} } = options
    const token = this.getToken()

    const config: RequestInit = {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...headers,
      },
    }

    if (body) {
      config.body = JSON.stringify(body)
    }

    const response = await fetch(`${this.baseUrl}${endpoint}`, config)

    if (!response.ok) {
      if (response.status === 401) {
        // Сбрасываем auth — TelegramWebAppAuth при следующем рендере переlogинит
        try {
          const storage = localStorage.getItem('auth-storage')
          if (storage) {
            const parsed = JSON.parse(storage)
            parsed.state.isAuthenticated = false
            parsed.state.accessToken = null
            parsed.state.refreshToken = null
            localStorage.setItem('auth-storage', JSON.stringify(parsed))
          }
        } catch {}
        // Перезагружаем страницу чтобы TelegramWebAppAuth запустился заново
        if (window.Telegram?.WebApp?.initData) {
          window.location.reload()
          return Promise.reject(new Error('Session expired')) as any
        }
      }
      const error = await response.json().catch(() => ({ error: 'Unknown error' }))
      throw new Error(error.error || 'Request failed')
    }

    return response.json()
  }

  get<T>(endpoint: string) {
    return this.request<T>(endpoint)
  }

  post<T>(endpoint: string, body?: unknown) {
    return this.request<T>(endpoint, { method: 'POST', body })
  }

  put<T>(endpoint: string, body?: unknown) {
    return this.request<T>(endpoint, { method: 'PUT', body })
  }

  delete<T>(endpoint: string) {
    return this.request<T>(endpoint, { method: 'DELETE' })
  }
}

export const api = new ApiClient(API_BASE_URL)

export interface GalleryItem {
  id: number
  model: string
  output: string
  prompt: string
}

export const publicApi = {
  getGallery: (limit = 30, offset = 0) =>
    api.get<{ data: GalleryItem[]; total: number }>(`/gallery?limit=${limit}&offset=${offset}`),
}

export const authApi = {
  telegramLogin: (data: {
    id: number
    first_name: string
    last_name?: string
    username?: string
    photo_url?: string
    auth_date: number
    hash: string
  }) => api.post('/auth/telegram', data),

  miniAppLogin: (initData: string) =>
    api.post('/auth/telegram/miniapp', { init_data: initData }),

  createWebToken: () =>
    api.post<{ token: string }>('/auth/telegram/web-token'),

  createLinkToken: () =>
    api.post<{ token: string }>('/me/telegram/link-token'),

  getWebTokenStatus: (token: string) =>
    api.get<{ status: string; access_token?: string; refresh_token?: string }>(
      `/auth/telegram/web-token/${token}/status`
    ),

  refresh: (refreshToken: string) =>
    api.post('/auth/refresh', { refresh_token: refreshToken }),

  logout: () => api.post('/auth/logout'),
}

export const userApi = {
  getMe: () => api.get('/me'),
  updateProfile: (data: { username?: string; first_name?: string; last_name?: string }) =>
    api.put('/me', data),
  getQuota: () => api.get('/me/quota'),
  getHistory: (limit = 10, offset = 0) =>
    api.get(`/me/history?limit=${limit}&offset=${offset}`),
  claimChannelBonus: () =>
    api.post<{ subscribed: boolean; already_claimed: boolean }>('/me/channel-bonus/claim'),
}

export interface GenerationCreateParams {
  model: string
  prompt: string
  image_urls?: string[]
  aspect_ratio?: string
  // Nano Banana 2 параметры
  resolution?: string
  google_search?: string
  // Видео параметры (Wan, Kling)
  duration?: string
  sound?: string
  // Suno Music параметры
  instrumental?: boolean
  vocal_gender?: string
  // История чата для текстовых моделей
  messages?: { role: string; content: string }[]
}

export const generationApi = {
  getAll: (limit = 20, offset = 0) =>
    api.get(`/generations?limit=${limit}&offset=${offset}`),
  getById: (id: number) => api.get(`/generations/${id}`),
  getStatus: (id: number) => api.get(`/generations/${id}/status`),
  getModels: () => api.get<{ id: string; name: string; type: string; description: string; token_cost: number }[]>('/models'),
  create: (data: GenerationCreateParams) =>
    api.post<{ id: number; status: string }>('/generations', data),
  getUserUploads: () =>
    api.get<{ uploads: { url: string; filename: string }[] }>('/user-uploads'),
}

export const adminApi = {
  getStats: (period = 'all') =>
    api.get<any>(`/admin/stats?period=${period}`),
  getUsers: (limit = 20, offset = 0) =>
    api.get<any>(`/admin/users?limit=${limit}&offset=${offset}`),
  updateUser: (id: number, data: { is_admin?: boolean; is_blocked?: boolean }) =>
    api.put<any>(`/admin/users/${id}`, data),
  setSubscription: (id: number, plan: string, days: number) =>
    api.post<any>(`/admin/users/${id}/subscription`, { plan, days }),
  removeSubscription: (id: number) =>
    api.delete<any>(`/admin/users/${id}/subscription`),
  getTopUsers: () =>
    api.get<any>('/admin/top-users'),
  broadcast: (text: string) =>
    api.post<{ sent: number; failed: number; total: number }>('/admin/broadcast', { text }),
  getGenerations: (limit = 20, offset = 0) =>
    api.get<any>(`/admin/generations?limit=${limit}&offset=${offset}`),
  getPayments: (limit = 20, offset = 0) =>
    api.get<any>(`/admin/payments?limit=${limit}&offset=${offset}`),
  getGalleryIdeas: (limit = 20, offset = 0) =>
    api.get<any>(`/admin/gallery-ideas?limit=${limit}&offset=${offset}`),
  createGalleryIdea: (data: { model: string; output: string; prompt: string }) =>
    api.post<any>('/admin/gallery-ideas', data),
  updateGalleryIdea: (id: number, data: { model: string; output: string; prompt: string }) =>
    api.put<any>(`/admin/gallery-ideas/${id}`, data),
  deleteGalleryIdea: (id: number) =>
    api.delete<any>(`/admin/gallery-ideas/${id}`),
}

export const paymentApi = {
  getPackages: () => api.get('/payments/packages'),
  getSubscriptions: () => api.get('/payments/subscriptions'),
  getPhotoDiscount: () => api.get<{ percent: number; end_time: number }>('/payments/photo-discount'),
  getHistory: (limit = 20, offset = 0) =>
    api.get(`/payments/history?limit=${limit}&offset=${offset}`),
  create: (category: string, qty: number) =>
    api.post('/payments/create', { category, qty }),
  createSubscription: (subscriptionName: string) =>
    api.post('/payments/subscription', { subscription_name: subscriptionName }),
}

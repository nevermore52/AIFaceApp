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
}

export const generationApi = {
  getAll: (limit = 20, offset = 0) =>
    api.get(`/generations?limit=${limit}&offset=${offset}`),
  getById: (id: number) => api.get(`/generations/${id}`),
}

export const paymentApi = {
  getPackages: () => api.get('/payments/packages'),
  getSubscriptions: () => api.get('/payments/subscriptions'),
  getHistory: (limit = 20, offset = 0) =>
    api.get(`/payments/history?limit=${limit}&offset=${offset}`),
  create: (category: string, qty: number) =>
    api.post('/payments/create', { category, qty }),
}

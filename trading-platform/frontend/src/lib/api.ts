import { API_URL } from './constants'

class ApiClient {
  private baseUrl: string
  private token: string | null = null

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl
    if (typeof window !== 'undefined') {
      this.token = localStorage.getItem('auth_token')
    }
  }

  setToken(token: string) {
    this.token = token
    if (typeof window !== 'undefined') {
      localStorage.setItem('auth_token', token)
    }
  }

  clearToken() {
    this.token = null
    if (typeof window !== 'undefined') {
      localStorage.removeItem('auth_token')
    }
  }

  private async request(path: string, options: RequestInit = {}) {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...((options.headers as Record<string, string>) || {}),
    }

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }

    const res = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers,
    })

    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new Error(body.error || `API error: ${res.status}`)
    }

    return res.json()
  }

  get(path: string) {
    return this.request(path)
  }

  post(path: string, body: any) {
    return this.request(path, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  }

  put(path: string, body: any) {
    return this.request(path, {
      method: 'PUT',
      body: JSON.stringify(body),
    })
  }

  delete(path: string) {
    return this.request(path, { method: 'DELETE' })
  }
}

export const api = new ApiClient(API_URL)

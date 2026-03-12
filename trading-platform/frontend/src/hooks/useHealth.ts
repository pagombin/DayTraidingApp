'use client'

import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'

type ServiceHealth = {
  name: string
  status: string
  last_check: string
  response_time_ms?: number
}

export function useHealth() {
  const [services, setServices] = useState<ServiceHealth[]>([])
  const [loading, setLoading] = useState(true)

  const fetchHealth = useCallback(async () => {
    try {
      const data = await api.get('/api/health/services')
      setServices(data.services || [])
    } catch {
      // If API is unreachable, show gateway as unhealthy
      setServices([
        { name: 'gateway', status: 'unhealthy', last_check: new Date().toISOString() },
      ])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchHealth()
    const interval = setInterval(fetchHealth, 10000)
    return () => clearInterval(interval)
  }, [fetchHealth])

  return { services, loading, refresh: fetchHealth }
}

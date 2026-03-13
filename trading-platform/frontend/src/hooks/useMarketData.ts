'use client'

import { useState, useEffect, useCallback, useRef } from 'react'
import { api } from '@/lib/api'

export type Quote = {
  symbol: string
  bid: number
  ask: number
  last: number
  volume: number
  timestamp: string
  prevLast?: number
}

export type MarketStatus = {
  is_open: boolean
  is_pre_market: boolean
  is_after_hours: boolean
  status: string
  eastern_time: string
}

export type MarketDataStatus = {
  connected: boolean
  provider: string
  symbol_count: number
  market_status: string
  metrics?: {
    total_ticks: number
    ticks_per_second: number
    validation_rejected: number
  }
}

export function useMarketQuotes() {
  const [quotes, setQuotes] = useState<Map<string, Quote>>(new Map())
  const [loading, setLoading] = useState(true)
  const prevQuotes = useRef<Map<string, Quote>>(new Map())

  const fetchQuotes = useCallback(async () => {
    try {
      const data = await api.get('/api/market/quotes')
      const newQuotes = new Map<string, Quote>()
      for (const q of data.quotes || []) {
        const prev = prevQuotes.current.get(q.symbol)
        newQuotes.set(q.symbol, {
          symbol: q.symbol,
          bid: Number(q.bid) || 0,
          ask: Number(q.ask) || 0,
          last: Number(q.last) || 0,
          volume: Number(q.volume) || 0,
          timestamp: q.timestamp || '',
          prevLast: prev?.last,
        })
      }
      prevQuotes.current = newQuotes
      setQuotes(newQuotes)
    } catch {
      // API not available yet
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchQuotes()
    const interval = setInterval(fetchQuotes, 3000) // poll every 3s
    return () => clearInterval(interval)
  }, [fetchQuotes])

  return { quotes, loading, refresh: fetchQuotes }
}

export function useMarketCalendar() {
  const [calendar, setCalendar] = useState<MarketStatus | null>(null)

  const fetchCalendar = useCallback(async () => {
    try {
      const data = await api.get('/api/market/calendar')
      setCalendar(data)
    } catch {
      // fallback
    }
  }, [])

  useEffect(() => {
    fetchCalendar()
    const interval = setInterval(fetchCalendar, 30000) // every 30s
    return () => clearInterval(interval)
  }, [fetchCalendar])

  return calendar
}

export function useMarketDataStatus() {
  const [status, setStatus] = useState<MarketDataStatus | null>(null)

  const fetchStatus = useCallback(async () => {
    try {
      const data = await api.get('/api/market/status')
      setStatus(data)
    } catch {
      setStatus(null)
    }
  }, [])

  useEffect(() => {
    fetchStatus()
    const interval = setInterval(fetchStatus, 10000)
    return () => clearInterval(interval)
  }, [fetchStatus])

  return status
}

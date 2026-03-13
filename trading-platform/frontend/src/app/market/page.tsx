'use client'

import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'

type Quote = {
  symbol: string
  last: string
  bid: string
  ask: string
  volume: string
  change: string
  change_pct: string
  timestamp: string
}

type Bar = {
  time: string
  open: string
  high: string
  low: string
  close: string
  volume: number
}

export default function MarketPage() {
  const [quotes, setQuotes] = useState<Quote[]>([])
  const [selectedSymbol, setSelectedSymbol] = useState('SPY')
  const [bars, setBars] = useState<Bar[]>([])
  const [calendar, setCalendar] = useState<any>(null)
  const [loading, setLoading] = useState(true)

  const fetchData = useCallback(async () => {
    try {
      const [quotesRes, calendarRes] = await Promise.all([
        api.get('/api/market/quotes').catch(() => ({ quotes: [] })),
        api.get('/api/market/calendar').catch(() => null),
      ])
      setQuotes(quotesRes.quotes || [])
      if (calendarRes) setCalendar(calendarRes)
    } catch { /* ignore */ }
    finally { setLoading(false) }
  }, [])

  const fetchBars = useCallback(async (symbol: string) => {
    try {
      const res = await api.get(`/api/market/bars/${symbol}?count=60`)
      setBars(res.bars || [])
    } catch { setBars([]) }
  }, [])

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 3000)
    return () => clearInterval(interval)
  }, [fetchData])

  useEffect(() => {
    fetchBars(selectedSymbol)
    const interval = setInterval(() => fetchBars(selectedSymbol), 5000)
    return () => clearInterval(interval)
  }, [selectedSymbol, fetchBars])

  const isMarketOpen = calendar?.is_open ?? false

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-white"></div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Market Data</h1>
        <div className={`px-3 py-1 rounded-full text-sm font-medium ${
          isMarketOpen ? 'bg-green-900/50 text-green-300' : 'bg-red-900/50 text-red-300'
        }`}>
          {isMarketOpen ? 'Market Open' : 'Market Closed'}
        </div>
      </div>

      {/* Market Quotes */}
      <div className="bg-gray-800 rounded-lg overflow-hidden">
        <div className="px-4 py-3 border-b border-gray-700">
          <h2 className="text-lg font-semibold">Watchlist</h2>
        </div>
        {quotes.length === 0 ? (
          <div className="p-8 text-center text-gray-400">
            No market data available. Configure your data provider in Settings.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-400 text-left border-b border-gray-700">
                  <th className="px-4 py-2">Symbol</th>
                  <th className="px-4 py-2 text-right">Last</th>
                  <th className="px-4 py-2 text-right">Bid</th>
                  <th className="px-4 py-2 text-right">Ask</th>
                  <th className="px-4 py-2 text-right">Change</th>
                  <th className="px-4 py-2 text-right">Change %</th>
                  <th className="px-4 py-2 text-right">Volume</th>
                </tr>
              </thead>
              <tbody>
                {quotes.map(q => {
                  const change = parseFloat(q.change || '0')
                  const changeColor = change >= 0 ? 'text-green-400' : 'text-red-400'
                  return (
                    <tr
                      key={q.symbol}
                      className={`border-b border-gray-700/50 hover:bg-gray-700/30 cursor-pointer ${
                        selectedSymbol === q.symbol ? 'bg-gray-700/40' : ''
                      }`}
                      onClick={() => setSelectedSymbol(q.symbol)}
                    >
                      <td className="px-4 py-2 font-medium">{q.symbol}</td>
                      <td className="px-4 py-2 text-right">${q.last || '-'}</td>
                      <td className="px-4 py-2 text-right text-gray-400">${q.bid || '-'}</td>
                      <td className="px-4 py-2 text-right text-gray-400">${q.ask || '-'}</td>
                      <td className={`px-4 py-2 text-right ${changeColor}`}>
                        {change >= 0 ? '+' : ''}{q.change || '0.00'}
                      </td>
                      <td className={`px-4 py-2 text-right ${changeColor}`}>
                        {change >= 0 ? '+' : ''}{q.change_pct || '0.00'}%
                      </td>
                      <td className="px-4 py-2 text-right text-gray-400">{q.volume || '-'}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Price Bars */}
      <div className="bg-gray-800 rounded-lg p-4">
        <h2 className="text-lg font-semibold mb-3">
          {selectedSymbol} - 1 Min Bars (Last 60)
        </h2>
        {bars.length === 0 ? (
          <div className="text-gray-400 text-sm">No bar data available for {selectedSymbol}.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-400 text-left border-b border-gray-700">
                  <th className="px-3 py-1">Time</th>
                  <th className="px-3 py-1 text-right">Open</th>
                  <th className="px-3 py-1 text-right">High</th>
                  <th className="px-3 py-1 text-right">Low</th>
                  <th className="px-3 py-1 text-right">Close</th>
                  <th className="px-3 py-1 text-right">Volume</th>
                </tr>
              </thead>
              <tbody>
                {bars.slice(0, 20).map((bar, i) => (
                  <tr key={i} className="border-b border-gray-700/30 hover:bg-gray-700/20">
                    <td className="px-3 py-1 text-gray-400 text-xs">
                      {new Date(bar.time).toLocaleTimeString()}
                    </td>
                    <td className="px-3 py-1 text-right">{bar.open}</td>
                    <td className="px-3 py-1 text-right text-green-400">{bar.high}</td>
                    <td className="px-3 py-1 text-right text-red-400">{bar.low}</td>
                    <td className="px-3 py-1 text-right font-medium">{bar.close}</td>
                    <td className="px-3 py-1 text-right text-gray-400">{bar.volume}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Market Calendar */}
      {calendar && (
        <div className="bg-gray-800 rounded-lg p-4">
          <h2 className="text-lg font-semibold mb-3">Market Hours</h2>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <div>
              <div className="text-gray-400">Pre-Market</div>
              <div>4:00 AM - 9:30 AM ET</div>
            </div>
            <div>
              <div className="text-gray-400">Regular Session</div>
              <div>9:30 AM - 4:00 PM ET</div>
            </div>
            <div>
              <div className="text-gray-400">After Hours</div>
              <div>4:00 PM - 8:00 PM ET</div>
            </div>
            <div>
              <div className="text-gray-400">Current Time (ET)</div>
              <div>{calendar.current_time || new Date().toLocaleTimeString('en-US', { timeZone: 'America/New_York' })}</div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

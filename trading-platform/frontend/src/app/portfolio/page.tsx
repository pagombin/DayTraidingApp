'use client'

import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'
import { useWebSocket } from '@/hooks/useWebSocket'

type Position = {
  position_id: string
  symbol: string
  quantity: number
  avg_cost: string
  current_price: string
  market_value: string
  unrealized_pnl: string
  weight_pct?: string
  side: string
  strategy_id: string
  opened_at: string
}

type PortfolioData = {
  total_equity: string
  cash: string
  buying_power: string
  unrealized_pnl: string
  realized_pnl_today: string
  daily_pnl: string
  position_count: number
  net_delta: string
  long_exposure: string
  short_exposure: string
}

export default function PortfolioPage() {
  const [portfolio, setPortfolio] = useState<PortfolioData | null>(null)
  const [positions, setPositions] = useState<Position[]>([])
  const [history, setHistory] = useState<any[]>([])
  const [pnlBreakdown, setPnlBreakdown] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  const fetchData = useCallback(async () => {
    try {
      const [portfolioRes, positionsRes, historyRes, pnlRes] = await Promise.all([
        api.get('/api/portfolio').catch(() => null),
        api.get('/api/positions').catch(() => ({ positions: [] })),
        api.get('/api/portfolio/history?days=30').catch(() => ({ history: [] })),
        api.get('/api/portfolio/pnl').catch(() => ({ breakdown: [] })),
      ])
      if (portfolioRes) setPortfolio(portfolioRes)
      setPositions(positionsRes.positions || [])
      setHistory(historyRes.history || [])
      setPnlBreakdown(pnlRes.breakdown || [])
    } catch { /* ignore */ }
    finally { setLoading(false) }
  }, [])

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 5000)
    return () => clearInterval(interval)
  }, [fetchData])

  useWebSocket(useCallback((msg) => {
    if (msg.channel === 'portfolio:snapshot') {
      setPortfolio(msg.data)
    }
  }, []))

  const dailyPnl = parseFloat(portfolio?.daily_pnl || '0')
  const pnlColor = dailyPnl >= 0 ? 'text-green-400' : 'text-red-400'

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-white"></div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Portfolio</h1>

      {/* Summary Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <SummaryCard label="Total Equity" value={`$${portfolio?.total_equity || '100,000.00'}`} />
        <SummaryCard label="Cash" value={`$${portfolio?.cash || '100,000.00'}`} />
        <SummaryCard label="Buying Power" value={`$${portfolio?.buying_power || '200,000.00'}`} />
        <SummaryCard
          label="Today's PnL"
          value={`${dailyPnl >= 0 ? '+' : ''}$${portfolio?.daily_pnl || '0.00'}`}
          valueClass={pnlColor}
        />
      </div>

      {/* PnL Breakdown */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-gray-800 rounded-lg p-4">
          <div className="text-sm text-gray-400 mb-1">Unrealized PnL</div>
          <div className={`text-xl font-bold ${parseFloat(portfolio?.unrealized_pnl || '0') >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            ${portfolio?.unrealized_pnl || '0.00'}
          </div>
        </div>
        <div className="bg-gray-800 rounded-lg p-4">
          <div className="text-sm text-gray-400 mb-1">Realized Today</div>
          <div className={`text-xl font-bold ${parseFloat(portfolio?.realized_pnl_today || '0') >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            ${portfolio?.realized_pnl_today || '0.00'}
          </div>
        </div>
        <div className="bg-gray-800 rounded-lg p-4">
          <div className="text-sm text-gray-400 mb-1">Net Delta</div>
          <div className="text-xl font-bold">{portfolio?.net_delta || '0'}</div>
        </div>
      </div>

      {/* Exposure */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="bg-gray-800 rounded-lg p-4">
          <div className="text-sm text-gray-400 mb-2">Exposure</div>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="text-green-400">Long</span>
              <span>${portfolio?.long_exposure || '0.00'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-red-400">Short</span>
              <span>${portfolio?.short_exposure || '0.00'}</span>
            </div>
          </div>
        </div>
        <div className="bg-gray-800 rounded-lg p-4">
          <div className="text-sm text-gray-400 mb-2">PnL by Strategy</div>
          {pnlBreakdown.length === 0 ? (
            <div className="text-gray-500 text-sm">No trades today</div>
          ) : (
            <div className="space-y-1">
              {pnlBreakdown.map((item, i) => (
                <div key={i} className="flex justify-between text-sm">
                  <span>{item.strategy_id}: {item.symbol}</span>
                  <span className={parseFloat(item.total_pnl) >= 0 ? 'text-green-400' : 'text-red-400'}>
                    ${item.total_pnl}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Positions Table */}
      <div className="bg-gray-800 rounded-lg overflow-hidden">
        <div className="px-4 py-3 border-b border-gray-700">
          <h2 className="text-lg font-semibold">Positions ({positions.length})</h2>
        </div>
        {positions.length === 0 ? (
          <div className="p-8 text-center text-gray-400">
            No open positions. Submit an order to get started.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-400 text-left border-b border-gray-700">
                  <th className="px-4 py-2">Symbol</th>
                  <th className="px-4 py-2">Side</th>
                  <th className="px-4 py-2 text-right">Qty</th>
                  <th className="px-4 py-2 text-right">Avg Cost</th>
                  <th className="px-4 py-2 text-right">Current</th>
                  <th className="px-4 py-2 text-right">Market Value</th>
                  <th className="px-4 py-2 text-right">PnL</th>
                  <th className="px-4 py-2">Strategy</th>
                </tr>
              </thead>
              <tbody>
                {positions.map(pos => {
                  const pnl = parseFloat(pos.unrealized_pnl)
                  return (
                    <tr key={pos.position_id} className="border-b border-gray-700/50 hover:bg-gray-700/30">
                      <td className="px-4 py-2 font-medium">{pos.symbol}</td>
                      <td className="px-4 py-2">
                        <span className={`px-2 py-0.5 rounded text-xs ${
                          pos.side === 'long' ? 'bg-green-900/50 text-green-300' : 'bg-red-900/50 text-red-300'
                        }`}>
                          {pos.side.toUpperCase()}
                        </span>
                      </td>
                      <td className="px-4 py-2 text-right">{Math.abs(pos.quantity)}</td>
                      <td className="px-4 py-2 text-right">${pos.avg_cost}</td>
                      <td className="px-4 py-2 text-right">${pos.current_price}</td>
                      <td className="px-4 py-2 text-right">${pos.market_value}</td>
                      <td className={`px-4 py-2 text-right font-medium ${pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {pnl >= 0 ? '+' : ''}${pos.unrealized_pnl}
                      </td>
                      <td className="px-4 py-2 text-gray-400">{pos.strategy_id}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Equity Curve */}
      {history.length > 0 && (
        <div className="bg-gray-800 rounded-lg p-4">
          <h2 className="text-lg font-semibold mb-3">Equity History</h2>
          <div className="space-y-1">
            {history.slice(0, 10).map((h, i) => (
              <div key={i} className="flex justify-between text-sm">
                <span className="text-gray-400">{h.date}</span>
                <span>${h.total_equity}</span>
                <span className={parseFloat(h.daily_pnl) >= 0 ? 'text-green-400' : 'text-red-400'}>
                  {parseFloat(h.daily_pnl) >= 0 ? '+' : ''}${h.daily_pnl}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function SummaryCard({ label, value, valueClass = '' }: { label: string; value: string; valueClass?: string }) {
  return (
    <div className="bg-gray-800 rounded-lg p-4">
      <div className="text-sm text-gray-400 mb-1">{label}</div>
      <div className={`text-xl font-bold ${valueClass}`}>{value}</div>
    </div>
  )
}

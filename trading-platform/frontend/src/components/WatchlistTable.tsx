'use client'

import { useMarketQuotes } from '@/hooks/useMarketData'

export default function WatchlistTable() {
  const { quotes, loading } = useMarketQuotes()

  if (loading) {
    return (
      <div className="bg-gray-800 rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">Watchlist</h2>
        <p className="text-gray-400">Loading market data...</p>
      </div>
    )
  }

  const sortedQuotes = Array.from(quotes.values()).sort((a, b) =>
    a.symbol.localeCompare(b.symbol)
  )

  if (sortedQuotes.length === 0) {
    return (
      <div className="bg-gray-800 rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">Watchlist</h2>
        <p className="text-gray-400 text-center py-8">
          No market data yet. Prices will appear when the market data service connects.
        </p>
      </div>
    )
  }

  return (
    <div className="bg-gray-800 rounded-lg p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Watchlist</h2>
        <span className="text-xs text-gray-500">Auto-refreshes every 3s</span>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-gray-400 border-b border-gray-700">
              <th className="text-left py-2 px-3">Symbol</th>
              <th className="text-right py-2 px-3">Last</th>
              <th className="text-right py-2 px-3">Change</th>
              <th className="text-right py-2 px-3">Bid</th>
              <th className="text-right py-2 px-3">Ask</th>
              <th className="text-right py-2 px-3">Spread</th>
              <th className="text-right py-2 px-3">Volume</th>
            </tr>
          </thead>
          <tbody>
            {sortedQuotes.map((q) => {
              const change = q.prevLast ? q.last - q.prevLast : 0
              const changePct = q.prevLast && q.prevLast > 0
                ? ((q.last - q.prevLast) / q.prevLast) * 100
                : 0
              const spread = q.ask - q.bid
              const isUp = change > 0
              const isDown = change < 0

              return (
                <tr
                  key={q.symbol}
                  className={`border-b border-gray-700/50 transition-colors duration-500 ${
                    isUp ? 'bg-green-900/10' : isDown ? 'bg-red-900/10' : ''
                  }`}
                >
                  <td className="py-2.5 px-3 font-medium">{q.symbol}</td>
                  <td className={`text-right py-2.5 px-3 font-mono ${
                    isUp ? 'text-green-400' : isDown ? 'text-red-400' : 'text-white'
                  }`}>
                    ${q.last.toFixed(2)}
                  </td>
                  <td className={`text-right py-2.5 px-3 font-mono text-xs ${
                    isUp ? 'text-green-400' : isDown ? 'text-red-400' : 'text-gray-400'
                  }`}>
                    {change >= 0 ? '+' : ''}{change.toFixed(2)} ({changePct >= 0 ? '+' : ''}{changePct.toFixed(2)}%)
                  </td>
                  <td className="text-right py-2.5 px-3 font-mono text-gray-300">
                    ${q.bid.toFixed(2)}
                  </td>
                  <td className="text-right py-2.5 px-3 font-mono text-gray-300">
                    ${q.ask.toFixed(2)}
                  </td>
                  <td className="text-right py-2.5 px-3 font-mono text-gray-500 text-xs">
                    ${spread.toFixed(3)}
                  </td>
                  <td className="text-right py-2.5 px-3 font-mono text-gray-400">
                    {q.volume.toLocaleString()}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}

'use client'

import { useMarketDataStatus } from '@/hooks/useMarketData'

export default function MarketDataCard() {
  const status = useMarketDataStatus()

  if (!status) {
    return (
      <div className="bg-gray-800 rounded-lg p-5 border border-gray-700">
        <div className="flex items-center justify-between mb-3">
          <h3 className="font-semibold">Market Data</h3>
          <div className="w-3 h-3 rounded-full bg-gray-500" />
        </div>
        <p className="text-sm text-gray-400">Service not available</p>
      </div>
    )
  }

  const isConnected = status.connected
  const dotColor = isConnected ? 'bg-green-400' : 'bg-red-400'
  const statusText = isConnected ? 'Connected' : 'Disconnected'

  return (
    <div className={`bg-gray-800 rounded-lg p-5 border ${
      isConnected ? 'border-green-900/50' : 'border-red-900/50'
    }`}>
      <div className="flex items-center justify-between mb-3">
        <h3 className="font-semibold">Market Data</h3>
        <div className={`w-3 h-3 rounded-full ${dotColor}`} />
      </div>

      <div className="space-y-2 text-sm">
        <div className="flex justify-between">
          <span className="text-gray-400">Status</span>
          <span className={isConnected ? 'text-green-400' : 'text-red-400'}>{statusText}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-gray-400">Provider</span>
          <span>{status.provider || 'N/A'}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-gray-400">Symbols</span>
          <span>{status.symbol_count || 0}</span>
        </div>
        {status.metrics && (
          <>
            <div className="flex justify-between">
              <span className="text-gray-400">Ticks/sec</span>
              <span className="font-mono">{status.metrics.ticks_per_second}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400">Total Ticks</span>
              <span className="font-mono">{status.metrics.total_ticks.toLocaleString()}</span>
            </div>
          </>
        )}
        <div className="flex justify-between">
          <span className="text-gray-400">Market</span>
          <span className="text-xs">{status.market_status}</span>
        </div>
      </div>
    </div>
  )
}

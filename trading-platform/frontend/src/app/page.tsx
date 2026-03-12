'use client'

import { useState, useEffect } from 'react'
import HealthCard from '@/components/HealthCard'
import KillSwitch from '@/components/KillSwitch'
import OnboardingTour from '@/components/OnboardingTour'
import { useHealth } from '@/hooks/useHealth'

export default function HomePage() {
  const { services, loading } = useHealth()
  const [showTour, setShowTour] = useState(false)

  useEffect(() => {
    const tourCompleted = localStorage.getItem('tour_completed')
    if (!tourCompleted) {
      setShowTour(true)
    }
  }, [])

  const allHealthy = services.every(s => s.status === 'healthy')

  return (
    <div className="space-y-6">
      {showTour && (
        <OnboardingTour onDismiss={() => {
          setShowTour(false)
          localStorage.setItem('tour_completed', 'true')
        }} />
      )}

      {/* Paper Trading Banner */}
      <div className="bg-green-900/30 border border-green-700 rounded-lg p-4 flex items-center gap-3">
        <div className="w-3 h-3 rounded-full bg-green-400 animate-pulse" />
        <span className="text-green-300 font-medium">
          System is running in Paper Trading mode — no real money will be used
        </span>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <SummaryCard title="Total Equity" value="$100,000.00" subtitle="Paper account" />
        <SummaryCard title="Today's PnL" value="$0.00" subtitle="No trades today" color="neutral" />
        <SummaryCard title="Active Strategies" value="0" subtitle="Configure in Settings" />
        <SummaryCard title="Open Positions" value="0" subtitle="No open positions" />
      </div>

      {/* System Health Summary */}
      <div className="bg-gray-800 rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">System Health</h2>
        {loading ? (
          <p className="text-gray-400">Checking services...</p>
        ) : (
          <>
            <div className="flex items-center gap-2 mb-4">
              <div className={`w-3 h-3 rounded-full ${allHealthy ? 'bg-green-400' : 'bg-yellow-400'}`} />
              <span className={allHealthy ? 'text-green-300' : 'text-yellow-300'}>
                {allHealthy ? 'All systems operational' : 'Some services need attention'}
              </span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3">
              {services.map(svc => (
                <HealthCard key={svc.name} service={svc} compact />
              ))}
            </div>
          </>
        )}
      </div>

      {/* Equity Curve Placeholder */}
      <div className="bg-gray-800 rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">Equity Curve</h2>
        <div className="h-64 flex items-center justify-center text-gray-500 border border-gray-700 rounded-lg border-dashed">
          Chart will appear here once trading begins — Coming in Module 5
        </div>
      </div>
    </div>
  )
}

function SummaryCard({ title, value, subtitle, color = 'default' }: {
  title: string
  value: string
  subtitle: string
  color?: string
}) {
  return (
    <div className="bg-gray-800 rounded-lg p-5">
      <p className="text-sm text-gray-400">{title}</p>
      <p className="text-2xl font-bold mt-1">{value}</p>
      <p className="text-xs text-gray-500 mt-1">{subtitle}</p>
    </div>
  )
}

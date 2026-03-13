'use client'

import { useState, useEffect } from 'react'
import KillSwitch from './KillSwitch'
import { useHealth } from '@/hooks/useHealth'
import { useMarketCalendar } from '@/hooks/useMarketData'

export default function Header({
  darkMode,
  onToggleDarkMode,
}: {
  darkMode: boolean
  onToggleDarkMode: () => void
}) {
  const [currentTime, setCurrentTime] = useState<Date | null>(null)
  const { services } = useHealth()
  const calendarStatus = useMarketCalendar()

  useEffect(() => {
    setCurrentTime(new Date())
    const timer = setInterval(() => setCurrentTime(new Date()), 1000)
    return () => clearInterval(timer)
  }, [])

  const allHealthy = services.length === 0 || services.every(s => s.status === 'healthy')
  const someUnhealthy = services.some(s => s.status === 'unhealthy')

  const marketStatus = calendarStatus?.status || (currentTime ? getMarketStatus(currentTime) : 'Loading')
  const isOpen = calendarStatus?.is_open || false
  const isPreMarket = calendarStatus?.is_pre_market || false

  return (
    <header className="fixed top-0 left-0 right-0 z-50 h-14 bg-gray-800 border-b border-gray-700 flex items-center justify-between px-4">
      <div className="flex items-center gap-4">
        <h1 className="text-lg font-bold bg-gradient-to-r from-brand-500 to-purple-500 bg-clip-text text-transparent">
          Trading Platform
        </h1>

        <span className="text-sm text-gray-400">
          {currentTime ? currentTime.toLocaleTimeString('en-US', { hour12: true }) : '--:--:--'}
        </span>

        <span className={`text-xs px-2 py-0.5 rounded-full ${
          isOpen ? 'bg-green-900/50 text-green-300' :
          isPreMarket ? 'bg-yellow-900/50 text-yellow-300' :
          'bg-gray-700 text-gray-400'
        }`}>
          {marketStatus}
        </span>
      </div>

      <div className="flex items-center gap-4">
        <span className="text-sm text-gray-400">PnL: $0.00</span>

        {/* Health indicator */}
        <div className="flex items-center gap-1.5" title={allHealthy ? 'All systems healthy' : 'Issues detected'}>
          <div className={`w-2.5 h-2.5 rounded-full ${
            someUnhealthy ? 'bg-red-400' : allHealthy ? 'bg-green-400' : 'bg-yellow-400'
          }`} />
          <span className="text-xs text-gray-400">
            {someUnhealthy ? 'Issues' : allHealthy ? 'Healthy' : 'Degraded'}
          </span>
        </div>

        {/* Dark mode toggle */}
        <button
          onClick={onToggleDarkMode}
          className="p-1.5 rounded-md hover:bg-gray-700 text-gray-400 transition-colors"
          title={darkMode ? 'Switch to light mode' : 'Switch to dark mode'}
        >
          {darkMode ? '\u2600\uFE0F' : '\uD83C\uDF19'}
        </button>

        <KillSwitch />
      </div>
    </header>
  )
}

function getMarketStatus(now: Date): string {
  const est = new Date(now.toLocaleString('en-US', { timeZone: 'America/New_York' }))
  const day = est.getDay()
  const hours = est.getHours()
  const minutes = est.getMinutes()
  const time = hours * 60 + minutes

  if (day === 0 || day === 6) return 'Closed'
  if (time >= 570 && time < 630) return 'Pre-Market' // 9:30 AM
  if (time >= 630 && time < 960) return 'Open'       // 9:30 AM - 4:00 PM
  if (time >= 960 && time < 1080) return 'After-Hours'
  return 'Closed'
}

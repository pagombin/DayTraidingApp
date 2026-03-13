'use client'

import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'
import { useWebSocket } from '@/hooks/useWebSocket'
import Toast from '@/components/Toast'

type RiskGauge = {
  name: string
  current: number
  limit: number
  unit: string
  status: 'ok' | 'warning' | 'critical'
}

type CircuitBreakerInfo = {
  state: string
  daily_pnl: string
  thresholds: {
    warning: number
    throttle: number
    halt: number
    emergency: number
  }
}

type RiskCheckResult = {
  check_name: string
  passed: boolean
  message: string
  timestamp: string
}

export default function RiskPage() {
  const [gauges, setGauges] = useState<RiskGauge[]>([])
  const [circuitBreaker, setCircuitBreaker] = useState<CircuitBreakerInfo | null>(null)
  const [killSwitchActive, setKillSwitchActive] = useState(false)
  const [autonomyLevel, setAutonomyLevel] = useState(1)
  const [recentChecks, setRecentChecks] = useState<RiskCheckResult[]>([])
  const [loading, setLoading] = useState(true)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const [confirmKill, setConfirmKill] = useState(false)
  const [confirmResume, setConfirmResume] = useState(false)

  const fetchData = useCallback(async () => {
    try {
      const [riskRes, ksRes, autonomyRes] = await Promise.all([
        api.get('/api/risk/status').catch(() => ({
          gauges: [],
          circuit_breaker: null,
          recent_checks: [],
        })),
        api.get('/api/killswitch/status').catch(() => ({ active: false })),
        api.get('/api/autonomy/level').catch(() => ({ level: 1 })),
      ])
      setGauges(riskRes.gauges || [])
      setCircuitBreaker(riskRes.circuit_breaker || null)
      setRecentChecks(riskRes.recent_checks || [])
      setKillSwitchActive(ksRes.active)
      setAutonomyLevel(autonomyRes.level)
    } catch { /* ignore */ }
    finally { setLoading(false) }
  }, [])

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 5000)
    return () => clearInterval(interval)
  }, [fetchData])

  useWebSocket(useCallback((msg) => {
    if (msg.channel === 'oms:risk_alerts') {
      fetchData()
    }
    if (msg.channel === 'oms:killswitch') {
      setKillSwitchActive(msg.data?.active ?? false)
    }
  }, [fetchData]))

  async function handleActivateKillSwitch() {
    if (!confirmKill) {
      setConfirmKill(true)
      setTimeout(() => setConfirmKill(false), 10000)
      return
    }
    try {
      await api.post('/api/killswitch/activate', { reason: 'Manual activation from dashboard' })
      setToast({ message: 'Kill switch activated — all trading halted', type: 'success' })
      setKillSwitchActive(true)
      setConfirmKill(false)
    } catch (err: any) {
      setToast({ message: err.message || 'Failed to activate kill switch', type: 'error' })
    }
  }

  async function handleDeactivateKillSwitch() {
    if (!confirmResume) {
      setConfirmResume(true)
      setTimeout(() => setConfirmResume(false), 10000)
      return
    }
    try {
      await api.post('/api/killswitch/deactivate', { confirmation: 'CONFIRM' })
      setToast({ message: 'Kill switch deactivated — trading resumed', type: 'success' })
      setKillSwitchActive(false)
      setConfirmResume(false)
    } catch (err: any) {
      setToast({ message: err.message || 'Failed to deactivate kill switch', type: 'error' })
    }
  }

  async function handleSetAutonomy(level: number) {
    try {
      await api.put('/api/autonomy/level', { level })
      setAutonomyLevel(level)
      setToast({ message: `Autonomy level set to ${level}`, type: 'success' })
    } catch (err: any) {
      setToast({ message: err.message || 'Failed to set autonomy level', type: 'error' })
    }
  }

  const cbState = circuitBreaker?.state || 'normal'
  const cbColor = {
    normal: 'bg-green-900/50 text-green-300 border-green-700',
    warning: 'bg-yellow-900/50 text-yellow-300 border-yellow-700',
    throttle: 'bg-orange-900/50 text-orange-300 border-orange-700',
    halt: 'bg-red-900/50 text-red-300 border-red-700',
    emergency: 'bg-red-900/80 text-red-200 border-red-500',
  }[cbState] || 'bg-gray-700 text-gray-300 border-gray-600'

  const autonomyLabels = [
    { level: 0, label: 'Observer', desc: 'AI cannot trade' },
    { level: 1, label: 'Supervised', desc: 'Every order needs approval' },
    { level: 2, label: 'Guided', desc: 'Small orders auto-execute' },
    { level: 3, label: 'Autonomous', desc: 'All orders auto-execute' },
    { level: 4, label: 'Adaptive', desc: 'AI adjusts risk params' },
  ]

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-white"></div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Risk & Controls</h1>

      {/* Kill Switch Banner */}
      {killSwitchActive && (
        <div className="bg-red-900/50 border-2 border-red-500 rounded-lg p-4 text-center animate-pulse">
          <div className="text-2xl font-bold text-red-300">TRADING HALTED</div>
          <div className="text-sm text-red-400 mt-1">Kill switch is active. All trading is suspended.</div>
        </div>
      )}

      {/* Kill Switch & Circuit Breaker Row */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Kill Switch */}
        <div className="bg-gray-800 rounded-lg p-6">
          <h2 className="text-lg font-semibold mb-4">Kill Switch</h2>
          {killSwitchActive ? (
            <div className="space-y-3">
              <div className="text-red-400 text-sm">All orders cancelled. All positions being flattened.</div>
              <button
                onClick={handleDeactivateKillSwitch}
                className={`w-full py-3 rounded-lg font-bold text-lg transition-all ${
                  confirmResume
                    ? 'bg-green-600 hover:bg-green-700 ring-2 ring-green-400'
                    : 'bg-green-900/50 hover:bg-green-800 border border-green-700 text-green-300'
                }`}
              >
                {confirmResume ? 'CONFIRM — Resume Trading' : 'Resume Trading'}
              </button>
            </div>
          ) : (
            <button
              onClick={handleActivateKillSwitch}
              className={`w-full py-3 rounded-lg font-bold text-lg transition-all ${
                confirmKill
                  ? 'bg-red-600 hover:bg-red-700 ring-2 ring-red-400 text-white'
                  : 'bg-red-900/50 hover:bg-red-800 border border-red-700 text-red-300'
              }`}
            >
              {confirmKill ? 'CONFIRM — Halt All Trading' : 'Activate Kill Switch'}
            </button>
          )}
        </div>

        {/* Circuit Breaker */}
        <div className="bg-gray-800 rounded-lg p-6">
          <h2 className="text-lg font-semibold mb-4">Circuit Breaker</h2>
          <div className={`px-4 py-3 rounded-lg border ${cbColor} mb-3`}>
            <div className="text-lg font-bold capitalize">{cbState}</div>
            <div className="text-sm mt-1">
              Daily PnL: ${circuitBreaker?.daily_pnl || '0.00'}
            </div>
          </div>
          {circuitBreaker?.thresholds && (
            <div className="space-y-1 text-sm">
              <div className="flex justify-between text-gray-400">
                <span>Warning (50%)</span>
                <span>-${circuitBreaker.thresholds.warning}</span>
              </div>
              <div className="flex justify-between text-gray-400">
                <span>Throttle (75%)</span>
                <span>-${circuitBreaker.thresholds.throttle}</span>
              </div>
              <div className="flex justify-between text-gray-400">
                <span>Halt (100%)</span>
                <span>-${circuitBreaker.thresholds.halt}</span>
              </div>
              <div className="flex justify-between text-gray-400">
                <span>Emergency (150%)</span>
                <span>-${circuitBreaker.thresholds.emergency}</span>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Risk Gauges */}
      <div className="bg-gray-800 rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">Risk Gauges</h2>
        {gauges.length === 0 ? (
          <div className="text-gray-400 text-sm">No risk data available yet. Start trading to see risk metrics.</div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {gauges.map((gauge, i) => {
              const pct = gauge.limit > 0 ? (gauge.current / gauge.limit) * 100 : 0
              const barColor = gauge.status === 'critical' ? 'bg-red-500' :
                gauge.status === 'warning' ? 'bg-yellow-500' : 'bg-green-500'
              return (
                <div key={i} className="bg-gray-700/50 rounded-lg p-4">
                  <div className="flex justify-between text-sm mb-2">
                    <span className="text-gray-300">{gauge.name}</span>
                    <span className="text-gray-400">
                      {gauge.current}{gauge.unit} / {gauge.limit}{gauge.unit}
                    </span>
                  </div>
                  <div className="h-2 bg-gray-600 rounded-full overflow-hidden">
                    <div
                      className={`h-full rounded-full transition-all ${barColor}`}
                      style={{ width: `${Math.min(pct, 100)}%` }}
                    />
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Autonomy Level */}
      <div className="bg-gray-800 rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">Autonomy Level</h2>
        <div className="grid grid-cols-5 gap-2">
          {autonomyLabels.map(({ level, label, desc }) => (
            <button
              key={level}
              onClick={() => handleSetAutonomy(level)}
              className={`p-3 rounded-lg text-center transition-all ${
                autonomyLevel === level
                  ? 'bg-brand-600/30 border-2 border-brand-500 text-brand-300'
                  : 'bg-gray-700/50 border border-gray-600 text-gray-400 hover:bg-gray-700'
              }`}
            >
              <div className="text-lg font-bold">{level}</div>
              <div className="text-xs font-medium mt-1">{label}</div>
              <div className="text-xs mt-1 opacity-75">{desc}</div>
            </button>
          ))}
        </div>
      </div>

      {/* Recent Risk Checks */}
      <div className="bg-gray-800 rounded-lg overflow-hidden">
        <div className="px-4 py-3 border-b border-gray-700">
          <h2 className="text-lg font-semibold">Recent Risk Checks</h2>
        </div>
        {recentChecks.length === 0 ? (
          <div className="p-8 text-center text-gray-400">No risk checks recorded yet.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-400 text-left border-b border-gray-700">
                  <th className="px-4 py-2">Time</th>
                  <th className="px-4 py-2">Check</th>
                  <th className="px-4 py-2">Result</th>
                  <th className="px-4 py-2">Message</th>
                </tr>
              </thead>
              <tbody>
                {recentChecks.slice(0, 20).map((check, i) => (
                  <tr key={i} className="border-b border-gray-700/50 hover:bg-gray-700/30">
                    <td className="px-4 py-2 text-gray-400 text-xs">
                      {new Date(check.timestamp).toLocaleString()}
                    </td>
                    <td className="px-4 py-2 font-medium">{check.check_name}</td>
                    <td className="px-4 py-2">
                      <span className={`px-2 py-0.5 rounded text-xs ${
                        check.passed ? 'bg-green-900/50 text-green-300' : 'bg-red-900/50 text-red-300'
                      }`}>
                        {check.passed ? 'PASS' : 'FAIL'}
                      </span>
                    </td>
                    <td className="px-4 py-2 text-gray-400 max-w-xs truncate" title={check.message}>
                      {check.message}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {toast && <Toast message={toast.message} type={toast.type} onDismiss={() => setToast(null)} />}
    </div>
  )
}

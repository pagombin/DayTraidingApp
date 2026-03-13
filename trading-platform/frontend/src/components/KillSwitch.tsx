'use client'

import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'
import { useWebSocket } from '@/hooks/useWebSocket'

export default function KillSwitch() {
  const [confirming, setConfirming] = useState(false)
  const [activated, setActivated] = useState(false)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    api.get('/api/killswitch/status')
      .then(res => setActivated(res.active))
      .catch(() => {})
  }, [])

  useWebSocket(useCallback((msg) => {
    if (msg.channel === 'oms:killswitch') {
      setActivated(msg.data?.active ?? false)
    }
  }, []))

  async function handleClick() {
    if (!confirming) {
      setConfirming(true)
      setTimeout(() => setConfirming(false), 5000)
      return
    }

    setLoading(true)
    try {
      await api.post('/api/killswitch/activate', { reason: 'Manual activation from header' })
      setActivated(true)
    } catch (err) {
      console.error('Failed to activate kill switch:', err)
    } finally {
      setConfirming(false)
      setLoading(false)
    }
  }

  async function handleResume() {
    setLoading(true)
    try {
      await api.post('/api/killswitch/deactivate', { confirmation: 'CONFIRM' })
      setActivated(false)
    } catch (err) {
      console.error('Failed to deactivate kill switch:', err)
    } finally {
      setLoading(false)
    }
  }

  if (activated) {
    return (
      <button
        onClick={handleResume}
        disabled={loading}
        className="px-3 py-1.5 bg-red-700 text-white rounded-md text-sm font-bold animate-pulse disabled:opacity-50"
      >
        {loading ? 'Resuming...' : 'TRADING HALTED — Click to Resume'}
      </button>
    )
  }

  return (
    <button
      onClick={handleClick}
      disabled={loading}
      className={`px-3 py-1.5 rounded-md text-sm font-bold transition-all ${
        confirming
          ? 'bg-red-600 text-white ring-2 ring-red-400'
          : 'bg-red-900/50 text-red-300 hover:bg-red-800 border border-red-700'
      } disabled:opacity-50`}
    >
      {loading ? 'Activating...' : confirming ? 'CONFIRM KILL SWITCH' : 'Kill Switch'}
    </button>
  )
}

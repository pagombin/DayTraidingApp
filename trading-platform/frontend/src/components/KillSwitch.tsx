'use client'

import { useState } from 'react'

export default function KillSwitch() {
  const [confirming, setConfirming] = useState(false)
  const [activated, setActivated] = useState(false)

  function handleClick() {
    if (!confirming) {
      setConfirming(true)
      setTimeout(() => setConfirming(false), 5000)
      return
    }

    setActivated(true)
    setConfirming(false)
    // In a real implementation, this would call the API to halt all trading
    console.warn('KILL SWITCH ACTIVATED — All trading halted')
  }

  if (activated) {
    return (
      <button
        onClick={() => setActivated(false)}
        className="px-3 py-1.5 bg-red-700 text-white rounded-md text-sm font-bold animate-pulse"
      >
        TRADING HALTED — Click to Resume
      </button>
    )
  }

  return (
    <button
      onClick={handleClick}
      className={`px-3 py-1.5 rounded-md text-sm font-bold transition-all ${
        confirming
          ? 'bg-red-600 text-white ring-2 ring-red-400'
          : 'bg-red-900/50 text-red-300 hover:bg-red-800 border border-red-700'
      }`}
    >
      {confirming ? 'CONFIRM KILL SWITCH' : 'Kill Switch'}
    </button>
  )
}

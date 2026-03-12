'use client'

import { useState } from 'react'

const STEPS = [
  {
    title: 'Welcome to Your Trading Platform',
    content: 'This is your central command center for automated trading. Everything you need is accessible from the sidebar.',
  },
  {
    title: 'System Health',
    content: 'The System Health page shows you the status of every service. Green means healthy, yellow means degraded, red means an issue that needs attention.',
  },
  {
    title: 'Settings',
    content: 'Configure your broker, risk parameters, notifications, and watchlist from the Settings page. Changes are applied in real-time — no restart required.',
  },
  {
    title: 'Kill Switch',
    content: 'The red Kill Switch button in the header immediately halts all trading. Use it in emergencies. It requires a confirmation click to prevent accidents.',
  },
  {
    title: 'Paper Trading Mode',
    content: 'You\'re currently in Paper Trading mode — all trades use simulated money. You can switch to live trading in Settings after you\'re comfortable.',
  },
]

export default function OnboardingTour({ onDismiss }: { onDismiss: () => void }) {
  const [step, setStep] = useState(0)

  const current = STEPS[step]
  const isLast = step === STEPS.length - 1

  return (
    <div className="fixed inset-0 z-[100] bg-black/60 flex items-center justify-center p-4">
      <div className="bg-gray-800 rounded-xl p-6 max-w-md w-full border border-gray-700 shadow-2xl">
        {/* Progress dots */}
        <div className="flex gap-1.5 mb-4">
          {STEPS.map((_, i) => (
            <div
              key={i}
              className={`h-1 flex-1 rounded-full ${
                i <= step ? 'bg-brand-500' : 'bg-gray-600'
              }`}
            />
          ))}
        </div>

        <h2 className="text-xl font-bold mb-2">{current.title}</h2>
        <p className="text-gray-300 text-sm leading-relaxed mb-6">{current.content}</p>

        <div className="flex justify-between">
          <button
            onClick={onDismiss}
            className="text-sm text-gray-400 hover:text-white transition-colors"
          >
            Skip tour
          </button>
          <button
            onClick={() => {
              if (isLast) onDismiss()
              else setStep(s => s + 1)
            }}
            className="px-4 py-2 bg-brand-600 hover:bg-brand-700 rounded-lg text-sm font-medium transition-colors"
          >
            {isLast ? 'Get Started' : 'Next'}
          </button>
        </div>
      </div>
    </div>
  )
}

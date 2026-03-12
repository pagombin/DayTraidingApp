'use client'

import { useEffect } from 'react'

export default function Toast({
  message,
  type = 'success',
  onDismiss,
}: {
  message: string
  type: 'success' | 'error'
  onDismiss: () => void
}) {
  useEffect(() => {
    const timer = setTimeout(onDismiss, 4000)
    return () => clearTimeout(timer)
  }, [onDismiss])

  return (
    <div className="fixed bottom-4 right-4 z-[100] animate-slide-up">
      <div className={`px-4 py-3 rounded-lg shadow-lg flex items-center gap-3 ${
        type === 'success'
          ? 'bg-green-900/90 border border-green-700 text-green-200'
          : 'bg-red-900/90 border border-red-700 text-red-200'
      }`}>
        <span>{type === 'success' ? '\u2713' : '\u2717'}</span>
        <span className="text-sm">{message}</span>
        <button
          onClick={onDismiss}
          className="ml-2 text-gray-400 hover:text-white"
        >
          \u00D7
        </button>
      </div>
    </div>
  )
}

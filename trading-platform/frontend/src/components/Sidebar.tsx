'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'

const NAV_ITEMS = [
  { href: '/', label: 'Home', icon: '\uD83C\uDFE0' },
  { href: '/market', label: 'Market', icon: '\uD83D\uDCC8' },
  { href: '/portfolio', label: 'Portfolio', icon: '\uD83D\uDCBC' },
  { href: '/orders', label: 'Orders', icon: '\uD83D\uDCCB' },
  { href: '/strategies', label: 'Strategies', icon: '\uD83E\uDDE0', placeholder: 'Module 5' },
  { href: '/events', label: 'Events', icon: '\uD83D\uDCF0', placeholder: 'Module 4' },
  { href: '/backtesting', label: 'Backtesting', icon: '\u23EA', placeholder: 'Module 5' },
  { href: '/risk', label: 'Risk & Controls', icon: '\uD83D\uDEE1\uFE0F' },
  { href: '/health', label: 'System Health', icon: '\uD83D\uDC9A' },
  { href: '/settings', label: 'Settings', icon: '\u2699\uFE0F' },
]

export default function Sidebar({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  const pathname = usePathname()

  return (
    <aside className={`fixed top-14 left-0 bottom-0 z-40 bg-gray-800 border-r border-gray-700 transition-all ${
      open ? 'w-64' : 'w-16'
    }`}>
      <button
        onClick={onToggle}
        className="absolute -right-3 top-4 w-6 h-6 bg-gray-700 rounded-full flex items-center justify-center text-xs hover:bg-gray-600 transition-colors border border-gray-600"
      >
        {open ? '\u2039' : '\u203A'}
      </button>

      <nav className="mt-4 space-y-1 px-2">
        {NAV_ITEMS.map(item => {
          const isActive = pathname === item.href
          const isPlaceholder = !!item.placeholder

          if (isPlaceholder) {
            return (
              <div
                key={item.href}
                className="flex items-center gap-3 px-3 py-2 rounded-lg text-gray-500 cursor-not-allowed"
                title={`Coming in ${item.placeholder}`}
              >
                <span className="text-lg w-6 text-center">{item.icon}</span>
                {open && (
                  <span className="text-sm truncate">{item.label}</span>
                )}
              </div>
            )
          }

          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 px-3 py-2 rounded-lg transition-colors ${
                isActive
                  ? 'bg-brand-600/20 text-brand-400'
                  : 'text-gray-300 hover:bg-gray-700 hover:text-white'
              }`}
            >
              <span className="text-lg w-6 text-center">{item.icon}</span>
              {open && (
                <span className="text-sm truncate">{item.label}</span>
              )}
            </Link>
          )
        })}
      </nav>
    </aside>
  )
}

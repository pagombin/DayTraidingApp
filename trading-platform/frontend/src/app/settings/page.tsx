'use client'

import { useState, useEffect, useCallback } from 'react'
import Toast from '@/components/Toast'
import { api } from '@/lib/api'

type ConfigItem = {
  key: string
  value: any
  category: string
  label: string
  description: string | null
  value_type: string
  constraints: any
  updated_at: string
  updated_by: string
}

const TABS = [
  { id: 'broker', label: 'Broker' },
  { id: 'data', label: 'Data Provider' },
  { id: 'risk', label: 'Risk Parameters' },
  { id: 'notifications', label: 'Notifications' },
  { id: 'watchlist', label: 'Watchlist' },
  { id: 'system', label: 'System' },
]

export default function SettingsPage() {
  const [activeTab, setActiveTab] = useState('broker')
  const [config, setConfig] = useState<Record<string, ConfigItem[]>>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const [changes, setChanges] = useState<Record<string, any>>({})

  useEffect(() => {
    loadConfig()
  }, [])

  async function loadConfig() {
    try {
      const data = await api.get('/api/config')
      setConfig(data.config || {})
    } catch {
      setConfig({})
    } finally {
      setLoading(false)
    }
  }

  function handleChange(key: string, value: any) {
    setChanges(prev => ({ ...prev, [key]: value }))
  }

  async function handleSave() {
    setSaving(true)
    try {
      const items = Object.entries(changes).map(([key, value]) => ({
        key,
        value: typeof value === 'string' ? JSON.stringify(value) : value,
      }))

      if (items.length === 0) {
        setToast({ message: 'No changes to save', type: 'success' })
        setSaving(false)
        return
      }

      await api.post('/api/config/bulk', { items })
      setToast({ message: 'Settings saved successfully', type: 'success' })
      setChanges({})
      loadConfig()
    } catch {
      setToast({ message: 'Failed to save settings', type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  const tabItems = config[activeTab] || []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Settings</h1>
        <button
          onClick={handleSave}
          disabled={saving || Object.keys(changes).length === 0}
          className="px-4 py-2 bg-brand-600 hover:bg-brand-700 rounded-lg font-medium disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {saving ? 'Saving...' : 'Save Changes'}
        </button>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 bg-gray-800 rounded-lg p-1">
        {TABS.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${
              activeTab === tab.id
                ? 'bg-gray-700 text-white'
                : 'text-gray-400 hover:text-white hover:bg-gray-700/50'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Config Items */}
      {loading ? (
        <div className="flex items-center justify-center h-64">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-white"></div>
        </div>
      ) : (
        <div className="bg-gray-800 rounded-lg divide-y divide-gray-700">
          {tabItems.length === 0 ? (
            <div className="p-8 text-center text-gray-400">
              No settings available for this category yet.
            </div>
          ) : (
            tabItems.map(item => (
              <ConfigField
                key={item.key}
                item={item}
                value={changes[item.key] !== undefined ? changes[item.key] : item.value}
                onChange={val => handleChange(item.key, val)}
              />
            ))
          )}

          {/* Special buttons per tab */}
          {activeTab === 'broker' && (
            <div className="p-4">
              <button className="px-4 py-2 bg-gray-700 hover:bg-gray-600 rounded-lg text-sm transition-colors">
                Test Connection
              </button>
            </div>
          )}
          {activeTab === 'data' && (
            <DataProviderStatus />
          )}
          {activeTab === 'notifications' && (
            <div className="p-4">
              <button className="px-4 py-2 bg-gray-700 hover:bg-gray-600 rounded-lg text-sm transition-colors">
                Send Test Notification
              </button>
            </div>
          )}
        </div>
      )}

      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onDismiss={() => setToast(null)}
        />
      )}
    </div>
  )
}

function ConfigField({ item, value, onChange }: {
  item: ConfigItem
  value: any
  onChange: (val: any) => void
}) {
  const isSecret = item.key.includes('secret') || item.key.includes('password') || item.key.includes('token')

  return (
    <div className="p-4 flex flex-col sm:flex-row sm:items-center gap-3">
      <div className="flex-1">
        <label className="text-sm font-medium">{item.label}</label>
        {item.description && (
          <p className="text-xs text-gray-400 mt-0.5">{item.description}</p>
        )}
      </div>
      <div className="sm:w-72">
        {item.value_type === 'boolean' && (
          <button
            onClick={() => onChange(!value)}
            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
              value ? 'bg-brand-600' : 'bg-gray-600'
            }`}
          >
            <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
              value ? 'translate-x-6' : 'translate-x-1'
            }`} />
          </button>
        )}

        {item.value_type === 'number' && (
          <input
            type="number"
            value={value}
            onChange={e => onChange(Number(e.target.value))}
            min={item.constraints?.min}
            max={item.constraints?.max}
            step={item.constraints?.step}
            className="w-full px-3 py-1.5 bg-gray-700 rounded-md border border-gray-600 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        )}

        {item.value_type === 'string' && (
          <input
            type={isSecret ? 'password' : 'text'}
            value={typeof value === 'string' ? value : ''}
            onChange={e => onChange(e.target.value)}
            className="w-full px-3 py-1.5 bg-gray-700 rounded-md border border-gray-600 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        )}

        {item.value_type === 'select' && (
          <select
            value={value}
            onChange={e => onChange(e.target.value)}
            className="w-full px-3 py-1.5 bg-gray-700 rounded-md border border-gray-600 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            {item.constraints?.options?.map((opt: string) => (
              <option key={opt} value={opt}>{opt}</option>
            ))}
          </select>
        )}

        {item.value_type === 'json' && (
          <input
            type="text"
            value={Array.isArray(value) ? value.join(', ') : JSON.stringify(value)}
            onChange={e => {
              const symbols = e.target.value.split(',').map(s => s.trim()).filter(Boolean)
              onChange(symbols)
            }}
            placeholder="SPY, QQQ, AAPL..."
            className="w-full px-3 py-1.5 bg-gray-700 rounded-md border border-gray-600 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        )}
      </div>
    </div>
  )
}

function DataProviderStatus() {
  const [status, setStatus] = useState<any>(null)
  const [testing, setTesting] = useState(false)

  const testConnection = useCallback(async () => {
    setTesting(true)
    setStatus(null)
    try {
      const data = await api.get('/api/market/status')
      setStatus(data)
    } catch (err: any) {
      setStatus({ error: err?.message || 'Market data service not reachable' })
    } finally {
      setTesting(false)
    }
  }, [])

  useEffect(() => {
    testConnection()
  }, [testConnection])

  return (
    <div className="p-4 space-y-3">
      <button
        onClick={testConnection}
        disabled={testing}
        className="px-4 py-2 bg-gray-700 hover:bg-gray-600 rounded-lg text-sm transition-colors disabled:opacity-50"
      >
        {testing ? 'Testing...' : 'Test Connection'}
      </button>

      {status && !status.error && (
        <div className="bg-green-900/20 border border-green-800 rounded-lg p-3 text-sm space-y-1">
          <div className="flex justify-between">
            <span className="text-gray-400">Provider</span>
            <span className="font-medium">{status.provider || 'N/A'}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-gray-400">Connected</span>
            <span className={status.connected ? 'text-green-400' : 'text-red-400'}>
              {status.connected ? 'Yes' : 'No'}
            </span>
          </div>
          <div className="flex justify-between">
            <span className="text-gray-400">Symbols</span>
            <span>{status.symbol_count || 0}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-gray-400">Market</span>
            <span>{status.market_status || 'Unknown'}</span>
          </div>
          {status.metrics && (
            <>
              <div className="flex justify-between">
                <span className="text-gray-400">Ticks/sec</span>
                <span className="font-mono">{status.metrics.ticks_per_second || 0}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-400">Total Ticks</span>
                <span className="font-mono">{(status.metrics.total_ticks || 0).toLocaleString()}</span>
              </div>
            </>
          )}
        </div>
      )}

      {status?.error && (
        <div className="bg-red-900/20 border border-red-800 rounded-lg p-3 text-sm text-red-300">
          {status.error}
        </div>
      )}
    </div>
  )
}

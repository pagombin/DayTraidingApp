'use client'

type ServiceHealth = {
  name: string
  status: string
  last_check: string
  response_time_ms?: number
}

const STATUS_COLORS: Record<string, string> = {
  healthy: 'bg-green-400',
  degraded: 'bg-yellow-400',
  unhealthy: 'bg-red-400',
  unknown: 'bg-gray-400',
}

const STATUS_TEXT_COLORS: Record<string, string> = {
  healthy: 'text-green-300',
  degraded: 'text-yellow-300',
  unhealthy: 'text-red-300',
  unknown: 'text-gray-300',
}

export default function HealthCard({ service, compact = false }: {
  service: ServiceHealth
  compact?: boolean
}) {
  const dotColor = STATUS_COLORS[service.status] || STATUS_COLORS.unknown
  const textColor = STATUS_TEXT_COLORS[service.status] || STATUS_TEXT_COLORS.unknown

  if (compact) {
    return (
      <div className="flex items-center gap-2 bg-gray-700/50 rounded-lg px-3 py-2">
        <div className={`w-2 h-2 rounded-full ${dotColor}`} />
        <span className="text-sm capitalize">{service.name}</span>
        <span className={`text-xs ml-auto ${textColor}`}>{service.status}</span>
      </div>
    )
  }

  const lastCheck = service.last_check
    ? new Date(service.last_check).toLocaleTimeString()
    : 'Never'

  return (
    <div className="bg-gray-800 rounded-lg p-5 border border-gray-700">
      <div className="flex items-center justify-between mb-3">
        <h3 className="font-semibold capitalize">{service.name}</h3>
        <div className={`w-3 h-3 rounded-full ${dotColor}`} />
      </div>

      <div className="space-y-2 text-sm">
        <div className="flex justify-between">
          <span className="text-gray-400">Status</span>
          <span className={textColor}>{service.status}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-gray-400">Last Check</span>
          <span>{lastCheck}</span>
        </div>
        {service.response_time_ms !== undefined && (
          <div className="flex justify-between">
            <span className="text-gray-400">Response Time</span>
            <span>{service.response_time_ms}ms</span>
          </div>
        )}
      </div>

      {service.status === 'unhealthy' && (
        <button className="mt-3 w-full px-3 py-1.5 bg-red-900/30 hover:bg-red-900/50 border border-red-800 rounded-md text-sm text-red-300 transition-colors">
          Restart Service
        </button>
      )}
    </div>
  )
}

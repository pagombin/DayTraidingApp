'use client'

import { useHealth } from '@/hooks/useHealth'
import HealthCard from '@/components/HealthCard'

export default function HealthPage() {
  const { services, loading } = useHealth()

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">System Health</h1>
        <span className="text-sm text-gray-400">Auto-refreshes every 10 seconds</span>
      </div>

      {loading ? (
        <div className="flex items-center justify-center h-64">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-white"></div>
        </div>
      ) : (
        <>
          {/* Service Health Cards */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {services.map(svc => (
              <HealthCard key={svc.name} service={svc} />
            ))}
            {services.length === 0 && (
              <div className="col-span-full text-center py-12 text-gray-400">
                No services registered yet. Services will appear after they start reporting health.
              </div>
            )}
          </div>

          {/* Resource Usage Placeholder */}
          <div className="bg-gray-800 rounded-lg p-6">
            <h2 className="text-lg font-semibold mb-4">Resource Usage</h2>
            <div className="h-48 flex items-center justify-center text-gray-500 border border-gray-700 rounded-lg border-dashed">
              CPU and memory monitoring — Coming in a future update
            </div>
          </div>
        </>
      )}
    </div>
  )
}

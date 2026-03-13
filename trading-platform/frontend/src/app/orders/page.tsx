'use client'

import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'
import Toast from '@/components/Toast'

type Order = {
  order_id: string
  symbol: string
  side: string
  order_type: string
  quantity: number
  limit_price?: number
  stop_price?: number
  state: string
  filled_qty: number
  avg_fill_price?: number
  commission?: number
  rejection_reason?: string
  strategy_id: string
  time_in_force: string
  created_at: string
  submitted_at?: string
  filled_at?: string
  expires_in_seconds?: number
}

export default function OrdersPage() {
  const [orders, setOrders] = useState<Order[]>([])
  const [pending, setPending] = useState<Order[]>([])
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const [autonomyLevel, setAutonomyLevel] = useState(1)

  // Order form state
  const [symbol, setSymbol] = useState('')
  const [side, setSide] = useState('buy')
  const [orderType, setOrderType] = useState('market')
  const [quantity, setQuantity] = useState(1)
  const [limitPrice, setLimitPrice] = useState('')
  const [stopPrice, setStopPrice] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const fetchData = useCallback(async () => {
    try {
      const [ordersRes, pendingRes, autonomyRes] = await Promise.all([
        api.get('/api/orders?limit=50').catch(() => ({ orders: [] })),
        api.get('/api/autonomy/pending').catch(() => ({ pending: [] })),
        api.get('/api/autonomy/level').catch(() => ({ level: 1 })),
      ])
      setOrders(ordersRes.orders || [])
      setPending(pendingRes.pending || [])
      setAutonomyLevel(autonomyRes.level)
    } catch { /* ignore */ }
    finally { setLoading(false) }
  }, [])

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 3000)
    return () => clearInterval(interval)
  }, [fetchData])

  async function handleSubmitOrder() {
    if (!symbol || quantity <= 0) {
      setToast({ message: 'Symbol and quantity are required', type: 'error' })
      return
    }
    setSubmitting(true)
    try {
      const body: any = {
        symbol: symbol.toUpperCase(),
        side,
        order_type: orderType,
        quantity,
      }
      if (limitPrice) body.limit_price = limitPrice
      if (stopPrice) body.stop_price = stopPrice

      await api.post('/api/orders', body)
      setToast({ message: `Order submitted: ${side.toUpperCase()} ${quantity} ${symbol.toUpperCase()}`, type: 'success' })
      setSymbol('')
      setQuantity(1)
      setLimitPrice('')
      setStopPrice('')
      setShowForm(false)
      fetchData()
    } catch (err: any) {
      setToast({ message: err.message || 'Failed to submit order', type: 'error' })
    } finally {
      setSubmitting(false)
    }
  }

  async function handleApprove(orderID: string) {
    try {
      await api.put(`/api/orders/${orderID}/approve`, {})
      setToast({ message: 'Order approved', type: 'success' })
      fetchData()
    } catch (err: any) {
      setToast({ message: err.message || 'Failed to approve', type: 'error' })
    }
  }

  async function handleReject(orderID: string) {
    try {
      await api.put(`/api/orders/${orderID}/reject`, { reason: 'Rejected from dashboard' })
      setToast({ message: 'Order rejected', type: 'success' })
      fetchData()
    } catch (err: any) {
      setToast({ message: err.message || 'Failed to reject', type: 'error' })
    }
  }

  async function handleCancel(orderID: string) {
    try {
      await api.put(`/api/orders/${orderID}/cancel`, {})
      setToast({ message: 'Order cancelled', type: 'success' })
      fetchData()
    } catch (err: any) {
      setToast({ message: err.message || 'Failed to cancel', type: 'error' })
    }
  }

  const stateColor = (state: string) => {
    switch (state) {
      case 'FILLED': return 'bg-green-900/50 text-green-300'
      case 'PARTIAL_FILL': return 'bg-green-900/30 text-green-200'
      case 'SUBMITTED': case 'ACCEPTED': return 'bg-blue-900/50 text-blue-300'
      case 'AWAITING_APPROVAL': return 'bg-yellow-900/50 text-yellow-300'
      case 'PENDING': case 'APPROVED': return 'bg-gray-700 text-gray-300'
      case 'REJECTED': return 'bg-red-900/50 text-red-300'
      case 'CANCELLED': return 'bg-gray-700 text-gray-400'
      case 'FAILED': return 'bg-red-900/50 text-red-300'
      default: return 'bg-gray-700 text-gray-300'
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-white"></div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Orders</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="px-4 py-2 bg-brand-600 hover:bg-brand-700 rounded-lg font-medium transition-colors"
        >
          {showForm ? 'Cancel' : 'New Order'}
        </button>
      </div>

      {/* Manual Order Form */}
      {showForm && (
        <div className="bg-gray-800 rounded-lg p-6 space-y-4">
          <h2 className="text-lg font-semibold">Manual Order</h2>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <label className="block text-sm text-gray-400 mb-1">Symbol</label>
              <input
                type="text"
                value={symbol}
                onChange={e => setSymbol(e.target.value.toUpperCase())}
                placeholder="AAPL"
                className="w-full px-3 py-2 bg-gray-700 rounded-md border border-gray-600 focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Side</label>
              <select value={side} onChange={e => setSide(e.target.value)}
                className="w-full px-3 py-2 bg-gray-700 rounded-md border border-gray-600 focus:outline-none focus:ring-2 focus:ring-brand-500">
                <option value="buy">Buy</option>
                <option value="sell">Sell</option>
              </select>
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Type</label>
              <select value={orderType} onChange={e => setOrderType(e.target.value)}
                className="w-full px-3 py-2 bg-gray-700 rounded-md border border-gray-600 focus:outline-none focus:ring-2 focus:ring-brand-500">
                <option value="market">Market</option>
                <option value="limit">Limit</option>
                <option value="stop">Stop</option>
                <option value="stop_limit">Stop Limit</option>
              </select>
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Quantity</label>
              <input
                type="number"
                value={quantity}
                onChange={e => setQuantity(parseInt(e.target.value) || 0)}
                min={1}
                className="w-full px-3 py-2 bg-gray-700 rounded-md border border-gray-600 focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
          </div>
          {(orderType === 'limit' || orderType === 'stop_limit') && (
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1">Limit Price</label>
                <input type="text" value={limitPrice} onChange={e => setLimitPrice(e.target.value)} placeholder="0.00"
                  className="w-full px-3 py-2 bg-gray-700 rounded-md border border-gray-600 focus:outline-none focus:ring-2 focus:ring-brand-500" />
              </div>
              {(orderType === 'stop' || orderType === 'stop_limit') && (
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Stop Price</label>
                  <input type="text" value={stopPrice} onChange={e => setStopPrice(e.target.value)} placeholder="0.00"
                    className="w-full px-3 py-2 bg-gray-700 rounded-md border border-gray-600 focus:outline-none focus:ring-2 focus:ring-brand-500" />
                </div>
              )}
            </div>
          )}
          {orderType === 'stop' && (
            <div>
              <label className="block text-sm text-gray-400 mb-1">Stop Price</label>
              <input type="text" value={stopPrice} onChange={e => setStopPrice(e.target.value)} placeholder="0.00"
                className="w-full px-3 py-2 bg-gray-700 rounded-md border border-gray-600 focus:outline-none focus:ring-2 focus:ring-brand-500" />
            </div>
          )}
          <button
            onClick={handleSubmitOrder}
            disabled={submitting || !symbol || quantity <= 0}
            className={`px-6 py-2 rounded-lg font-medium transition-colors ${
              side === 'buy'
                ? 'bg-green-600 hover:bg-green-700 disabled:bg-green-900'
                : 'bg-red-600 hover:bg-red-700 disabled:bg-red-900'
            } disabled:opacity-50 disabled:cursor-not-allowed`}
          >
            {submitting ? 'Submitting...' : `${side === 'buy' ? 'Buy' : 'Sell'} ${quantity} ${symbol || '...'}`}
          </button>
        </div>
      )}

      {/* Pending Approvals */}
      {autonomyLevel === 1 && pending.length > 0 && (
        <div className="space-y-3">
          <h2 className="text-lg font-semibold text-yellow-400">Pending Approvals ({pending.length})</h2>
          {pending.map(order => (
            <div key={order.order_id} className="bg-yellow-900/20 border border-yellow-800 rounded-lg p-4">
              <div className="flex items-center justify-between">
                <div>
                  <span className="font-medium">
                    {order.side.toUpperCase()} {order.quantity} shares of {order.symbol}
                  </span>
                  {order.limit_price && <span className="text-gray-400 ml-2">@ ${order.limit_price}</span>}
                  <div className="text-sm text-gray-400 mt-1">
                    Strategy: {order.strategy_id} | Expires in {order.expires_in_seconds}s
                  </div>
                </div>
                <div className="flex gap-2">
                  <button onClick={() => handleApprove(order.order_id)}
                    className="px-4 py-1.5 bg-green-600 hover:bg-green-700 rounded-md text-sm font-medium">
                    Approve
                  </button>
                  <button onClick={() => handleReject(order.order_id)}
                    className="px-4 py-1.5 bg-red-600 hover:bg-red-700 rounded-md text-sm font-medium">
                    Reject
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Orders Table */}
      <div className="bg-gray-800 rounded-lg overflow-hidden">
        <div className="px-4 py-3 border-b border-gray-700">
          <h2 className="text-lg font-semibold">Order History</h2>
        </div>
        {orders.length === 0 ? (
          <div className="p-8 text-center text-gray-400">No orders yet. Click "New Order" to submit one.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-400 text-left border-b border-gray-700">
                  <th className="px-4 py-2">Time</th>
                  <th className="px-4 py-2">Symbol</th>
                  <th className="px-4 py-2">Side</th>
                  <th className="px-4 py-2">Type</th>
                  <th className="px-4 py-2 text-right">Qty</th>
                  <th className="px-4 py-2 text-right">Filled</th>
                  <th className="px-4 py-2 text-right">Avg Price</th>
                  <th className="px-4 py-2">Status</th>
                  <th className="px-4 py-2">Actions</th>
                </tr>
              </thead>
              <tbody>
                {orders.map(order => (
                  <tr key={order.order_id} className="border-b border-gray-700/50 hover:bg-gray-700/30">
                    <td className="px-4 py-2 text-gray-400 text-xs">
                      {new Date(order.created_at).toLocaleString()}
                    </td>
                    <td className="px-4 py-2 font-medium">{order.symbol}</td>
                    <td className="px-4 py-2">
                      <span className={order.side === 'buy' ? 'text-green-400' : 'text-red-400'}>
                        {order.side.toUpperCase()}
                      </span>
                    </td>
                    <td className="px-4 py-2 text-gray-400">{order.order_type}</td>
                    <td className="px-4 py-2 text-right">{order.quantity}</td>
                    <td className="px-4 py-2 text-right">{order.filled_qty}</td>
                    <td className="px-4 py-2 text-right">
                      {order.avg_fill_price ? `$${order.avg_fill_price}` : '-'}
                    </td>
                    <td className="px-4 py-2">
                      <span className={`px-2 py-0.5 rounded text-xs ${stateColor(order.state)}`}>
                        {order.state}
                      </span>
                      {order.rejection_reason && (
                        <div className="text-xs text-red-400 mt-1 max-w-xs truncate" title={order.rejection_reason}>
                          {order.rejection_reason}
                        </div>
                      )}
                    </td>
                    <td className="px-4 py-2">
                      {!['FILLED', 'CANCELLED', 'REJECTED', 'EXPIRED', 'FAILED'].includes(order.state) && (
                        <button onClick={() => handleCancel(order.order_id)}
                          className="px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs">
                          Cancel
                        </button>
                      )}
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

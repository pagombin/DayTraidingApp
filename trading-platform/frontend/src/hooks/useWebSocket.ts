'use client'

import { useEffect, useRef, useState, useCallback } from 'react'
import { WS_URL } from '@/lib/constants'

export type WSMessage = {
  channel: string
  data: any
}

export function useWebSocket(onMessage?: (msg: WSMessage) => void) {
  const [connected, setConnected] = useState(false)
  const [reconnecting, setReconnecting] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const retriesRef = useRef(0)
  const maxRetryDelay = 30000

  const connect = useCallback(() => {
    try {
      const ws = new WebSocket(`${WS_URL}/ws`)
      wsRef.current = ws

      ws.onopen = () => {
        setConnected(true)
        setReconnecting(false)
        retriesRef.current = 0
      }

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data) as WSMessage
          onMessage?.(msg)
        } catch {
          // ignore malformed messages
        }
      }

      ws.onclose = () => {
        setConnected(false)
        scheduleReconnect()
      }

      ws.onerror = () => {
        ws.close()
      }
    } catch {
      scheduleReconnect()
    }
  }, [onMessage])

  function scheduleReconnect() {
    setReconnecting(true)
    const delay = Math.min(1000 * Math.pow(2, retriesRef.current), maxRetryDelay)
    retriesRef.current++
    setTimeout(connect, delay)
  }

  useEffect(() => {
    connect()
    return () => {
      wsRef.current?.close()
    }
  }, [connect])

  return { connected, reconnecting }
}

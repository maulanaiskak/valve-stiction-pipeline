import { useEffect, useRef, useState } from 'react'
import { fetchSensors, wsURL } from '../api'
import type { SensorStatus, WSMessage } from '../types'

// Loads the initial snapshot over REST (so the dashboard isn't empty while
// the WebSocket connects), then keeps it current from the WS's
// snapshot/update messages (see backend/ws.go).
export function useSensors(): SensorStatus[] {
  const [sensors, setSensors] = useState<Record<string, SensorStatus>>({})
  const gotSnapshot = useRef(false)

  useEffect(() => {
    fetchSensors()
      .then((list) => {
        if (gotSnapshot.current) return // WS snapshot already arrived first
        setSensors(Object.fromEntries(list.map((s) => [s.sensor_id, s])))
      })
      .catch((err) => console.error('failed to fetch initial sensors', err))

    let ws: WebSocket
    let reconnectTimer: ReturnType<typeof setTimeout>

    function connect() {
      ws = new WebSocket(wsURL())
      ws.onmessage = (event) => {
        const msg = JSON.parse(event.data) as WSMessage
        if (msg.type === 'snapshot') {
          gotSnapshot.current = true
          setSensors(Object.fromEntries(msg.sensors.map((s) => [s.sensor_id, s])))
        } else if (msg.type === 'update') {
          setSensors((prev) => ({ ...prev, [msg.sensor.sensor_id]: msg.sensor }))
        }
      }
      ws.onclose = () => {
        reconnectTimer = setTimeout(connect, 2000)
      }
    }
    connect()

    return () => {
      clearTimeout(reconnectTimer)
      ws?.close()
    }
  }, [])

  return Object.values(sensors).sort((a, b) => a.sensor_id.localeCompare(b.sensor_id))
}

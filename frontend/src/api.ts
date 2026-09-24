import type { SensorStatus, WindowSample } from './types'

export async function fetchSensors(): Promise<SensorStatus[]> {
  const res = await fetch('/api/sensors')
  if (!res.ok) throw new Error(`GET /api/sensors: ${res.status}`)
  return res.json()
}

export async function fetchSensorWindows(sensorId: string, limit = 50): Promise<WindowSample[]> {
  const res = await fetch(`/api/sensors/${encodeURIComponent(sensorId)}/windows?limit=${limit}`)
  if (!res.ok) throw new Error(`GET /api/sensors/${sensorId}/windows: ${res.status}`)
  return res.json()
}

export function wsURL(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/ws`
}

// Mirrors backend/db.go's SensorStatus/WindowSample JSON shape exactly --
// no translation layer, same reasoning as the gRPC contract on the Python
// side (see pipeline docs/V1_PLAN.md).
export interface SensorStatus {
  sensor_id: string
  window_start: string // RFC3339, from Go's time.Time JSON encoding
  label: 'yes' | 'no' | 'uncertain'
  ellipse_index: number
  kano_verdict: boolean
  rf_label: 'yes' | 'no' | null
  rf_probability: number | null
}

export interface WindowSample {
  window_start: string
  label: 'yes' | 'no' | 'uncertain'
  pv: number[]
  op: number[]
}

export type WSMessage =
  | { type: 'snapshot'; sensors: SensorStatus[] }
  | { type: 'update'; sensor: SensorStatus }

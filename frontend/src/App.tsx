import { useState } from 'react'
import './App.css'
import { SensorPanel } from './components/SensorPanel'
import { useSensors } from './hooks/useSensors'

function App() {
  const sensors = useSensors()
  const [selected, setSelected] = useState<string | null>(null)
  const activeSensor = sensors.find((s) => s.sensor_id === selected) ?? sensors[0]

  return (
    <div className="app">
      <header className="app-header">
        <h1>Valve Stiction Dashboard</h1>
        <nav className="sensor-tabs">
          {sensors.length === 0 && <span className="empty">Waiting for sensors…</span>}
          {sensors.map((s) => (
            <button
              key={s.sensor_id}
              className={s.sensor_id === activeSensor?.sensor_id ? 'tab active' : 'tab'}
              onClick={() => setSelected(s.sensor_id)}
            >
              {s.sensor_id}
              <span className={`dot dot-${s.label}`} />
            </button>
          ))}
        </nav>
      </header>

      <main>{activeSensor && <SensorPanel status={activeSensor} />}</main>
    </div>
  )
}

export default App

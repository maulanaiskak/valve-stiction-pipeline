import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

export function TimeSeriesChart({
  values,
  label,
  color,
}: {
  values: number[]
  label: string
  color: string
}) {
  const data = values.map((v, i) => ({ i, v }))

  return (
    <ResponsiveContainer width="100%" height={160}>
      <LineChart data={data} margin={{ top: 8, right: 16, bottom: 8, left: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--grid-color)" />
        <XAxis dataKey="i" stroke="var(--axis-color)" tick={false} />
        <YAxis stroke="var(--axis-color)" width={40} />
        <Tooltip
          formatter={(v) => [typeof v === 'number' ? v.toFixed(2) : v, label]}
          labelFormatter={() => ''}
        />
        <Line type="monotone" dataKey="v" stroke={color} dot={false} strokeWidth={1.5} isAnimationActive={false} />
      </LineChart>
    </ResponsiveContainer>
  )
}

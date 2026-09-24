import { CartesianGrid, ResponsiveContainer, Scatter, ScatterChart, Tooltip, XAxis, YAxis } from 'recharts'

// PV vs OP -- the same shape the ellipse-fit detector scores (a healthy
// valve traces a near-diagonal line; stiction opens it into a fat loop).
// See valve-stiction-ml classic.py's ellipse_stiction_index.
export function PhasePlot({ pv, op }: { pv: number[]; op: number[] }) {
  const data = pv.map((v, i) => ({ op: op[i], pv: v }))

  return (
    <ResponsiveContainer width="100%" height={220}>
      <ScatterChart margin={{ top: 8, right: 16, bottom: 8, left: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--grid-color)" />
        <XAxis type="number" dataKey="op" name="OP" stroke="var(--axis-color)" />
        <YAxis type="number" dataKey="pv" name="PV" stroke="var(--axis-color)" />
        <Tooltip cursor={{ strokeDasharray: '3 3' }} />
        <Scatter data={data} fill="var(--accent-color)" line shape="circle" />
      </ScatterChart>
    </ResponsiveContainer>
  )
}

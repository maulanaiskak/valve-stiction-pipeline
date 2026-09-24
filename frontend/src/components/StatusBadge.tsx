type Label = 'yes' | 'no' | 'uncertain'

const COLORS: Record<Label, string> = {
  yes: '#dc2626', // stiction detected -- red
  no: '#16a34a', // healthy -- green
  uncertain: '#d97706', // amber
}

const TEXT: Record<Label, string> = {
  yes: 'Stiction',
  no: 'Healthy',
  uncertain: 'Uncertain',
}

export function StatusBadge({ label, prefix }: { label: Label; prefix?: string }) {
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        padding: '4px 10px',
        borderRadius: 999,
        background: `${COLORS[label]}22`,
        color: COLORS[label],
        fontWeight: 600,
        fontSize: 13,
      }}
    >
      <span
        style={{
          width: 8,
          height: 8,
          borderRadius: '50%',
          background: COLORS[label],
          display: 'inline-block',
        }}
      />
      {prefix ? `${prefix}: ` : ''}
      {TEXT[label]}
    </span>
  )
}

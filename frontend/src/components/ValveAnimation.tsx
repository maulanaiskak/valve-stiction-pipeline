import './ValveAnimation.css'

// A sticking valve's stem moves in flat-then-jump steps (the classic
// stick-slip signature the ellipse/Kano detectors and RF model all score
// for) instead of tracking its setpoint smoothly. `sticking` swaps the
// stem's CSS animation between a smooth easing curve and a stepped one to
// make that difference visible at a glance, not just as a number.
export function ValveAnimation({ sticking }: { sticking: boolean }) {
  return (
    <div className="valve" role="img" aria-label={sticking ? 'Sticking valve' : 'Healthy valve'}>
      <svg viewBox="0 0 160 140" width="100%" height="140">
        <line x1="80" y1="0" x2="80" y2="30" stroke="#94a3b8" strokeWidth="6" />
        <rect x="40" y="30" width="80" height="18" rx="3" fill="#64748b" />

        <g className={sticking ? 'stem stem-sticking' : 'stem stem-healthy'}>
          <line x1="80" y1="48" x2="80" y2="78" stroke="#334155" strokeWidth="6" />
          <polygon points="55,78 105,78 80,110" fill={sticking ? '#dc2626' : '#16a34a'} />
        </g>

        <path d="M20,130 L60,110 L100,110 L140,130" fill="none" stroke="#94a3b8" strokeWidth="8" />
      </svg>
    </div>
  )
}

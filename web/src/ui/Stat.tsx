const nf = new Intl.NumberFormat('en-US')

/** Key figure with its change from the previous period (up, down, or unchanged). */
export function Stat({ label, value, previous }: { label: string; value: number; previous?: number }) {
  let change = null
  if (previous !== undefined) {
    const diff = value - previous
    const arrow = diff > 0 ? '↑' : diff < 0 ? '↓' : '='
    const color = diff > 0 ? 'text-success' : diff < 0 ? 'text-danger' : 'text-muted'
    const pct = previous === 0 ? null : Math.round((diff / previous) * 100)
    change = (
      <span className={`text-xs font-medium ${color}`} title={`Previous period: ${nf.format(previous)}`}>
        {arrow} {pct === null ? nf.format(Math.abs(diff)) : `${Math.abs(pct)}%`}
      </span>
    )
  }
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs text-muted">{label}</span>
      <span className="flex items-baseline gap-2">
        <span className="text-2xl font-semibold tabular-nums">{nf.format(value)}</span>
        {change}
      </span>
    </div>
  )
}

const nf = new Intl.NumberFormat('en-US')
const df = new Intl.DateTimeFormat('en-US', { day: 'numeric', month: 'short', year: 'numeric' })
const rf = new Intl.RelativeTimeFormat('en-US', { numeric: 'auto' })

export const number = (n: number) => nf.format(n)

/** "Oct 26, 2026" from "2026-10-26" or an ISO instant. */
export function date(value: string | null | undefined): string {
  if (!value) return '—'
  const parsed = new Date(value.length === 10 ? `${value}T12:00:00` : value)
  return Number.isNaN(parsed.getTime()) ? value : df.format(parsed)
}

/** "3 days ago". */
export function ago(value: string | null | undefined, now = new Date()): string {
  if (!value) return 'never'
  const instant = new Date(value.length === 10 ? `${value}T12:00:00` : value)
  if (Number.isNaN(instant.getTime())) return 'unknown date'
  const day = (d: Date) => Date.UTC(d.getFullYear(), d.getMonth(), d.getDate())
  const days = Math.round((day(instant) - day(now)) / 86_400_000)
  return rf.format(days, 'day')
}

export const shortSha = (sha: string | null | undefined) => (sha ? sha.slice(0, 7) : '—')

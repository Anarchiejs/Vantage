const nf = new Intl.NumberFormat('fr-FR')
const df = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' })
const rf = new Intl.RelativeTimeFormat('fr-FR', { numeric: 'auto' })

export const nombre = (n: number) => nf.format(n)

/** « 26 oct. 2026 » depuis « 2026-10-26 » ou un instant ISO. */
export function date(s: string | null | undefined): string {
  if (!s) return '—'
  const d = new Date(s.length === 10 ? `${s}T12:00:00` : s)
  return Number.isNaN(d.getTime()) ? s : df.format(d)
}

/** « il y a 3 jours ». */
export function depuis(s: string | null | undefined, maintenant = new Date()): string {
  if (!s) return 'jamais'
  const jours = Math.round((new Date(s).getTime() - maintenant.getTime()) / 86_400_000)
  return rf.format(jours, 'day')
}

export const shaCourt = (sha: string | null | undefined) => (sha ? sha.slice(0, 7) : '—')

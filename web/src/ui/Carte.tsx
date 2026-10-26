import type { ReactNode } from 'react'

/** Bloc de contenu : titre, action facultative à droite, corps. */
export function Carte({
  titre,
  action,
  children,
  className = '',
}: {
  titre?: ReactNode
  action?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <section className={`rounded-xl border border-bordure bg-surface p-4 ${className}`}>
      {(titre || action) && (
        <header className="mb-3 flex items-center justify-between gap-2">
          {titre && <h2 className="text-sm font-semibold tracking-wide text-doux uppercase">{titre}</h2>}
          {action}
        </header>
      )}
      {children}
    </section>
  )
}

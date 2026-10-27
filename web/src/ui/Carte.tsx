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
    <section className={`panel ${className}`}>
      {(titre || action) && (
        <header className="panel-header">
          {titre && <h2>{titre}</h2>}
          {action}
        </header>
      )}
      {children}
    </section>
  )
}

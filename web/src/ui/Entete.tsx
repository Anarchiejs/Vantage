import type { ReactNode } from 'react'
export function Entete({ titre, description, action }: { titre: string; description: string; action?: ReactNode }) {
  return <header className="page-heading"><div><p className="eyebrow">VOTRE ESPACE DE DÉVELOPPEMENT</p><h1>{titre}</h1><p className="page-description">{description}</p></div>{action}</header>
}

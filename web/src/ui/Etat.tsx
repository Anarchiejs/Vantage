import type { ReactNode } from 'react'
import type { EtatApi } from '../api/useApi'

export function Chargement({ lignes = 3 }: { lignes?: number }) {
  return (
    <div className="flex animate-pulse flex-col gap-2" aria-busy="true" aria-label="Chargement">
      {Array.from({ length: lignes }, (_, i) => (
        <div key={i} className="h-4 rounded bg-surface-2" style={{ width: `${90 - i * 15}%` }} />
      ))}
    </div>
  )
}

export function Vide({ children }: { children: ReactNode }) {
  return <p className="py-6 text-center text-sm text-doux">{children}</p>
}

export function ErreurAffichee({ erreur }: { erreur: Error }) {
  return (
    <p role="alert" className="rounded-lg border border-danger/40 bg-danger/10 p-3 text-sm text-danger">
      {erreur.message}
    </p>
  )
}

/** Affiche chargement / erreur, sinon appelle `children` avec les données. */
export function Charge<T>({ etat, children, lignes }: { etat: EtatApi<T>; children: (d: T) => ReactNode; lignes?: number }) {
  if (etat.statut === 'chargement') return <Chargement lignes={lignes} />
  if (etat.statut === 'erreur') return <ErreurAffichee erreur={etat.erreur} />
  return <>{children(etat.donnees)}</>
}

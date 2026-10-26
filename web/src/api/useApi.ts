import { useCallback, useEffect, useState } from 'react'
import { get } from './client'

export type EtatApi<T> =
  | { statut: 'chargement'; donnees?: undefined; erreur?: undefined }
  | { statut: 'ok'; donnees: T; erreur?: undefined }
  | { statut: 'erreur'; donnees?: undefined; erreur: Error }

/**
 * Charge un GET de l'API ; se recharge quand `chemin` change, et quand le serveur annonce une mise à jour
 * (événement « observatoire:maj » émis par la connexion SSE de App.tsx). `chemin` null = ne rien charger.
 */
export function useApi<T>(chemin: string | null): EtatApi<T> & { recharger: () => void } {
  const [etat, setEtat] = useState<EtatApi<T>>({ statut: 'chargement' })
  const [tour, setTour] = useState(0)
  const recharger = useCallback(() => setTour((t) => t + 1), [])

  useEffect(() => {
    window.addEventListener('observatoire:maj', recharger)
    return () => window.removeEventListener('observatoire:maj', recharger)
  }, [recharger])

  useEffect(() => {
    if (chemin === null) return
    const ctrl = new AbortController()
    setEtat((e) => (e.statut === 'ok' ? e : { statut: 'chargement' }))
    get<T>(chemin, ctrl.signal)
      .then((donnees) => setEtat({ statut: 'ok', donnees }))
      .catch((erreur: Error) => {
        if (erreur.name !== 'AbortError') setEtat({ statut: 'erreur', erreur })
      })
    return () => ctrl.abort()
  }, [chemin, tour])

  return { ...etat, recharger }
}

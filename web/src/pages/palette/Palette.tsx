import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router'
import type { ProjetResume } from '../../api/types'
import { useApi } from '../../api/useApi'
import { Vide } from '../../ui/Etat'
import { ordonner, type Libelle } from './flou'

interface Commande extends Libelle {
  agir: () => void
}

const PAGES: { id: string; libelle: string; detail: string; vers: string }[] = [
  { id: 'accueil', libelle: 'Accueil', detail: 'Tableau de bord', vers: '/' },
  { id: 'projets', libelle: 'Projets', detail: 'Liste des projets', vers: '/projets' },
  { id: 'technos', libelle: 'Technos & IA', detail: 'Technologies', vers: '/technos' },
  { id: 'graphe', libelle: 'Graphe', detail: 'Notes liées', vers: '/graphe' },
  { id: 'recherche', libelle: 'Recherche', detail: 'Notes et commits', vers: '/recherche' },
  { id: 'console', libelle: 'Console SQL', detail: 'Requêtes SQLite', vers: '/console' },
]

function focusables(racine: HTMLElement): HTMLElement[] {
  return [...racine.querySelectorAll<HTMLElement>('button, [href], input, select, textarea')].filter(
    (el) => !el.hasAttribute('disabled') && el.tabIndex >= 0 && !el.hidden,
  )
}

export default function Palette({ fermer }: { fermer: () => void }) {
  const naviguer = useNavigate()
  const projets = useApi<ProjetResume[]>('/api/projets')
  const donneesProjets = projets.statut === 'ok' ? projets.donnees : null
  const [q, setQ] = useState('')
  const [index, setIndex] = useState(0)
  const champ = useRef<HTMLInputElement>(null)

  useEffect(() => {
    champ.current?.focus()
  }, [])

  useEffect(() => {
    setIndex(0)
  }, [q])

  const commandes = useMemo(() => {
    const vers = (chemin: string) => {
      naviguer(chemin)
      fermer()
    }
    const pages: Commande[] = PAGES.map((page) => ({
      id: `page-${page.id}`,
      libelle: page.libelle,
      detail: page.detail,
      agir: () => vers(page.vers),
    }))
    const ouvris: Commande[] = (donneesProjets ?? []).map((projet) => ({
      id: `projet-${projet.id}`,
      libelle: projet.nom,
      detail: 'Ouvrir le projet',
      agir: () => vers(`/projets/${projet.id}`),
    }))
    const theme: Commande = {
      id: 'theme',
      libelle: 'Basculer le thème',
      detail: 'Clair ou sombre',
      agir: () => {
        window.dispatchEvent(new Event('observatoire:basculer-theme'))
        fermer()
      },
    }
    return [...pages, ...ouvris, theme]
  }, [donneesProjets, naviguer, fermer])

  const visibles = useMemo(() => {
    const tries = ordonner(commandes, q)
    const texte = q.trim()
    if (!texte) return tries
    const chercher: Commande = {
      id: 'chercher',
      libelle: `Rechercher « ${texte} »`,
      detail: 'Notes et commits',
      agir: () => {
        naviguer(`/recherche?q=${encodeURIComponent(texte)}`)
        fermer()
      },
    }
    return [...tries, chercher]
  }, [commandes, q, naviguer, fermer])

  const courant = visibles.length === 0 ? -1 : Math.min(index, visibles.length - 1)

  useEffect(() => {
    if (courant < 0) return
    const el = document.getElementById(`palette-${visibles[courant].id}`)
    try {
      el?.scrollIntoView?.({ block: 'nearest' })
    } catch {
      /* jsdom sans défilement */
    }
  }, [courant, visibles])

  function surClavier(e: KeyboardEvent<HTMLDivElement>) {
    if (e.key === 'Escape') {
      e.preventDefault()
      fermer()
      return
    }
    if (e.key === 'Tab') {
      const els = focusables(e.currentTarget)
      if (els.length === 0) return
      const premier = els[0]
      const dernier = els[els.length - 1]
      const actif = document.activeElement
      if (e.shiftKey && (actif === premier || !e.currentTarget.contains(actif))) {
        e.preventDefault()
        dernier.focus()
      } else if (!e.shiftKey && actif === dernier) {
        e.preventDefault()
        premier.focus()
      }
      return
    }
    if (visibles.length === 0) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setIndex((i) => Math.min(Math.min(Math.max(i, 0), visibles.length - 1) + 1, visibles.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setIndex((i) => Math.max(Math.min(i, visibles.length - 1) - 1, 0))
    } else if (e.key === 'Enter' && (e.target as HTMLElement).tagName !== 'BUTTON') {
      e.preventDefault()
      if (courant >= 0) visibles[courant].agir()
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-fond/80 px-4 pt-[12vh]" onClick={fermer}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Palette de commandes"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={surClavier}
        className="flex max-h-[70vh] w-full max-w-xl flex-col overflow-hidden rounded-xl border border-bordure bg-surface"
      >
        <input
          ref={champ}
          autoFocus
          aria-label="Commande"
          aria-expanded="true"
          aria-controls="palette-liste"
          aria-activedescendant={courant >= 0 ? `palette-${visibles[courant].id}` : undefined}
          autoComplete="off"
          placeholder="Aller à une page, un projet, une recherche…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          className="w-full border-b border-bordure bg-transparent px-4 py-3 text-texte outline-none"
        />
        {visibles.length === 0 ? (
          <Vide>Aucune commande.</Vide>
        ) : (
          <ul id="palette-liste" role="listbox" aria-label="Commandes" className="overflow-auto">
            {visibles.map((commande, i) => (
              <li key={commande.id}>
                <button
                  id={`palette-${commande.id}`}
                  type="button"
                  role="option"
                  aria-selected={i === courant}
                  onMouseEnter={() => setIndex(i)}
                  onClick={commande.agir}
                  className={`flex w-full items-baseline justify-between gap-3 px-4 py-2 text-left text-sm ${
                    i === courant ? 'bg-accent-doux text-texte' : 'hover:bg-surface-2'
                  }`}
                >
                  <span>{commande.libelle}</span>
                  <span className="shrink-0 text-xs text-doux">{commande.detail}</span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

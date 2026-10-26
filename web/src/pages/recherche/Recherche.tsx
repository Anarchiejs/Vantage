import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import type { ProjetResume, ResultatRecherche } from '../../api/types'
import { avecParams } from '../../api/client'
import { useApi } from '../../api/useApi'
import { Carte } from '../../ui/Carte'
import { Chargement, ErreurAffichee, Vide } from '../../ui/Etat'
import { date } from '../../ui/format'
import { htmlSurligne } from './surligne'

const EXEMPLES = ['"phrase exacte"', 'go AND sqlite', 'pref*']

function sourceApi(valeur: string): string {
  return valeur === 'note' || valeur === 'commit' ? valeur : ''
}

function cible(resultat: ResultatRecherche, projets: ProjetResume[]): string {
  if (resultat.source === 'note') return resultat.id_note != null ? `/graphe/${resultat.id_note}` : '/graphe'
  const projet = projets.find((p) => p.nom === resultat.projet)
  return projet ? `/projets/${projet.id}` : '/projets'
}

function TexteSurligne({ texte }: { texte: string }) {
  return <span dangerouslySetInnerHTML={{ __html: htmlSurligne(texte) }} />
}

export default function Recherche() {
  const naviguer = useNavigate()
  const [params, setParams] = useSearchParams()
  const qUrl = params.get('q') ?? ''
  const source = params.get('source') ?? ''
  const projet = params.get('projet') ?? ''
  const dossier = params.get('dossier') ?? ''
  const depuis = params.get('depuis') ?? ''
  const jusqua = params.get('jusqua') ?? ''

  const [saisie, setSaisie] = useState(qUrl)
  const ecrit = useRef(qUrl)
  const champ = useRef<HTMLInputElement>(null)
  const [actif, setActif] = useState(0)

  useEffect(() => {
    champ.current?.focus()
  }, [])

  useEffect(() => {
    if (qUrl === ecrit.current) return
    ecrit.current = qUrl
    setSaisie(qUrl)
  }, [qUrl])

  useEffect(() => {
    if (saisie === qUrl) return
    const timer = window.setTimeout(() => {
      ecrit.current = saisie
      setParams(
        (actuel) => {
          const suivant = new URLSearchParams(actuel)
          if (saisie) suivant.set('q', saisie)
          else suivant.delete('q')
          return suivant
        },
        { replace: true },
      )
    }, 150)
    return () => window.clearTimeout(timer)
  }, [saisie, qUrl, setParams])

  function maj(partiel: Record<string, string>) {
    setParams(
      (actuel) => {
        const suivant = new URLSearchParams(actuel)
        for (const [cle, valeur] of Object.entries(partiel)) {
          if (valeur) suivant.set(cle, valeur)
          else suivant.delete(cle)
        }
        return suivant
      },
      { replace: true },
    )
  }

  const chemin = qUrl.trim()
    ? avecParams('/api/recherche', {
        q: qUrl,
        source: sourceApi(source),
        projet,
        dossier,
        depuis,
        jusqua,
      })
    : null
  const etat = useApi<ResultatRecherche[]>(chemin)
  const etatProjets = useApi<ProjetResume[]>('/api/projets')
  const projets = etatProjets.statut === 'ok' ? etatProjets.donnees : []
  const resultats = qUrl.trim() && etat.statut === 'ok' ? etat.donnees : []
  const index = resultats.length === 0 ? -1 : Math.min(actif, resultats.length - 1)

  useEffect(() => {
    setActif(0)
  }, [chemin])

  const nomsProjets = projets.map((p) => p.nom)
  const optionsProjet = projet && !nomsProjets.includes(projet) ? [projet, ...nomsProjets] : nomsProjets

  function ouvrir(resultat: ResultatRecherche) {
    naviguer(cible(resultat, projets))
  }

  function surTouche(e: KeyboardEvent<HTMLInputElement>) {
    if (resultats.length === 0) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActif((i) => Math.min(Math.max(i, 0) + 1, resultats.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActif((i) => Math.max(Math.min(i, resultats.length - 1) - 1, 0))
    } else if (e.key === 'Enter' && index >= 0) {
      e.preventDefault()
      ouvrir(resultats[index])
    }
  }

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Recherche</h1>
      <form role="search" className="space-y-3" onSubmit={(e) => e.preventDefault()}>
        <input
          ref={champ}
          type="search"
          aria-label="Recherche"
          aria-controls="resultats"
          aria-activedescendant={index >= 0 ? `resultat-${index}` : undefined}
          autoComplete="off"
          placeholder="Notes, commits, projets…"
          value={saisie}
          onChange={(e) => setSaisie(e.target.value)}
          onKeyDown={surTouche}
          className="w-full rounded-lg border border-bordure bg-surface px-3 py-2 text-texte outline-none focus:border-accent"
        />
        <div className="grid grid-cols-5 gap-3">
          <label className="flex flex-col gap-1 text-xs text-doux">
            Source
            <select
              aria-label="Source"
              value={sourceApi(source)}
              onChange={(e) => maj({ source: e.target.value })}
              className="rounded-md border border-bordure bg-surface px-2 py-1 text-sm text-texte"
            >
              <option value="">Tout</option>
              <option value="note">Notes</option>
              <option value="commit">Commits</option>
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs text-doux">
            Projet
            <select
              aria-label="Projet"
              value={projet}
              onChange={(e) => maj({ projet: e.target.value })}
              className="rounded-md border border-bordure bg-surface px-2 py-1 text-sm text-texte"
            >
              <option value="">Tous les projets</option>
              {optionsProjet.map((nom) => (
                <option key={nom} value={nom}>
                  {nom}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs text-doux">
            Dossier
            <input
              aria-label="Dossier"
              value={dossier}
              placeholder="Projets, Journal…"
              onChange={(e) => maj({ dossier: e.target.value })}
              className="rounded-md border border-bordure bg-surface px-2 py-1 text-sm text-texte"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-doux">
            Depuis
            <input
              type="date"
              aria-label="Depuis"
              value={depuis}
              onChange={(e) => maj({ depuis: e.target.value })}
              className="rounded-md border border-bordure bg-surface px-2 py-1 text-sm text-texte"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-doux">
            Jusqu'à
            <input
              type="date"
              aria-label="Jusqu'à"
              value={jusqua}
              onChange={(e) => maj({ jusqua: e.target.value })}
              className="rounded-md border border-bordure bg-surface px-2 py-1 text-sm text-texte"
            />
          </label>
        </div>
      </form>

      {!qUrl.trim() && (
        <Carte titre="Syntaxe FTS5">
          <p className="mb-2 text-sm text-doux">
            La recherche porte sur les notes du coffre et les messages de commit. Quelques exemples :
          </p>
          <ul className="flex flex-col gap-1 font-mono text-sm">
            {EXEMPLES.map((exemple) => (
              <li key={exemple}>
                <code>{exemple}</code>
              </li>
            ))}
          </ul>
        </Carte>
      )}

      {qUrl.trim() && etat.statut === 'chargement' && <Chargement lignes={5} />}
      {qUrl.trim() && etat.statut === 'erreur' && <ErreurAffichee erreur={etat.erreur} />}
      {qUrl.trim() && etat.statut === 'ok' && resultats.length === 0 && (
        <Vide>Aucun résultat pour « {qUrl} ».</Vide>
      )}
      {resultats.length > 0 && (
        <div id="resultats" role="listbox" aria-label="Résultats" className="overflow-hidden rounded-xl border border-bordure bg-surface">
          {resultats.map((resultat, i) => (
            <Link
              key={`${resultat.source}-${resultat.ref}-${i}`}
              id={`resultat-${i}`}
              to={cible(resultat, projets)}
              role="option"
              aria-selected={i === index}
              onMouseEnter={() => setActif(i)}
              className={`flex items-start gap-3 border-b border-bordure px-3 py-2 last:border-b-0 ${
                i === index ? 'bg-accent-doux' : 'hover:bg-surface-2'
              }`}
            >
              <span className="sr-only">{resultat.source === 'note' ? 'Note' : 'Commit'}</span>
              <span aria-hidden="true" className="mt-0.5 w-5 text-center font-mono text-xs text-accent">
                {resultat.source === 'note' ? '✦' : '›'}
              </span>
              <span className="min-w-0 flex-1">
                <span className="block font-medium">
                  <TexteSurligne texte={resultat.titre} />
                </span>
                <span className="block truncate text-sm text-doux">
                  <TexteSurligne texte={resultat.extrait} />
                </span>
              </span>
              <span className="shrink-0 text-sm text-doux">{resultat.projet ?? '—'}</span>
              <span className="w-28 shrink-0 text-right text-sm text-doux tabular-nums">{date(resultat.date)}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}

import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import type { EChartsOption } from 'echarts'
import type { Chemin, Graphe as Donnees, NoeudGraphe, NoteDetail } from '../../api/types'
import { useApi, type EtatApi } from '../../api/useApi'
import { avecParams, get } from '../../api/client'
import { Carte } from '../../ui/Carte'
import { Charge, Vide } from '../../ui/Etat'
import { Graphique } from '../../ui/Graphique'
import { date } from '../../ui/format'
import { jeton, series } from '../../theme/theme'
import { Entete } from '../../ui/Entete'

export const tailleNoeud = (degre: number) => Math.max(9, Math.min(44, 9 + Math.sqrt(Math.max(0, degre)) * 4))
export function selectionNote(d: Donnees, id?: string, titre?: string | null) {
  return titre ? d.noeuds.find(n => n.titre === titre) : d.noeuds.find(n => n.id === Number(id))
}

/** Le client statique ignore les paramètres : le mode maquette suit son propre graphe. */
export function cheminMaquette(d: Donnees, de: number, a: number): Chemin {
  const noeuds = new Map(d.noeuds.map(n => [n.id, n]))
  if (!noeuds.has(de) || !noeuds.has(a)) return { chemin: [] }
  const voisins = new Map<number, number[]>()
  for (const l of d.liens) {
    voisins.set(l.source, [...(voisins.get(l.source) ?? []), l.cible])
    voisins.set(l.cible, [...(voisins.get(l.cible) ?? []), l.source])
  }
  const precedents = new Map<number, number | null>([[de, null]])
  const file = [de]
  for (let i = 0; i < file.length && !precedents.has(a); i++) {
    for (const id of voisins.get(file[i]) ?? []) if (!precedents.has(id) && noeuds.has(id)) {
      precedents.set(id, file[i]); file.push(id)
    }
  }
  if (!precedents.has(a)) return { chemin: [] }
  const chemin: NoeudGraphe[] = []
  for (let id: number | null = a; id !== null; id = precedents.get(id) ?? null) chemin.push(noeuds.get(id)!)
  return { chemin: chemin.reverse() }
}

export function optionGraphe(d: Donnees, dossiers: string[], selection?: number, chemin: number[] = []): EChartsOption {
  const categories = [...new Set(d.noeuds.map(n => n.dossier))].sort()
  const couleurs = series()
  const visibles = d.noeuds.filter(n => dossiers.includes(n.dossier))
  const ids = new Set(visibles.map(n => n.id))
  // Le classement de l'API est limité à dix notes ; calculer les quinze sur le graphe complet.
  const centrales = new Set([...d.noeuds].sort((a, b) => b.degre - a.degre || a.id - b.id).slice(0, 15).map(n => n.id))
  const proches = new Set([selection, ...d.liens.flatMap(l => l.source === selection ? [l.cible] : l.cible === selection ? [l.source] : [])])
  const trajet = new Set(chemin)
  const aretes = new Set(chemin.slice(1).flatMap((id, i) => [`${chemin[i]}:${id}`, `${id}:${chemin[i]}`]))
  const grand = d.noeuds.length > 500
  return { animation: !grand, xAxis: { show: false }, yAxis: { show: false }, tooltip: { trigger: 'item', formatter: '{b}', renderMode: 'richText' }, series: [{ type: 'graph', layout: 'force', roam: true, draggable: true, scaleLimit: { min: .2, max: 5 }, label: { color: jeton('texte') },
    animation: !grand, force: { repulsion: grand ? 35 : 140, edgeLength: [35, 90], layoutAnimation: !grand },
    categories: categories.map((name, i) => ({ name, itemStyle: { color: couleurs[i % couleurs.length] } })),
    data: visibles.map(n => ({ id: String(n.id), name: n.titre, category: categories.indexOf(n.dossier), symbolSize: tailleNoeud(n.degre),
      label: { show: centrales.has(n.id) || n.id === selection },
      itemStyle: { opacity: selection !== undefined && !proches.has(n.id) && !trajet.has(n.id) ? 0.15 : 1,
        borderWidth: trajet.has(n.id) || n.id === selection ? 3 : 0, borderColor: jeton('accent') } })),
    links: d.liens.filter(l => ids.has(l.source) && ids.has(l.cible)).map(l => ({ source: String(l.source), target: String(l.cible),
      lineStyle: { color: aretes.has(`${l.source}:${l.cible}`) ? jeton('accent') : jeton('bordure'),
        width: aretes.has(`${l.source}:${l.cible}`) ? 4 : 1, opacity: selection !== undefined && !proches.has(l.source) && !proches.has(l.cible) ? 0.12 : 0.7 } })),
    emphasis: { focus: 'self', label: { show: true } } }] }
}

export function ContenuNote({ contenu, noeuds }: { contenu: string; noeuds: NoeudGraphe[] }) {
  const titres = new Map(noeuds.map(n => [n.titre, n]))
  return <div className="whitespace-pre-wrap break-words text-sm leading-relaxed">{contenu.split(/(\[\[[^\]]+\]\])/g).map((partie, i) => {
    if (!partie.startsWith('[[') || !partie.endsWith(']]')) return <span key={i}>{partie}</span>
    const [cible, alias] = partie.slice(2, -2).split('|')
    const n = titres.get(cible.split('#')[0])
    return n ? <Link key={i} className="text-accent underline" to={`/graphe/${n.id}`}>{alias || cible}</Link> : <span key={i}>{partie}</span>
  })}</div>
}

function Panneau({ id, noeuds }: { id: number; noeuds: NoeudGraphe[] }) {
  const note = useApi<NoteDetail>(`/api/notes/${id}`)
  return <Carte titre="Note sélectionnée"><Charge etat={note}>{n => n.id === id ? <>
    <div className="mb-4 flex justify-between gap-3"><h2 className="font-semibold">{n.titre}</h2><Link aria-label="Fermer la note" to="/graphe">×</Link></div>
    <p className="mb-4 text-xs text-doux">{n.dossier} · {date(n.maj)}</p>
    <ContenuNote contenu={n.contenu} noeuds={noeuds} />
    <h3 className="mt-5 mb-2 font-semibold">Voisins</h3><Liste noeuds={n.voisins} />
  </> : <Vide>Chargement de la note…</Vide>}</Charge></Carte>
}

function Liste({ noeuds }: { noeuds: NoeudGraphe[] }) {
  return noeuds.length ? <ul className="space-y-1 text-sm">{noeuds.map(n => <li key={n.id}><Link className="text-accent hover:underline" to={`/graphe/${n.id}`}>{n.titre}</Link> <span className="text-doux tabular-nums">({n.degre})</span></li>)}</ul> : <Vide>Aucune note.</Vide>
}

function Exploration({ d }: { d: Donnees }) {
  const { id } = useParams()
  const [params] = useSearchParams()
  const naviguer = useNavigate()
  const selection = selectionNote(d, id, params.get('note'))
  const [exclus, setExclus] = useState<string[]>([])
  const [recherche, setRecherche] = useState('')
  const [de, setDe] = useState('')
  const [a, setA] = useState('')
  const [requete, setRequete] = useState<string | null>(null)
  const [invalide, setInvalide] = useState(false)
  const [tour, setTour] = useState(0)
  const [reponse, setReponse] = useState<{ url: string | null; etat: EtatApi<Chemin> }>({ url: null, etat: { statut: 'chargement' } })
  useEffect(() => {
    if (!requete) return
    const ctrl = new AbortController()
    setReponse({ url: requete, etat: { statut: 'chargement' } })
    get<Chemin>(requete, ctrl.signal).then(donnees => {
      if (ctrl.signal.aborted) return
      const params = new URLSearchParams(requete.split('?')[1])
      const chemin = import.meta.env.VITE_MOCK === '1' ? cheminMaquette(d, Number(params.get('de')), Number(params.get('a'))) : donnees
      setReponse({ url: requete, etat: { statut: 'ok', donnees: chemin } })
    }).catch((erreur: Error) => {
      if (!ctrl.signal.aborted) setReponse({ url: requete, etat: { statut: 'erreur', erreur } })
    })
    return () => ctrl.abort()
  }, [requete, tour, d])
  const resultat: EtatApi<Chemin> = reponse.url === requete ? reponse.etat : { statut: 'chargement' }
  const dossiers = useMemo(() => [...new Set(d.noeuds.map(n => n.dossier))].sort(), [d])
  const trajet = resultat.statut === 'ok' && requete ? resultat.donnees.chemin : undefined
  const selectionId = selection?.id
  const option = useCallback(() => optionGraphe(d, dossiers.filter(x => !exclus.includes(x)), selectionId, trajet?.map(n => n.id)), [d, dossiers, exclus, selectionId, trajet])
  const trouver = (titre: string) => d.noeuds.find(n => n.titre === titre)
  return <>
    <Carte titre="Explorer le coffre">
      <form className="mb-4 flex gap-2" onSubmit={e => { e.preventDefault(); const n = trouver(recherche); if (n) naviguer(`/graphe/${n.id}`) }}>
        <input className="flex-1 rounded border border-bordure bg-surface-2 p-2" aria-label="Rechercher un nœud par titre" list="titres-graphe" value={recherche} onChange={e => setRecherche(e.target.value)} placeholder="Titre d’une note…" />
        <button className="rounded border border-bordure px-3" disabled={!trouver(recherche)}>Ouvrir</button>
      </form>
      <datalist id="titres-graphe">{d.noeuds.map(n => <option key={n.id} value={n.titre} />)}</datalist>
      <fieldset className="flex flex-wrap gap-4 text-sm"><legend className="mb-2 text-doux">Dossiers</legend>{dossiers.map(dossier => <label className="flex items-center gap-2" key={dossier}>
        <input type="checkbox" checked={!exclus.includes(dossier)} onChange={e => setExclus(v => e.target.checked ? v.filter(x => x !== dossier) : [...v, dossier])} />{dossier}</label>)}</fieldset>
    </Carte>
    {(id || params.has('note')) && !selection && <p role="status" className="text-doux">Note introuvable.</p>}
    <div className={`grid gap-4 ${selection ? 'grid-cols-[minmax(0,2fr)_minmax(280px,1fr)]' : 'grid-cols-1'}`}>
      <Carte titre={`${d.noeuds.length} notes · ${d.liens.length} liens`}>
        {d.noeuds.length ? dossiers.every(x => exclus.includes(x)) ? <Vide>Aucun dossier sélectionné.</Vide> : <Graphique hauteur={560} aria-label="Graphe des liens du coffre" option={option} surClic={p => {
          const n = p.data as { id?: string }
          if (p.dataType === 'node' && n.id) naviguer(`/graphe/${n.id}`)
        }} /> : <Vide>Le coffre ne contient aucune note.</Vide>}
      </Carte>
      {selection && <Panneau key={selection.id} id={selection.id} noeuds={d.noeuds} />}
    </div>
    <Carte titre="Plus court chemin">
      <form className="flex flex-wrap items-end gap-3" onSubmit={e => { e.preventDefault(); const depart = trouver(de); const arrivee = trouver(a); setInvalide(!depart || !arrivee); if (depart && arrivee) {
        const url = avecParams('/api/graphe/chemin', { de: depart.id, a: arrivee.id }); setRequete(url); if (url === requete) setTour(v => v + 1)
      } }}>
        <label className="flex flex-col gap-1 text-sm">De<input className="rounded border border-bordure bg-surface-2 p-2" list="titres-graphe" value={de} onChange={e => { setDe(e.target.value); setRequete(null) }} /></label>
        <label className="flex flex-col gap-1 text-sm">À<input className="rounded border border-bordure bg-surface-2 p-2" list="titres-graphe" value={a} onChange={e => { setA(e.target.value); setRequete(null) }} /></label>
        <button className="rounded bg-accent-doux px-4 py-2">Chercher le chemin</button>
      </form>
      {invalide && <p role="alert">Choisissez deux titres de notes existantes.</p>}
      {requete && <Charge etat={resultat}>{r => r.chemin.length ? <nav aria-label="Plus court chemin" className="mt-4 flex flex-wrap items-center gap-2">{r.chemin.map((n, i) => <span key={n.id}>{i > 0 && <span aria-hidden="true"> → </span>}<Link className="text-accent underline" to={`/graphe/${n.id}`}>{n.titre}</Link></span>)}</nav> : <p role="status" className="mt-4 text-doux">Aucun chemin.</p>}</Charge>}
    </Carte>
    <div className="grid grid-cols-3 gap-4">
      <Carte titre="Notes orphelines"><Liste noeuds={d.orphelines} /></Carte>
      <Carte titre="10 notes les plus centrales"><Liste noeuds={d.centrales.slice(0, 10)} /></Carte>
      <Carte titre="Liens cassés">{d.casses.length ? <ul className="space-y-2 text-sm">{d.casses.map((l, i) => <li key={i}><Link className="text-accent" to={`/graphe/${l.source}`}>{l.source_titre}</Link> → <span className="text-danger">{l.cible_titre}</span></li>)}</ul> : <Vide>Aucun lien cassé.</Vide>}</Carte>
    </div>
  </>
}

export default function Graphe() {
  const graphe = useApi<Donnees>('/api/graphe')
  return <div className="space-y-5"><Entete titre="Graphe du cerveau" description="Explorez les connexions entre vos notes. Déplacez les nœuds, zoomez et ouvrez une note pour suivre ses liens." /><Charge etat={graphe}>{d => <Exploration d={d} />}</Charge></div>
}

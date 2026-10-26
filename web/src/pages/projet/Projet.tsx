import { useState } from 'react'
import { Link, useParams } from 'react-router'
import type { EvenementFrise, ProjetDetail, Sante, TypeEvenement } from '../../api/types'
import { useApi } from '../../api/useApi'
import { Carte } from '../../ui/Carte'
import { Charge, Vide } from '../../ui/Etat'
import { Graphique } from '../../ui/Graphique'
import { date, nombre, shaCourt } from '../../ui/format'
import { optionEvolution, optionLangages, optionVolume } from './graphiques'

export function SanteFiche({ sante }: { sante: Sante }) {
  return <Carte titre="Santé de la fiche"><p className={`mb-3 font-semibold ${sante.a_jour ? 'text-succes' : 'text-alerte'}`}>{sante.a_jour ? 'À jour' : 'Périmée'}</p>
    <dl className="grid grid-cols-2 gap-2 text-sm"><dt className="text-doux">SHA vérifié</dt><dd className="font-mono" title={sante.verifie_commit ?? undefined}>{shaCourt(sante.verifie_commit)}</dd><dt className="text-doux">Dernier SHA</dt><dd className="font-mono" title={sante.dernier_sha ?? undefined}>{shaCourt(sante.dernier_sha)}</dd><dt className="text-doux">Vérifiée le</dt><dd>{date(sante.verifie_le)}</dd><dt className="text-doux">Commits depuis</dt><dd>{sante.commits_depuis < 0 ? 'Référence inconnue du dépôt' : nombre(sante.commits_depuis)}</dd></dl>
    {sante.notes_manquantes.length ? <div className="mt-3 text-sm text-alerte">Notes manquantes :<ul className="list-inside list-disc">{sante.notes_manquantes.map(n => <li key={n}>{n}</li>)}</ul></div> : <p className="mt-3 text-xs text-doux">Aucune note manquante.</p>}
  </Carte>
}
const libelles: Record<TypeEvenement, string> = { commit: 'Commits', session: 'Sessions', journal: 'Journal' }
const pastilles: Record<TypeEvenement, string> = { commit: 'bg-accent', session: 'bg-succes', journal: 'bg-alerte' }
export function filtrerFrise(frise: EvenementFrise[], types: TypeEvenement[]) {
  return [...frise].sort((a, b) => b.date.localeCompare(a.date)).slice(0, 500).filter(e => types.includes(e.type))
}
export function Frise({ evenements }: { evenements: EvenementFrise[] }) {
  const [types, setTypes] = useState<TypeEvenement[]>(['commit', 'session', 'journal'])
  const [limite, setLimite] = useState(50)
  const filtres = filtrerFrise(evenements, types)
  const groupes = new Map<string, EvenementFrise[]>()
  for (const e of filtres.slice(0, limite)) { const mois = e.date.slice(0, 7); groupes.set(mois, [...(groupes.get(mois) ?? []), e]) }
  return <Carte titre="Frise chronologique"><div className="mb-4 flex gap-4" role="group" aria-label="Types d’événements">{(Object.keys(libelles) as TypeEvenement[]).map(t => <label key={t} className="flex items-center gap-2 text-sm"><input type="checkbox" checked={types.includes(t)} onChange={() => { setTypes(types.includes(t) ? types.filter(x => x !== t) : [...types, t]); setLimite(50) }} />{libelles[t]}</label>)}</div>
    <p className="mb-3 text-xs text-doux">{nombre(filtres.length)} événements · 500 événements récents au maximum</p>
    {filtres.length ? [...groupes].map(([mois, lignes]) => <section key={mois} className="mb-4"><h3 className="mb-3 text-sm font-semibold">{new Intl.DateTimeFormat('fr-FR', { month: 'long', year: 'numeric' }).format(new Date(`${mois}-01T12:00:00`))}</h3><ol className="ml-2 border-l border-bordure">{lignes.map((e, i) => <li key={`${e.type}-${e.ref}-${i}`} className="relative mb-3 pl-5"><span aria-hidden="true" className={`absolute -left-1 top-2 h-2 w-2 rounded-full ${pastilles[e.type]}`} /><div className="flex flex-wrap gap-2 text-xs text-doux"><time dateTime={e.date}>{date(e.date)}</time><span>{libelles[e.type]}</span>{e.ia && <span className="text-accent">{e.ia}</span>}</div><p className="text-sm break-words">{e.titre}</p><p className="truncate font-mono text-xs text-doux" title={e.ref}>{e.type === 'commit' ? shaCourt(e.ref) : e.ref}</p></li>)}</ol></section>) : <Vide>Aucun événement pour ces filtres.</Vide>}
    {filtres.length > limite && <button type="button" onClick={() => setLimite(n => n + 50)} className="rounded border border-bordure px-3 py-2 text-sm text-accent">Afficher plus</button>}
  </Carte>
}
function ContenuProjet({ d }: { d: ProjetDetail }) {
  const p = d.projet
  return <>
    <header className="space-y-2"><div className="flex items-center gap-3"><h1 className="text-xl font-semibold">{p.nom}</h1><span className="rounded bg-surface-2 px-2 py-1 text-xs">{p.statut || 'Statut inconnu'}</span></div><p className="font-mono text-xs text-doux break-all">{p.chemin_source || 'Chemin source non renseigné'}</p><div className="flex flex-wrap gap-2">{p.technos.map(t => <span key={t} className="rounded bg-accent-doux px-2 py-1 text-xs text-accent">{t}</span>)}</div><Link className="inline-block text-sm text-accent hover:underline" to={`/graphe?note=${encodeURIComponent(p.nom)}`}>Voir dans le graphe →</Link></header>
    <div className="grid grid-cols-[minmax(0,2fr)_minmax(280px,1fr)] gap-4"><Frise key={p.id} evenements={d.frise} /><div className="space-y-4"><SanteFiche sante={d.sante} /><Carte titre="IA du projet">{d.ia.length ? <ul className="divide-y divide-bordure">{d.ia.map(a => <li key={a.ia} className="py-3"><p className="flex justify-between text-sm"><span>{a.ia}</span><span className="tabular-nums">{nombre(a.sessions)} sessions</span></p><p className="text-xs text-doux">Du {date(a.premiere)} au {date(a.derniere)}</p></li>)}</ul> : <Vide>Aucune session IA enregistrée.</Vide>}</Carte></div></div>
    <Carte titre="Volume de code par semaine">{d.volume.length ? <Graphique aria-label="Lignes ajoutées et supprimées par semaine" option={() => optionVolume(d.volume)} /> : <Vide>Aucun volume de code enregistré.</Vide>}</Carte>
    <div className="grid grid-cols-2 gap-4"><Carte titre="Langages — dernière mesure">{d.langages.length ? <Graphique aria-label="Répartition des lignes de code par langage" option={() => optionLangages(d.langages)} /> : <Vide>Aucune mesure de langage.</Vide>}</Carte><Carte titre="Évolution des langages">{new Set(d.langages_evolution.map(l => l.date_mesure)).size > 1 ? <Graphique aria-label="Évolution des lignes de code par langage" option={() => optionEvolution(d.langages_evolution)} /> : <Vide>Deux mesures sont nécessaires pour afficher l’évolution.</Vide>}</Carte></div>
    <Carte titre="Points chauds — 20 fichiers">{d.points_chauds.length ? <table className="w-full text-left text-sm"><thead><tr>{['Fichier', 'Modifications', 'Ajouts', 'Suppressions'].map(t => <th className="border-b border-bordure p-2" key={t}>{t}</th>)}</tr></thead><tbody>{d.points_chauds.slice(0, 20).map(f => <tr key={f.chemin} className="border-b border-bordure"><td className="p-2 font-mono text-xs break-all">{f.chemin}</td><td className="p-2 tabular-nums">{nombre(f.modifications)}</td><td className="p-2 text-succes tabular-nums">+{nombre(f.ajouts)}</td><td className="p-2 text-danger tabular-nums">−{nombre(f.suppressions)}</td></tr>)}</tbody></table> : <Vide>Aucun fichier modifié.</Vide>}</Carte>
  </>
}
export default function Projet() {
  const { id } = useParams()
  const etat = useApi<ProjetDetail>(`/api/projets/${encodeURIComponent(id ?? '')}`)
  return <div className="space-y-5"><Link to="/projets" className="text-sm text-accent hover:underline">← Tous les projets</Link><Charge etat={etat}>{d => <ContenuProjet d={d} />}</Charge></div>
}

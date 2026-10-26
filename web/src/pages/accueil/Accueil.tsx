import { useState } from 'react'
import { Link } from 'react-router'
import type { Accueil as Donnees, Chiffres } from '../../api/types'
import { useApi } from '../../api/useApi'
import { Carte } from '../../ui/Carte'
import { Charge, Vide } from '../../ui/Etat'
import { Graphique } from '../../ui/Graphique'
import { Stat } from '../../ui/Stat'
import { depuis, nombre, shaCourt } from '../../ui/format'
import { optionCalendrier } from './calendrier'
import './accueil.css'
export { optionCalendrier } from './calendrier'

const indicateurs: [keyof Chiffres, string][] = [['commits', 'Commits'], ['ajouts', 'Lignes ajoutées'], ['suppressions', 'Lignes supprimées'], ['sessions', 'Sessions'], ['projets_actifs', 'Projets actifs']]
export default function Accueil() {
  const etat = useApi<Donnees>('/api/accueil')
  const [periode, setPeriode] = useState<'semaine' | 'mois'>('semaine')
  return <div className="space-y-5"><h1 className="text-xl font-semibold">Accueil</h1><Charge etat={etat}>{d => <>
    <Carte titre="Activité sur 12 mois" className="min-w-0">{d.calendrier.length ? <Graphique className="min-w-0" hauteur={240} aria-label="Calendrier des commits sur 365 jours" option={() => optionCalendrier(d.calendrier)} /> : <Vide>Aucune activité enregistrée.</Vide>}</Carte>
    <Carte titre="Chiffres clés" action={<div className="flex gap-2" role="group" aria-label="Période">{(['semaine', 'mois'] as const).map(p => <button key={p} type="button" aria-pressed={periode === p} onClick={() => setPeriode(p)} className={`rounded px-3 py-1 text-sm ${periode === p ? 'bg-accent-doux text-accent' : 'bg-surface-2 text-doux'}`}>{p === 'semaine' ? 'Semaine' : 'Mois'}</button>)}</div>}>
      <p className="mb-4 text-xs text-doux">{periode === 'semaine' ? '7 derniers jours / 7 jours précédents' : '30 derniers jours / 30 jours précédents'}</p>
      <div className="chiffres-accueil grid grid-cols-5 gap-4">{indicateurs.map(([k, label]) => <Stat key={k} libelle={label} valeur={d[periode][k]} avant={d[periode === 'semaine' ? 'semaine_precedente' : 'mois_precedent'][k]} />)}</div>
    </Carte>
    <h2 className="font-semibold">En ce moment</h2><div className="grid grid-cols-3 gap-4">
      <Carte titre="Projets actifs">{d.actifs.length ? <ul className="divide-y divide-bordure">{d.actifs.map(p => <li className="py-3" key={p.id}><Link className="text-accent hover:underline" to={`/projets/${p.id}`}>{p.nom}</Link><p className="text-xs text-doux">{nombre(p.commits_30j)} commits sur 30 j · {depuis(p.dernier_commit)}</p></li>)}</ul> : <Vide>Aucun projet actif.</Vide>}</Carte>
      <Carte titre="Projets qui dorment">{d.dormants.length ? <ul className="divide-y divide-bordure">{d.dormants.map(p => <li className="py-3" key={p.id}><Link className="text-accent hover:underline" to={`/projets/${p.id}`}>{p.nom}</Link><p className="text-xs text-doux">{p.dernier_commit ? `${nombre(p.jours)} jours sans commit` : 'Aucun commit'}</p></li>)}</ul> : <Vide>Aucun projet dormant.</Vide>}</Carte>
      <Carte titre="Fiches périmées">{d.fiches_perimees.length ? <ul className="divide-y divide-bordure">{d.fiches_perimees.map(p => <li className="py-3" key={p.id}><Link className="text-accent hover:underline" to={`/projets/${p.id}`}>{p.nom}</Link><p className="font-mono text-xs text-doux">{shaCourt(p.verifie_commit)} → {shaCourt(p.dernier_sha)}</p><p className="text-xs text-alerte">{p.commits_depuis < 0 ? 'Référence inconnue du dépôt' : `${nombre(p.commits_depuis)} commits depuis la vérification`}</p></li>)}</ul> : <Vide>Aucune fiche périmée.</Vide>}</Carte>
    </div>
  </>}</Charge></div>
}

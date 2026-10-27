import { useState } from 'react'
import { Link } from 'react-router'
import type { Accueil as Donnees, Chiffres } from '../../api/types'
import { useApi } from '../../api/useApi'
import { Carte } from '../../ui/Carte'
import { Charge, Vide } from '../../ui/Etat'
import { Graphique } from '../../ui/Graphique'
import { Stat } from '../../ui/Stat'
import { date, depuis, nombre, shaCourt } from '../../ui/format'
import { Entete } from '../../ui/Entete'
import { optionActivite } from './activite'
import { optionCalendrier } from './calendrier'
import './accueil.css'
export { optionCalendrier } from './calendrier'

const indicateurs: [keyof Chiffres, string][] = [['commits', 'Commits'], ['ajouts', 'Lignes ajoutées'], ['suppressions', 'Lignes supprimées'], ['sessions', 'Sessions'], ['projets_actifs', 'Projets actifs']]
export default function Accueil() {
  const etat = useApi<Donnees>('/api/accueil')
  const [periode, setPeriode] = useState<'semaine' | 'mois'>('semaine')
  const [duree, setDuree] = useState(90)
  const [mesure, setMesure] = useState<'commits' | 'sessions'>('commits')
  return <div className="space-y-5"><Entete titre="Votre travail prend forme." description="Une vue d’ensemble de vos projets, de vos idées et de leur progression." action={<span className="date-chip">{date(new Date().toLocaleDateString('sv-SE'))}</span>} /><Charge etat={etat}>{d => <>
    <Carte titre="Chiffres clés" action={<div className="segmented" role="group" aria-label="Période">{(['semaine', 'mois'] as const).map(p => <button key={p} type="button" aria-pressed={periode === p} onClick={() => setPeriode(p)}>{p === 'semaine' ? 'Semaine' : 'Mois'}</button>)}</div>}>
      <p className="mb-4 text-xs text-doux">{periode === 'semaine' ? '7 derniers jours / 7 jours précédents' : '30 derniers jours / 30 jours précédents'}</p>
      <div className="chiffres-accueil grid grid-cols-5 gap-4">{indicateurs.map(([k, label]) => <Stat key={k} libelle={label} valeur={d[periode][k]} avant={d[periode === 'semaine' ? 'semaine_precedente' : 'mois_precedent'][k]} />)}</div>
    </Carte>
    <Carte titre="Le rythme de vos projets" action={<div className="flex flex-wrap gap-3"><select aria-label="Mesure de l’activité" value={mesure} onChange={e => setMesure(e.target.value as 'commits' | 'sessions')} className="border border-bordure bg-surface px-2 text-xs"><option value="commits">Commits</option><option value="sessions">Sessions</option></select><div className="segmented" role="group" aria-label="Plage du graphique">{[30, 90, 365].map(n => <button key={n} aria-pressed={duree === n} onClick={() => setDuree(n)}>{n === 365 ? '1 an' : `${n} jours`}</button>)}</div></div>}>
      {d.calendrier.length ? <><Graphique hauteur={270} aria-label={`${mesure === 'commits' ? 'Commits' : 'Sessions'} par jour sur ${duree} jours`} option={() => optionActivite(d.calendrier.slice(-duree), mesure)} /><p className="chart-footnote">Du {date(d.calendrier.slice(-duree)[0]?.jour)} au {date(d.calendrier.at(-1)?.jour)} · {nombre(d.calendrier.slice(-duree).reduce((s, j) => s + j[mesure], 0))} {mesure}. Jours sans activité inclus.</p>
      <details className="data-details"><summary>Consulter les valeurs par date</summary><div><table><thead><tr><th>Date</th><th>Commits</th><th>Sessions</th><th>Ajouts</th><th>Suppressions</th></tr></thead><tbody>{d.calendrier.slice(-duree).slice().reverse().map(j => <tr key={j.jour}><td>{date(j.jour)}</td><td>{nombre(j.commits)}</td><td>{nombre(j.sessions)}</td><td>{nombre(j.ajouts)}</td><td>{nombre(j.suppressions)}</td></tr>)}</tbody></table></div></details></> : <Vide>Aucune donnée pour cette période.</Vide>}
    </Carte>
    <Carte titre="Activité sur 12 mois" className="min-w-0">{d.calendrier.length ? <><div className="chart-scroll"><div className="chart-wide"><Graphique hauteur={210} aria-label="Calendrier des commits sur 365 jours" option={() => optionCalendrier(d.calendrier)} /></div></div><p className="chart-footnote">Historique Git importé, y compris les commits amont des dépôts clonés. Chaque case représente un jour. Survolez-la pour consulter les commits, les sessions et les projets concernés.</p></> : <Vide>Aucune activité enregistrée.</Vide>}</Carte>
    <h2 className="font-semibold">En ce moment</h2><div className="current-projects grid grid-cols-3 items-start gap-4">
      <Carte titre="Projets actifs">{d.actifs.length ? <ul className="divide-y divide-bordure">{d.actifs.map(p => <li className="py-3" key={p.id}><Link className="text-accent hover:underline" to={`/projets/${p.id}`}>{p.nom}</Link><p className="text-xs text-doux">{nombre(p.commits_30j)} commits sur 30 j · {depuis(p.dernier_commit)}</p></li>)}</ul> : <Vide>Aucun projet actif.</Vide>}</Carte>
      <Carte titre="Projets qui dorment">{d.dormants.length ? <ul className="divide-y divide-bordure">{d.dormants.map(p => <li className="py-3" key={p.id}><Link className="text-accent hover:underline" to={`/projets/${p.id}`}>{p.nom}</Link><p className="text-xs text-doux">{p.dernier_commit ? `${nombre(p.jours)} jours sans commit` : 'Aucun commit'}</p></li>)}</ul> : <Vide>Aucun projet dormant.</Vide>}</Carte>
      <Carte titre="Fiches périmées">{d.fiches_perimees.length ? <ul className="divide-y divide-bordure">{d.fiches_perimees.map(p => <li className="py-3" key={p.id}><Link className="text-accent hover:underline" to={`/projets/${p.id}`}>{p.nom}</Link><p className="font-mono text-xs text-doux">{shaCourt(p.verifie_commit)} → {shaCourt(p.dernier_sha)}</p><p className="text-xs text-alerte">{p.commits_depuis < 0 ? 'Référence inconnue du dépôt' : `${nombre(p.commits_depuis)} commits depuis la vérification`}</p></li>)}</ul> : <Vide>Aucune fiche périmée.</Vide>}</Carte>
    </div>
  </>}</Charge></div>
}

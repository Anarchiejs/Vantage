import { Link, useNavigate } from 'react-router'
import type { EChartsOption } from 'echarts'
import type { IA, ProjetResume, Technos as Donnees, TendanceTechno } from '../../api/types'
import { useApi } from '../../api/useApi'
import { Carte } from '../../ui/Carte'
import { Charge, Vide } from '../../ui/Etat'
import { Graphique } from '../../ui/Graphique'
import { series } from '../../theme/theme'
import { nombre } from '../../ui/format'

export function hauteurSankey(d: Donnees): number {
  const projets = d.sankey.noeuds.filter(n => n.type === 'projet').length
  const technos = d.sankey.noeuds.length - projets
  return Math.max(360, Math.max(projets, technos) * 22)
}

export function optionSankey(d: Donnees): EChartsOption {
  const couleurs = series()
  return { xAxis: { show: false }, yAxis: { show: false }, tooltip: { trigger: 'item', renderMode: 'richText' }, series: [{ type: 'sankey', left: 12, right: 130, top: 12, bottom: 12, nodeGap: 14, label: { fontSize: 12 },
    data: d.sankey.noeuds.map((n, i) => ({ name: `${n.type}:${n.nom}`, nom: n.nom, typeNoeud: n.type,
      depth: n.type === 'projet' ? 0 : 1, itemStyle: { color: couleurs[i % couleurs.length] }, label: { formatter: n.nom } })),
    links: d.sankey.liens.map(l => ({ source: `projet:${l.source}`, target: `techno:${l.cible}`, value: l.valeur })),
    lineStyle: { color: 'source', opacity: 0.3 }, emphasis: { focus: 'adjacency' } }] }
}

/** Douze mois calendaires ; les mois absents comptent pour zéro. */
export function moisTendance(points: TendanceTechno[]): string[] {
  const dernier = points.map(p => p.mois).sort().at(-1)
  if (!dernier) return []
  const [an, mois] = dernier.split('-').map(Number)
  return Array.from({ length: 12 }, (_, i) => new Date(Date.UTC(an, mois - 12 + i, 1)).toISOString().slice(0, 7))
}

export function tendances(points: TendanceTechno[]) {
  const mois = moisTendance(points)
  const valeursParTechno = new Map<string, Map<string, number>>()
  for (const p of points) {
    if (!valeursParTechno.has(p.techno)) valeursParTechno.set(p.techno, new Map())
    const valeurs = valeursParTechno.get(p.techno)!
    valeurs.set(p.mois, (valeurs.get(p.mois) ?? 0) + p.commits)
  }
  return [...valeursParTechno].map(([techno, points]) => {
    const valeurs = mois.map(m => points.get(m) ?? 0)
    const moyenne = (v: number[]) => v.reduce((s, n) => s + n, 0) / 3
    return { techno, valeurs, total: valeurs.reduce((s, n) => s + n, 0), pente: moyenne(valeurs.slice(-3)) - moyenne(valeurs.slice(-6, -3)) }
  }).sort((a, b) => b.total - a.total || a.techno.localeCompare(b.techno))
}

export function optionTendances(d: Donnees): EChartsOption {
  const lignes = tendances(d.tendance)
  return { color: series(), tooltip: { trigger: 'axis' }, legend: { type: 'scroll', selected: Object.fromEntries(lignes.map((l, i) => [l.techno, i < 8])) },
    grid: { top: 50, left: 50, right: 20, bottom: 30 }, xAxis: { type: 'category', data: moisTendance(d.tendance) }, yAxis: { type: 'value', name: 'Commits' },
    series: lignes.map(l => ({ type: 'line', name: l.techno, data: l.valeurs, showSymbol: false })) }
}

export function optionIA(ias: IA[]): EChartsOption {
  const mois = [...new Set(ias.flatMap(ia => ia.par_mois.map(p => p.mois)))].sort()
  return { color: series(), tooltip: { trigger: 'axis' }, legend: { type: 'scroll' }, grid: { top: 50, left: 50, right: 20, bottom: 30 },
    xAxis: { type: 'category', data: mois }, yAxis: { type: 'value', minInterval: 1 },
    series: ias.map(ia => ({ name: ia.ia, type: 'bar', stack: 'sessions', data: mois.map(m => ia.par_mois.find(p => p.mois === m)?.sessions ?? 0) })) }
}

export default function Technos() {
  const technos = useApi<Donnees>('/api/technos')
  const ias = useApi<IA[]>('/api/ia')
  const projets = useApi<ProjetResume[]>('/api/projets')
  const naviguer = useNavigate()
  const projet = (nom: string) => projets.donnees?.find(p => p.nom === nom)
  return <div className="space-y-5">
    <h1 className="text-2xl font-semibold">Technos & IA</h1>
    <Charge etat={technos}>{d => <>
      <Carte titre="Projets → technos">{d.sankey.noeuds.length ? <Graphique aria-label="Relations entre projets et technologies" hauteur={hauteurSankey(d)} option={() => optionSankey(d)} surClic={p => {
        const n = p.data as { nom?: string; typeNoeud?: string }
        const cible = n?.typeNoeud === 'projet' && n.nom ? projet(n.nom) : undefined
        if (cible) naviguer(`/projets/${cible.id}`)
      }} /> : <Vide>Aucune relation entre projet et techno.</Vide>}</Carte>
      <Carte titre="Évolution sur 12 mois">{d.tendance.length ? <Graphique aria-label="Commits par technologie sur douze mois" option={() => optionTendances(d)} hauteur={300} /> : <Vide>Aucune activité technologique.</Vide>}
        <div className="grid grid-cols-2 gap-4 text-sm">{[true, false].map(hausse => <div key={String(hausse)}><h3 className={hausse ? 'text-succes' : 'text-danger'}>{hausse ? 'En hausse' : 'En baisse'}</h3>
          <p className="mb-2 text-xs text-doux">Moyenne des 3 derniers mois − 3 précédents</p>
          <ul>{tendances(d.tendance).filter(t => hausse ? t.pente > 0 : t.pente < 0).map(t => <li key={t.techno}>{t.techno} <span className="tabular-nums text-doux">{t.pente > 0 ? '+' : ''}{nombre(Math.round(t.pente * 10) / 10)} commits/mois</span></li>)}</ul>
          {!tendances(d.tendance).some(t => hausse ? t.pente > 0 : t.pente < 0) && <p className="text-doux">Aucune techno {hausse ? 'en hausse' : 'en baisse'}.</p>}</div>)}</div>
      </Carte>
      <p className="text-sm text-doux">Technos sans projet : {d.sans_projet.join(', ') || 'aucune'}.</p>
    </>}</Charge>
    <Charge etat={ias}>{d => d.length ? <>
      <div className="grid grid-cols-3 gap-4">{d.map(ia => <Carte key={ia.ia} titre={ia.ia}><p className="mb-3 text-xl tabular-nums">{nombre(ia.sessions)} <span className="text-sm text-doux">sessions</span></p>
        <div className="flex flex-wrap gap-2">{ia.projets.map(n => <span className="rounded bg-accent-doux px-2 py-1 text-xs" key={n}>{projet(n) ? <Link to={`/projets/${projet(n)!.id}`}>{n}</Link> : n}</span>)}</div>
        {!ia.projets.length && <p className="text-sm text-doux">Aucun projet lié.</p>}</Carte>)}</div>
      <Carte titre="Sessions par mois et par IA"><Graphique aria-label="Sessions mensuelles par intelligence artificielle" option={() => optionIA(d)} hauteur={300} /></Carte>
    </> : <Vide>Aucune session IA.</Vide>}</Charge>
  </div>
}

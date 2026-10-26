import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import type { ProjetResume } from '../../api/types'
import { useApi } from '../../api/useApi'
import { Carte } from '../../ui/Carte'
import { Charge, Vide } from '../../ui/Etat'
import { date, nombre } from '../../ui/format'
export type Colonne = 'nom' | 'statut' | 'commits' | 'dernier_commit' | 'langage_principal'
export function trierProjets(projets: ProjetResume[], colonne: Colonne, desc = false): ProjetResume[] {
  return [...projets].sort((a, b) => {
    const x = a[colonne], y = b[colonne]
    if (x === null) return y === null ? a.id - b.id : 1
    if (y === null) return -1
    const resultat = typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y), 'fr', { sensitivity: 'base' })
    return (desc ? -resultat : resultat) || a.id - b.id
  })
}
const colonnes: [Colonne, string][] = [['nom', 'Nom'], ['statut', 'Statut'], ['commits', 'Commits'], ['dernier_commit', 'Dernier commit'], ['langage_principal', 'Langage principal']]
export default function Projets() {
  const etat = useApi<ProjetResume[]>('/api/projets')
  const [filtre, setFiltre] = useState('')
  const [tri, setTri] = useState<Colonne>('nom')
  const [desc, setDesc] = useState(false)
  const naviguer = useNavigate()
  return <div className="space-y-5"><h1 className="text-xl font-semibold">Projets</h1><Carte>
    <label className="mb-4 block text-sm">Filtrer les projets<input className="ml-3 rounded border border-bordure bg-fond px-3 py-2" type="search" value={filtre} onChange={e => setFiltre(e.target.value)} placeholder="Nom, statut, langage…" /></label>
    <Charge etat={etat}>{projets => {
      const lignes = trierProjets(projets.filter(p => [p.nom, p.statut, p.langage_principal].some(v => v?.toLocaleLowerCase('fr').includes(filtre.trim().toLocaleLowerCase('fr')))), tri, desc)
      return lignes.length ? <table className="w-full text-left text-sm"><thead><tr>{colonnes.map(([k, label]) => <th key={k} className="border-b border-bordure p-3" aria-sort={tri === k ? desc ? 'descending' : 'ascending' : 'none'}><button type="button" onClick={() => { setTri(k); setDesc(tri === k ? !desc : false) }}>{label}{tri === k ? desc ? ' ↓' : ' ↑' : ''}</button></th>)}</tr></thead><tbody>{lignes.map(p => <tr key={p.id} className="cursor-pointer border-b border-bordure hover:bg-surface-2" onClick={() => naviguer(`/projets/${p.id}`)}><td className="p-3"><Link className="text-accent hover:underline" to={`/projets/${p.id}`} onClick={e => e.stopPropagation()}>{p.nom}</Link></td><td className="p-3">{p.statut || '—'}</td><td className="p-3 tabular-nums">{nombre(p.commits)}</td><td className="p-3">{date(p.dernier_commit)}</td><td className="p-3">{p.langage_principal || '—'}</td></tr>)}</tbody></table> : <Vide>{projets.length ? 'Aucun projet ne correspond au filtre.' : 'Aucun projet enregistré.'}</Vide>
    }}</Charge>
  </Carte></div>
}

import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { EChartsOption } from 'echarts'
import type { IA, Technos as Donnees } from '../../api/types'
import Technos, { hauteurSankey, moisTendance, optionIA, optionSankey, optionTendances, tendances } from './Technos'

vi.mock('../../ui/Graphique', () => ({ Graphique: ({ hauteur, surClic, 'aria-label': label }: { hauteur?: number; surClic?: (p: unknown) => void; 'aria-label': string }) => <div role="img" aria-label={label} style={{ height: hauteur }}>
  {surClic && <><button onClick={() => surClic({ data: { nom: 'Observatoire', typeNoeud: 'projet' } })}>Projet connu</button><button onClick={() => surClic({ data: { nom: 'Inconnu', typeNoeud: 'projet' } })}>Projet inconnu</button></>}
</div> }))
const fichiers = import.meta.glob<string>('../../../public/mocks/**/*.json', { query: '?raw', import: 'default', eager: true })
const lire = (f: string) => JSON.parse(fichiers[`../../../public/mocks/${f}.json`].replace(/^\uFEFF/, ''))
const donnees: Donnees = lire('technos')
const ias: IA[] = lire('ia')
const seriesOption = (option: EChartsOption) => option.series as Array<{ data: unknown[]; links: Array<{ source: string; target: string; value: number }>; stack: string }>

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async (url: string) => ({ ok: true, json: async () => lire(url.replace('/api/', '').split('?')[0]) })))
})
afterEach(() => { cleanup(); vi.unstubAllGlobals() })
function afficher() { render(<MemoryRouter initialEntries={['/technos']}><Routes><Route path="/technos" element={<Technos />} /><Route path="/projets/:id" element={<p>Page projet</p>} /></Routes></MemoryRouter>) }

describe('Technos & IA', () => {
  it('dimensionne la plus grande colonne à 22 px par nœud, avec un minimum de 360 px', () => {
    expect(hauteurSankey({ sankey: { noeuds: [], liens: [] }, tendance: [], sans_projet: [] })).toBe(360)
    expect(hauteurSankey({ ...donnees, sankey: { noeuds: donnees.sankey.noeuds.slice(0, 2), liens: [] } })).toBe(360)
    expect(hauteurSankey(donnees)).toBe(990)
    const inverse: Donnees = { ...donnees, sankey: { ...donnees.sankey, noeuds: donnees.sankey.noeuds.map(n => ({ ...n, type: n.type === 'projet' ? 'techno' : 'projet' })) } }
    expect(hauteurSankey(inverse)).toBe(990)
  })
  it('fournit 17 projets et 45 technos reliés, avec douze mois de tendances', () => {
    const projets = donnees.sankey.noeuds.filter(n => n.type === 'projet').map(n => n.nom)
    const technos = donnees.sankey.noeuds.filter(n => n.type === 'techno').map(n => n.nom)
    expect(new Set(projets).size).toBe(17)
    expect(new Set(technos).size).toBe(45)
    expect(new Set(donnees.sankey.liens.map(l => l.source))).toEqual(new Set(projets))
    expect(new Set(donnees.sankey.liens.map(l => l.cible))).toEqual(new Set(technos))
    expect(donnees.sankey.liens.every(l => l.valeur > 0)).toBe(true)
    expect(donnees.tendance).toHaveLength(45 * 12)
  })
  it('construit les deux colonnes du Sankey et les liens pondérés sans collision de noms', () => {
    const o = seriesOption(optionSankey({ sankey: { noeuds: [{ nom: 'Go', type: 'projet' }, { nom: 'Go', type: 'techno' }], liens: [{ source: 'Go', cible: 'Go', valeur: 8 }] }, tendance: [], sans_projet: [] }))[0]
    expect(o.data).toMatchObject([{ name: 'projet:Go', depth: 0 }, { name: 'techno:Go', depth: 1 }])
    expect(o.links).toEqual([{ source: 'projet:Go', target: 'techno:Go', value: 8 }])
  })
  it('calcule hausse et baisse sur les six derniers mois en complétant les mois vides', () => {
    const points = [1,2,3,4,5,6].flatMap((m,i) => [{ mois: `2026-0${m}`, techno: 'Go', commits: i < 3 ? 3 : 9 }, { mois: `2026-0${m}`, techno: 'Rust', commits: i < 3 ? 8 : 2 }])
    expect(tendances(points).find(t => t.techno === 'Go')?.pente).toBe(6)
    expect(tendances(points).find(t => t.techno === 'Rust')?.pente).toBe(-6)
    expect(moisTendance(points)).toHaveLength(12)
    expect(moisTendance(points)[0]).toBe('2025-07')
    expect(tendances([{ mois: '2026-06', techno: 'Go', commits: 9 }])[0].pente).toBe(3)
    expect(tendances([])).toEqual([])
  })
  it('affiche les huit premières courbes et laisse les autres disponibles dans la légende', () => {
    const o = optionTendances(donnees)
    const selected = (o.legend as { selected: Record<string, boolean> }).selected
    expect(Object.values(selected).filter(Boolean)).toHaveLength(8)
    expect(seriesOption(o)).toHaveLength(new Set(donnees.tendance.map(t => t.techno)).size)
    expect(seriesOption(o)[0].data).toHaveLength(12)
  })
  it('empile les sessions mensuelles pour chaque IA', () => {
    const s = seriesOption(optionIA(ias))
    expect(s).toHaveLength(ias.length)
    expect(s.every(l => l.stack === 'sessions')).toBe(true)
    expect(s[0].data.reduce<number>((s, n) => s + Number(n), 0)).toBe(ias[0].sessions)
  })
  it('rend les maquettes et ouvre uniquement un projet connu', async () => {
    afficher()
    expect(await screen.findByText('Claude Code')).toBeTruthy()
    expect(screen.getByRole('img', { name: 'Relations entre projets et technologies' }).style.height).toBe('990px')
    expect(screen.getByText(/Docker, Redis/)).toBeTruthy()
    fireEvent.click(screen.getByText('Projet inconnu'))
    expect(screen.queryByText('Page projet')).toBeNull()
    fireEvent.click(screen.getByText('Projet connu'))
    expect(await screen.findByText('Page projet')).toBeTruthy()
  })
  it('annonce les données vides', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => ({ ok: true, json: async () => url.includes('technos') ? { sankey: { noeuds: [], liens: [] }, tendance: [], sans_projet: [] } : [] })))
    afficher()
    expect(await screen.findByText('Aucune relation entre projet et techno.')).toBeTruthy()
    expect(await screen.findByText('Aucune session IA.')).toBeTruthy()
  })
  it('affiche les erreurs réseau', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new Error('API indisponible') }))
    afficher()
    expect((await screen.findAllByRole('alert'))[0].textContent).toContain('API indisponible')
  })
})



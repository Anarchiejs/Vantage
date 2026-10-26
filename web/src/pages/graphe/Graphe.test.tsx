import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { EChartsOption } from 'echarts'
import type { Graphe as Donnees } from '../../api/types'
import Graphe, { cheminMaquette, ContenuNote, optionGraphe, selectionNote, tailleNoeud } from './Graphe'

const capture = vi.hoisted(() => ({ option: {} as EChartsOption }))
vi.mock('../../ui/Graphique', () => ({ Graphique: ({ option, surClic, 'aria-label': label }: { option: () => EChartsOption; surClic: (p: unknown) => void; 'aria-label': string }) => {
  capture.option = option()
  return <div role="img" aria-label={label}><button onClick={() => surClic({ dataType: 'node', data: { id: '1' } })}>Sélectionner Observatoire</button></div>
} }))
const fichiers = import.meta.glob<string>('../../../public/mocks/**/*.json', { query: '?raw', import: 'default', eager: true })
const lire = (f: string) => JSON.parse(fichiers[`../../../public/mocks/${f}.json`].replace(/^\uFEFF/, ''))
const d: Donnees = lire('graphe')
type Serie = { data: Array<{ id: string; category: number; symbolSize: number; label: { show: boolean }; itemStyle: { opacity: number; borderWidth: number } }>; categories: Array<{ name: string; itemStyle: { color: string } }>; links: Array<{ source: string; target: string; lineStyle: { width: number } }>; force: { layoutAnimation: boolean }; emphasis: { focus: string; label: { show: boolean } } }
const serie = (o: EChartsOption) => (o.series as Serie[])[0]
beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async (url: string) => ({ ok: true, json: async () => lire(url.replace('/api/', '').split('?')[0]) })))
  for (let i=1;i<=8;i++) document.documentElement.style.setProperty(`--serie-${i}`, `var(--test-${i})`)
})
afterEach(() => { cleanup(); vi.unstubAllGlobals() })
function afficher(url = '/graphe') { render(<MemoryRouter initialEntries={[url]}><Routes><Route path="/graphe" element={<Graphe />} /><Route path="/graphe/:id" element={<Graphe />} /></Routes></MemoryRouter>) }

describe('Graphe du cerveau', () => {
  it('fournit 300 notes et 800 liens uniques, avec degrés et analyses cohérents', () => {
    expect(d.noeuds).toHaveLength(300)
    const ids = new Set(d.noeuds.map(n => n.id))
    expect(ids.size).toBe(300)
    expect(d.liens).toHaveLength(800)
    expect(new Set(d.liens.map(l => `${l.source}:${l.cible}`)).size).toBe(800)
    expect(d.liens.every(l => ids.has(l.source) && ids.has(l.cible) && l.source !== l.cible)).toBe(true)
    for (const n of d.noeuds) expect(n.degre).toBe(d.liens.filter(l => l.source === n.id || l.cible === n.id).length)
    expect(d.orphelines).toEqual(d.noeuds.filter(n => n.degre === 0))
    expect(d.centrales).toEqual([...d.noeuds].sort((a, b) => b.degre - a.degre || a.id - b.id).slice(0, 10))
  })
  it('limite les étiquettes aux quinze plus hauts degrés et à la sélection, avec révélation au survol seul', () => {
    const dossiers = [...new Set(d.noeuds.map(n => n.dossier))]
    const attendus = [...d.noeuds].sort((a, b) => b.degre - a.degre || a.id - b.id).slice(0, 15).map(n => String(n.id))
    const s = serie(optionGraphe({ ...d, centrales: [] }, dossiers))
    expect(s.data.filter(n => n.label.show).map(n => n.id).sort()).toEqual(attendus.sort())
    const selection = serie(optionGraphe(d, dossiers, 51))
    expect(selection.data.filter(n => n.label.show)).toHaveLength(16)
    expect(selection.data.find(n => n.id === '51')?.label.show).toBe(true)
    expect(selection.emphasis).toEqual({ focus: 'self', label: { show: true } })
    const filtre = serie(optionGraphe(d, ['Technos']))
    expect(filtre.data.filter(n => n.label.show).every(n => attendus.includes(n.id))).toBe(true)
  })
  it('départage les degrés égaux par identifiant, indépendamment de l’ordre des notes', () => {
    const noeuds = Array.from({ length: 30 }, (_, i) => ({ id: i + 1, titre: `Note ${i}`, dossier: 'Notes', degre: 4 })).reverse()
    const s = serie(optionGraphe({ ...d, noeuds, liens: [] }, ['Notes']))
    expect(s.data.filter(n => n.label.show).map(n => Number(n.id)).sort((a, b) => a - b)).toEqual(Array.from({ length: 15 }, (_, i) => i + 1))
  })
  it('calcule les chemins de démonstration sans ignorer les extrémités choisies', () => {
    expect(cheminMaquette(d, 1, 2).chemin.map(n => n.id)).toEqual([1, 21, 2])
    expect(cheminMaquette(d, 2, 1).chemin.map(n => n.id)).toEqual([2, 21, 1])
    expect(cheminMaquette(d, 1, 1).chemin.map(n => n.id)).toEqual([1])
    expect(cheminMaquette(d, 1, 51).chemin).toEqual([])
    expect(cheminMaquette(d, 1, 999).chemin).toEqual([])
  })
  it('attribue les couleurs dans l’ordre trié des dossiers, même après filtrage', () => {
    const complet = serie(optionGraphe(d, d.noeuds.map(n => n.dossier)))
    const filtre = serie(optionGraphe({ ...d, noeuds: [...d.noeuds].reverse() }, ['Technos']))
    expect(complet.categories.map(c => c.name)).toEqual([...new Set(d.noeuds.map(n => n.dossier))].sort())
    expect(filtre.categories).toEqual(complet.categories)
    expect(filtre.data.every(n => n.category === complet.categories.findIndex(c => c.name === 'Technos'))).toBe(true)
    expect(filtre.links).toEqual([])
  })
  it('borne les tailles selon la racine carrée du degré', () => {
    expect(tailleNoeud(-1)).toBe(9)
    expect(tailleNoeud(0)).toBe(9)
    expect(tailleNoeud(4)).toBe(17)
    expect(tailleNoeud(1e9)).toBe(44)
  })
  it('sélectionne par titre et par identifiant', () => {
    expect(selectionNote(d, undefined, 'Observatoire')?.id).toBe(1)
    expect(selectionNote(d, '2')?.titre).toBe('Compass')
    expect(selectionNote(d, '99999')).toBeUndefined()
  })
  it('estompe les notes éloignées et surligne les arêtes du chemin dans les deux sens', () => {
    const t = d.noeuds.find(n => n.titre === 'TypeScript')!
    const s = serie(optionGraphe(d, [...new Set(d.noeuds.map(n => n.dossier))], 1, [2, t.id, 1]))
    expect(s.data.find(n => n.id === '51')?.itemStyle.opacity).toBe(0.15)
    expect(s.data.find(n => n.id === String(t.id))?.itemStyle.opacity).toBe(1)
    expect(s.links.filter(l => l.lineStyle.width === 4)).toHaveLength(2)
  })
  it('désactive l’animation au-delà de 500 notes et conserve seulement les quinze étiquettes prioritaires', () => {
    const gros: Donnees = { ...d, noeuds: Array.from({ length: 2000 }, (_, i) => ({ id: i+100, titre: `Note ${i}`, dossier: 'Notes', degre: i })), liens: [] }
    const o = optionGraphe(gros, ['Notes'])
    expect(o.animation).toBe(false)
    expect(serie(o).data).toHaveLength(2000)
    expect(serie(o).data.filter(n => n.label.show)).toHaveLength(15)
    expect(serie(o).force.layoutAnimation).toBe(false)
  })
  it('rend le contenu comme texte avec liens résolus, alias et ancres', () => {
    render(<MemoryRouter><ContenuNote contenu={'<script>texte</script> [[Observatoire|projet]] [[Compass#Plan]] [[Absent]]'} noeuds={d.noeuds} /></MemoryRouter>)
    expect(screen.getByRole('link', { name: 'projet' }).getAttribute('href')).toBe('/graphe/1')
    expect(screen.getByRole('link', { name: 'Compass#Plan' }).getAttribute('href')).toBe('/graphe/2')
    expect(screen.getByText(/\[\[Absent\]\]/)).toBeTruthy()
    expect(document.querySelector('script')).toBeNull()
  })
  it('ouvre le panneau depuis ?note= et les liens voisins', async () => {
    afficher('/graphe?note=Observatoire')
    expect(await screen.findByRole('heading', { name: 'Observatoire' })).toBeTruthy()
    expect(screen.getByRole('heading', { name: 'Voisins' })).toBeTruthy()
    expect(vi.mocked(fetch).mock.calls.some(([url]) => url === '/api/notes/1')).toBe(true)
  })
  it('filtre les dossiers et ouvre un nœud recherché', async () => {
    afficher()
    const checkbox = await screen.findByRole('checkbox', { name: 'Technos' })
    fireEvent.click(checkbox)
    expect(serie(capture.option).data.some(n => n.category === serie(capture.option).categories.findIndex(c => c.name === 'Technos'))).toBe(false)
    fireEvent.change(screen.getByLabelText('Rechercher un nœud par titre'), { target: { value: 'Compass' } })
    fireEvent.click(screen.getByRole('button', { name: 'Ouvrir' }))
    expect(await screen.findByRole('heading', { name: 'Compass' })).toBeTruthy()
  })
  it('ouvre une note par clic sur le graphe', async () => {
    afficher()
    fireEvent.click(await screen.findByRole('button', { name: 'Sélectionner Observatoire' }))
    expect(await screen.findByRole('heading', { name: 'Observatoire' })).toBeTruthy()
  })
  it('affiche le plus court chemin renvoyé par l’API et le surligne', async () => {
    afficher()
    await screen.findByRole('checkbox', { name: 'Technos' })
    fireEvent.change(screen.getByLabelText('De'), { target: { value: 'Observatoire' } })
    fireEvent.change(screen.getByLabelText('À'), { target: { value: 'Compass' } })
    fireEvent.click(screen.getByRole('button', { name: 'Chercher le chemin' }))
    const fil = await screen.findByRole('navigation', { name: 'Plus court chemin' })
    expect(within(fil).getAllByRole('link').map(l => l.textContent)).toEqual(['Observatoire', 'TypeScript', 'Compass'])
    expect(vi.mocked(fetch).mock.calls.some(([url]) => url === '/api/graphe/chemin?de=1&a=2')).toBe(true)
    await waitFor(() => expect(serie(capture.option).links.filter(l => l.lineStyle.width === 4)).toHaveLength(2))
  })
  it('annonce aucun chemin et refuse les titres inconnus', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url: string) => ({ ok: true, json: async () => url.includes('/chemin') ? { chemin: [] } : lire(url.replace('/api/', '')) })))
    afficher()
    fireEvent.click(await screen.findByRole('button', { name: 'Chercher le chemin' }))
    expect(screen.getByRole('alert').textContent).toContain('deux titres')
    fireEvent.change(screen.getByLabelText('De'), { target: { value: 'Observatoire' } })
    fireEvent.change(screen.getByLabelText('À'), { target: { value: 'Idée à explorer' } })
    fireEvent.click(screen.getByRole('button', { name: 'Chercher le chemin' }))
    expect(await screen.findByText('Aucun chemin.')).toBeTruthy()
  })
  it('annonce un coffre vide', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => ({ ok: true, json: async () => ({ noeuds: [], liens: [], centrales: [], orphelines: [], casses: [] }) })))
    afficher()
    expect(await screen.findByText('Le coffre ne contient aucune note.')).toBeTruthy()
  })
  it('annonce une note introuvable et une erreur de chargement de note', async () => {
    afficher('/graphe/9999')
    expect(await screen.findByText('Note introuvable.')).toBeTruthy()
    cleanup()
    vi.stubGlobal('fetch', vi.fn(async (url: string) => { if (url.includes('/notes/')) throw new Error('Note inaccessible'); return { ok: true, json: async () => d } }))
    afficher('/graphe/1')
    expect((await screen.findByRole('alert')).textContent).toBe('Note inaccessible')
  })
})



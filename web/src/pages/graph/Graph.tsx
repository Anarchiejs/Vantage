import { useCallback, useEffect, useRef, useState } from 'react'
import type { ECharts } from 'echarts/core'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import type { EChartsOption } from 'echarts'
import type { Graph as GraphData, GraphNode, GraphPath, NoteDetail } from '../../api/types'
import { useApi, type ApiState } from '../../api/useApi'
import { get, withParams } from '../../api/client'
import { Card } from '../../ui/Card'
import { Empty, Load } from '../../ui/Status'
import { Chart } from '../../ui/Chart'
import { date } from '../../ui/format'
import { series, token } from '../../theme/theme'
import { Header } from '../../ui/Header'

/** A note at the vault root has an empty folder name. */
export const folderLabel = (folder: string) => folder || 'Vault root'

/** One color per folder, stable whatever the folder filter shows. */
export function folderColors(folders: string[]): (folder: string) => string {
  const colors = series()
  const sorted = [...folders].sort()
  return folder => colors[Math.max(0, sorted.indexOf(folder)) % colors.length]
}

export const nodeSize = (degree: number) => Math.max(9, Math.min(44, 9 + Math.sqrt(Math.max(0, degree)) * 4))
export function selectedNote(data: GraphData, id?: string, title?: string | null) {
  return title ? data.nodes.find(node => node.title === title) : data.nodes.find(node => node.id === Number(id))
}

/** The static client ignores parameters: mock mode walks its own graph. */
export function mockPath(data: GraphData, from: number, to: number): GraphPath {
  const nodes = new Map(data.nodes.map(node => [node.id, node]))
  if (!nodes.has(from) || !nodes.has(to)) return { path: [] }
  const neighbors = new Map<number, number[]>()
  for (const link of data.links) {
    neighbors.set(link.source, [...(neighbors.get(link.source) ?? []), link.target])
    neighbors.set(link.target, [...(neighbors.get(link.target) ?? []), link.source])
  }
  const previous = new Map<number, number | null>([[from, null]])
  const queue = [from]
  for (let i = 0; i < queue.length && !previous.has(to); i++) {
    for (const id of neighbors.get(queue[i]) ?? []) if (!previous.has(id) && nodes.has(id)) {
      previous.set(id, queue[i]); queue.push(id)
    }
  }
  if (!previous.has(to)) return { path: [] }
  const path: GraphNode[] = []
  for (let id: number | null = to; id !== null; id = previous.get(id) ?? null) path.push(nodes.get(id)!)
  return { path: path.reverse() }
}

export function graphOption(data: GraphData, folders: string[], selected?: number, path: number[] = []): EChartsOption {
  const categories = [...new Set([...folders, ...data.nodes.map(node => node.folder)])].sort()
  const colors = series()
  const visible = data.nodes.filter(node => folders.includes(node.folder))
  const ids = new Set(visible.map(node => node.id))
  const central = new Set([...data.nodes].sort((a, b) => b.degree - a.degree || a.id - b.id).slice(0, 15).map(node => node.id))
  const nearby = new Set([selected, ...data.links.flatMap(link => link.source === selected ? [link.target] : link.target === selected ? [link.source] : [])])
  const route = new Set(path)
  const edges = new Set(path.slice(1).flatMap((id, index) => [`${path[index]}:${id}`, `${id}:${path[index]}`]))
  const large = data.nodes.length > 500
  return { animation: !large, xAxis: { show: false }, yAxis: { show: false }, tooltip: { trigger: 'item', formatter: '{b}', renderMode: 'richText' }, series: [{ type: 'graph', layout: 'force', roam: true, draggable: true, scaleLimit: { min: .2, max: 5 }, label: { color: token('ink'), position: 'right', distance: 6, fontSize: 12, textBorderColor: token('surface'), textBorderWidth: 3 },
    animation: !large, force: { repulsion: large ? 35 : 140, edgeLength: [35, 90], layoutAnimation: !large },
    categories: categories.map((name, index) => ({ name: folderLabel(name), itemStyle: { color: colors[index % colors.length] } })),
    data: visible.map(node => ({ id: String(node.id), name: node.title, category: categories.indexOf(node.folder), symbolSize: nodeSize(node.degree),
      label: { show: central.has(node.id) || node.id === selected || nearby.has(node.id), fontWeight: node.id === selected ? 650 : 500 },
      itemStyle: { opacity: selected !== undefined && !nearby.has(node.id) && !route.has(node.id) ? 0.15 : 1,
        borderWidth: route.has(node.id) || node.id === selected ? 3 : 0, borderColor: token('accent') } })),
    links: data.links.filter(link => ids.has(link.source) && ids.has(link.target)).map(link => ({ source: String(link.source), target: String(link.target),
      lineStyle: { color: edges.has(`${link.source}:${link.target}`) ? token('accent') : token('line'),
        width: edges.has(`${link.source}:${link.target}`) ? 4 : 1, opacity: selected !== undefined && !nearby.has(link.source) && !nearby.has(link.target) ? 0.12 : 0.7 } })),
    emphasis: { focus: 'adjacency', label: { show: true }, lineStyle: { width: 2 } } }] }
}

export function resolveNote(raw: string, sourceFolder: string, nodes: GraphNode[]): GraphNode | undefined {
  const cleaned = raw.split('|')[0].split('#')[0].split('^')[0].trim().replaceAll('\\', '/').replace(/\.md$/i, '')
  const pathHits = nodes.filter(node => node.path.replace(/\.md$/i, '').toLowerCase() === cleaned.toLowerCase())
  if (pathHits.length === 1) return pathHits[0]
  const title = cleaned.split('/').pop() ?? cleaned
  const titleHits = nodes.filter(node => node.title.toLowerCase() === title.toLowerCase())
  if (titleHits.length !== 1 && titleHits.length !== 0) {
    const same = titleHits.filter(node => node.folder === sourceFolder)
    const pool = same.length ? same : titleHits
    return [...pool].sort((a, b) => a.path.length - b.path.length || a.path.localeCompare(b.path))[0]
  }
  return titleHits[0]
}

export function NoteBody({ content, nodes, folder }: { content: string; nodes: GraphNode[]; folder: string }) {
  return <div className="whitespace-pre-wrap break-words text-sm leading-relaxed">{content.split(/(\[\[[^\]]+\]\])/g).map((part, index) => {
    if (!part.startsWith('[[') || !part.endsWith(']]')) return <span key={index}>{part}</span>
    const body = part.slice(2, -2)
    const alias = body.includes('|') ? body.slice(body.indexOf('|') + 1) : ''
    const node = resolveNote(body, folder, nodes)
    return node ? <Link key={index} className="text-accent underline" to={`/graph/${node.id}`}>{alias || node.title}</Link> : <span key={index}>{part}</span>
  })}</div>
}

function Panel({ id, nodes, colorOf }: { id: number; nodes: GraphNode[]; colorOf: (folder: string) => string }) {
  const note = useApi<NoteDetail>(`/api/notes/${id}`)
  return <Card title="Selected note"><Load state={note}>{item => item.id === id ? <>
    <div className="mb-1 flex items-center justify-between gap-3"><span className="flex items-center gap-2 text-xs font-semibold text-muted"><span className="dot" style={{ background: colorOf(item.folder) }} aria-hidden="true" />{folderLabel(item.folder)}</span><Link aria-label="Close the note" className="grid h-8 w-8 place-items-center rounded-lg text-muted hover:bg-surface-2" to="/graph">×</Link></div>
    <h2 className="text-xl font-semibold tracking-tight">{item.title}</h2>
    <p className="mt-1 mb-4 text-xs text-muted">{item.updated ? `Updated ${date(item.updated)} · ` : ''}{item.neighbors.length} linked {item.neighbors.length === 1 ? 'note' : 'notes'}</p>
    {(item.obsidian_url || item.project) && <div className="mb-4 flex flex-wrap gap-2">{item.obsidian_url && <a className="btn btn-primary" href={item.obsidian_url}>Open in Obsidian</a>}{item.project && <Link className="btn btn-secondary" to={`/projects/${item.project.id}`}>{item.project.name} project page</Link>}</div>}
    <NoteBody content={item.content} nodes={nodes} folder={item.folder} />
    <h3 className="mt-5 mb-2 text-sm font-semibold text-muted">Linked notes</h3><NodeList nodes={item.neighbors} colorOf={colorOf} />
  </> : <Empty>Loading the note…</Empty>}</Load></Card>
}

function NodeList({ nodes, colorOf }: { nodes: GraphNode[]; colorOf?: (folder: string) => string }) {
  return nodes.length ? <ul className="text-sm">{nodes.map(node => <li key={node.id}><Link className="link-row min-h-9!" to={`/graph/${node.id}`}>{colorOf && <span className="dot" style={{ background: colorOf(node.folder) }} aria-hidden="true" />}<span className="min-w-0 flex-1 truncate">{node.title}</span><span className="text-xs text-muted tabular-nums" title="Links">{node.degree}</span></Link></li>)}</ul> : <Empty>No note.</Empty>
}

function Explore({ data, hidden, setHidden, properties, setProperties, counts }: { data: GraphData; hidden: string[]; setHidden: (value: string[] | ((values: string[]) => string[])) => void; properties: boolean; setProperties: (value: boolean) => void; counts: Record<string, number> }) {
  const chart = useRef<ECharts | null>(null)
  const onReady = useCallback((instance: ECharts | null) => { chart.current = instance }, [])
  const zoom = (factor: number) => {
    const instance = chart.current
    if (instance) instance.dispatchAction({ type: 'graphRoam', seriesIndex: 0, zoom: factor, originX: instance.getWidth() / 2, originY: instance.getHeight() / 2 })
  }
  const { id } = useParams()
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const selected = selectedNote(data, id, params.get('note'))
  const [search, setSearch] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [request, setRequest] = useState<string | null>(null)
  const [invalid, setInvalid] = useState(false)
  const [turn, setTurn] = useState(0)
  const [response, setResponse] = useState<{ url: string | null; state: ApiState<GraphPath> }>({ url: null, state: { status: 'loading' } })
  useEffect(() => {
    if (!request) return
    const ctrl = new AbortController()
    setResponse({ url: request, state: { status: 'loading' } })
    get<GraphPath>(request, ctrl.signal).then(payload => {
      if (ctrl.signal.aborted) return
      const query = new URLSearchParams(request.split('?')[1])
      const path = import.meta.env.VITE_MOCK === '1' ? mockPath(data, Number(query.get('from')), Number(query.get('to'))) : payload
      setResponse({ url: request, state: { status: 'ok', data: path } })
    }).catch((error: Error) => {
      if (!ctrl.signal.aborted) setResponse({ url: request, state: { status: 'error', error } })
    })
    return () => ctrl.abort()
  }, [request, turn, data])
  const result: ApiState<GraphPath> = response.url === request ? response.state : { status: 'loading' }
  const folders = data.folders.length ? data.folders : [...new Set(data.nodes.map(node => node.folder))].sort()
  const route = result.status === 'ok' && request ? result.data.path : undefined
  const selectedId = selected?.id
  const [layoutTurn, setLayoutTurn] = useState(0)
  const option = useCallback(() => graphOption(data, folders, selectedId, route?.map(node => node.id)), [data, folders, selectedId, route, layoutTurn])
  // Same folder set as the chart's categories, root notes included, so dots and nodes agree.
  const colorOf = folderColors([...new Set([...folders, ...data.nodes.map(node => node.folder)])])
  const find = (title: string) => data.nodes.find(node => node.title === title)
  return <>
    <Card title="Explore the vault">
      <form className="mb-4 flex gap-2" onSubmit={e => { e.preventDefault(); const node = find(search); if (node) navigate(`/graph/${node.id}`) }}>
        <label className="field flex-1"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className="text-muted" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg><input aria-label="Find a node by title" list="graph-titles" value={search} onChange={e => setSearch(e.target.value)} placeholder="A note title…" /></label>
        <button className={`btn ${find(search) ? 'btn-primary' : 'btn-secondary'}`} disabled={!find(search)}>Open</button>
      </form>
      <datalist id="graph-titles">{data.nodes.map(node => <option key={node.id} value={node.title} />)}</datalist>
      <fieldset className="flex flex-wrap gap-2 text-sm"><legend className="mb-2 text-xs font-semibold text-muted">Folders</legend>{folders.map(folder => <label className="flex min-h-9 cursor-pointer items-center gap-2 rounded-lg border border-line px-3 has-[:checked]:bg-surface-2" key={folder}>
        <input type="checkbox" checked={!hidden.includes(folder)} onChange={e => setHidden(values => e.target.checked ? values.filter(item => item !== folder) : [...values, folder])} /><span className="dot" style={{ background: colorOf(folder) }} aria-hidden="true" />{folderLabel(folder)}{counts[folder] !== undefined && <span className="text-xs text-muted tabular-nums">{counts[folder]}</span>}</label>)}</fieldset>
      <label className="mt-3 flex items-center gap-2 text-sm"><input type="checkbox" checked={properties} onChange={e => setProperties(e.target.checked)} />Include links from properties</label>
    </Card>
    {(id || params.has('note')) && !selected && <p role="status" className="text-muted">Note not found.</p>}
    <div className={`grid gap-4 ${selected ? 'grid-cols-[minmax(0,2fr)_minmax(280px,1fr)]' : 'grid-cols-1'}`}>
      <Card title={`${data.nodes.length} notes · ${data.links.length} links`} action={<span className="text-xs text-muted">Node size = number of links</span>}>
        {data.nodes.length ? folders.every(folder => hidden.includes(folder)) ? <Empty>No folder selected.</Empty> : <div className="relative"><Chart height={560} aria-label="Graph of vault links" option={option} onReady={onReady} onClick={event => {
          const node = event.data as { id?: string }
          if (event.dataType === 'node' && node.id) navigate(`/graph/${node.id}`)
        }} /><div className="absolute right-2 bottom-2 flex flex-col overflow-hidden rounded-xl border border-line bg-surface shadow-sm">
          <button type="button" aria-label="Zoom in" className="grid h-10 w-10 place-items-center text-lg hover:bg-surface-2" onClick={() => zoom(1.25)}>+</button>
          <button type="button" aria-label="Zoom out" className="grid h-10 w-10 place-items-center border-t border-line text-lg hover:bg-surface-2" onClick={() => zoom(0.8)}>−</button>
          <button type="button" aria-label="Reset the view" className="grid h-10 w-10 place-items-center border-t border-line hover:bg-surface-2" onClick={() => setLayoutTurn(n => n + 1)}><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true"><path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5" /></svg></button>
        </div></div> : <Empty>The vault contains no notes.</Empty>}
      </Card>
      {selected && <Panel key={selected.id} id={selected.id} nodes={data.nodes} colorOf={colorOf} />}
    </div>
    <Card title="Shortest path">
      <form className="flex flex-wrap items-end gap-3" onSubmit={e => { e.preventDefault(); const start = find(from); const end = find(to); setInvalid(!start || !end); if (start && end) {
        const url = withParams('/api/graph/path', { from: start.id, to: end.id }); setRequest(url); if (url === request) setTurn(value => value + 1)
      } }}>
        <label className="flex flex-col gap-1 text-sm">From<input className="rounded-[10px] border border-line bg-surface px-3" list="graph-titles" value={from} onChange={e => { setFrom(e.target.value); setRequest(null) }} /></label>
        <label className="flex flex-col gap-1 text-sm">To<input className="rounded-[10px] border border-line bg-surface px-3" list="graph-titles" value={to} onChange={e => { setTo(e.target.value); setRequest(null) }} /></label>
        <button className="btn btn-secondary">Find the path</button>
      </form>
      {invalid && <p role="alert">Choose two existing note titles.</p>}
      {request && <Load state={result}>{path => path.path.length ? <nav aria-label="Shortest path" className="mt-4 flex flex-wrap items-center gap-2">{path.path.map((node, index) => <span key={node.id}>{index > 0 && <span aria-hidden="true"> → </span>}<Link className="text-accent underline" to={`/graph/${node.id}`}>{node.title}</Link></span>)}</nav> : <p role="status" className="mt-4 text-muted">No path.</p>}</Load>}
    </Card>
    <div className="grid grid-cols-3 gap-4">
      <Card title="Orphan notes"><NodeList nodes={data.orphans} /></Card>
      <Card title="10 most central notes"><NodeList nodes={data.central.slice(0, 10)} /></Card>
      <Card title="Broken links">{data.broken.length ? <ul className="space-y-2 text-sm">{data.broken.map((link, index) => <li key={index}><Link className="text-accent" to={`/graph/${link.source}`}>{link.source_title}</Link> → <span className="text-danger">{link.target_title}</span></li>)}</ul> : <Empty>No broken link.</Empty>}</Card>
    </div>
  </>
}

export default function Graph() {
  const [hidden, setHidden] = useState<string[]>([])
  const [properties, setProperties] = useState(true)
  const [folders, setFolders] = useState<string[]>([])
  const visible = folders.filter(folder => !hidden.includes(folder))
  const filtering = folders.length > 0 && visible.length !== folders.length
  const params = new URLSearchParams()
  if (filtering) for (const folder of visible) params.append('folder', folder)
  if (!properties) params.set('properties', '0')
  const query = params.toString()
  const graph = useApi<GraphData>(`/api/graph${query ? `?${query}` : ''}`)
  const folderKey = graph.status === 'ok' ? graph.data.folders.join('\n') : ''
  useEffect(() => {
    if (folderKey) setFolders(folderKey.split('\n'))
  }, [folderKey])
  // Notes per folder, counted while no folder is hidden so hidden folders keep their count.
  const [counts, setCounts] = useState<Record<string, number>>({})
  const unfiltered = graph.status === 'ok' && !filtering ? graph.data : null
  useEffect(() => {
    if (!unfiltered) return
    const next: Record<string, number> = {}
    for (const node of unfiltered.nodes) next[node.folder] = (next[node.folder] ?? 0) + 1
    setCounts(next)
  }, [unfiltered])
  return <div className="space-y-5"><Header title="Note graph" description="Explore the connections between your notes. Move the nodes, zoom, and open a note to follow its links." /><Load state={graph}>{data => <Explore data={data} hidden={hidden} setHidden={setHidden} properties={properties} setProperties={setProperties} counts={counts} />}</Load></div>
}

import { Header } from '../../ui/Header'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { sql, SQLite } from '@codemirror/lang-sql'
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { Compartment, EditorState, Prec } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'
import { tags } from '@lezer/highlight'
import { basicSetup } from 'codemirror'
import type { SavedQuery, SchemaTable, SQLResult } from '../../api/types'
import { post } from '../../api/client'
import { useApi } from '../../api/useApi'
import { Card } from '../../ui/Card'
import { Empty, ErrorMessage, Load, Loading } from '../../ui/Status'
import { Chart } from '../../ui/Chart'
import { number } from '../../ui/format'
import { token } from '../../theme/theme'
import { chartOption, numericColumns, validName, type ChartShape } from './chart'

export { chartOption, validName } from './chart'

const SQL_KEY = 'vantage.console.sql'

const DEFAULT_SQL = `SELECT p.name, SUM(a.commit_count) AS commits, SUM(a.additions) AS additions
FROM daily_activity a
JOIN project p ON p.id = a.project_id
GROUP BY p.name
ORDER BY commits DESC`

type View = 'table' | ChartShape

function readSql(): string {
  try { return localStorage.getItem(SQL_KEY) ?? DEFAULT_SQL } catch { return DEFAULT_SQL }
}

function writeSql(text: string) {
  try { localStorage.setItem(SQL_KEY, text) } catch { /* storage unavailable */ }
}

function sqlExtension(tables: SchemaTable[]) {
  const schema: Record<string, string[]> = {}
  for (const table of tables) schema[table.table] = table.columns.map((column) => column.name)
  return sql({ dialect: SQLite, schema, upperCaseKeywords: true })
}

function editorTheme() {
  const accent = token('accent') || 'currentColor'
  const ink = token('ink') || 'inherit'
  const muted = token('muted') || 'inherit'
  const success = token('success') || accent
  const style = HighlightStyle.define([
    { tag: tags.keyword, color: accent, fontWeight: '600' },
    { tag: tags.string, color: success },
    { tag: [tags.number, tags.bool, tags.null], color: token('warning') || accent },
    { tag: tags.comment, color: muted, fontStyle: 'italic' },
    { tag: [tags.typeName, tags.className, tags.propertyName], color: token('series-1') || accent },
    { tag: tags.operator, color: ink },
  ])
  return [
    syntaxHighlighting(style),
    EditorView.theme({
      '.cm-content': { fontFamily: 'var(--font-mono), ui-monospace, monospace', caretColor: accent, padding: '8px 0' },
      '.cm-gutters': { backgroundColor: token('surface-2') || 'transparent', color: muted, border: 'none' },
      '&.cm-focused': { outline: 'none' },
      '&.cm-focused .cm-cursor': { borderLeftColor: accent },
      '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: token('surface-2') || 'transparent' },
      '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': { backgroundColor: token('accent-soft') || 'transparent' },
      '.cm-tooltip, .cm-tooltip-autocomplete': { backgroundColor: token('surface') || 'transparent', color: ink, border: `1px solid ${token('line') || 'transparent'}` },
      '.cm-tooltip-autocomplete ul li[aria-selected]': { backgroundColor: token('accent-soft') || 'transparent', color: ink },
      '&': { height: '100%', backgroundColor: token('surface') || 'transparent', color: ink },
      '.cm-scroller': { overflow: 'auto' },
    }, { dark: document.documentElement.dataset.theme === 'dark' }),
  ]
}

function buttonClass(active: boolean): string {
  return `rounded px-3 py-1 text-sm disabled:opacity-40 ${active ? 'bg-accent-soft text-accent' : 'bg-surface-2 text-muted'}`
}

export default function Console() {
  const schema = useApi<SchemaTable[]>('/api/schema')
  const saved = useApi<SavedQuery[]>('/api/queries')
  const parentRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)
  const sqlComp = useRef(new Compartment())
  const themeComp = useRef(new Compartment())
  const runRef = useRef<() => void>(() => {})

  const [result, setResult] = useState<SQLResult | null>(null)
  const [sqlError, setSqlError] = useState<string | null>(null)
  const [running, setRunning] = useState(false)
  const [view, setView] = useState<View>('table')
  const [extra, setExtra] = useState<SavedQuery[]>([])
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [nameError, setNameError] = useState<string | null>(null)
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set())

  async function run() {
    const text = viewRef.current?.state.doc.toString() ?? readSql()
    setRunning(true)
    setSqlError(null)
    try {
      setResult(await post<SQLResult>('/api/sql', { sql: text }))
      setView('table')
    } catch (error) {
      setResult(null)
      setSqlError(error instanceof Error ? error.message : 'SQL error')
    } finally {
      setRunning(false)
    }
  }
  runRef.current = () => { void run() }

  useEffect(() => {
    const parent = parentRef.current
    if (!parent) return
    const editor = new EditorView({
      parent,
      state: EditorState.create({
        doc: readSql(),
        extensions: [
          basicSetup,
          EditorView.lineWrapping,
          sqlComp.current.of(sqlExtension([])),
          themeComp.current.of(editorTheme()),
          Prec.high(keymap.of([{ key: 'Ctrl-Enter', preventDefault: true, run: () => { runRef.current(); return true } }])),
          EditorView.updateListener.of((update) => { if (update.docChanged) writeSql(update.state.doc.toString()) }),
          EditorView.contentAttributes.of({ 'aria-label': 'SQL query' }),
        ],
      }),
    })
    viewRef.current = editor
    const measure = new ResizeObserver(() => editor.requestMeasure())
    measure.observe(parent)
    const onTheme = () => editor.dispatch({ effects: themeComp.current.reconfigure(editorTheme()) })
    window.addEventListener('vantage:theme', onTheme)
    return () => { measure.disconnect(); window.removeEventListener('vantage:theme', onTheme); editor.destroy(); viewRef.current = null }
  }, [])

  useEffect(() => {
    const editor = viewRef.current
    if (!editor || schema.status !== 'ok') return
    editor.dispatch({ effects: sqlComp.current.reconfigure(sqlExtension(schema.data)) })
  }, [schema.status, schema.data])

  function toggleTable(table: string) {
    setCollapsed((current) => {
      const next = new Set(current)
      if (next.has(table)) next.delete(table)
      else next.add(table)
      return next
    })
  }

  function insert(text: string) {
    const editor = viewRef.current
    if (!editor) return
    const sel = editor.state.selection.main
    editor.dispatch({ changes: { from: sel.from, to: sel.to, insert: text }, selection: { anchor: sel.from + text.length } })
    editor.focus()
  }

  function load(text: string) {
    const editor = viewRef.current
    if (!editor) return
    editor.dispatch({ changes: { from: 0, to: editor.state.doc.length, insert: text }, selection: { anchor: text.length } })
    editor.focus()
  }

  async function save(submitted: string) {
    const text = viewRef.current?.state.doc.toString() ?? readSql()
    try {
      const created = await post<SavedQuery>('/api/queries', { name: submitted, sql: text })
      setExtra((list) => [...list.filter((item) => item.name !== created.name), created])
      setOpen(false)
      setName('')
      setNameError(null)
      saved.reload()
    } catch (error) {
      setNameError(error instanceof Error ? error.message : 'Could not save')
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    const cleaned = name.trim()
    if (!validName(cleaned)) {
      setNameError('The name can contain only lowercase letters, digits, and hyphens.')
      return
    }
    void save(cleaned)
  }

  const stored = saved.status === 'ok' ? saved.data : []
  const names = new Set(stored.map((item) => item.name))
  const all = [...stored, ...extra.filter((item) => !names.has(item.name))]
  const canChart = result != null && numericColumns(result).length > 0
  const numeric = result ? new Set(numericColumns(result)) : new Set<number>()
  const shapeLabel = { bars: 'Bars', line: 'Line', pie: 'Pie' } as const

  return (
    <div className="console-page flex min-h-[780px] flex-col gap-4">
      <Header title="SQL console" description="Explore your data, ask questions, and chart the results." />
      <div className="grid min-h-0 flex-1 grid-cols-[260px_minmax(0,1fr)] items-stretch gap-4">
        <aside className="flex min-h-0 flex-col gap-4">
          <Card title="Schema" className="flex min-h-0 flex-1 flex-col overflow-hidden [&>header]:shrink-0">
            <div className="min-h-0 flex-1 overflow-y-auto">
              <Load state={schema} lines={8}>{(tables) => (
                <ul className="flex flex-col gap-2">
                  {tables.map((table) => {
                    const expanded = !collapsed.has(table.table)
                    return (
                      <li key={table.table}>
                        <div className="flex items-center gap-1">
                          <button type="button" aria-expanded={expanded} aria-controls={`schema-${table.table}`} aria-label={`${expanded ? 'Collapse' : 'Expand'} ${table.table}`} onClick={() => toggleTable(table.table)} className="w-4 shrink-0 text-center text-xs text-muted">{expanded ? '▾' : '▸'}</button>
                          <button type="button" aria-label={`Insert ${table.table}`} onClick={() => insert(table.table)} className="truncate font-mono text-sm font-semibold text-ink hover:text-accent">{table.table}</button>
                        </div>
                        {expanded && (
                          <ul id={`schema-${table.table}`} className="mt-1 flex flex-col pl-5">
                            {table.columns.map((column) => (
                              <li key={column.name}>
                                <button type="button" aria-label={`Column ${column.name} of ${table.table}`} onClick={() => insert(column.name)} className="flex w-full items-baseline justify-between gap-2 rounded px-1 py-0.5 text-left font-mono text-xs hover:bg-surface-2">
                                  <span className="truncate">{column.name}</span>
                                  <span className="shrink-0 text-muted">{column.type}</span>
                                </button>
                              </li>
                            ))}
                          </ul>
                        )}
                      </li>
                    )
                  })}
                </ul>
              )}</Load>
            </div>
          </Card>
          <Card title="Saved queries" className="flex min-h-0 flex-1 flex-col overflow-hidden [&>header]:shrink-0">
            <div className="shrink-0">
              {open ? (
                <form noValidate onSubmit={submit} className="mb-3 flex flex-col gap-2">
                  <label className="flex flex-col gap-1 text-xs text-muted">Query name
                    <input autoFocus value={name} onChange={(e) => setName(e.target.value)} className="rounded-md border border-line bg-surface px-2 py-1 font-mono text-sm text-ink" />
                  </label>
                  {nameError && <ErrorMessage error={new Error(nameError)} />}
                  <button type="submit" className={buttonClass(true)}>Save</button>
                </form>
              ) : (
                <button type="button" onClick={() => setOpen(true)} className={`mb-3 ${buttonClass(false)}`}>Save as…</button>
              )}
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto">
              {saved.status === 'loading' && <Loading />}
              {saved.status === 'error' && <ErrorMessage error={saved.error} />}
              {all.length === 0 && saved.status === 'ok' && <Empty>No saved query.</Empty>}
              <ul className="flex flex-col gap-1">
                {all.map((query) => (
                  <li key={query.name}>
                    <button type="button" onClick={() => load(query.sql)} className="w-full rounded-md px-2 py-1 text-left hover:bg-surface-2">
                      <span className="block font-mono text-sm">{query.name}</span>
                      <span className="block truncate text-xs text-muted">{query.sql}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          </Card>
        </aside>
        <div className="flex min-h-0 min-w-0 flex-col gap-3">
          <div className="h-[220px] max-h-[70vh] min-h-[120px] shrink-0 resize-y overflow-hidden rounded-lg border border-line focus-within:border-accent"><div ref={parentRef} className="h-full" /></div>
          {sqlError && <ErrorMessage error={new Error(sqlError)} />}
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            <button type="button" onClick={() => void run()} disabled={running} className={buttonClass(true)}>{running ? 'Running…' : 'Run'}</button>
            <div role="group" aria-label="Presentation" className="flex gap-2">
              <button type="button" aria-pressed={view === 'table'} onClick={() => setView('table')} className={buttonClass(view === 'table')}>Table</button>
              {(['bars', 'line', 'pie'] as const).map((shape) => (
                <button key={shape} type="button" disabled={!canChart} title={canChart ? undefined : 'No numeric column'} aria-pressed={view === shape} onClick={() => setView(shape)} className={buttonClass(view === shape)}>{shapeLabel[shape]}</button>
              ))}
            </div>
            <p className="text-xs text-muted"><kbd className="font-mono">Ctrl</kbd> + <kbd className="font-mono">Enter</kbd> to run.</p>
            <p className="rounded-lg border border-line bg-surface-2 px-3 py-1 text-sm text-muted">Read-only: the database cannot be changed from here.</p>
          </div>
          <Card title="Results" className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden [&>header]:shrink-0">
            <div className="min-h-0 flex-1 overflow-auto">
              {!result && <Empty>Run a query to see the rows.</Empty>}
              {result && (
                <div className="space-y-3">
                  <p className="text-xs text-muted tabular-nums">{number(result.duration_ms)} ms · {number(result.rows.length)} {result.rows.length === 1 ? 'row' : 'rows'}</p>
                  {result.truncated && <p role="status" className="rounded-lg border border-warning/40 bg-warning/10 px-3 py-2 text-sm text-warning">Result truncated: only the first 1,000 rows are shown.</p>}
                  {view === 'table' && result.rows.length === 0 && <Empty>The query returned no rows.</Empty>}
                  {view === 'table' && result.rows.length > 0 && (
                    <table className="w-full border-collapse text-sm">
                      <thead><tr>{result.columns.map((column, index) => <th key={column} className={`sticky top-0 bg-surface px-2 py-1 text-xs font-semibold text-muted ${numeric.has(index) ? 'text-right' : 'text-left'}`}>{column}</th>)}</tr></thead>
                      <tbody>{result.rows.map((row, rowIndex) => <tr key={rowIndex} className="border-t border-line">{row.map((cell, index) => <td key={index} className={`px-2 py-1 ${typeof cell === 'number' ? 'text-right font-mono tabular-nums' : ''}`}>{cell === null ? <span className="text-muted">NULL</span> : String(cell)}</td>)}</tr>)}</tbody>
                    </table>
                  )}
                  {view !== 'table' && chartOption(result, view) && <Chart aria-label={`${shapeLabel[view]} chart of the SQL result`} option={() => chartOption(result, view) ?? {}} />}
                </div>
              )}
            </div>
          </Card>
        </div>
      </div>
    </div>
  )
}

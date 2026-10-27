import { Entete } from '../../ui/Entete'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { sql, SQLite } from '@codemirror/lang-sql'
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { Compartment, EditorState, Prec } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'
import { tags } from '@lezer/highlight'
import { basicSetup } from 'codemirror'
import type { RequeteEnregistree, ResultatSQL, TableSchema } from '../../api/types'
import { post } from '../../api/client'
import { useApi } from '../../api/useApi'
import { Carte } from '../../ui/Carte'
import { Charge, Chargement, ErreurAffichee, Vide } from '../../ui/Etat'
import { Graphique } from '../../ui/Graphique'
import { nombre } from '../../ui/format'
import { jeton } from '../../theme/theme'
import { colonnesNumeriques, nomValide, optionGraphique, type FormeGraphique } from './graphique'

export { nomValide, optionGraphique } from './graphique'

const CLE_SQL = 'observatoire.console.sql'

const SQL_DEFAUT = `SELECT p.nom, SUM(a.nb_commits) AS commits, SUM(a.ajouts) AS ajouts
FROM activite_jour a
JOIN projet p ON p.id = a.projet_id
GROUP BY p.nom
ORDER BY commits DESC`

type Vue = 'tableau' | FormeGraphique

function lireSql(): string {
  try {
    return localStorage.getItem(CLE_SQL) ?? SQL_DEFAUT
  } catch {
    return SQL_DEFAUT
  }
}

function ecrireSql(texte: string) {
  try {
    localStorage.setItem(CLE_SQL, texte)
  } catch {
    /* stockage indisponible */
  }
}

function extensionSql(tables: TableSchema[]) {
  const schema: Record<string, string[]> = {}
  for (const table of tables) schema[table.table] = table.colonnes.map((c) => c.nom)
  return sql({ dialect: SQLite, schema, upperCaseKeywords: true })
}

function themeEditeur() {
  const accent = jeton('accent') || 'currentColor'
  const texte = jeton('texte') || 'inherit'
  const doux = jeton('doux') || 'inherit'
  const succes = jeton('succes') || accent
  const style = HighlightStyle.define([
    { tag: tags.keyword, color: accent, fontWeight: '600' },
    { tag: tags.string, color: succes },
    { tag: [tags.number, tags.bool, tags.null], color: jeton('alerte') || accent },
    { tag: tags.comment, color: doux, fontStyle: 'italic' },
    { tag: [tags.typeName, tags.className, tags.propertyName], color: jeton('serie-1') || accent },
    { tag: tags.operator, color: texte },
  ])
  return [
    syntaxHighlighting(style),
    EditorView.theme(
      {
        '.cm-content': { fontFamily: 'var(--font-mono), ui-monospace, monospace', caretColor: accent, padding: '8px 0' },
        '.cm-gutters': { backgroundColor: jeton('surface-2') || 'transparent', color: doux, border: 'none' },
        '&.cm-focused': { outline: 'none' },
        '&.cm-focused .cm-cursor': { borderLeftColor: accent },
        '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: jeton('surface-2') || 'transparent' },
        '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
          backgroundColor: jeton('accent-doux') || 'transparent',
        },
        '.cm-tooltip, .cm-tooltip-autocomplete': {
          backgroundColor: jeton('surface') || 'transparent',
          color: texte,
          border: `1px solid ${jeton('bordure') || 'transparent'}`,
        },
        '.cm-tooltip-autocomplete ul li[aria-selected]': {
          backgroundColor: jeton('accent-doux') || 'transparent',
          color: texte,
        },
        '&': { height: '100%', backgroundColor: jeton('surface') || 'transparent', color: texte },
        '.cm-scroller': { overflow: 'auto' },
      },
      { dark: document.documentElement.dataset.theme === 'sombre' },
    ),
  ]
}

function classeBouton(actif: boolean): string {
  return `rounded px-3 py-1 text-sm disabled:opacity-40 ${actif ? 'bg-accent-doux text-accent' : 'bg-surface-2 text-doux'}`
}

export default function Console() {
  const schema = useApi<TableSchema[]>('/api/schema')
  const requetes = useApi<RequeteEnregistree[]>('/api/requetes')
  const parentRef = useRef<HTMLDivElement>(null)
  const vueRef = useRef<EditorView | null>(null)
  const sqlComp = useRef(new Compartment())
  const themeComp = useRef(new Compartment())
  const executerRef = useRef<() => void>(() => {})

  const [resultat, setResultat] = useState<ResultatSQL | null>(null)
  const [erreurSql, setErreurSql] = useState<string | null>(null)
  const [enCours, setEnCours] = useState(false)
  const [vue, setVue] = useState<Vue>('tableau')
  const [extra, setExtra] = useState<RequeteEnregistree[]>([])
  const [ouvert, setOuvert] = useState(false)
  const [nom, setNom] = useState('')
  const [erreurNom, setErreurNom] = useState<string | null>(null)
  const [tablesFermees, setTablesFermees] = useState<ReadonlySet<string>>(new Set())

  async function executer() {
    const texte = vueRef.current?.state.doc.toString() ?? lireSql()
    setEnCours(true)
    setErreurSql(null)
    try {
      const recu = await post<ResultatSQL>('/api/sql', { sql: texte })
      setResultat(recu)
      setVue('tableau')
    } catch (e) {
      setResultat(null)
      setErreurSql(e instanceof Error ? e.message : 'Erreur SQL')
    } finally {
      setEnCours(false)
    }
  }
  executerRef.current = () => {
    void executer()
  }

  useEffect(() => {
    const parent = parentRef.current
    if (!parent) return
    const editeur = new EditorView({
      parent,
      state: EditorState.create({
        doc: lireSql(),
        extensions: [
          basicSetup,
          EditorView.lineWrapping,
          sqlComp.current.of(extensionSql([])),
          themeComp.current.of(themeEditeur()),
          Prec.high(
            keymap.of([
              {
                key: 'Ctrl-Enter',
                preventDefault: true,
                run: () => {
                  executerRef.current()
                  return true
                },
              },
            ]),
          ),
          EditorView.updateListener.of((maj) => {
            if (maj.docChanged) ecrireSql(maj.state.doc.toString())
          }),
          EditorView.contentAttributes.of({ 'aria-label': 'Requête SQL' }),
        ],
      }),
    })
    vueRef.current = editeur
    const mesure = new ResizeObserver(() => editeur.requestMeasure())
    mesure.observe(parent)
    const surTheme = () => editeur.dispatch({ effects: themeComp.current.reconfigure(themeEditeur()) })
    window.addEventListener('observatoire:theme', surTheme)
    return () => {
      mesure.disconnect()
      window.removeEventListener('observatoire:theme', surTheme)
      editeur.destroy()
      vueRef.current = null
    }
  }, [])

  useEffect(() => {
    const editeur = vueRef.current
    if (!editeur || schema.statut !== 'ok') return
    editeur.dispatch({ effects: sqlComp.current.reconfigure(extensionSql(schema.donnees)) })
  }, [schema.statut, schema.donnees])

  function replierTable(table: string) {
    setTablesFermees((actuelles) => {
      const suivant = new Set(actuelles)
      if (suivant.has(table)) suivant.delete(table)
      else suivant.add(table)
      return suivant
    })
  }

  function inserer(texte: string) {
    const editeur = vueRef.current
    if (!editeur) return
    const sel = editeur.state.selection.main
    editeur.dispatch({
      changes: { from: sel.from, to: sel.to, insert: texte },
      selection: { anchor: sel.from + texte.length },
    })
    editeur.focus()
  }

  function charger(texte: string) {
    const editeur = vueRef.current
    if (!editeur) return
    editeur.dispatch({
      changes: { from: 0, to: editeur.state.doc.length, insert: texte },
      selection: { anchor: texte.length },
    })
    editeur.focus()
  }

  async function enregistrer(nomSoumis: string) {
    const texte = vueRef.current?.state.doc.toString() ?? lireSql()
    try {
      const cree = await post<RequeteEnregistree>('/api/requetes', { nom: nomSoumis, sql: texte })
      setExtra((liste) => [...liste.filter((r) => r.nom !== cree.nom), cree])
      setOuvert(false)
      setNom('')
      setErreurNom(null)
      requetes.recharger()
    } catch (e) {
      setErreurNom(e instanceof Error ? e.message : 'Enregistrement impossible')
    }
  }

  function soumettre(e: FormEvent) {
    e.preventDefault()
    const nettoye = nom.trim()
    if (!nomValide(nettoye)) {
      setErreurNom('Le nom ne peut contenir que des minuscules, des chiffres et des tirets.')
      return
    }
    void enregistrer(nettoye)
  }

  const base = requetes.statut === 'ok' ? requetes.donnees : []
  const noms = new Set(base.map((r) => r.nom))
  const toutes = [...base, ...extra.filter((r) => !noms.has(r.nom))]
  const peutGraphiquer = resultat != null && colonnesNumeriques(resultat).length > 0
  const indices = resultat ? new Set(colonnesNumeriques(resultat)) : new Set<number>()

  return (
    <div className="console-page flex min-h-[780px] flex-col gap-4">
      <Entete titre="Console SQL" description="Explorez vos données, posez vos questions et visualisez les résultats." />
      <div className="grid min-h-0 flex-1 grid-cols-[260px_minmax(0,1fr)] items-stretch gap-4">
        <aside className="flex min-h-0 flex-col gap-4">
          <Carte titre="Schéma" className="flex min-h-0 flex-1 flex-col overflow-hidden [&>header]:shrink-0">
            <div className="min-h-0 flex-1 overflow-y-auto">
              <Charge etat={schema} lignes={8}>
                {(tables) => (
                  <ul className="flex flex-col gap-2">
                    {tables.map((table) => {
                      const ouverte = !tablesFermees.has(table.table)
                      return (
                        <li key={table.table}>
                          <div className="flex items-center gap-1">
                            <button
                              type="button"
                              aria-expanded={ouverte}
                              aria-controls={`schema-${table.table}`}
                              aria-label={`${ouverte ? 'Replier' : 'Déplier'} ${table.table}`}
                              onClick={() => replierTable(table.table)}
                              className="w-4 shrink-0 text-center text-xs text-doux"
                            >
                              {ouverte ? '▾' : '▸'}
                            </button>
                            <button
                              type="button"
                              aria-label={`Insérer ${table.table}`}
                              onClick={() => inserer(table.table)}
                              className="truncate font-mono text-sm font-semibold text-texte hover:text-accent"
                            >
                              {table.table}
                            </button>
                          </div>
                          {ouverte && (
                            <ul id={`schema-${table.table}`} className="mt-1 flex flex-col pl-5">
                              {table.colonnes.map((colonne) => (
                                <li key={colonne.nom}>
                                  <button
                                    type="button"
                                    aria-label={`Colonne ${colonne.nom} de ${table.table}`}
                                    onClick={() => inserer(colonne.nom)}
                                    className="flex w-full items-baseline justify-between gap-2 rounded px-1 py-0.5 text-left font-mono text-xs hover:bg-surface-2"
                                  >
                                    <span className="truncate">{colonne.nom}</span>
                                    <span className="shrink-0 text-doux">{colonne.type}</span>
                                  </button>
                                </li>
                              ))}
                            </ul>
                          )}
                        </li>
                      )
                    })}
                  </ul>
                )}
              </Charge>
            </div>
          </Carte>

          <Carte titre="Requêtes enregistrées" className="flex min-h-0 flex-1 flex-col overflow-hidden [&>header]:shrink-0">
            <div className="shrink-0">
              {ouvert ? (
                <form noValidate onSubmit={soumettre} className="mb-3 flex flex-col gap-2">
                  <label className="flex flex-col gap-1 text-xs text-doux">
                    Nom de la requête
                    <input
                      autoFocus
                      value={nom}
                      onChange={(e) => setNom(e.target.value)}
                      className="rounded-md border border-bordure bg-surface px-2 py-1 font-mono text-sm text-texte"
                    />
                  </label>
                  {erreurNom && <ErreurAffichee erreur={new Error(erreurNom)} />}
                  <button type="submit" className={classeBouton(true)}>
                    Enregistrer
                  </button>
                </form>
              ) : (
                <button type="button" onClick={() => setOuvert(true)} className={`mb-3 ${classeBouton(false)}`}>
                  Enregistrer sous…
                </button>
              )}
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto">
              {requetes.statut === 'chargement' && <Chargement />}
              {requetes.statut === 'erreur' && <ErreurAffichee erreur={requetes.erreur} />}
              {toutes.length === 0 && requetes.statut === 'ok' && <Vide>Aucune requête enregistrée.</Vide>}
              <ul className="flex flex-col gap-1">
                {toutes.map((requete) => (
                  <li key={requete.nom}>
                    <button
                      type="button"
                      onClick={() => charger(requete.sql)}
                      className="w-full rounded-md px-2 py-1 text-left hover:bg-surface-2"
                    >
                      <span className="block font-mono text-sm">{requete.nom}</span>
                      <span className="block truncate text-xs text-doux">{requete.sql}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          </Carte>
        </aside>

        <div className="flex min-h-0 min-w-0 flex-col gap-3">
          <div className="h-[220px] max-h-[70vh] min-h-[120px] shrink-0 resize-y overflow-hidden rounded-lg border border-bordure focus-within:border-accent">
            <div ref={parentRef} className="h-full" />
          </div>
          {erreurSql && <ErreurAffichee erreur={new Error(erreurSql)} />}
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            <button type="button" onClick={() => void executer()} disabled={enCours} className={classeBouton(true)}>
              {enCours ? 'Exécution…' : 'Exécuter'}
            </button>
            <div role="group" aria-label="Présentation" className="flex gap-2">
              <button type="button" aria-pressed={vue === 'tableau'} onClick={() => setVue('tableau')} className={classeBouton(vue === 'tableau')}>
                Tableau
              </button>
              {(['barres', 'courbe', 'secteurs'] as const).map((forme) => (
                <button
                  key={forme}
                  type="button"
                  disabled={!peutGraphiquer}
                  title={peutGraphiquer ? undefined : 'Aucune colonne numérique'}
                  aria-pressed={vue === forme}
                  onClick={() => setVue(forme)}
                  className={classeBouton(vue === forme)}
                >
                  {forme === 'barres' ? 'Barres' : forme === 'courbe' ? 'Courbe' : 'Secteurs'}
                </button>
              ))}
            </div>
            <p className="text-xs text-doux">
              <kbd className="font-mono">Ctrl</kbd> + <kbd className="font-mono">Entrée</kbd> pour exécuter.
            </p>
            <p className="rounded-lg border border-bordure bg-surface-2 px-3 py-1 text-sm text-doux">
              Lecture seule : la base ne peut pas être modifiée d'ici.
            </p>
          </div>

          <Carte titre="Résultats" className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden [&>header]:shrink-0">
            <div className="min-h-0 flex-1 overflow-auto">
              {!resultat && <Vide>Exécutez une requête pour voir les lignes.</Vide>}
              {resultat && (
                <div className="space-y-3">
                  <p className="text-xs text-doux tabular-nums">
                    {nombre(resultat.duree_ms)} ms · {nombre(resultat.lignes.length)} {resultat.lignes.length > 1 ? 'lignes' : 'ligne'}
                  </p>
                  {resultat.tronque && (
                    <p role="status" className="rounded-lg border border-alerte/40 bg-alerte/10 px-3 py-2 text-sm text-alerte">
                      Résultat tronqué : seules les 1 000 premières lignes sont affichées.
                    </p>
                  )}
                  {vue === 'tableau' && resultat.lignes.length === 0 && <Vide>La requête n'a retourné aucune ligne.</Vide>}
                  {vue === 'tableau' && resultat.lignes.length > 0 && (
                    <table className="w-full border-collapse text-sm">
                      <thead>
                        <tr>
                          {resultat.colonnes.map((colonne, j) => (
                            <th
                              key={colonne}
                              className={`sticky top-0 bg-surface px-2 py-1 text-xs font-semibold text-doux ${indices.has(j) ? 'text-right' : 'text-left'}`}
                            >
                              {colonne}
                            </th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {resultat.lignes.map((ligne, i) => (
                          <tr key={i} className="border-t border-bordure">
                            {ligne.map((cellule, j) => (
                              <td key={j} className={`px-2 py-1 ${typeof cellule === 'number' ? 'text-right font-mono tabular-nums' : ''}`}>
                                {cellule === null ? <span className="text-doux">NULL</span> : String(cellule)}
                              </td>
                            ))}
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                  {vue !== 'tableau' && optionGraphique(resultat, vue) && (
                    <Graphique
                      aria-label={`Graphique en ${vue === 'barres' ? 'barres' : vue === 'courbe' ? 'courbe' : 'secteurs'} du résultat SQL`}
                      option={() => optionGraphique(resultat, vue) ?? {}}
                    />
                  )}
                </div>
              )}
            </div>
          </Carte>
        </div>
      </div>
    </div>
  )
}

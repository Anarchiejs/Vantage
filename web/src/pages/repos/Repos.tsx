import type { RepoCard } from '../../api/types'
import { useApi } from '../../api/useApi'
import { Card } from '../../ui/Card'
import { Empty, Load } from '../../ui/Status'
import { Header } from '../../ui/Header'
import { Link } from 'react-router'

export default function Repos() {
  const state = useApi<RepoCard[]>('/api/repos')
  return <div className="space-y-5">
    <Header title="Repository map" description="The path on each project card, and the path that was actually imported." />
    <Load state={state}>{cards => cards.length ? <div className="space-y-4">{cards.map(card => <Card key={card.id} title={card.name}>
      <p className="text-sm">Card path: <span className="font-mono break-all">{card.source_path || 'not set'}</span> {card.source_path && <span className="text-xs text-muted">{card.source_exists ? 'readable' : 'not readable'}</span>}</p>
      {card.branch && <p className="mt-1 text-xs text-muted">Branch {card.branch}</p>}
      {card.repos.length ? <ul className="mt-3 space-y-2 text-sm">{card.repos.map(repo => <li key={repo.path} className="font-mono break-all">{repo.path} <span className="text-xs text-muted">{repo.exists ? 'exists' : 'missing'}{repo.branch ? ` · ${repo.branch}` : ''}</span></li>)}</ul> : <Empty>No repository imported for this card.</Empty>}
      <Link className="mt-3 inline-block text-sm text-accent hover:underline" to={`/projects/${card.id}`}>Open the project</Link>
    </Card>)}</div> : <Empty>No project card.</Empty>}</Load>
  </div>
}

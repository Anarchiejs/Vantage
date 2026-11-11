import type { ReactNode } from 'react'
import type { ApiState } from '../api/useApi'

export function Loading({ lines = 3 }: { lines?: number }) {
  return (
    <div className="flex animate-pulse flex-col gap-2" aria-busy="true" aria-label="Loading">
      {Array.from({ length: lines }, (_, i) => (
        <div key={i} className="h-4 rounded bg-surface-2" style={{ width: `${90 - i * 15}%` }} />
      ))}
    </div>
  )
}

export function Empty({ children }: { children: ReactNode }) {
  return <p className="py-6 text-center text-sm text-muted">{children}</p>
}

export function ErrorMessage({ error }: { error: Error }) {
  return (
    <p role="alert" className="rounded-lg border border-danger/40 bg-danger/10 p-3 text-sm text-danger">
      {error.message}
    </p>
  )
}

/** Shows loading or an error, otherwise calls `children` with the data. */
export function Load<T>({ state, children, lines }: { state: ApiState<T>; children: (data: T) => ReactNode; lines?: number }) {
  if (state.status === 'loading') return <Loading lines={lines} />
  if (state.status === 'error') return <ErrorMessage error={state.error} />
  return <>{children(state.data)}</>
}

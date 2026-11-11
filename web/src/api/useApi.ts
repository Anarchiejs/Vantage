import { useCallback, useEffect, useState } from 'react'
import { get } from './client'

export type ApiState<T> =
  | { status: 'loading'; data?: undefined; error?: undefined }
  | { status: 'ok'; data: T; error?: undefined }
  | { status: 'error'; data?: undefined; error: Error }

/**
 * Loads a GET from the API. Reloads when `path` changes, and when the server announces an update
 * (the "observatoire:update" event emitted by the SSE connection in App.tsx). `path` null means load nothing.
 */
export function useApi<T>(path: string | null): ApiState<T> & { reload: () => void } {
  const [response, setResponse] = useState<{ path: string | null; state: ApiState<T> }>({ path: null, state: { status: 'loading' } })
  const [turn, setTurn] = useState(0)
  const reload = useCallback(() => setTurn((value) => value + 1), [])

  useEffect(() => {
    window.addEventListener('observatoire:update', reload)
    return () => window.removeEventListener('observatoire:update', reload)
  }, [reload])

  useEffect(() => {
    if (path === null) return
    const ctrl = new AbortController()
    get<T>(path, ctrl.signal)
      .then((data) => { if (!ctrl.signal.aborted) setResponse({ path, state: { status: 'ok', data } }) })
      .catch((error: Error) => {
        if (!ctrl.signal.aborted) setResponse({ path, state: { status: 'error', error } })
      })
    return () => ctrl.abort()
  }, [path, turn])

  const state: ApiState<T> = path !== null && response.path === path ? response.state : { status: 'loading' }
  return { ...state, reload }
}

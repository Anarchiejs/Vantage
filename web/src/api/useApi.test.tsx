import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { useApi } from './useApi'
afterEach(() => vi.unstubAllGlobals())
test('un changement de projet masque immédiatement les anciennes données', async () => {
  let finir!: (v: unknown) => void
  vi.stubGlobal('fetch', vi.fn((url: string) => url.endsWith('/1') ? Promise.resolve({ ok: true, json: async () => ({ nom: 'Premier' }) }) : new Promise(resolve => { finir = resolve })))
  const { result, rerender } = renderHook(({ url }) => useApi<{ nom: string }>(url), { initialProps: { url: '/api/projets/1' } })
  await waitFor(() => expect(result.current.donnees?.nom).toBe('Premier'))
  rerender({ url: '/api/projets/2' })
  expect(result.current.statut).toBe('chargement')
  expect(result.current.donnees).toBeUndefined()
  await act(async () => finir({ ok: true, json: async () => ({ nom: 'Second' }) }))
  await waitFor(() => expect(result.current.donnees?.nom).toBe('Second'))
})

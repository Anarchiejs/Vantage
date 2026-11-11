import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { useApi } from './useApi'
afterEach(() => vi.unstubAllGlobals())
test('switching projects hides the previous data immediately', async () => {
  let finish!: (v: unknown) => void
  vi.stubGlobal('fetch', vi.fn((url: string) => url.endsWith('/1') ? Promise.resolve({ ok: true, json: async () => ({ name: 'First' }) }) : new Promise(resolve => { finish = resolve })))
  const { result, rerender } = renderHook(({ url }) => useApi<{ name: string }>(url), { initialProps: { url: '/api/projects/1' } })
  await waitFor(() => expect(result.current.data?.name).toBe('First'))
  rerender({ url: '/api/projects/2' })
  expect(result.current.status).toBe('loading')
  expect(result.current.data).toBeUndefined()
  await act(async () => finish({ ok: true, json: async () => ({ name: 'Second' }) }))
  await waitFor(() => expect(result.current.data?.name).toBe('Second'))
})

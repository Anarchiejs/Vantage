// API client. In mock mode (`npm run dev:mock`, VITE_MOCK=1), GETs read public/mocks/<path>.json:
//   /api/home            → /mocks/home.json
//   /api/projects/3      → /mocks/projects/3.json
//   /api/search?q=x      → /mocks/search.json (parameters ignored)
// and POSTs read /mocks/<path>.post.json.

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message)
  }
}

const MOCK = import.meta.env.VITE_MOCK === '1'

function mockUrl(path: string, post: boolean): string {
  const withoutParams = path.split('?')[0].replace(/^\/api\//, '')
  return `/mocks/${withoutParams}${post ? '.post' : ''}.json`
}

async function read<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`
    try {
      const body = await response.json()
      if (body?.error) message = body.error
    } catch {
      /* body is not JSON */
    }
    throw new ApiError(message, response.status)
  }
  return response.json() as Promise<T>
}

export async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const url = MOCK ? mockUrl(path, false) : path
  return read<T>(await fetch(url, { signal }))
}

export async function post<T>(path: string, body: unknown, signal?: AbortSignal): Promise<T> {
  if (MOCK) return read<T>(await fetch(mockUrl(path, true), { signal }))
  return read<T>(
    await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal,
    }),
  )
}

/** Builds "/api/x?a=1&b=2", omitting empty values. */
export function withParams(path: string, params: Record<string, string | number | undefined | null>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) if (value !== undefined && value !== null && value !== '') search.set(key, String(value))
  const text = search.toString()
  return text ? `${path}?${text}` : path
}

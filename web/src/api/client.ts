// Client de l'API. En mode maquette (`npm run dev:mock`, VITE_MOCK=1), les GET lisent public/mocks/<chemin>.json :
//   /api/accueil            → /mocks/accueil.json
//   /api/projets/3          → /mocks/projets/3.json
//   /api/recherche?q=x      → /mocks/recherche.json (paramètres ignorés)
// et les POST lisent /mocks/<chemin>.post.json.

export class ErreurApi extends Error {
  constructor(
    message: string,
    readonly statut: number,
  ) {
    super(message)
  }
}

const MAQUETTE = import.meta.env.VITE_MOCK === '1'

function urlMaquette(chemin: string, post: boolean): string {
  const sansParams = chemin.split('?')[0].replace(/^\/api\//, '')
  return `/mocks/${sansParams}${post ? '.post' : ''}.json`
}

async function lire<T>(r: Response): Promise<T> {
  if (!r.ok) {
    let message = `${r.status} ${r.statusText}`
    try {
      const corps = await r.json()
      if (corps?.erreur) message = corps.erreur
    } catch {
      /* corps non JSON */
    }
    throw new ErreurApi(message, r.status)
  }
  return r.json() as Promise<T>
}

export async function get<T>(chemin: string, signal?: AbortSignal): Promise<T> {
  const url = MAQUETTE ? urlMaquette(chemin, false) : chemin
  return lire<T>(await fetch(url, { signal }))
}

export async function post<T>(chemin: string, corps: unknown, signal?: AbortSignal): Promise<T> {
  if (MAQUETTE) return lire<T>(await fetch(urlMaquette(chemin, true), { signal }))
  return lire<T>(
    await fetch(chemin, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(corps),
      signal,
    }),
  )
}

/** Construit « /api/x?a=1&b=2 » en omettant les valeurs vides. */
export function avecParams(chemin: string, params: Record<string, string | number | undefined | null>): string {
  const p = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== null && v !== '') p.set(k, String(v))
  const s = p.toString()
  return s ? `${chemin}?${s}` : chemin
}

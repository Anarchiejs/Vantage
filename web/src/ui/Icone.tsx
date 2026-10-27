const chemins: Record<string, string> = {
  accueil: 'M3 10 12 3l9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1Z',
  projets: 'M3 7V5a1 1 0 0 1 1-1h6l2 3h8a1 1 0 0 1 1 1v11a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1Z',
  technos: 'm8 4-6 8 6 8m8-16 6 8-6 8M14 3l-4 18',
  graphe: 'M9 5a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm12 14a3 3 0 1 1-6 0 3 3 0 0 1 6 0ZM9 19a3 3 0 1 1-6 0 3 3 0 0 1 6 0ZM6 8v8m2-9 8 10M9 19h6',
  recherche: 'M16 10a6 6 0 1 1-12 0 6 6 0 0 1 12 0Zm-1 5 6 6',
  console: 'm5 7 5 5-5 5m8 0h6',
  soleil: 'M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0ZM12 2v2m0 16v2M2 12h2m16 0h2M5 5l1 1m12 12 1 1M5 19l1-1M18 6l1-1',
  lune: 'M21 13A9 9 0 0 1 11 3a9 9 0 1 0 10 10Z',
}
export function Icone({ nom }: { nom: string }) {
  return <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={chemins[nom] ?? chemins.accueil} /></svg>
}

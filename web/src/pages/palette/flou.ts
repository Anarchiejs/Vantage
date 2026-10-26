export interface Libelle {
  id: string
  libelle: string
  detail: string
}

function plier(texte: string): string {
  return texte.toLocaleLowerCase('fr').normalize('NFD').replace(/\p{M}/gu, '')
}

/** Correspondance floue : chaque caractère de la requête apparaît dans l'ordre. */
export function scoreFlou(texte: string, requete: string): number | null {
  const cible = plier(texte)
  const aiguille = plier(requete).trim()
  if (!aiguille) return 0
  let depuis = 0
  let score = 0
  let suite = 0
  for (const caractere of aiguille) {
    const i = cible.indexOf(caractere, depuis)
    if (i < 0) return null
    suite = i === depuis ? suite + 1 : 1
    score += suite
    if (i === 0 || cible[i - 1] === ' ') score += 3
    depuis = i + 1
  }
  return score
}

export function ordonner<T extends Libelle>(commandes: T[], requete: string): T[] {
  const q = requete.trim()
  if (!q) return commandes
  const notes = commandes.flatMap((commande, i) => {
    const score = scoreFlou(`${commande.libelle} ${commande.detail}`, q)
    return score === null ? [] : [{ commande, score, i }]
  })
  notes.sort((a, b) => b.score - a.score || a.i - b.i)
  return notes.map((note) => note.commande)
}

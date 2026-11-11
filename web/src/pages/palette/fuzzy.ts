export interface Labeled {
  id: string
  label: string
  detail: string
}

function fold(text: string): string {
  return text.toLocaleLowerCase('en').normalize('NFD').replace(/\p{M}/gu, '')
}

/** Fuzzy match: every character of the query appears in order. */
export function fuzzyScore(text: string, query: string): number | null {
  const haystack = fold(text)
  const needle = fold(query).trim()
  if (!needle) return 0
  let from = 0
  let score = 0
  let run = 0
  for (const character of needle) {
    const index = haystack.indexOf(character, from)
    if (index < 0) return null
    run = index === from ? run + 1 : 1
    score += run
    if (index === 0 || haystack[index - 1] === ' ') score += 3
    from = index + 1
  }
  return score
}

export function rank<T extends Labeled>(commands: T[], query: string): T[] {
  const text = query.trim()
  if (!text) return commands
  const scored = commands.flatMap((command, index) => {
    const score = fuzzyScore(`${command.label} ${command.detail}`, text)
    return score === null ? [] : [{ command, score, index }]
  })
  scored.sort((a, b) => b.score - a.score || a.index - b.index)
  return scored.map((item) => item.command)
}

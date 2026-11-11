const START = '\uE000'
const END = '\uE001'

/**
 * The server wraps matched terms in `<mark>`. Escape all HTML,
 * then restore only those two tags.
 */
export function highlightedHtml(raw: string): string {
  const reserved = raw.replaceAll('<mark>', START).replaceAll('</mark>', END)
  const escaped = reserved
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
  return escaped.replaceAll(START, '<mark>').replaceAll(END, '</mark>')
}

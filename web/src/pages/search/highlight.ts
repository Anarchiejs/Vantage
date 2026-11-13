const START = '\uE000'
const END = '\uE001'

/**
 * The server wraps matched terms in U+E000 / U+E001.
 * Escape all HTML, then turn those sentinels into mark tags.
 */
export function highlightedHtml(raw: string): string {
  const escaped = raw
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
  return escaped.replaceAll(START, '<mark>').replaceAll(END, '</mark>')
}

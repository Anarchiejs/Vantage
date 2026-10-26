const DEBUT = '\uE000'
const FIN = '\uE001'

/**
 * Le serveur entoure les termes trouvés de `<mark>`. On échappe tout le HTML,
 * puis on rétablit uniquement ces deux balises.
 */
export function htmlSurligne(brut: string): string {
  const reserve = brut.replaceAll('<mark>', DEBUT).replaceAll('</mark>', FIN)
  const echappe = reserve
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
  return echappe.replaceAll(DEBUT, '<mark>').replaceAll(FIN, '</mark>')
}

import { test as base, expect } from '@playwright/test'

/** Échoue le parcours si le navigateur signale une erreur pendant le test. */
export const test = base.extend({
  page: async ({ page }, use, info) => {
    const erreurs: string[] = []
    page.on('pageerror', (err) => erreurs.push(`exception: ${err.message}`))
    page.on('console', (msg) => {
      if (msg.type() !== 'error') return
      const texte = msg.text()
      // Chromium journalise tout seul le 403 renvoyé pour ATTACH. Le parcours vérifie
      // le message affiché ; ce n'est pas un console.error de l'application.
      if (
        info.title.includes('ATTACH') &&
        texte.startsWith('Failed to load resource: the server responded with a status of 403')
      ) {
        return
      }
      erreurs.push(texte)
    })
    await use(page)
    expect(erreurs, `Console du navigateur (${info.title}):\n${erreurs.join('\n')}`).toEqual([])
  },
})

export { expect }

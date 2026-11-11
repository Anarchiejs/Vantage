import { test as base, expect } from '@playwright/test'

/** Fail the flow if the browser reports an error during the test. */
export const test = base.extend({
  page: async ({ page }, use, info) => {
    const errors: string[] = []
    page.on('pageerror', (err) => errors.push(`exception: ${err.message}`))
    page.on('console', (msg) => {
      if (msg.type() !== 'error') return
      const text = msg.text()
      // Chromium logs the 403 returned for ATTACH on its own. The flow checks
      // the message shown in the page; that is not an application console.error.
      if (
        info.title.includes('ATTACH') &&
        text.startsWith('Failed to load resource: the server responded with a status of 403')
      ) {
        return
      }
      errors.push(text)
    })
    await use(page)
    expect(errors, `Browser console (${info.title}):\n${errors.join('\n')}`).toEqual([])
  },
})

export { expect }

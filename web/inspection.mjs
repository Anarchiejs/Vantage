import { chromium } from '@playwright/test'
const browser = await chromium.launch({ headless: true })
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 1 })
const errors = []
page.on('pageerror', (error) => errors.push(error.message))
const projects = await (await page.request.get('http://127.0.0.1:5173/api/projects')).json()
const id = projects.find((project) => project.name === 'Observatoire').id
const routes = [['home', '/'], ['projects', '/projects'], ['project', `/projects/${id}`], ['tech', '/tech'], ['graph', '/graph'], ['search', '/search'], ['console', '/console']]
for (const [name, route] of routes) {
  await page.goto(`http://127.0.0.1:5173${route}`)
  await page.locator('h1').waitFor()
  await page.waitForTimeout(700)
  await page.screenshot({ path: `.orchestra/layout-${name}.png`, fullPage: true })
  console.log(name, JSON.stringify({ overflow: await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), charts: await page.locator('[role=img] canvas').count() }))
}
await page.goto('http://127.0.0.1:5173')
await page.getByRole('button', { name: /Dark theme/ }).click()
await page.waitForTimeout(400)
await page.screenshot({ path: '.orchestra/layout-dark.png', fullPage: true })
await page.getByRole('button', { name: /Light theme/ }).click()
await page.setViewportSize({ width: 390, height: 844 })
for (const [name, route] of routes) {
  await page.goto(`http://127.0.0.1:5173${route}`)
  await page.locator('h1').waitFor()
  await page.waitForTimeout(400)
  await page.screenshot({ path: `.orchestra/mobile-${name}.png`, fullPage: true })
  console.log('mobile', name, JSON.stringify({ overflow: await page.evaluate(() => document.documentElement.scrollWidth > innerWidth) }))
}
console.log(JSON.stringify({ errors }))
await browser.close()

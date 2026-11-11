import { chromium } from '@playwright/test'
import assert from 'node:assert/strict'
const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
const base = 'http://127.0.0.1:5173'
async function option(label) {
  await page.getByRole('img', { name: label, exact: true }).locator('canvas').first().waitFor()
  return page.evaluate(async (label) => {
    const entry = performance.getEntriesByType('resource').find((item) => item.name.includes('/echarts.js'))
    const ec = await import(entry.name)
    return JSON.parse(JSON.stringify(ec.getInstanceByDom(document.querySelector(`[aria-label="${label}"]`)).getOption()))
  }, label)
}
await page.goto(base + '/tech')
const api = await (await page.request.get(base + '/api/technologies')).json()
const graph = await option('Commits per technology over twelve months')
for (const series of graph.series) for (let i = 0; i < graph.xAxis[0].data.length; i++) {
  const month = graph.xAxis[0].data[i]
  const expected = api.trend.filter((point) => point.technology === series.name && point.month.slice(0, 7) === month).reduce((sum, point) => sum + point.commits, 0)
  assert.equal(series.data[i], expected, series.name + ' ' + month)
}
console.log('PASS: ' + graph.series.length + ' lines x 12 months compared with the live API')
await page.getByLabel('Diagram project').selectOption('Observatoire')
const sankey = await option('Relationships between projects and technologies')
assert.equal(sankey.series[0].links.length, api.sankey.links.filter((link) => link.source === 'Observatoire').length)
console.log('PASS: live Sankey filter')
await page.goto(base)
const home = await (await page.request.get(base + '/api/home')).json()
for (const n of [30, 90, 365]) {
  await page.getByRole('button', { name: n === 365 ? '1 year' : `${n} days`, exact: true }).click()
  const opt = await option(`Commits per day over ${n} days`)
  assert.deepEqual(opt.series[0].data, home.calendar.slice(-n).map((day) => day.commits))
  assert.deepEqual(opt.xAxis[0].data, home.calendar.slice(-n).map((day) => day.day))
}
await page.getByLabel('Activity measure').selectOption('sessions')
const sessions = await option('Sessions per day over 365 days')
assert.deepEqual(sessions.series[0].data, home.calendar.map((day) => day.sessions))
await page.getByRole('button', { name: 'Dark theme' }).click()
await page.setViewportSize({ width: 390, height: 844 })
const resized = await option('Sessions per day over 365 days')
assert.deepEqual(resized.series[0].data, sessions.series[0].data)
console.log('PASS: commits for 30/90/365 days, sessions, theme, and resize leave the values unchanged')
await browser.close()

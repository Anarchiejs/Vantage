import { chromium } from '@playwright/test'
import assert from 'node:assert/strict'
const browser=await chromium.launch(); const page=await browser.newPage({viewport:{width:1440,height:1000}})
const base='http://127.0.0.1:5173'
async function option(label) {
 await page.getByRole('img',{name:label,exact:true}).locator('canvas').first().waitFor()
 return page.evaluate(async label=>{
  const ec=await import(performance.getEntriesByType('resource').find(e=>e.name.includes('/echarts.js')).name)
  return JSON.parse(JSON.stringify(ec.getInstanceByDom(document.querySelector(`[aria-label="${label}"]`)).getOption()))
 },label)
}
await page.goto(base+'/technos')
const api=await(await page.request.get(base+'/api/technos')).json()
const graph=await option('Commits par technologie sur douze mois')
for (const serie of graph.series) for (let i=0;i<graph.xAxis[0].data.length;i++) {
 const month=graph.xAxis[0].data[i]
 const expected=api.tendance.filter(p=>p.techno===serie.name&&p.mois.slice(0,7)===month).reduce((s,p)=>s+p.commits,0)
 assert.equal(serie.data[i],expected,serie.name+' '+month)
}
console.log('PASS: '+graph.series.length+' courbes x 12 mois comparées aux données API réelles')
await page.getByLabel('Projet du diagramme').selectOption('Observatoire')
const sankey=await option('Relations entre projets et technologies')
assert.equal(sankey.series[0].links.length,api.sankey.liens.filter(l=>l.source==='Observatoire').length)
console.log('PASS: filtre Sankey réel')
await page.goto(base)
const home=await(await page.request.get(base+'/api/accueil')).json()
for (const n of [30,90,365]) {
 await page.getByRole('button',{name:n===365?'1 an':`${n} jours`,exact:true}).click()
 const opt=await option(`Commits par jour sur ${n} jours`)
 assert.deepEqual(opt.series[0].data,home.calendrier.slice(-n).map(j=>j.commits))
 assert.deepEqual(opt.xAxis[0].data,home.calendrier.slice(-n).map(j=>j.jour))
}
await page.getByLabel('Mesure de l’activité').selectOption('sessions')
const sessions=await option('Sessions par jour sur 365 jours')
assert.deepEqual(sessions.series[0].data,home.calendrier.map(j=>j.sessions))
await page.getByRole('button',{name:'Thème sombre ☾'}).click()
await page.setViewportSize({width:390,height:844})
const resized=await option('Sessions par jour sur 365 jours')
assert.deepEqual(resized.series[0].data,sessions.series[0].data)
console.log('PASS: commits 30/90/365 jours, sessions, thème et redimensionnement sans altération des valeurs')
await browser.close()

import type { EChartsOption } from 'echarts'
import type { PartLangage, PointLangage, VolumeSemaine } from '../../api/types'
import { series } from '../../theme/theme'
export function optionVolume(volume: VolumeSemaine[]): EChartsOption {
  return { color: series(), tooltip: { trigger: 'axis' }, legend: { data: ['Ajouts', 'Suppressions'] }, xAxis: { type: 'category', data: volume.map(v => v.semaine) }, yAxis: { type: 'value' }, series: [
    { name: 'Ajouts', type: 'bar', stack: 'volume', data: volume.map(v => v.ajouts) },
    { name: 'Suppressions', type: 'bar', stack: 'volume', data: volume.map(v => -v.suppressions) },
  ] }
}
export function optionLangages(langages: PartLangage[]): EChartsOption {
  return { xAxis: { show: false }, yAxis: { show: false }, color: series(), tooltip: { trigger: 'item', renderMode: 'richText' }, legend: { bottom: 0 }, series: [{ type: 'pie', radius: ['40%', '65%'], center: ['50%', '43%'], label: { show: false }, data: langages.map(l => ({ name: l.langage, value: l.lignes_code })) }] }
}
export function optionEvolution(points: PointLangage[]): EChartsOption {
  const dates = [...new Set(points.map(p => p.date_mesure))].sort()
  const langages = [...new Set(points.map(p => p.langage))]
  return { color: series(), tooltip: { trigger: 'axis', renderMode: 'richText' }, legend: { data: langages }, xAxis: { type: 'category', data: dates }, yAxis: { type: 'value' }, series: langages.map(nom => ({ name: nom, type: 'line', data: dates.map(d => points.find(p => p.date_mesure === d && p.langage === nom)?.lignes_code ?? null) })) }
}


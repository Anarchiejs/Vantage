import type { EChartsOption } from 'echarts'
import type { LanguagePoint, LanguageShare, WeeklyVolume } from '../../api/types'
import { series, token } from '../../theme/theme'
export function volumeOption(volume: WeeklyVolume[]): EChartsOption {
  volume = [...volume].sort((a, b) => a.week.localeCompare(b.week))
  return { color: [token('series-1'), token('danger')], tooltip: { trigger: 'axis', valueFormatter: value => typeof value === 'number' ? Math.abs(value).toLocaleString('en-US') : String(value) }, legend: { data: ['Additions', 'Deletions'] }, xAxis: { type: 'category', data: volume.map(item => item.week), axisLabel: { formatter: (value: string) => new Date(`${value}T12:00:00`).toLocaleDateString('en-US', { day: 'numeric', month: 'short' }), hideOverlap: true } }, yAxis: { type: 'value' }, series: [
    { name: 'Additions', type: 'bar', stack: 'volume', data: volume.map(item => item.additions) },
    { name: 'Deletions', type: 'bar', stack: 'volume', data: volume.map(item => -item.deletions) },
  ] }
}
export function languagesOption(languages: LanguageShare[]): EChartsOption {
  return { xAxis: { show: false }, yAxis: { show: false }, color: series(), tooltip: { trigger: 'item', renderMode: 'richText' }, legend: { bottom: 0, type: 'scroll' }, series: [{ type: 'pie', radius: ['40%', '65%'], center: ['50%', '43%'], label: { show: false }, data: languages.map(item => ({ name: item.language, value: item.code_lines })) }] }
}
export function evolutionOption(points: LanguagePoint[]): EChartsOption {
  const dates = [...new Set(points.map(point => point.measured_on))].sort()
  const languages = [...new Set(points.map(point => point.language))]
  return { color: series(), tooltip: { trigger: 'axis', renderMode: 'richText' }, legend: { data: languages, type: 'scroll' }, xAxis: { type: 'category', data: dates }, yAxis: { type: 'value' }, series: languages.map(name => ({ name, type: 'line', data: dates.map(day => points.find(point => point.measured_on === day && point.language === name)?.code_lines ?? null) })) }
}

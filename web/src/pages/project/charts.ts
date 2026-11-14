import type { EChartsOption } from 'echarts'
import type { LanguagePoint, WeeklyVolume } from '../../api/types'
import { series, token } from '../../theme/theme'
/** Diverging bars: additions above zero, deletions below, so both stay visible. */
export function volumeOption(volume: WeeklyVolume[]): EChartsOption {
  volume = [...volume].sort((a, b) => a.week.localeCompare(b.week))
  const bar = { type: 'bar' as const, stack: 'volume', barMaxWidth: 22 }
  return { color: [token('series-1'), token('danger')], tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, valueFormatter: value => typeof value === 'number' ? Math.abs(value).toLocaleString('en-US') : String(value) }, legend: { data: ['Additions', 'Deletions'], right: 0 }, xAxis: { type: 'category', data: volume.map(item => item.week), axisTick: { show: false }, axisLabel: { formatter: (value: string) => new Date(`${value}T12:00:00`).toLocaleDateString('en-US', { day: 'numeric', month: 'short' }), hideOverlap: true } }, yAxis: { type: 'value', axisLabel: { formatter: (value: number) => Math.abs(value).toLocaleString('en-US') } }, series: [
    { ...bar, name: 'Additions', itemStyle: { borderRadius: [3, 3, 0, 0] }, data: volume.map(item => item.additions) },
    { ...bar, name: 'Deletions', itemStyle: { borderRadius: [0, 0, 3, 3] }, data: volume.map(item => -item.deletions), markLine: { silent: true, symbol: 'none', label: { show: false }, lineStyle: { color: token('muted'), type: 'solid', width: 1 }, data: [{ yAxis: 0 }] } },
  ] }
}
export function evolutionOption(points: LanguagePoint[]): EChartsOption {
  const dates = [...new Set(points.map(point => point.measured_on))].sort()
  const languages = [...new Set(points.map(point => point.language))]
  return { color: series(), tooltip: { trigger: 'axis', renderMode: 'richText' }, legend: { data: languages, type: 'scroll' }, xAxis: { type: 'category', data: dates }, yAxis: { type: 'value' }, series: languages.map(name => ({ name, type: 'line', data: dates.map(day => points.find(point => point.measured_on === day && point.language === name)?.code_lines ?? null) })) }
}

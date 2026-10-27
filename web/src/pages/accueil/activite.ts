import type { EChartsOption } from 'echarts'
import type { JourActivite } from '../../api/types'
import { jeton } from '../../theme/theme'
import { date } from '../../ui/format'

export function optionActivite(jours: JourActivite[], mesure: 'commits' | 'sessions'): EChartsOption {
  const lignes = [...jours].sort((a, b) => a.jour.localeCompare(b.jour))
  return {
    tooltip: { trigger: 'axis', renderMode: 'richText' },
    grid: { top: 20, left: 12, right: 18, bottom: 22, containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: lignes.map(j => j.jour), axisLabel: { formatter: (v: string) => date(v).replace(/ \d{4}$/, ''), hideOverlap: true } },
    yAxis: { type: 'value', min: 0, minInterval: 1 },
    series: [{ name: mesure === 'commits' ? 'Commits' : 'Sessions', type: 'line', showSymbol: lignes.length < 15, symbolSize: 6,
      lineStyle: { width: 2.5 }, itemStyle: { color: jeton('serie-1') }, areaStyle: { opacity: .1 }, data: lignes.map(j => j[mesure]) }],
  }
}

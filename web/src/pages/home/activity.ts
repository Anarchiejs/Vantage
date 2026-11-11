import type { EChartsOption } from 'echarts'
import type { ActivityDay } from '../../api/types'
import { token } from '../../theme/theme'
import { date } from '../../ui/format'

export function activityOption(days: ActivityDay[], measure: 'commits' | 'sessions'): EChartsOption {
  const rows = [...days].sort((a, b) => a.day.localeCompare(b.day))
  return {
    tooltip: { trigger: 'axis', renderMode: 'richText' },
    grid: { top: 20, left: 12, right: 18, bottom: 22, containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: rows.map(day => day.day), axisLabel: { formatter: (value: string) => date(value).replace(/, \d{4}$/, ''), hideOverlap: true } },
    yAxis: { type: 'value', min: 0, minInterval: 1 },
    series: [{ name: measure === 'commits' ? 'Commits' : 'Sessions', type: 'line', showSymbol: rows.length < 15, symbolSize: 6,
      lineStyle: { width: 2.5 }, itemStyle: { color: token('series-1') }, areaStyle: { opacity: .1 }, data: rows.map(day => day[measure]) }],
  }
}

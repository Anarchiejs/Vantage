import type { EChartsOption } from 'echarts'
import type { ActivityDay } from '../../api/types'
import { token } from '../../theme/theme'
import { date } from '../../ui/format'

/** Daily counts are discrete, so bars: a line between days drew spikes that read as noise. */
export function activityOption(days: ActivityDay[], measure: 'commits' | 'sessions'): EChartsOption {
  const rows = [...days].sort((a, b) => a.day.localeCompare(b.day))
  return {
    tooltip: { trigger: 'axis', renderMode: 'richText', axisPointer: { type: 'shadow' } },
    legend: { show: false }, // one series, already named by the measure picker
    grid: { top: 16, left: 12, right: 18, bottom: 22, containLabel: true },
    xAxis: { type: 'category', data: rows.map(day => day.day), axisTick: { show: false }, axisLabel: { formatter: (value: string) => date(value).replace(/, \d{4}$/, ''), hideOverlap: true } },
    yAxis: { type: 'value', min: 0, minInterval: 1 },
    series: [{ name: measure === 'commits' ? 'Commits' : 'Sessions', type: 'bar', barMaxWidth: 14, barCategoryGap: '25%',
      itemStyle: { color: token(measure === 'commits' ? 'series-1' : 'series-2'), borderRadius: [3, 3, 0, 0] }, data: rows.map(day => day[measure]) }],
  }
}

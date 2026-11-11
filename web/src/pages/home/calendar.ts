import type { EChartsOption } from 'echarts'
import type { ActivityDay } from '../../api/types'
import { heat, token } from '../../theme/theme'
import { date, number } from '../../ui/format'

/** Zero, then four quantile buckets, without splitting identical values. */
export function calendarOption(days: ActivityDay[]): EChartsOption {
  const rows = [...days].sort((a, b) => a.day.localeCompare(b.day)).slice(-365)
  const positive = rows.map(day => day.commits).filter(n => n > 0).sort((a, b) => a - b)
  const thresholds = [0.25, 0.5, 0.75].map(q => positive[Math.max(0, Math.ceil(positive.length * q) - 1)] ?? 0)
  const colors = heat()
  return {
    xAxis: { show: false }, yAxis: { show: false },
    tooltip: { renderMode: 'richText', formatter: point => {
      const item = Array.isArray(point) ? point[0] : point
      const day = rows.find(row => row.day === (item.value as unknown[])?.[0])
      return day ? `${date(day.day)}\n${number(day.commits)} commits · +${number(day.additions)} / −${number(day.deletions)} lines\n${number(day.sessions)} sessions\n${day.projects.join(', ') || 'No project'}` : ''
    } },
    calendar: { range: rows.length ? [rows[0].day, rows.at(-1)!.day] : new Date().getFullYear(), cellSize: ['auto', 19], left: 44, right: 20, top: 35, bottom: 45,
      itemStyle: { borderColor: token('surface'), borderWidth: 3 }, splitLine: { show: false }, yearLabel: { show: false },
      dayLabel: { firstDay: 1, nameMap: ['S', 'M', 'T', 'W', 'T', 'F', 'S'], color: token('muted') },
      monthLabel: { nameMap: ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'], color: token('muted') } },
    visualMap: { type: 'piecewise', orient: 'horizontal', bottom: 0, right: 0, dimension: 1, textStyle: { color: token('muted') },
      pieces: colors.map((color, value) => ({ value, color, label: ['None', 'Low', 'Moderate', 'High', 'Peak'][value] })) },
    series: [{ type: 'heatmap', coordinateSystem: 'calendar', data: rows.map(day => [day.day, day.commits === 0 ? 0 : 1 + thresholds.filter(threshold => day.commits > threshold).length]) }],
  }
}

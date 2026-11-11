import type { EChartsOption } from 'echarts'
import type { SQLResult } from '../../api/types'

export type ChartShape = 'bars' | 'line' | 'pie'

/** Indexes of numeric columns after the first one (the X axis). */
export function numericColumns(result: SQLResult): number[] {
  return result.columns.map((_, index) => index).filter((index) => {
    if (index === 0) return false
    let seen = false
    for (const row of result.rows) {
      const value = row[index]
      if (value === null || value === undefined) continue
      if (typeof value !== 'number') return false
      seen = true
    }
    return seen
  })
}

/** Bars, line, or pie. Null when there is no numeric column. */
export function chartOption(result: SQLResult, shape: ChartShape): EChartsOption | null {
  const indexes = numericColumns(result)
  if (indexes.length === 0) return null
  const labels = result.rows.map((row) => (row[0] == null ? 'NULL' : String(row[0])))
  if (shape === 'pie') {
    const index = indexes[0]
    return {
      tooltip: { trigger: 'item' },
      series: [{
        type: 'pie',
        name: result.columns[index],
        data: result.rows.map((row, rowIndex) => ({
          name: labels[rowIndex],
          value: typeof row[index] === 'number' ? row[index] : 0,
        })),
      }],
    }
  }
  return {
    tooltip: { trigger: 'axis' },
    legend: {},
    xAxis: { type: 'category', data: labels },
    yAxis: { type: 'value' },
    series: indexes.map((index) => ({
      name: result.columns[index],
      type: shape === 'line' ? 'line' : 'bar',
      data: result.rows.map((row) => (typeof row[index] === 'number' ? row[index] : null)),
    })),
  }
}

/** File name of a saved query: `[a-z0-9-]+`. */
export function validName(name: string): boolean {
  return /^[a-z0-9-]+$/.test(name)
}

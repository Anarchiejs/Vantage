import { useEffect, useRef } from 'react'
import { init, use as registerECharts, type ECharts, type ECElementEvent } from 'echarts/core'
import { BarChart, GraphChart, HeatmapChart, LineChart, PieChart, SankeyChart } from 'echarts/charts'
import {
  AxisPointerComponent,
  CalendarComponent,
  GridComponent,
  LegendComponent,
  LegendScrollComponent,
  TooltipComponent,
  VisualMapComponent,
  VisualMapPiecewiseComponent,
} from 'echarts/components'
import { LabelLayout, LegacyGridContainLabel } from 'echarts/features'
import { CanvasRenderer } from 'echarts/renderers'
import type { EChartsOption } from 'echarts'
import { series, token } from '../theme/theme'

registerECharts([
  BarChart,
  LineChart,
  PieChart,
  SankeyChart,
  GraphChart,
  HeatmapChart,
  GridComponent,
  TooltipComponent,
  AxisPointerComponent,
  LegendComponent,
  LegendScrollComponent,
  VisualMapComponent,
  VisualMapPiecewiseComponent,
  CalendarComponent,
  LabelLayout,
  LegacyGridContainLabel,
  CanvasRenderer,
])

/** Options shared by both themes, recomputed on every theme change. */
function base(): EChartsOption {
  const ink = token('ink')
  const muted = token('muted')
  const line = token('line')
  return {
    color: series(),
    backgroundColor: 'transparent',
    textStyle: { color: ink, fontFamily: getComputedStyle(document.body).fontFamily },
    tooltip: {
      confine: true,
      renderMode: 'richText',
      backgroundColor: token('surface'),
      borderColor: line,
      textStyle: { color: ink },
    },
    animationDuration: 300,
    animation: !window.matchMedia?.('(prefers-reduced-motion: reduce)').matches,
    legend: { type: 'scroll', textStyle: { color: muted }, top: 0, itemWidth: 12, itemHeight: 8 },
    grid: { left: 16, right: 20, top: 48, bottom: 28, containLabel: true },
    xAxis: { axisLine: { lineStyle: { color: line } }, axisLabel: { color: muted }, splitLine: { show: false } },
    yAxis: { axisLine: { show: false }, axisLabel: { color: muted }, splitLine: { lineStyle: { color: line } } },
  }
}

/**
 * ECharts chart that follows the theme and the size of its container.
 * `option` is merged over the base options; pass a function to read theme tokens
 * (for example `() => ({ visualMap: { inRange: { color: heat() } } })`).
 */
export function Chart({
  option,
  height = 260,
  onClick,
  className = '',
  'aria-label': label,
}: {
  option: EChartsOption | (() => EChartsOption)
  height?: number
  onClick?: (event: ECElementEvent) => void
  className?: string
  'aria-label'?: string
}) {
  const ref = useRef<HTMLDivElement>(null)
  const chart = useRef<ECharts | null>(null)

  useEffect(() => {
    if (!ref.current) return
    const instance = init(ref.current, undefined, { renderer: 'canvas' })
    chart.current = instance
    let frame = 0
    const observer = new ResizeObserver(() => { cancelAnimationFrame(frame); frame = requestAnimationFrame(() => { if (!instance.isDisposed()) instance.resize() }) })
    observer.observe(ref.current)
    return () => {
      observer.disconnect()
      cancelAnimationFrame(frame)
      instance.dispose()
      chart.current = null
    }
  }, [])

  useEffect(() => {
    const apply = () => {
      const instance = chart.current
      if (!instance) return
      const next = typeof option === 'function' ? option() : option
      const defaults = base()
      // Preserve themed axes and tooltips when a page supplies data or a formatter.
      // Non-cartesian charts must not inherit a pair of unrelated axes.
      const items = Array.isArray(next.series) ? next.series : next.series ? [next.series] : []
      const cartesian = items.some(item => ['line', 'bar', 'scatter'].includes(String(item.type)))
      const merged = { ...defaults, ...next } as EChartsOption
      for (const key of ['tooltip', 'legend', 'grid', 'xAxis', 'yAxis'] as const) {
        const specific = next[key]
        if (!Array.isArray(specific)) {
          const common = defaults[key] as Record<string, unknown>
          const value = specific as Record<string, unknown> | undefined
          const combined = { ...common, ...value }
          for (const child of ['axisLabel', 'axisLine', 'splitLine', 'textStyle']) {
            if (common?.[child] || value?.[child]) combined[child] = { ...(common?.[child] as object), ...(value?.[child] as object) }
          }
          Object.assign(merged, { [key]: combined })
        }
      }
      if (!cartesian) { delete merged.xAxis; delete merged.yAxis; delete merged.grid }
      if (next.legend && !Array.isArray(next.legend) && next.legend.bottom !== undefined && !Array.isArray(merged.legend)) {
        delete merged.legend!.top
      }
      instance.setOption(merged, { notMerge: true })
    }
    apply()
    window.addEventListener('observatoire:theme', apply)
    return () => window.removeEventListener('observatoire:theme', apply)
  }, [option])

  useEffect(() => {
    const instance = chart.current
    if (!instance || !onClick) return
    instance.on('click', onClick)
    return () => {
      instance.off('click', onClick)
    }
  }, [onClick])

  return <div ref={ref} role="img" aria-label={label} className={className} style={{ height, width: '100%' }} />
}

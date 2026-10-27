import { useEffect, useRef } from 'react'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import { jeton, series } from '../theme/theme'

/** Options communes aux deux thèmes, recalculées à chaque changement de thème. */
function base(): EChartsOption {
  const texte = jeton('texte')
  const doux = jeton('doux')
  const bordure = jeton('bordure')
  return {
    color: series(),
    backgroundColor: 'transparent',
    textStyle: { color: texte, fontFamily: getComputedStyle(document.body).fontFamily },
    tooltip: {
      confine: true,
      renderMode: 'richText',
      backgroundColor: jeton('surface'),
      borderColor: bordure,
      textStyle: { color: texte },
    },
    animationDuration: 300,
    animation: !window.matchMedia?.('(prefers-reduced-motion: reduce)').matches,
    legend: { type: 'scroll', textStyle: { color: doux }, top: 0, itemWidth: 12, itemHeight: 8 },
    grid: { left: 16, right: 20, top: 48, bottom: 28, containLabel: true },
    xAxis: { axisLine: { lineStyle: { color: bordure } }, axisLabel: { color: doux }, splitLine: { show: false } },
    yAxis: { axisLine: { show: false }, axisLabel: { color: doux }, splitLine: { lineStyle: { color: bordure } } },
  }
}

/**
 * Graphique ECharts qui suit le thème et la taille de son conteneur.
 * `option` est fusionnée par-dessus les options de base ; passer une fonction pour lire les jetons du thème
 * (ex. `() => ({ visualMap: { inRange: { color: chaleur() } } })`).
 */
export function Graphique({
  option,
  hauteur = 260,
  surClic,
  className = '',
  'aria-label': libelle,
}: {
  option: EChartsOption | (() => EChartsOption)
  hauteur?: number
  surClic?: (p: echarts.ECElementEvent) => void
  className?: string
  'aria-label'?: string
}) {
  const ref = useRef<HTMLDivElement>(null)
  const graph = useRef<echarts.ECharts | null>(null)

  useEffect(() => {
    if (!ref.current) return
    const g = echarts.init(ref.current, undefined, { renderer: 'canvas' })
    graph.current = g
    let frame = 0
    const obs = new ResizeObserver(() => { cancelAnimationFrame(frame); frame = requestAnimationFrame(() => { if (!g.isDisposed()) g.resize() }) })
    obs.observe(ref.current)
    return () => {
      obs.disconnect()
      cancelAnimationFrame(frame)
      g.dispose()
      graph.current = null
    }
  }, [])

  useEffect(() => {
    const appliquer = () => {
      const g = graph.current
      if (!g) return
      const o = typeof option === 'function' ? option() : option
      const defaults = base()
      // Preserve themed axes/tooltips when a page supplies data or a formatter.
      // Non-cartesian charts must not inherit a pair of unrelated axes.
      const s = Array.isArray(o.series) ? o.series : o.series ? [o.series] : []
      const cartesien = s.some(v => ['line', 'bar', 'scatter'].includes(String(v.type)))
      const fusion = { ...defaults, ...o } as EChartsOption
      for (const cle of ['tooltip', 'legend', 'grid', 'xAxis', 'yAxis'] as const) {
        const specifique = o[cle]
        if (!Array.isArray(specifique)) {
          const commun = defaults[cle] as Record<string, unknown>
          const valeur = specifique as Record<string, unknown> | undefined
          const ensemble = { ...commun, ...valeur }
          for (const sousCle of ['axisLabel', 'axisLine', 'splitLine', 'textStyle']) {
            if (commun?.[sousCle] || valeur?.[sousCle]) ensemble[sousCle] = { ...(commun?.[sousCle] as object), ...(valeur?.[sousCle] as object) }
          }
          Object.assign(fusion, { [cle]: ensemble })
        }
      }
      if (!cartesien) { delete fusion.xAxis; delete fusion.yAxis; delete fusion.grid }
      if (o.legend && !Array.isArray(o.legend) && o.legend.bottom !== undefined && !Array.isArray(fusion.legend)) {
        delete fusion.legend!.top
      }
      g.setOption(fusion, { notMerge: true })
    }
    appliquer()
    window.addEventListener('observatoire:theme', appliquer)
    return () => window.removeEventListener('observatoire:theme', appliquer)
  }, [option])

  useEffect(() => {
    const g = graph.current
    if (!g || !surClic) return
    g.on('click', surClic)
    return () => {
      g.off('click', surClic)
    }
  }, [surClic])

  return <div ref={ref} role="img" aria-label={libelle} className={className} style={{ height: hauteur, width: '100%' }} />
}

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
      backgroundColor: jeton('surface'),
      borderColor: bordure,
      textStyle: { color: texte },
    },
    legend: { textStyle: { color: doux } },
    grid: { left: 40, right: 16, top: 24, bottom: 32, containLabel: true },
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
    const obs = new ResizeObserver(() => g.resize())
    obs.observe(ref.current)
    return () => {
      obs.disconnect()
      g.dispose()
      graph.current = null
    }
  }, [])

  useEffect(() => {
    const appliquer = () => {
      const g = graph.current
      if (!g) return
      const o = typeof option === 'function' ? option() : option
      g.setOption({ ...base(), ...o }, { notMerge: true })
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

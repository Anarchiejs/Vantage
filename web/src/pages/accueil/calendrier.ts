import type { EChartsOption } from 'echarts'
import type { JourActivite } from '../../api/types'
import { chaleur, jeton } from '../../theme/theme'
import { date, nombre } from '../../ui/format'

/** Zéro puis quatre classes de quantiles, sans séparer les valeurs identiques. */
export function optionCalendrier(jours: JourActivite[]): EChartsOption {
  const donnees = [...jours].sort((a, b) => a.jour.localeCompare(b.jour)).slice(-365)
  const positifs = donnees.map(j => j.commits).filter(n => n > 0).sort((a, b) => a - b)
  const seuils = [0.25, 0.5, 0.75].map(q => positifs[Math.max(0, Math.ceil(positifs.length * q) - 1)] ?? 0)
  const couleurs = chaleur()
  return {
    xAxis: { show: false }, yAxis: { show: false },
    tooltip: { renderMode: 'richText', formatter: p => {
      const point = Array.isArray(p) ? p[0] : p
      const j = donnees.find(j => j.jour === (point.value as unknown[])?.[0])
      return j ? `${date(j.jour)}\n${nombre(j.commits)} commits · +${nombre(j.ajouts)} / −${nombre(j.suppressions)} lignes\n${nombre(j.sessions)} sessions\n${j.projets.join(', ') || 'Aucun projet'}` : ''
    } },
    calendar: { range: donnees.length ? [donnees[0].jour, donnees.at(-1)!.jour] : new Date().getFullYear(), cellSize: ['auto', 19], left: 44, right: 20, top: 35, bottom: 45,
      itemStyle: { borderColor: jeton('surface'), borderWidth: 3 }, splitLine: { show: false }, yearLabel: { show: false },
      dayLabel: { firstDay: 1, nameMap: ['D', 'L', 'M', 'M', 'J', 'V', 'S'], color: jeton('doux') },
      monthLabel: { nameMap: 'FR', color: jeton('doux') } },
    visualMap: { type: 'piecewise', orient: 'horizontal', bottom: 0, right: 0, dimension: 1, textStyle: { color: jeton('doux') },
      pieces: couleurs.map((color, value) => ({ value, color, label: ['Aucun', 'Faible', 'Modéré', 'Soutenu', 'Fort'][value] })) },
    series: [{ type: 'heatmap', coordinateSystem: 'calendar', data: donnees.map(j => [j.jour, j.commits === 0 ? 0 : 1 + seuils.filter(s => j.commits > s).length]) }],
  }
}


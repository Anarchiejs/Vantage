import type { EChartsOption } from 'echarts'
import type { ResultatSQL } from '../../api/types'

export type FormeGraphique = 'barres' | 'courbe' | 'secteurs'

/** Indices des colonnes numériques après la première (axe X). */
export function colonnesNumeriques(resultat: ResultatSQL): number[] {
  return resultat.colonnes.map((_, i) => i).filter((i) => {
    if (i === 0) return false
    let vu = false
    for (const ligne of resultat.lignes) {
      const valeur = ligne[i]
      if (valeur === null || valeur === undefined) continue
      if (typeof valeur !== 'number') return false
      vu = true
    }
    return vu
  })
}

/** Barres, courbe ou secteurs. Null s'il n'y a aucune colonne numérique. */
export function optionGraphique(resultat: ResultatSQL, forme: FormeGraphique): EChartsOption | null {
  const indices = colonnesNumeriques(resultat)
  if (indices.length === 0) return null
  const etiquettes = resultat.lignes.map((ligne) => (ligne[0] == null ? 'NULL' : String(ligne[0])))
  if (forme === 'secteurs') {
    const i = indices[0]
    return {
      tooltip: { trigger: 'item' },
      series: [
        {
          type: 'pie',
          name: resultat.colonnes[i],
          data: resultat.lignes.map((ligne, k) => ({
            name: etiquettes[k],
            value: typeof ligne[i] === 'number' ? ligne[i] : 0,
          })),
        },
      ],
    }
  }
  return {
    tooltip: { trigger: 'axis' },
    legend: {},
    xAxis: { type: 'category', data: etiquettes },
    yAxis: { type: 'value' },
    series: indices.map((i) => ({
      name: resultat.colonnes[i],
      type: forme === 'courbe' ? 'line' : 'bar',
      data: resultat.lignes.map((ligne) => (typeof ligne[i] === 'number' ? ligne[i] : null)),
    })),
  }
}

/** Nom de fichier d'une requête enregistrée : `[a-z0-9-]+`. */
export function nomValide(nom: string): boolean {
  return /^[a-z0-9-]+$/.test(nom)
}

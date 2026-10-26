const nf = new Intl.NumberFormat('fr-FR')

/** Chiffre clé avec sa variation par rapport à la période précédente (↑ ↓ =). */
export function Stat({ libelle, valeur, avant }: { libelle: string; valeur: number; avant?: number }) {
  let variation = null
  if (avant !== undefined) {
    const diff = valeur - avant
    const fleche = diff > 0 ? '↑' : diff < 0 ? '↓' : '='
    const couleur = diff > 0 ? 'text-succes' : diff < 0 ? 'text-danger' : 'text-doux'
    const pct = avant === 0 ? null : Math.round((diff / avant) * 100)
    variation = (
      <span className={`text-xs font-medium ${couleur}`} title={`Période précédente : ${nf.format(avant)}`}>
        {fleche} {pct === null ? nf.format(Math.abs(diff)) : `${Math.abs(pct)} %`}
      </span>
    )
  }
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs text-doux">{libelle}</span>
      <span className="flex items-baseline gap-2">
        <span className="text-2xl font-semibold tabular-nums">{nf.format(valeur)}</span>
        {variation}
      </span>
    </div>
  )
}

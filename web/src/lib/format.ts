const sgd = new Intl.NumberFormat('en-SG', { minimumFractionDigits: 2, maximumFractionDigits: 2 })

/** Money from the API is integer cents of SGD. */
export function formatCents(cents: number): string {
  return `$${sgd.format(cents / 100)}`
}

/** pct is (price - typical) / typical * 100, so negative means cheaper. */
export function formatVsTypical(pct: number | null): string {
  if (pct === null) return 'no typical price yet'
  const rounded = Math.round(Math.abs(pct))
  if (rounded === 0) return 'at typical'
  return pct < 0 ? `${rounded}% under typical` : `${rounded}% over typical`
}

const SITE_NAMES: Record<string, string> = {
  shopee: 'Shopee',
  lazada: 'Lazada',
  amazon: 'Amazon SG',
  courts: 'Courts',
}

export function siteName(site: string): string {
  return SITE_NAMES[site] ?? site
}

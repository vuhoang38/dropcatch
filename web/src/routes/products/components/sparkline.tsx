import type { SparkPoint } from '@/lib/api/products'

const W = 96
const H = 28

/** A tiny trend line of the daily cheapest price. */
export function Sparkline({ points }: { points: SparkPoint[] }) {
  if (points.length < 2) {
    return <div className="h-8 w-24" aria-hidden />
  }
  const values = points.map((p) => p.cents)
  const min = Math.min(...values)
  const span = Math.max(...values) - min || 1
  const coords = values
    .map((v, i) => `${(i / (values.length - 1)) * W},${H - 2 - ((v - min) / span) * (H - 4)}`)
    .join(' ')

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="h-8 w-24" aria-hidden>
      <polyline points={coords} fill="none" stroke="var(--chart-2)" strokeWidth={2} />
    </svg>
  )
}

import { useQuery } from '@tanstack/react-query'
import { format } from 'date-fns'
import { useState } from 'react'
import { CartesianGrid, Line, LineChart, ReferenceLine, XAxis, YAxis } from 'recharts'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { historyQueryOptions, type History, type HistoryRange } from '@/lib/api/products'
import { formatCents } from '@/lib/format'

// Contrasting chart tokens first, so the first few lines are easy to tell apart.
const COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-4)', 'var(--chart-3)', 'var(--chart-5)']

const RANGES: { value: HistoryRange; label: string }[] = [
  { value: '1d', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '30d', label: '30d' },
  { value: '90d', label: '90d' },
  { value: 'all', label: 'All' },
]

type Row = { t: number } & Record<string, number>

type Series = History['series'][number]

/** One row per check time, one column per variant, so recharts can draw a line each. */
function toRows(series: Series[]): Row[] {
  const byTime = new Map<number, Row>()
  for (const s of series) {
    for (const p of s.points) {
      const t = new Date(p.t).getTime()
      const row = byTime.get(t) ?? { t }
      row[`v${s.variantId}`] = p.cents
      byTime.set(t, row)
    }
  }
  return [...byTime.values()].sort((a, b) => a.t - b.t)
}

const DAY = 24 * 60 * 60 * 1000

/** Times of day when the chart covers a day or less, dates otherwise. */
function tickFormat(rows: Row[]) {
  const span = rows.length > 0 ? rows[rows.length - 1].t - rows[0].t : 0
  return span <= DAY ? 'HH:mm' : 'd MMM'
}

export function PriceChart({ productId }: { productId: number }) {
  const [range, setRange] = useState<HistoryRange>('30d')
  // By variant id, so a line stays hidden when you switch ranges.
  const [hidden, setHidden] = useState<ReadonlySet<number>>(new Set())
  const { data, isPending } = useQuery(historyQueryOptions(productId, range))

  const series = data?.series ?? []
  const config: ChartConfig = {}
  series.forEach((s, i) => {
    config[`v${s.variantId}`] = { label: s.label, color: COLORS[i % COLORS.length] }
  })
  const shown = series.filter((s) => !hidden.has(s.variantId))
  const rows = toRows(shown)

  function toggle(id: number, show: boolean) {
    setHidden((prev) => {
      const next = new Set(prev)
      if (show) next.delete(id)
      else next.add(id)
      return next
    })
  }

  return (
    <div className="grid min-w-0 gap-3 overflow-hidden rounded-md border p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-label">Best price over time</span>
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={range}
          onValueChange={(v) => {
            if (v) setRange(v as HistoryRange)
          }}
        >
          {RANGES.map((r) => (
            <ToggleGroupItem key={r.value} value={r.value}>
              {r.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </div>

      {series.length > 0 ? (
        <div className="flex flex-wrap gap-3">
          {series.map((s) => {
            const id = `series-${s.variantId}`
            return (
              <div key={s.variantId} className="flex items-center gap-2">
                <Checkbox
                  id={id}
                  checked={!hidden.has(s.variantId)}
                  onCheckedChange={(v) => toggle(s.variantId, v === true)}
                />
                <span
                  aria-hidden
                  className="size-2 shrink-0 rounded-sm"
                  style={{ backgroundColor: config[`v${s.variantId}`]?.color }}
                />
                <Label htmlFor={id} className="text-caption">
                  {s.label}
                </Label>
              </div>
            )
          })}
        </div>
      ) : null}

      {isPending ? (
        <Skeleton className="h-72 w-full" />
      ) : series.length > 0 && shown.length === 0 ? (
        <div className="text-body text-muted-foreground grid h-72 place-items-center">
          Tick a line above to see its prices.
        </div>
      ) : rows.length === 0 ? (
        <div className="text-body text-muted-foreground grid h-72 place-items-center">
          No prices recorded in this range yet.
        </div>
      ) : (
        <ChartContainer config={config} className="h-72 w-full min-w-0">
          <LineChart data={rows} margin={{ left: 4, right: 4, top: 8 }}>
            <CartesianGrid vertical={false} strokeDasharray="3 3" />
            <XAxis
              dataKey="t"
              type="number"
              scale="time"
              domain={['dataMin', 'dataMax']}
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              tickFormatter={(t: number) => format(t, tickFormat(rows))}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              width={64}
              domain={['auto', 'auto']}
              tickFormatter={(c: number) => formatCents(c).replace(/\.00$/, '')}
            />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  labelFormatter={(_, payload) => {
                    const t = payload[0]?.payload as Row | undefined
                    return t ? format(t.t, 'd MMM, HH:mm') : ''
                  }}
                  formatter={(value, name) => (
                    <div className="flex w-full justify-between gap-4">
                      <span className="text-muted-foreground">{config[String(name)]?.label}</span>
                      <span className="tabular-nums">{formatCents(Number(value))}</span>
                    </div>
                  )}
                />
              }
            />
            {data?.typicalCents ? (
              <ReferenceLine
                y={data.typicalCents}
                stroke="var(--muted-foreground)"
                strokeDasharray="6 4"
                label={{
                  value: `typical ${formatCents(data.typicalCents)}`,
                  position: 'insideTopLeft',
                  fill: 'var(--muted-foreground)',
                  fontSize: 12,
                }}
              />
            ) : null}
            {shown.map((s) => (
              <Line
                key={s.variantId}
                dataKey={`v${s.variantId}`}
                type="stepAfter"
                stroke={`var(--color-v${s.variantId})`}
                strokeWidth={2}
                dot={false}
                connectNulls
                isAnimationActive={false}
              />
            ))}
          </LineChart>
        </ChartContainer>
      )}
    </div>
  )
}

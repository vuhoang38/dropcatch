# Add a chart

## When

Visualising a series of numbers. Built on recharts through the shadcn
`ChartContainer`, which wires the tooltip, legend and colors to the tokens.

## Files

```
src/routes/<name>/components/<thing>-chart.tsx
```

## Pattern

```tsx
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts'
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { formatDate } from '@/lib/format'

// Series labels and colors in one place. Colors must be chart tokens so the
// chart re-skins with the app.
const CHART_CONFIG = {
  requests: { label: 'Requests', color: 'var(--chart-1)' },
} satisfies ChartConfig

export function RequestsChart({ data }: { data: TrafficPoint[] }) {
  return (
    <ChartContainer config={CHART_CONFIG} className="h-64 w-full">
      <AreaChart data={data} margin={{ left: 4, right: 4, top: 4 }}>
        <CartesianGrid vertical={false} strokeDasharray="3 3" />
        <XAxis
          dataKey="date"
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          tickFormatter={formatDate}
        />
        <YAxis tickLine={false} axisLine={false} tickMargin={8} width={44} />
        <ChartTooltip content={<ChartTooltipContent />} />
        <ChartLegend content={<ChartLegendContent />} />
        <Area
          dataKey="requests"
          type="monotone"
          stroke="var(--color-requests)"
          fill="var(--color-requests)"
          fillOpacity={0.15}
          strokeWidth={2}
          isAnimationActive={false}
        />
      </AreaChart>
    </ChartContainer>
  )
}
```

A key in `CHART_CONFIG` becomes `var(--color-<key>)` inside the chart. That is
where `var(--color-requests)` comes from.

Working example: `src/routes/dashboard/components/traffic-chart.tsx`, which also
shows two series on separate axes using `ComposedChart`.

## Mistakes that happen here

- **Leaving animation on.** `bun run screenshot` then captures the chart
  mid-draw and the result looks broken. Always set
  `isAnimationActive={false}`.
- **Giving the container no height.** Recharts measures its parent, so without
  `h-64` or similar the chart renders at zero height and you see nothing.
- **Plotting series with very different magnitudes on one axis.** Requests in
  the thousands next to errors in the tens flattens the error line onto zero.
  Use two `YAxis` elements with `yAxisId` and a `ComposedChart`.
- **Hard-coding hex colors.** Use `--chart-1` through `--chart-5`.
- **Two adjacent chart tokens for two series.** `--chart-1` and `--chart-5` are
  both warm and hard to tell apart. Pick colors that contrast.

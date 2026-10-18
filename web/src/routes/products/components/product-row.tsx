import { Link } from 'react-router'
import { Badge } from '@/components/ui/badge'
import type { ProductSummary } from '@/lib/api/products'
import { formatCents, formatVsTypical, siteName } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Sparkline } from '@/routes/products/components/sparkline'

export function ProductRow({ product }: { product: ProductSummary }) {
  const cheaper = product.pctVsTypical !== null && product.pctVsTypical <= -1

  return (
    <Link
      to={`/products/${product.id}`}
      className="bg-card hover:bg-accent flex flex-wrap items-center gap-4 rounded-md border p-4"
    >
      <div className="grid min-w-0 grow basis-48 gap-1">
        <span className="text-label truncate">{product.name}</span>
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-caption text-muted-foreground">
            {product.linkCount === 1 ? '1 link' : `${product.linkCount} links`}
          </span>
          {product.paused ? <Badge variant="secondary">Paused</Badge> : null}
          {product.warnings.map((w) => (
            <Badge key={w} variant="outline" className="text-warning border-warning">
              {w}
            </Badge>
          ))}
        </div>
      </div>

      <div className="hidden sm:block">
        <Sparkline points={product.spark} />
      </div>

      <div className="ml-auto grid gap-1 text-right">
        {product.bestCents === null ? (
          <span className="text-body text-muted-foreground">No price yet</span>
        ) : (
          <>
            <span className="text-body-lg tabular-nums">{formatCents(product.bestCents)}</span>
            <span className={cn('text-caption', cheaper ? 'text-success' : 'text-muted-foreground')}>
              {formatVsTypical(product.pctVsTypical)}
            </span>
            <span className="text-caption text-muted-foreground">{siteName(product.bestSite)}</span>
          </>
        )}
      </div>
    </Link>
  )
}

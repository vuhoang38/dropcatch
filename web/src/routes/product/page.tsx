import { useQuery } from '@tanstack/react-query'
import { Pause, Play, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { toast } from 'sonner'
import { PageHeader } from '@/components/shared/page-header'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  productQueryOptions,
  useCheckNow,
  useDeleteProduct,
  useUpdateProduct,
  type ProductDetail,
} from '@/lib/api/products'
import { formatCents, formatVsTypical, siteName } from '@/lib/format'
import { AddLinkDialog } from '@/routes/product/components/add-link-dialog'
import { AlertSettingsForm } from '@/routes/product/components/alert-settings-form'
import { LinksTable } from '@/routes/product/components/links-table'
import { PriceChart } from '@/routes/product/components/price-chart'

export default function ProductPage() {
  const id = Number(useParams().id)
  const { data, isPending, isError, error } = useQuery(productQueryOptions(id))

  if (isError) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error.message}</AlertDescription>
      </Alert>
    )
  }
  if (isPending) return <Skeleton className="h-96 w-full" />
  return <ProductView product={data} />
}

function ProductView({ product }: { product: ProductDetail }) {
  const navigate = useNavigate()
  const update = useUpdateProduct(product.id)
  const checkNow = useCheckNow(product.id)
  const remove = useDeleteProduct()
  const [addingLink, setAddingLink] = useState(false)

  return (
    <>
      <PageHeader
        title={product.name}
        actions={
          <>
            <Button
              variant="outline"
              onClick={() =>
                checkNow.mutate(undefined, {
                  onSuccess: () => toast.success('Checking now. Refresh in a minute to see new prices.'),
                })
              }
            >
              <RefreshCw />
              Check now
            </Button>
            <Button variant="outline" onClick={() => update.mutate({ paused: !product.paused })}>
              {product.paused ? <Play /> : <Pause />}
              {product.paused ? 'Resume' : 'Pause'}
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Delete product"
              onClick={() => {
                if (window.confirm(`Delete ${product.name} and all its price history?`)) {
                  remove.mutate(product.id, { onSuccess: () => void navigate('/') })
                }
              }}
            >
              <Trash2 />
            </Button>
          </>
        }
      />

      {product.paused || product.warnings.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {product.paused ? <Badge variant="secondary">Paused</Badge> : null}
          {product.warnings.map((w) => (
            <Badge key={w} variant="outline" className="text-warning border-warning">
              {w}
            </Badge>
          ))}
        </div>
      ) : null}

      <div data-testid="product-stats" className="grid gap-3 sm:grid-cols-3">
        <Stat label="Best price now">
          {product.bestCents === null ? (
            <span className="text-h2 text-muted-foreground">None</span>
          ) : (
            <>
              <span className="text-h2 tabular-nums">{formatCents(product.bestCents)}</span>
              <span className="text-caption text-muted-foreground">{siteName(product.bestSite)}</span>
            </>
          )}
        </Stat>
        <Stat label={`Typical price (${product.settings.windowDays} days)`}>
          <span className="text-h2 tabular-nums">
            {product.typicalCents === null ? 'Not yet' : formatCents(product.typicalCents)}
          </span>
          <span className="text-caption text-muted-foreground">{formatVsTypical(product.pctVsTypical)}</span>
        </Stat>
        <Stat label="Alert threshold">
          <span className="text-h2">{product.settings.dropPct}% under</span>
          <span className="text-caption text-muted-foreground">
            after {product.settings.minHistoryDays} days of history
          </span>
        </Stat>
      </div>

      <PriceChart productId={product.id} />

      <div className="grid gap-3">
        <div className="flex items-center justify-between">
          <span className="text-h3">Links</span>
          <Button variant="outline" onClick={() => setAddingLink(true)}>
            <Plus />
            Add link
          </Button>
        </div>
        <LinksTable links={product.links} />
      </div>

      <AlertSettingsForm key={JSON.stringify(product.overrides)} product={product} />

      <AddLinkDialog productId={product.id} open={addingLink} onOpenChange={setAddingLink} />
    </>
  )
}

function Stat({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid content-start gap-1 rounded-md border p-4">
      <span className="text-caption text-muted-foreground">{label}</span>
      {children}
    </div>
  )
}

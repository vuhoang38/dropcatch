import { useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { PageHeader } from '@/components/shared/page-header'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { productsQueryOptions } from '@/lib/api/products'
import { AddProductDialog } from '@/routes/products/components/add-product-dialog'
import { ProductRow } from '@/routes/products/components/product-row'

export default function ProductsPage() {
  const { data, isPending, isError, error } = useQuery(productsQueryOptions())
  const [adding, setAdding] = useState(false)

  return (
    <>
      <PageHeader
        title="Products"
        description="Real prices across Shopee, Lazada, Amazon and Courts, compared to what they usually cost."
        actions={
          <Button onClick={() => setAdding(true)}>
            <Plus />
            Add product
          </Button>
        }
      />

      {isError ? (
        <Alert variant="destructive">
          <AlertDescription>{error.message}</AlertDescription>
        </Alert>
      ) : isPending ? (
        <Skeleton className="h-64 w-full" />
      ) : data.length === 0 ? (
        <div className="grid justify-items-center gap-3 rounded-md border border-dashed p-12 text-center">
          <span className="text-h3">Nothing tracked yet</span>
          <span className="text-body text-muted-foreground">
            Add a product and paste the listings you want to watch.
          </span>
          <Button onClick={() => setAdding(true)}>
            <Plus />
            Add product
          </Button>
        </div>
      ) : (
        <div className="grid gap-2">
          {data.map((p) => (
            <ProductRow key={p.id} product={p} />
          ))}
        </div>
      )}

      <AddProductDialog open={adding} onOpenChange={setAdding} />
    </>
  )
}

import { useQuery } from '@tanstack/react-query'
import { Plus, Search } from 'lucide-react'
import { useState } from 'react'
import { PageHeader } from '@/components/shared/page-header'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { productsQueryOptions } from '@/lib/api/products'
import { AddProductDialog } from '@/routes/products/components/add-product-dialog'
import { ProductRow } from '@/routes/products/components/product-row'

export default function ProductsPage() {
  const { data, isPending, isError, error } = useQuery(productsQueryOptions())
  const [adding, setAdding] = useState(false)
  const [search, setSearch] = useState('')

  const query = search.trim().toLowerCase()
  const shown = data?.filter((p) => p.name.toLowerCase().includes(query)) ?? []

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
        <div className="grid gap-4">
          <div className="relative">
            <Search className="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
            <Input
              type="search"
              aria-label="Search products"
              placeholder="Search products"
              className="pl-8"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          {shown.length === 0 ? (
            <div className="text-body text-muted-foreground rounded-md border border-dashed p-12 text-center">
              No products match "{search.trim()}".
            </div>
          ) : (
            <div className="grid gap-2">
              {shown.map((p) => (
                <ProductRow key={p.id} product={p} />
              ))}
            </div>
          )}
        </div>
      )}

      <AddProductDialog open={adding} onOpenChange={setAdding} />
    </>
  )
}

import { useQuery } from '@tanstack/react-query'
import { Fragment } from 'react'
import { Link, useLocation } from 'react-router'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { productQueryOptions } from '@/lib/api/products'

const LABELS: Record<string, string> = {
  settings: 'Settings',
}

function label(segment: string) {
  return LABELS[segment] ?? segment
}

// Shares the product page's query, so the name costs no extra request.
function ProductName({ id }: { id: number }) {
  const { data } = useQuery(productQueryOptions(id))
  return data?.name ?? 'Product'
}

function Label({ segment }: { segment: string }) {
  return /^\d+$/.test(segment) ? <ProductName id={Number(segment)} /> : label(segment)
}

export function Breadcrumbs() {
  const { pathname } = useLocation()
  // Products is the home page, so /products/1 reads "Products > AirPods Pro 3".
  const segments = pathname.split('/').filter(Boolean).filter((s) => s !== 'products')

  return (
    <Breadcrumb>
      <BreadcrumbList>
        <BreadcrumbItem>
          {segments.length === 0 ? (
            <BreadcrumbPage>Products</BreadcrumbPage>
          ) : (
            <BreadcrumbLink asChild>
              <Link to="/">Products</Link>
            </BreadcrumbLink>
          )}
        </BreadcrumbItem>
        {segments.map((segment, index) => (
          // The separator is a sibling <li>, never a child of the item.
          <Fragment key={segment}>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              {index === segments.length - 1 ? (
                <BreadcrumbPage>
                  <Label segment={segment} />
                </BreadcrumbPage>
              ) : (
                <BreadcrumbLink asChild>
                  <Link to={`/${segments.slice(0, index + 1).join('/')}`}>
                    <Label segment={segment} />
                  </Link>
                </BreadcrumbLink>
              )}
            </BreadcrumbItem>
          </Fragment>
        ))}
      </BreadcrumbList>
    </Breadcrumb>
  )
}

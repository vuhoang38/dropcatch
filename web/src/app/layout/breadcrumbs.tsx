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

const LABELS: Record<string, string> = {
  settings: 'Settings',
}

function label(segment: string) {
  return LABELS[segment] ?? (/^\d+$/.test(segment) ? 'Product' : segment)
}

export function Breadcrumbs() {
  const { pathname } = useLocation()
  // Products is the home page, so /products/1 reads "Products > Product".
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
                <BreadcrumbPage>{label(segment)}</BreadcrumbPage>
              ) : (
                <BreadcrumbLink asChild>
                  <Link to={`/${segments.slice(0, index + 1).join('/')}`}>
                    {label(segment)}
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

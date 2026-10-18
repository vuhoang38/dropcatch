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
  products: 'Products',
  settings: 'Settings',
}

export function Breadcrumbs() {
  const { pathname } = useLocation()
  const segments = pathname.split('/').filter(Boolean)

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
                <BreadcrumbPage>{LABELS[segment] ?? segment}</BreadcrumbPage>
              ) : (
                <BreadcrumbLink asChild>
                  <Link to={`/${segments.slice(0, index + 1).join('/')}`}>
                    {LABELS[segment] ?? segment}
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

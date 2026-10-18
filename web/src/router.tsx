import { Suspense, lazy } from 'react'
import { createBrowserRouter } from 'react-router'
import { AppShell } from '@/app/layout/app-shell'
import { ErrorPage } from '@/app/layout/error-page'
import { RouteFallback } from '@/components/shared/route-fallback'

/**
 * Put a page behind its own bundle. Without this the whole app, charts and all,
 * lands in one file that every visitor downloads to see the first screen.
 */
function lazyRoute(load: () => Promise<{ default: React.ComponentType }>) {
  const Page = lazy(load)

  return (
    <Suspense fallback={<RouteFallback />}>
      <Page />
    </Suspense>
  )
}

/**
 * Every route in the app lives here. One file, so the whole surface is readable
 * in one go.
 *
 * To add a route: create `src/routes/<name>/page.tsx` with a default export,
 * then add an entry below.
 */
export const router = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    errorElement: <ErrorPage />,
    children: [
      { index: true, element: lazyRoute(() => import('@/routes/products/page')) },
      { path: 'products/:id', element: lazyRoute(() => import('@/routes/product/page')) },
      { path: 'settings', element: lazyRoute(() => import('@/routes/settings/page')) },
    ],
  },
])

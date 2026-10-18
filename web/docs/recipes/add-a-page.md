# Add a page

## When

Any new screen. Start here, then layer on a table, form or chart recipe.

## Files

```
src/routes/<name>/page.tsx          the screen, default export
src/routes/<name>/components/       components only this screen uses
src/router.tsx                      register the route
```

## Do it

1. Create `src/routes/<name>/page.tsx` with a default export.
2. Register it in `src/router.tsx` as a child of the `AppShell` route so it gets
   the sidebar and header. Only full-bleed screens such as `/login` sit outside.
   Wrap it in `lazyRoute` so the page ships as its own bundle:

   ```tsx
   { path: 'reports', element: lazyRoute(() => import('@/routes/reports/page')) },
   ```
3. Add it to `NAV_ITEMS` in `src/app/layout/app-sidebar.tsx` if it needs a nav
   entry, and to `LABELS` in `src/app/layout/breadcrumbs.tsx` so the breadcrumb
   reads properly.

## Pattern

```tsx
import { PageHeader } from '@/components/shared/page-header'
import { Button } from '@/components/ui/button'

export default function ReportsPage() {
  return (
    <>
      <PageHeader
        title="Reports"
        description="Exports generated in the last 30 days."
        actions={<Button>New report</Button>}
      />

      {/* content */}
    </>
  )
}
```

The page returns a fragment, not a wrapper div. `AppShell` already supplies the
`<main>` element with its padding and `gap-6` between blocks, so an extra
wrapper double-pads the screen.

## Mistakes that happen here

- **Wrapping the page in its own padded container.** The shell handles it.
- **Writing a bespoke `<h1>` instead of `PageHeader`.** Titles then drift in size
  and spacing between screens. That drift is the thing this template exists to
  prevent.
- **Forgetting the breadcrumb label,** which leaves a raw url segment in the
  header.
- **Importing the page directly in `src/router.tsx`** instead of using
  `lazyRoute`. It still works, but the page then loads on first paint for every
  visitor, even those who never open it.

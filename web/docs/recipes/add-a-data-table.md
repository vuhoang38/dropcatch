# Add a data table

## When

Showing a list of records with sorting, filtering or paging.

## Files

```
src/routes/<name>/components/<thing>-columns.tsx   column definitions
src/routes/<name>/page.tsx                         fetches and renders
```

Do not write `useReactTable` again. `src/components/shared/data-table.tsx`
already does sorting, filtering and pagination. Supply columns and data.

## Pattern

Columns, in their own file because they get long:

```tsx
import type { ColumnDef } from '@tanstack/react-table'
import { ArrowUpDown } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { formatDateTime } from '@/lib/format'

export function createReportColumns(onOpen: (report: Report) => void): ColumnDef<Report>[] {
  return [
    {
      accessorKey: 'name',
      header: ({ column }) => (
        <Button
          variant="ghost"
          size="sm"
          className="-ml-3"
          onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
        >
          Name
          <ArrowUpDown className="size-3" />
        </Button>
      ),
      cell: ({ row }) => <span className="text-label">{row.original.name}</span>,
    },
    {
      accessorKey: 'createdAt',
      header: 'Created',
      cell: ({ row }) => (
        <span className="text-muted-foreground tabular-nums">
          {formatDateTime(row.original.createdAt)}
        </span>
      ),
    },
  ]
}
```

The page:

```tsx
const { data, isPending } = useQuery(reportsQueryOptions())
const columns = useMemo(() => createReportColumns(setOpenReport), [])

if (isPending || !data) return <Skeleton className="h-96 w-full" />

return (
  <DataTable
    columns={columns}
    data={data}
    filterColumn="name"
    filterPlaceholder="Filter reports..."
    emptyMessage="No reports yet."
  />
)
```

Working example: `src/routes/users/`.

## Mistakes that happen here

- **Building the table inline in the page.** Columns get long and bury the data
  fetching. Keep them in their own file.
- **Not memoising columns.** A fresh array every render resets sorting and
  filter state. `useMemo` with an empty dependency list is correct when the
  factory only closes over stable callbacks.
- **Passing a column id to `filterColumn` that does not exist.** The prop turns
  the search box on and filters globally; the value is used for the label, not
  for lookup, so a typo silently gives a box that searches everything.
- **Formatting dates inside the cell with `toLocaleString`.** Use the helpers in
  `src/lib/format.ts` so every table formats the same way.
- **Forgetting `tabular-nums` on numeric columns,** which makes digits jitter
  between rows.

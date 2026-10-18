# Fetch and mutate data

## When

A screen needs data from the server, or needs to change it.

## Files

```
src/lib/api/<resource>.ts    types, query keys, queryOptions factories, mutations
src/lib/api/client.ts        the fetch wrapper (already written, do not duplicate)
```

## Do it

1. Create `src/lib/api/<resource>.ts`.
2. Export the row type, a `<resource>Keys` object, and a `queryOptions` factory
   per query.
3. Components call `useQuery(thingQueryOptions())`. They never write a query key
   and never call `fetch`.

## Pattern

```ts
import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api/client'

export type Report = { id: string; name: string; createdAt: string }

// Keys live here and nowhere else. A key typed inline in a component is how
// invalidation silently stops working.
export const reportKeys = {
  all: ['reports'] as const,
  list: () => [...reportKeys.all, 'list'] as const,
  detail: (id: string) => [...reportKeys.all, 'detail', id] as const,
}

export function reportsQueryOptions() {
  return queryOptions({
    queryKey: reportKeys.list(),
    queryFn: () => api.get<Report[]>('/reports'),
  })
}

export function useCreateReport() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (input: { name: string }) => api.post<Report>('/reports', input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: reportKeys.all })
    },
  })
}
```

In the component:

```tsx
const { data, isPending } = useQuery(reportsQueryOptions())

if (isPending || !data) return <Skeleton className="h-96 w-full" />
```

## Loading and errors

- Loading: render a `Skeleton` roughly the size of the real content. A spinner
  that replaces the whole page makes the layout jump.
- Errors: `AppShell`'s `errorElement` catches route-level failures. For a query
  that can fail on its own, read `isError` and render an `Alert`.

## Mistakes that happen here

- **Writing the key inline** as `useQuery({ queryKey: ['reports'], ... })`. The
  invalidation in the mutation then misses it, and the list does not refresh.
- **Invalidating the narrow key.** Invalidate `reportKeys.all` so list and detail
  both refresh.
- **Calling `fetch` directly.** Then the base url and error handling live in two
  places.
- **Making `queryFn` `async` with no `await` inside.** Lint rejects it. Return
  the promise.

## Swapping the fixtures out

The example resources (`users.ts`, `metrics.ts`) resolve fixture data. Each one
marks the swap with a `TEMPLATE:` comment. Replace the `queryFn` body with the
`api.get` call, then delete `src/fixtures/`.

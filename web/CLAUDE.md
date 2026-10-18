# Working in this repo

Internal tools and dashboards. Vite, React, TypeScript, React Router, TanStack
Query, Tailwind v4, shadcn/ui. Bun is the runtime and package manager, so it is
`bun run`, never `npm run`.

Before building a feature, read `docs/recipes/README.md` and follow the recipe
that matches the task. The recipes have the file layout and the working code.

## Hard rules

Each one is enforced by lint, so breaking it fails `bun run verify`.

1. **Colors come from tokens.** `bg-background`, `text-muted-foreground`,
   `border-border`. Never `bg-zinc-800`, never `text-[#1a1a1a]`.
2. **Type comes from the scale.** `text-display`, `text-h1`, `text-h2`,
   `text-h3`, `text-body-lg`, `text-body`, `text-label`, `text-caption`. Never
   `text-sm` or `font-bold`. Each step already carries its line height and
   weight, so never pair a step with a `font-*` class.
3. **Spacing uses the rhythm.** Padding, margin and gap use 0, 1, 2, 3, 4, 6, 8,
   12, 16, 24 and nothing else.
4. **No arbitrary values.** No `w-[327px]`. Add a token instead.
5. **No bare interactive elements.** Use `Button`, `Input`, `Select` and
   `Textarea` from `@/components/ui`, never raw `<button>` or `<input>`.
6. **Never hand-edit `src/components/ui/`.** It is generated. Add a component
   with `bun run ui:add <name>`. Need a new variant? Extend it with `cva` in a
   sibling file.
7. **No `any`.**
8. **Query keys live in `src/lib/api/<resource>.ts`,** never inline in a
   component. Components call `useQuery(usersQueryOptions())`.

To change how the app looks, edit `src/styles/tokens.css`. That is the only
file that should hold a color or a size.

## Where things go

| Thing | Location |
|---|---|
| A page | `src/routes/<name>/page.tsx`, registered in `src/router.tsx` |
| A component one route uses | `src/routes/<name>/components/` |
| A component two routes use | `src/components/shared/` |
| Data fetching | `src/lib/api/<resource>.ts` as a `queryOptions` factory |
| Design tokens | `src/styles/tokens.css` |
| Sample data | `src/fixtures/` (delete once a real API exists) |

There is no backend. Example routes read from `src/fixtures/`, and each
`queryFn` carries a `TEMPLATE:` comment showing the `api.get` call to swap in.

## Definition of done

A task is not finished until all three pass. Do not report success before then.

1. `bun run verify` exits clean. It runs typecheck, lint and tests.
2. `bun run screenshot /your-route` exits clean, and **you opened the PNG in
   `.screenshots/` and looked at it**. It also fails on console errors, which is
   how you catch a runtime crash that typechecks fine.
3. New behavior has a test. Copy the pattern from the nearest existing
   `*.test.tsx`.

If a lint rule blocks something genuinely reasonable, say so rather than
scattering `eslint-disable` comments. A pile of disables means the rule is
wrong, and that is worth fixing properly.

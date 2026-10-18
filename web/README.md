# Dashboard template

A starting point for internal tools and dashboards, set up so an AI model
working in the repo produces consistent, working code without being told the
same things every session.

Vite, React, TypeScript, React Router, TanStack Query, Tailwind v4, shadcn/ui,
on Bun.

## Start a project

```bash
bun install
bunx playwright install chromium   # once, for the screenshot loop
bun run dev
```

Then rename the project in `package.json` and edit `src/styles/tokens.css` to
match your brand.

## Commands

| Command | Does |
|---|---|
| `bun run dev` | Dev server on http://localhost:5173 |
| `bun run verify` | Typecheck, lint and tests. The one command to trust. |
| `bun run screenshot /users` | Render a route, save PNGs, fail on console errors |
| `bun run screenshot --dark /users` | Same, in dark mode |
| `bun run ui:add dialog` | Add a shadcn component |
| `bun run build` | Production build |

## What makes this different from plain shadcn

shadcn gives you color tokens. This adds the three things that drift when a
model builds pages across separate sessions, and makes them lint errors rather
than style opinions.

- **A type scale.** `text-h2` carries its size, line height and weight together,
  so headings cannot be mismatched. `text-sm` and `font-bold` fail lint.
- **A spacing rhythm.** Padding, margin and gap use nine steps. `p-5` fails lint.
- **Surface tokens** for dashboard chrome, plus a row-height token so tables
  stay a consistent density.

Five rules in `eslint/house-rules.js` enforce all of it, and each error message
names the replacement. `eslint/house-rules.test.ts` keeps the rules honest.

## For the model

`CLAUDE.md` holds the rules and the definition of done. `docs/recipes/` holds a
short guide per common task, each with working code from the example routes.
Both are plain markdown, so Cursor, Copilot and the rest get the same guidance.

## Delete the examples

The four example routes exist so there is real, current code to copy. When you
do not need them:

```bash
rm -rf src/routes/dashboard src/routes/users src/routes/settings src/routes/login
rm -rf src/fixtures src/lib/api/users.ts src/lib/api/metrics.ts
rm -f src/lib/api/users.test.tsx src/routes/settings/page.test.tsx
```

Then clear the route entries from `src/router.tsx`, the nav entries from
`src/app/layout/app-sidebar.tsx`, and the labels from
`src/app/layout/breadcrumbs.tsx`.

Keep `src/components/shared/data-table.tsx` and `page-header.tsx`. They are the
shared pieces, not examples.

## Version constraints worth knowing

- **TypeScript is pinned to 6.x.** TypeScript 7 is available and much faster,
  but typescript-eslint hard-fails on it, which would take lint with it. Move
  once typescript-eslint supports TS 7.
- **TanStack Table is pinned to 8.x.** Version 9 is a full API rewrite
  (`createTableHook` instead of `useReactTable`). Models reliably produce v8
  code and shadcn's data-table docs target v8, so v8 is the right default here.

## Connect a real API

Set `VITE_API_URL` in `.env`. Then in each `src/lib/api/<resource>.ts`, replace
the `queryFn` body with the `api.get` call shown in its `TEMPLATE:` comment and
delete `src/fixtures/`.

# AI-Oriented Frontend Template, Design

Date 2026-10-17

## Purpose

A GitHub template repository for starting internal tools and dashboards. The
goal is not saving human setup time. The goal is that an AI model working in a
cloned repo produces consistent, good-looking, working code without being told
the same things every session.

Success means you clone the template, write one prompt describing a feature, and
the model ships a page that matches the rest of the app, passes `bun run verify`,
and needs no manual spacing or typography cleanup.

## Scope

In scope. Project scaffold, design token layer, lint enforcement, four example
routes, task recipes, verify command, screenshot loop, thin test setup.

Out of scope. Any backend. Authentication logic beyond a login screen shell.
A CLI scaffolder. Mock server tooling. Multiple theme presets.

## Stack

| Concern | Choice |
|---|---|
| Runtime and package manager | Bun |
| Build tool | Vite |
| UI | React with TypeScript |
| Routing | React Router |
| Server state | TanStack Query |
| Styling | Tailwind v4 |
| Components | shadcn/ui |
| Tables | @tanstack/react-table |
| Forms | react-hook-form with zod |
| Icons | lucide-react |
| Charts | recharts via the shadcn chart wrapper |
| Unit tests | Vitest with jsdom and Testing Library |
| Screenshots | Playwright |
| Lint | ESLint with a local plugin |

Rationale for Vite over Next.js. The template targets internal tools where SEO
does not matter and a separate API already exists. A pure SPA has no server and
client boundary, which removes the single most common category of AI mistake in
App Router projects. Bun support is also stronger on Vite.

There is no mock API layer. Example routes read from static typed fixture files.
Wiring a real API means changing one base URL and deleting the fixtures.

## Repository layout

```
CLAUDE.md                  rules, under 100 lines
README.md                  human-facing setup and delete-the-examples steps
components.json            shadcn CLI config
docs/recipes/              one file per common task, plus an index
.claude/
  settings.json            pre-allowed commands
  skills/                  one thin skill pointing at the recipes
eslint/                    local lint plugin, the house rules
scripts/screenshot.ts      Playwright capture and console error check
src/
  main.tsx
  router.tsx               every route, one file
  app/
    providers.tsx          QueryClient, theme, router
    layout/                AppShell, Sidebar, Topbar
  routes/
    dashboard/page.tsx
    users/
      page.tsx
      components/          used only by this route
    settings/page.tsx
    login/page.tsx
  components/
    ui/                    shadcn primitives, generated, never hand-edited
    shared/                promoted from routes when a second route imports it
  lib/
    api/client.ts          typed fetch wrapper
    api/users.ts           queryOptions factories per resource
    utils.ts               cn()
  fixtures/                static typed data, deleted when a real API lands
  styles/
    globals.css
    tokens.css
```

## Conventions

These five rules exist because each one prevents a specific failure mode.

**A route is a folder containing `page.tsx`.** Components used by only that route
live in the route's own `components/` folder. A component moves to
`components/shared/` the moment a second route imports it. This prevents the flat
eighty-file `components/` directory that models produce by default.

**Every route is registered in `src/router.tsx`.** One file shows the whole app.

**`components/ui/` is generated and never hand-edited.** New variants are added
with `cva` in a sibling file. Hand-edited primitives break when the shadcn CLI
regenerates them, and models hand-edit them freely unless told not to.

**Data access goes through `queryOptions` factories** in `src/lib/api/<resource>.ts`.
Components call `useQuery(usersQueryOptions())`. Query keys are never written
inline, which removes the cache invalidation bugs that appear when the same key is
typed slightly differently in two places.

**The API client is a thin `fetch` wrapper** in `src/lib/api/client.ts` reading its
base URL from an environment variable.

## Example routes

Four real working routes, not a parallel reference folder. Models copy real code
in the real structure far more reliably than they copy an `examples/` directory
they may read as stale.

1. **Dashboard.** Stat cards and one chart.
2. **Users.** Sortable and filterable data table built on @tanstack/react-table.
3. **Settings.** A validated form using react-hook-form and zod.
4. **Login.** A centered auth shell with no real authentication behind it.

All four read from `src/fixtures/`. `README.md` documents the delete step for
projects that do not need them.

## Design token layer

Stock shadcn already ships color tokens, so the added layer covers the three gaps
that cause visual drift. It lives in `src/styles/tokens.css` and runs about eighty
lines.

### Typography

Tailwind v4 allows a named text step to carry its line height and font weight.
The template defines roles rather than sizes.

```css
@theme {
  --text-display: 2.25rem;
  --text-display--line-height: 2.5rem;
  --text-display--font-weight: 600;

  --text-h1: 1.5rem;
  --text-h1--line-height: 2rem;
  --text-h1--font-weight: 600;

  --text-h2: 1.25rem;
  --text-h2--line-height: 1.75rem;
  --text-h2--font-weight: 600;

  --text-h3: 1rem;
  --text-h3--line-height: 1.5rem;
  --text-h3--font-weight: 600;

  --text-body-lg: 1rem;
  --text-body-lg--line-height: 1.5rem;
  --text-body-lg--font-weight: 400;

  --text-body: 0.875rem;
  --text-body--line-height: 1.25rem;
  --text-body--font-weight: 400;

  --text-label: 0.8125rem;
  --text-label--line-height: 1.25rem;
  --text-label--font-weight: 500;

  --text-caption: 0.75rem;
  --text-caption--line-height: 1rem;
  --text-caption--font-weight: 400;
}
```

The model writes `text-h2` and gets size, leading, and weight together. Pairing an
h2 size with a body weight is impossible because no such class exists. This is the
single highest-value piece of the token layer.

### Spacing

Allowed steps are 1, 2, 3, 4, 6, 8, 12, 16, and 24. Every other spacing value
fails lint. Nine options cover dashboard work and are narrow enough that gutters
align without deliberate effort.

### Surfaces

shadcn already provides `--background`, `--card`, `--popover`, and sidebar
variables. The template adds `--surface-sunken` for inset panels and table headers,
and `--row-height` so table density stays consistent. Light and dark are both
defined in `tokens.css` and toggled by class on the root element.

## Lint enforcement

The house rules live in a local ESLint plugin under `eslint/`, roughly 120 lines
of code. They are not taken from `eslint-plugin-tailwindcss`, whose Tailwind v4
support has lagged. The template's core guarantee should not depend on an external
release schedule.

| Rule | Fails on | Message points to |
|---|---|---|
| `no-raw-color` | `bg-[#1a1a1a]`, `text-gray-500`, `bg-zinc-800` | the semantic token |
| `no-raw-type` | `text-sm`, `text-2xl`, `font-bold` | the named type step |
| `approved-spacing` | any spacing step outside the nine allowed | the nearest allowed step |
| `no-arbitrary-values` | `w-[327px]` and similar | a token or utility |
| `use-ui-primitives` | bare `<button>`, `<input>`, `<select>`, `<textarea>` | the shadcn component |

Every message names the replacement, because the model reads the error rather than
the rule source. All rules are escapable with an eslint-disable comment so genuine
one-offs stay possible.

Standard TypeScript rules apply alongside these, including no implicit `any`.

## Pre-installed components

Around thirty-five shadcn primitives, chosen to cover dashboard work so the model
never stops to install a package mid-feature.

Forms and inputs, card, badge, avatar, separator, skeleton, table, tabs, dialog,
sheet, drawer, dropdown-menu, popover, tooltip, command, sonner, alert,
alert-dialog, breadcrumb, pagination, scroll-area, sidebar, calendar, chart,
progress, accordion, collapsible.

## Guidance system

### CLAUDE.md

Under 100 lines. It contains only what applies to every task.

- The stack, in two lines.
- The hard rules, one sentence each, each mirroring a lint rule so the model
  receives the same instruction twice.
- Where routes, queries, and shared components belong.
- The verify command.
- A pointer to `docs/recipes/README.md`.
- A definition of done.

Done means `bun run verify` passes, the page has been screenshotted and viewed,
and the browser console is clean. This section matters most, because models stop
early without it.

### Recipes

Six task-shaped guides in `docs/recipes/`.

1. Add a page
2. Add a data table
3. Add a form
4. Fetch and mutate data
5. Add a dialog or sheet
6. Add a chart

Each follows the same four-part shape. When to use it. The exact files to create.
A short working snippet taken from the template's own example routes. The two or
three mistakes that happen at this task.

`docs/recipes/README.md` lists one line per recipe so the model selects without
reading all six.

Recipes can rot when the template changes. The defense is that every snippet comes
from a real example route, so a drifted recipe surfaces through verify or the
screenshot step.

### Claude Code wiring

Deliberately minimal, so other tools lose nothing.

- `.claude/settings.json` pre-allows `bun run verify`, `bun run screenshot`, and
  `bun run ui:add`.
- One skill that fires on feature work and says only to read the recipe index,
  pick the matching recipe, and follow the definition of done.

All substantive content stays in `docs/`, which keeps Cursor and other tools at
full value.

## Verify loop

`bun run verify` runs typecheck, then lint, then tests, in sequence. Target
runtime is ten to fifteen seconds on a template-sized project. It is one command
because a model asked to remember three will run one and claim success.

## Screenshot loop

`bun run screenshot /users` does the following.

1. Starts the dev server if it is not already running.
2. Loads the route in Playwright.
3. Captures at 1440 by 900 and at 390 by 844.
4. Writes both files into `.screenshots/`.
5. Collects console errors and exits non-zero if any occurred.

The model then opens the PNG and looks at it. The console check turns a silent
runtime crash into a failure that cannot be missed.

`.screenshots/` is gitignored.

## Testing

Vitest with jsdom and Testing Library. Three example tests showing the patterns
worth copying.

1. A component render test.
2. A form submission test.
3. A query hook test with a mocked client.

The template carries no further suite. The purpose is giving the model an obvious
place to put a test and an obvious shape to copy.

## Scripts

| Script | Does |
|---|---|
| `dev` | Vite dev server |
| `build` | typecheck and production build |
| `preview` | serve the build |
| `verify` | typecheck, lint, test |
| `typecheck` | `tsc --noEmit` |
| `lint` | ESLint across `src` |
| `test` | Vitest run |
| `screenshot` | Playwright capture for a route |
| `ui:add` | wraps the shadcn CLI |

## Distribution

A GitHub template repository. Clone or use the template button, rename the project
in `package.json`, run `bun install`, and start. No generator to maintain.

## Risks

**Lint rules too strict.** If the house rules block legitimate work, models will
scatter eslint-disable comments. Mitigation is that every rule carries a clear
replacement in its message, and the example routes pass with zero disables. If
disables start appearing, the rule is wrong and should be relaxed.

**Recipe drift.** Covered above by sourcing snippets from real routes.

**Example routes left behind.** A project that ships with an unused users table is
worse than one with none. Mitigation is a documented delete step in `README.md`
listing the exact folders and router entries to remove.

**Local ESLint plugin maintenance.** Roughly 120 lines of our own code to keep
working against future ESLint versions. Accepted in exchange for not depending on
third-party Tailwind v4 support.

---
name: building-features
description: Use when adding or changing any UI in this repo - a page, table, form, dialog, chart, or data fetching. Routes the work through the project recipes and the definition of done.
---

# Building features in this repo

All the detail lives in `docs/` so every tool gets it. This skill only makes
sure you go there first.

## Steps

1. Read `docs/recipes/README.md` and open the recipe matching the task. For work
   that spans two (a page containing a table), read both.
2. Read the working example the recipe points at before writing anything. The
   example is current; copy its shape.
3. Follow the hard rules in `CLAUDE.md`. They are enforced by lint, so breaking
   one fails the build.
4. Finish with the definition of done:
   - `bun run verify` exits clean.
   - `bun run screenshot /your-route` exits clean.
   - **Open the PNG in `.screenshots/` and actually look at it.** Add `--dark`
     to check the other theme. Skipping this is how a broken layout ships.
5. New behavior gets a test, copied from the nearest `*.test.tsx`.

## Do not

- Hand-edit `src/components/ui/`. Run `bun run ui:add <name>` instead.
- Reach for `npm`. This repo uses bun.
- Silence a house lint rule with `eslint-disable` to move faster. Say the rule
  is wrong and why.

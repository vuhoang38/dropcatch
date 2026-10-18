import { RuleTester } from 'eslint'
import tsParser from '@typescript-eslint/parser'
import { describe, it } from 'vitest'
import house from './house-rules.js'

/**
 * These tests guard the guardrails. If a house rule silently stops matching,
 * the whole template quietly degrades into plain shadcn, so the rules get the
 * same treatment as application code.
 */
const ruleTester = new RuleTester({
  languageOptions: {
    parser: tsParser,
    parserOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      ecmaFeatures: { jsx: true },
    },
  },
})

// RuleTester drives describe/it itself; vitest globals pick it up.
void describe
void it

ruleTester.run('no-raw-color', house.rules['no-raw-color'], {
  valid: [
    '<div className="bg-background text-muted-foreground" />',
    '<div className="bg-primary/80 border-border" />',
    '<div className="bg-surface-sunken" />',
    '<div className="bg-transparent fill-current" />',
    'cn("text-foreground", isActive && "bg-accent")',
  ],
  invalid: [
    { code: '<div className="bg-[#1a1a1a]" />', errors: [{ messageId: 'arbitrary' }] },
    { code: '<div className="text-gray-500" />', errors: [{ messageId: 'palette' }] },
    { code: '<div className="dark:bg-zinc-800" />', errors: [{ messageId: 'palette' }] },
    { code: '<div className="bg-zinc-800/50" />', errors: [{ messageId: 'palette' }] },
    { code: '<div className="text-white" />', errors: [{ messageId: 'palette' }] },
    { code: 'cn("border-red-500")', errors: [{ messageId: 'palette' }] },
    { code: '<div className={`bg-[rgb(0,0,0)]`} />', errors: [{ messageId: 'arbitrary' }] },
  ],
})

ruleTester.run('no-raw-type', house.rules['no-raw-type'], {
  valid: [
    '<p className="text-body" />',
    '<h2 className="text-h2" />',
    '<span className="text-caption text-muted-foreground" />',
    '<span className="font-mono" />',
    '<span className="text-balance" />',
  ],
  invalid: [
    { code: '<p className="text-sm" />', errors: [{ messageId: 'size' }] },
    { code: '<p className="text-2xl" />', errors: [{ messageId: 'size' }] },
    { code: '<p className="font-semibold" />', errors: [{ messageId: 'weight' }] },
    { code: '<p className="md:text-base" />', errors: [{ messageId: 'size' }] },
    { code: '<p className="text-[13px]" />', errors: [{ messageId: 'arbitrary' }] },
  ],
})

ruleTester.run('approved-spacing', house.rules['approved-spacing'], {
  valid: [
    '<div className="p-4 gap-2 mt-8" />',
    '<div className="px-0 space-y-6" />',
    '<div className="mx-auto p-px" />',
    '<div className="h-row w-full" />',
    '<div className="sm:gap-12" />',
  ],
  invalid: [
    { code: '<div className="p-5" />', errors: [{ messageId: 'offScale' }] },
    { code: '<div className="gap-7" />', errors: [{ messageId: 'offScale' }] },
    { code: '<div className="-mt-5" />', errors: [{ messageId: 'offScale' }] },
    { code: '<div className="lg:space-y-9" />', errors: [{ messageId: 'offScale' }] },
  ],
})

ruleTester.run('no-arbitrary-values', house.rules['no-arbitrary-values'], {
  valid: [
    '<div className="w-full max-w-2xl" />',
    '<div className="group-data-[collapsible=icon]:hidden" />',
    '<div className="data-[state=open]:rotate-180" />',
  ],
  invalid: [
    { code: '<div className="w-[327px]" />', errors: [{ messageId: 'arbitrary' }] },
    { code: '<div className="grid-cols-[1fr_auto]" />', errors: [{ messageId: 'arbitrary' }] },
  ],
})

ruleTester.run('use-ui-primitives', house.rules['use-ui-primitives'], {
  valid: [
    '<Button>Save</Button>',
    '<Input value="" />',
    '<div><span /></div>',
    '<form><Label /></form>',
  ],
  invalid: [
    { code: '<button>Save</button>', errors: [{ messageId: 'bare' }] },
    { code: '<input />', errors: [{ messageId: 'bare' }] },
    { code: '<textarea />', errors: [{ messageId: 'bare' }] },
    { code: '<select><option /></select>', errors: [{ messageId: 'bare' }] },
  ],
})

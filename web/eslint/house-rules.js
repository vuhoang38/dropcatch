/**
 * House rules. These turn the template's design conventions into build errors.
 *
 * Every message names the replacement, because the model reads the error and
 * not the rule source. Each rule is escapable with an eslint-disable comment for
 * the genuine one-off. If disables start piling up, the rule is wrong, not the
 * code.
 */

import { createClassVisitors, utilities } from './class-utils.js'

const COLOR_PREFIXES = [
  'bg', 'text', 'border', 'fill', 'stroke', 'ring', 'outline', 'shadow', 'accent',
  'caret', 'decoration', 'divide', 'placeholder', 'from', 'via', 'to',
]

const TAILWIND_PALETTES = [
  'slate', 'gray', 'zinc', 'neutral', 'stone', 'red', 'orange', 'amber', 'yellow',
  'lime', 'green', 'emerald', 'teal', 'cyan', 'sky', 'blue', 'indigo', 'violet',
  'purple', 'fuchsia', 'pink', 'rose',
]

const SEMANTIC_SUGGESTION = {
  bg: 'bg-background, bg-card, bg-muted, bg-primary or bg-surface-sunken',
  text: 'text-foreground, text-muted-foreground or text-primary-foreground',
  border: 'border-border or border-input',
  ring: 'ring-ring',
  fill: 'fill-primary or a chart token such as fill-chart-1',
  stroke: 'stroke-primary or a chart token such as stroke-chart-1',
}

const paletteColor = new RegExp(
  `^-?(${COLOR_PREFIXES.join('|')})-(${TAILWIND_PALETTES.join('|')})-(\\d{2,3})$`,
)
const plainColor = new RegExp(`^-?(${COLOR_PREFIXES.join('|')})-(white|black)$`)
const arbitraryColor = new RegExp(
  `^-?(${COLOR_PREFIXES.join('|')})-\\[\\s*(#|rgb|rgba|hsl|hsla|oklch|oklab|lab|lch|color-mix)`,
  'i',
)

const RAW_TEXT_SIZE = /^-?text-(xs|sm|base|lg|xl|[2-9]xl)$/
const RAW_FONT_WEIGHT =
  /^-?font-(thin|extralight|light|normal|medium|semibold|bold|extrabold|black)$/
const ARBITRARY_TEXT = /^-?text-\[/

const SPACING_PREFIXES = [
  'p', 'px', 'py', 'pt', 'pr', 'pb', 'pl', 'ps', 'pe',
  'm', 'mx', 'my', 'mt', 'mr', 'mb', 'ml', 'ms', 'me',
  'gap', 'gap-x', 'gap-y', 'space-x', 'space-y',
]
const ALLOWED_SPACING = ['0', '1', '2', '3', '4', '6', '8', '12', '16', '24', 'px', 'auto', 'row', 'row-compact']
const spacingUtility = new RegExp(`^-?(${SPACING_PREFIXES.join('|')})-(.+)$`)

const RAW_ELEMENT_REPLACEMENT = {
  button: 'Button from @/components/ui/button',
  input: 'Input from @/components/ui/input',
  select: 'Select from @/components/ui/select',
  textarea: 'Textarea from @/components/ui/textarea',
}

/** @type {import('eslint').Rule.RuleModule} */
const noRawColor = {
  meta: {
    type: 'problem',
    docs: { description: 'Use semantic color tokens instead of raw or palette colors.' },
    schema: [],
    messages: {
      arbitrary:
        '"{{klass}}" hard-codes a color. Use a semantic token ({{suggestion}}), or add one to src/styles/tokens.css.',
      palette:
        '"{{klass}}" uses a raw Tailwind palette color. Use a semantic token ({{suggestion}}) so the app re-skins from src/styles/tokens.css.',
    },
  },
  create(context) {
    return createClassVisitors((classString, node) => {
      for (const utility of utilities(classString)) {
        const prefix = utility.replace(/^-/, '').split('-')[0]
        const suggestion = SEMANTIC_SUGGESTION[prefix] ?? 'a token from src/styles/tokens.css'

        if (arbitraryColor.test(utility)) {
          context.report({ node, messageId: 'arbitrary', data: { klass: utility, suggestion } })
        } else if (paletteColor.test(utility) || plainColor.test(utility)) {
          context.report({ node, messageId: 'palette', data: { klass: utility, suggestion } })
        }
      }
    })
  },
}

/** @type {import('eslint').Rule.RuleModule} */
const noRawType = {
  meta: {
    type: 'problem',
    docs: { description: 'Use the named type scale instead of raw sizes and weights.' },
    schema: [],
    messages: {
      size:
        '"{{klass}}" sets a raw font size. Use a type step instead: text-display, text-h1, text-h2, text-h3, text-body-lg, text-body, text-label or text-caption. Each one carries its own line height and weight.',
      weight:
        '"{{klass}}" sets a font weight directly. Weight belongs to the type step, so pick the step that already has it (text-h3 and text-label are the semibold and medium steps).',
      arbitrary:
        '"{{klass}}" hard-codes a font size. Use a type step, or add one to the @theme block in src/styles/tokens.css.',
    },
  },
  create(context) {
    return createClassVisitors((classString, node) => {
      for (const utility of utilities(classString)) {
        if (RAW_TEXT_SIZE.test(utility)) {
          context.report({ node, messageId: 'size', data: { klass: utility } })
        } else if (RAW_FONT_WEIGHT.test(utility)) {
          context.report({ node, messageId: 'weight', data: { klass: utility } })
        } else if (ARBITRARY_TEXT.test(utility)) {
          context.report({ node, messageId: 'arbitrary', data: { klass: utility } })
        }
      }
    })
  },
}

function nearestAllowedSpacing(value) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return ALLOWED_SPACING.join(', ')
  const numeric = ALLOWED_SPACING.map(Number).filter(Number.isFinite)
  return String(
    numeric.reduce((best, step) =>
      Math.abs(step - parsed) < Math.abs(best - parsed) ? step : best,
    ),
  )
}

/** @type {import('eslint').Rule.RuleModule} */
const approvedSpacing = {
  meta: {
    type: 'problem',
    docs: { description: 'Restrict padding, margin and gap to the approved spacing rhythm.' },
    schema: [],
    messages: {
      offScale:
        '"{{klass}}" is off the spacing scale. Allowed steps are 0, 1, 2, 3, 4, 6, 8, 12, 16 and 24. Use {{prefix}}-{{nearest}} instead.',
    },
  },
  create(context) {
    return createClassVisitors((classString, node) => {
      for (const utility of utilities(classString)) {
        const match = spacingUtility.exec(utility)
        if (!match) continue
        const [, prefix, value] = match
        if (ALLOWED_SPACING.includes(value)) continue
        if (value.startsWith('[') || value.startsWith('(')) continue // handled by no-arbitrary-values
        context.report({
          node,
          messageId: 'offScale',
          data: { klass: utility, prefix, nearest: nearestAllowedSpacing(value) },
        })
      }
    })
  },
}

/** @type {import('eslint').Rule.RuleModule} */
const noArbitraryValues = {
  meta: {
    type: 'problem',
    docs: { description: 'Disallow arbitrary Tailwind values in application code.' },
    schema: [],
    messages: {
      arbitrary:
        '"{{klass}}" uses an arbitrary value. Use a scale step or a token from src/styles/tokens.css. If this one genuinely needs a custom value, add a token or disable this rule on the line with a reason.',
    },
  },
  create(context) {
    return createClassVisitors((classString, node) => {
      for (const utility of utilities(classString)) {
        if (utility.includes('[')) {
          context.report({ node, messageId: 'arbitrary', data: { klass: utility } })
        }
      }
    })
  },
}

/** @type {import('eslint').Rule.RuleModule} */
const useUiPrimitives = {
  meta: {
    type: 'problem',
    docs: { description: 'Use shadcn primitives instead of bare interactive HTML elements.' },
    schema: [],
    messages: {
      bare:
        '<{{element}}> is unstyled and skips the design system. Use {{replacement}} instead.',
    },
  },
  create(context) {
    return {
      JSXOpeningElement(node) {
        if (node.name.type !== 'JSXIdentifier') return
        const element = node.name.name
        const replacement = RAW_ELEMENT_REPLACEMENT[element]
        if (!replacement) return
        context.report({ node, messageId: 'bare', data: { element, replacement } })
      },
    }
  },
}

export default {
  meta: { name: 'house', version: '1.0.0' },
  rules: {
    'no-raw-color': noRawColor,
    'no-raw-type': noRawType,
    'approved-spacing': approvedSpacing,
    'no-arbitrary-values': noArbitraryValues,
    'use-ui-primitives': useUiPrimitives,
  },
}

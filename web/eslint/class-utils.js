/**
 * Shared helpers for the house Tailwind rules.
 *
 * These rules read class strings out of JSX `className` attributes and out of
 * calls to cn / clsx / cva / tv, then check each utility token.
 */

const CLASS_FUNCTIONS = new Set(['cn', 'clsx', 'cva', 'tv', 'twMerge', 'twJoin'])

/**
 * Strip variant prefixes (`sm:`, `hover:`, `group-data-[x=y]:`) from a token and
 * return the utility itself. Colons inside brackets or parens are not separators.
 */
function baseUtility(token) {
  let depth = 0
  let lastSeparator = -1
  for (let i = 0; i < token.length; i++) {
    const char = token[i]
    if (char === '[' || char === '(') depth++
    else if (char === ']' || char === ')') depth--
    else if (char === ':' && depth === 0) lastSeparator = i
  }
  return token.slice(lastSeparator + 1)
}

/** Drop the `!` important marker and any `/50` opacity modifier. */
function normalise(utility) {
  let value = utility.startsWith('!') ? utility.slice(1) : utility
  let depth = 0
  for (let i = 0; i < value.length; i++) {
    const char = value[i]
    if (char === '[' || char === '(') depth++
    else if (char === ']' || char === ')') depth--
    else if (char === '/' && depth === 0) return value.slice(0, i)
  }
  return value
}

/** Split a class string into normalised utility tokens, keeping blanks out. */
function utilities(classString) {
  return classString
    .split(/\s+/)
    .filter(Boolean)
    .map((token) => normalise(baseUtility(token)))
    .filter(Boolean)
}

/** Does this node sit inside a className attribute or a class-name function call? */
function collectClassStrings(node, report) {
  function walk(current) {
    if (!current || typeof current.type !== 'string') return

    if (current.type === 'Literal' && typeof current.value === 'string') {
      report(current.value, current)
      return
    }

    if (current.type === 'TemplateLiteral') {
      for (const quasi of current.quasis) report(quasi.value.cooked ?? '', quasi)
      for (const expression of current.expressions) walk(expression)
      return
    }

    if (current.type === 'ArrayExpression') {
      for (const element of current.elements) walk(element)
      return
    }

    if (current.type === 'ObjectExpression') {
      for (const property of current.properties) {
        if (property.type === 'Property') {
          if (property.key.type === 'Literal' && typeof property.key.value === 'string') {
            report(property.key.value, property.key)
          }
          walk(property.value)
        }
      }
      return
    }

    if (current.type === 'ConditionalExpression') {
      walk(current.consequent)
      walk(current.alternate)
      return
    }

    if (current.type === 'LogicalExpression') {
      walk(current.left)
      walk(current.right)
      return
    }

    if (current.type === 'CallExpression') {
      for (const argument of current.arguments) walk(argument)
    }
  }

  walk(node)
}

/**
 * Build the two visitors every house rule needs: one for className attributes
 * and one for cn / clsx / cva calls.
 */
function createClassVisitors(check) {
  return {
    JSXAttribute(node) {
      const name = node.name.type === 'JSXIdentifier' ? node.name.name : null
      if (name !== 'className' && name !== 'class') return
      if (!node.value) return

      const value =
        node.value.type === 'JSXExpressionContainer' ? node.value.expression : node.value
      collectClassStrings(value, check)
    },

    CallExpression(node) {
      const callee = node.callee
      const name =
        callee.type === 'Identifier'
          ? callee.name
          : callee.type === 'MemberExpression' && callee.property.type === 'Identifier'
            ? callee.property.name
            : null
      if (!name || !CLASS_FUNCTIONS.has(name)) return
      for (const argument of node.arguments) collectClassStrings(argument, check)
    },
  }
}

export { baseUtility, normalise, utilities, createClassVisitors }

import { defineAsyncComponent, h, type Component } from 'vue'

// Resolves a Tabler icon Vue component by its kebab-case name (e.g.
// "shopping-cart" → IconShoppingCart). The icon set is imported as a single
// lazy chunk the first time any icon mounts, so it stays out of the main
// bundle yet supports *any* icon name (e.g. whatever the LLM suggests).

let barrel: Promise<Record<string, unknown>> | null = null
function loadBarrel(): Promise<Record<string, unknown>> {
  if (!barrel) barrel = import('@tabler/icons-vue') as unknown as Promise<Record<string, unknown>>
  return barrel
}

// kebab-case → Tabler component name. "a-b-2" → "IconAB2".
function componentName(name: string): string {
  return 'Icon' + name.split('-').map((p) => p.charAt(0).toUpperCase() + p.slice(1)).join('')
}

const Empty: Component = { render: () => h('span') }

// Valid Tabler names are lowercase letters/digits joined by hyphens. Anything
// else (emoji, legacy free-text) is not an icon name.
export function isIconName(value?: string | null): boolean {
  return !!value && /^[a-z0-9-]+$/.test(value)
}

const cache = new Map<string, Component>()

export function tablerIcon(name: string): Component {
  let cmp = cache.get(name)
  if (!cmp) {
    const wanted = componentName(name)
    cmp = defineAsyncComponent(async () => {
      const mod = await loadBarrel()
      return (mod[wanted] as Component) ?? Empty
    })
    cache.set(name, cmp)
  }
  return cmp
}

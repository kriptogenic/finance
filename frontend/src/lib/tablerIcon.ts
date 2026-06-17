// Tabler icons render via the vendored webfont (see main.ts) as
// `<i class="ti ti-<name>" />`, so an icon value is just the bare name.
export { ALL_ICON_NAMES } from './tablerIconNames'

// Valid Tabler names are lowercase letters/digits joined by hyphens. Anything
// else (emoji, legacy free-text) is not an icon name.
export function isIconName(value?: string | null): boolean {
  return !!value && /^[a-z0-9-]+$/.test(value)
}

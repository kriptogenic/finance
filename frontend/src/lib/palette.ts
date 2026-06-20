// A fixed 20-color palette for category colors. New categories default to a
// random one; the swatches let the user pick another.
export const categoryColors = [
  '#ef4444', // red
  '#f97316', // orange
  '#f59e0b', // amber
  '#eab308', // yellow
  '#84cc16', // lime
  '#22c55e', // green
  '#10b981', // emerald
  '#14b8a6', // teal
  '#06b6d4', // cyan
  '#0ea5e9', // sky
  '#3b82f6', // blue
  '#6366f1', // indigo
  '#8b5cf6', // violet
  '#a855f7', // purple
  '#d946ef', // fuchsia
  '#ec4899', // pink
  '#f43f5e', // rose
  '#64748b', // slate
  '#78716c', // stone
  '#b45309', // brown
] as const

export function randomCategoryColor(): string {
  return categoryColors[Math.floor(Math.random() * categoryColors.length)]
}

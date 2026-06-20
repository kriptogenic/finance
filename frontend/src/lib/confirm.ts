import { reactive } from 'vue'

// Promise-based confirm dialog. `confirm(...)` opens the global <ConfirmDialog>
// (mounted once in App.vue) and resolves true/false when the user answers — a
// styled, awaitable replacement for window.confirm.

export interface ConfirmOptions {
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  tone?: 'danger' | 'default'
}

interface ConfirmState extends Required<ConfirmOptions> {
  open: boolean
  resolve: ((ok: boolean) => void) | null
}

export const confirmState = reactive<ConfirmState>({
  open: false,
  title: '',
  message: '',
  confirmText: 'Delete',
  cancelText: 'Cancel',
  tone: 'danger',
  resolve: null,
})

export function confirm(opts: ConfirmOptions | string): Promise<boolean> {
  const o = typeof opts === 'string' ? { message: opts } : opts
  return new Promise((resolve) => {
    confirmState.title = o.title ?? ''
    confirmState.message = o.message
    confirmState.confirmText = o.confirmText ?? 'Delete'
    confirmState.cancelText = o.cancelText ?? 'Cancel'
    confirmState.tone = o.tone ?? 'danger'
    confirmState.open = true
    confirmState.resolve = resolve
  })
}

export function resolveConfirm(ok: boolean) {
  confirmState.resolve?.(ok)
  confirmState.open = false
  confirmState.resolve = null
}

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { confirmState, resolveConfirm } from '../lib/confirm'

// Sits above any Modal (z-50) so a delete confirm raised from inside a sheet
// still appears on top.
function onKey(e: KeyboardEvent) {
  if (!confirmState.open) return
  if (e.key === 'Escape') resolveConfirm(false)
  else if (e.key === 'Enter') resolveConfirm(true)
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <Teleport to="body">
    <div v-if="confirmState.open" class="fixed inset-0 z-[60] flex items-center justify-center p-4">
      <div class="fixed inset-0 bg-slate-900/50 backdrop-blur-sm" style="animation: fade-in 0.15s ease-out" @click="resolveConfirm(false)" />
      <div class="relative w-full max-w-sm rounded-2xl bg-white p-6 shadow-2xl" style="animation: pop-in 0.15s ease-out">
        <div class="flex items-start gap-3">
          <span
            class="grid h-10 w-10 shrink-0 place-items-center rounded-full text-xl"
            :class="confirmState.tone === 'danger' ? 'bg-rose-50 text-rose-600' : 'bg-amber-100 text-amber-700'"
          >
            <i class="ti ti-alert-triangle" />
          </span>
          <div class="min-w-0 pt-0.5">
            <h3 class="text-base font-semibold text-slate-900">{{ confirmState.title || 'Are you sure?' }}</h3>
            <p class="mt-1 text-sm text-slate-500">{{ confirmState.message }}</p>
          </div>
        </div>
        <div class="mt-6 flex justify-end gap-2">
          <button class="btn btn-soft" @click="resolveConfirm(false)">{{ confirmState.cancelText }}</button>
          <button
            class="btn"
            :class="confirmState.tone === 'danger' ? 'bg-rose-600 text-white hover:bg-rose-500' : 'btn-primary'"
            @click="resolveConfirm(true)"
          >
            {{ confirmState.confirmText }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

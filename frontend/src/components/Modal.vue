<script setup lang="ts">
withDefaults(defineProps<{ title?: string; size?: 'md' | 'lg' | 'xl' }>(), { title: '', size: 'md' })
defineEmits<{ close: [] }>()

const sizeClass: Record<string, string> = { md: 'sm:max-w-md', lg: 'sm:max-w-lg', xl: 'sm:max-w-3xl' }
</script>

<template>
  <Teleport to="body">
    <!-- Bottom sheet on mobile, centered dialog on sm+. -->
    <div class="fixed inset-0 z-50 flex items-end justify-center sm:items-center sm:overflow-y-auto sm:p-4">
      <div class="fixed inset-0 bg-slate-900/40 backdrop-blur-sm" style="animation: fade-in 0.15s ease-out" @click="$emit('close')" />
      <div
        class="relative max-h-[92vh] w-full overflow-y-auto rounded-t-3xl bg-white p-5 pb-[max(1.25rem,env(safe-area-inset-bottom))] shadow-2xl sm:my-8 sm:max-h-none sm:rounded-2xl sm:p-6 sm:pb-6 sm:ring-1 sm:ring-slate-200"
        :class="sizeClass[size]"
        style="animation: sheet-up 0.24s cubic-bezier(0.16, 1, 0.3, 1)"
      >
        <div class="mx-auto mb-3 h-1.5 w-10 rounded-full bg-slate-200 sm:hidden" />
        <div class="mb-5 flex items-center justify-between">
          <h3 class="text-lg font-semibold text-slate-900">{{ title }}</h3>
          <button class="grid h-8 w-8 place-items-center rounded-lg text-lg text-slate-400 hover:bg-slate-100 hover:text-slate-600" @click="$emit('close')"><i class="ti ti-x" /></button>
        </div>
        <slot />
      </div>
    </div>
  </Teleport>
</template>

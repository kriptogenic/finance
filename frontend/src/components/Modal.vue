<script setup lang="ts">
withDefaults(defineProps<{ title?: string; size?: 'md' | 'lg' | 'xl' }>(), { title: '', size: 'md' })
defineEmits<{ close: [] }>()

const sizeClass: Record<string, string> = { md: 'max-w-md', lg: 'max-w-lg', xl: 'max-w-3xl' }
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4 sm:items-center">
      <div class="fixed inset-0 bg-slate-900/40 backdrop-blur-sm" @click="$emit('close')" />
      <div class="relative my-8 w-full rounded-2xl bg-white p-6 shadow-2xl ring-1 ring-slate-200" :class="sizeClass[size]">
        <div class="mb-5 flex items-center justify-between">
          <h3 class="text-lg font-semibold text-slate-900">{{ title }}</h3>
          <button class="grid h-8 w-8 place-items-center rounded-lg text-slate-400 hover:bg-slate-100 hover:text-slate-600" @click="$emit('close')">✕</button>
        </div>
        <slot />
      </div>
    </div>
  </Teleport>
</template>

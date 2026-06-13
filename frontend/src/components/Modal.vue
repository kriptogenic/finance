<script setup>
defineProps({ title: { type: String, default: '' } })
defineEmits(['close'])
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4 sm:items-center">
      <div class="fixed inset-0 bg-slate-900/40 backdrop-blur-sm" @click="$emit('close')" />
      <div class="relative my-8 w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl ring-1 ring-slate-200">
        <div class="mb-5 flex items-center justify-between">
          <h3 class="text-lg font-semibold text-slate-900">{{ title }}</h3>
          <button class="grid h-8 w-8 place-items-center rounded-lg text-slate-400 hover:bg-slate-100 hover:text-slate-600" @click="$emit('close')">✕</button>
        </div>
        <slot />
      </div>
    </div>
  </Teleport>
</template>

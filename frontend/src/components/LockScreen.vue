<script setup lang="ts">
import { computed, ref } from 'vue'
import { setPin, verifyPin, unlock } from '../lib/lock'

// Full-screen PIN gate. `mode` decides the flow:
//  - 'unlock' — verify the stored PIN to dismiss the lock.
//  - 'setup'  — enter a new PIN twice to confirm, then store it.
const props = defineProps<{ mode: 'unlock' | 'setup' }>()
const emit = defineEmits<{ done: []; cancel: [] }>()

const LEN = 4
const entry = ref('')
const confirming = ref(false)
const firstPin = ref('')
const error = ref('')
const shake = ref(false)

const title = computed(() => {
  if (props.mode === 'unlock') return 'Enter PIN'
  return confirming.value ? 'Confirm PIN' : 'Set a PIN'
})

const keys = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '', '0', 'back']

function buzz() {
  error.value = ''
  shake.value = true
  setTimeout(() => (shake.value = false), 400)
  entry.value = ''
}

async function complete() {
  if (props.mode === 'unlock') {
    if (await verifyPin(entry.value)) {
      unlock()
      emit('done')
    } else {
      error.value = 'Wrong PIN'
      buzz()
    }
    return
  }
  // setup
  if (!confirming.value) {
    firstPin.value = entry.value
    entry.value = ''
    confirming.value = true
    return
  }
  if (entry.value === firstPin.value) {
    await setPin(entry.value)
    emit('done')
  } else {
    error.value = "PINs don't match"
    confirming.value = false
    firstPin.value = ''
    buzz()
  }
}

function press(k: string) {
  if (k === 'back') {
    entry.value = entry.value.slice(0, -1)
    return
  }
  if (!k || entry.value.length >= LEN) return
  entry.value += k
  if (entry.value.length === LEN) complete()
}
</script>

<template>
  <div class="fixed inset-0 z-[70] flex flex-col items-center justify-center bg-emerald-950 px-6 text-white">
    <button
      v-if="mode === 'setup'"
      class="absolute top-4 right-4 text-sm font-medium text-slate-400 active:text-white"
      @click="emit('cancel')"
    >
      Cancel
    </button>

    <img src="/favicon.svg" alt="" class="mb-6 h-12 w-12 rounded-2xl bg-amber-400 p-1.5" />
    <h1 class="text-lg font-semibold tracking-tight">{{ title }}</h1>
    <p class="mt-1 h-5 text-sm" :class="error ? 'text-rose-400' : 'text-slate-400'">
      {{ error || (mode === 'setup' ? '4 digits' : 'Unlock to continue') }}
    </p>

    <!-- dots -->
    <div class="mt-6 mb-8 flex gap-4" :class="{ 'animate-shake': shake }">
      <span
        v-for="i in LEN"
        :key="i"
        class="h-3.5 w-3.5 rounded-full border-2 transition"
        :class="i <= entry.length ? 'border-amber-400 bg-amber-400' : 'border-slate-600'"
      />
    </div>

    <!-- keypad -->
    <div class="grid w-full max-w-xs grid-cols-3 gap-3">
      <button
        v-for="(k, idx) in keys"
        :key="idx"
        type="button"
        class="grid h-16 place-items-center rounded-2xl text-2xl font-medium select-none"
        :class="k === '' ? 'pointer-events-none opacity-0' : 'bg-white/5 active:bg-white/15'"
        @click="press(k)"
      >
        <svg v-if="k === 'back'" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 9.75 14.25 12m0 0 2.25 2.25M14.25 12l2.25-2.25M14.25 12 12 14.25m-2.58 4.92-6.374-6.375a1.125 1.125 0 0 1 0-1.59L9.42 4.83c.21-.211.497-.33.795-.33H19.5a2.25 2.25 0 0 1 2.25 2.25v10.5a2.25 2.25 0 0 1-2.25 2.25h-9.284c-.298 0-.585-.119-.795-.33Z" />
        </svg>
        <span v-else>{{ k }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
@keyframes shake {
  0%, 100% { transform: translateX(0); }
  20%, 60% { transform: translateX(-8px); }
  40%, 80% { transform: translateX(8px); }
}
.animate-shake { animation: shake 0.4s ease-in-out; }
</style>

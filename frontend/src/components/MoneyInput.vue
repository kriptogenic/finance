<script setup lang="ts">
// Masked amount input. Displays grouped digits ("100 000,23" — space thousands,
// comma decimal) while v-model stays a plain major-unit number, so callers keep
// passing it straight to toMinor(). Fraction digits follow the currency exponent.
import { ref, computed, watch } from 'vue'
import { exponentOf } from '../api/money'

const props = withDefaults(
  defineProps<{
    modelValue: number | string | null
    currency?: string
    allowNegative?: boolean
  }>(),
  { currency: 'UZS', allowNegative: false },
)
const emit = defineEmits<{ 'update:modelValue': [number | null] }>()

const display = ref('')
const maxFrac = computed(() => exponentOf(props.currency))

function group(intDigits: string): string {
  return intDigits.replace(/\B(?=(\d{3})+(?!\d))/g, ' ')
}

// Parse raw user text into a masked string + numeric value.
function sanitize(raw: string): { text: string; value: number | null } {
  const neg = props.allowNegative && raw.trim().startsWith('-')
  const cleaned = raw.replace(/\s/g, '').replace(/\./g, ',').replace(/[^\d,]/g, '')
  const comma = cleaned.indexOf(',')
  let intPart = comma === -1 ? cleaned : cleaned.slice(0, comma)
  let frac = comma === -1 ? null : cleaned.slice(comma + 1).replace(/,/g, '')

  intPart = intPart.replace(/^0+(?=\d)/, '')
  if (maxFrac.value === 0) frac = null
  else if (frac != null) frac = frac.slice(0, maxFrac.value)

  let text = group(intPart)
  if (frac != null) text = (text || '0') + ',' + frac
  if (neg && text !== '') text = '-' + text
  else if (neg) text = '-'

  const numStr = (neg ? '-' : '') + (intPart || '0') + (frac ? '.' + frac : '')
  const value = text === '' || text === '-' ? null : Number(numStr)
  return { text, value }
}

// Canonical display for an external numeric value ("1 500" -> "1 500,00").
function formatValue(v: number | string | null): string {
  if (v === '' || v == null) return ''
  const n = Number(v)
  if (Number.isNaN(n)) return ''
  const fixed = Math.abs(n).toFixed(maxFrac.value)
  const [int, frac] = fixed.split('.')
  let text = group(int)
  if (frac) text += ',' + frac
  return (n < 0 ? '-' : '') + text
}

// Position the caret so the same number of digits stay to its right; this is
// stable across regrouping and keeps the caret past a just-typed separator.
function caretFromDigitsRight(text: string, n: number): number {
  if (n <= 0) return text.length
  let count = 0
  for (let i = text.length - 1; i >= 0; i--) {
    if (/\d/.test(text[i])) {
      count++
      if (count === n) return i
    }
  }
  return text.startsWith('-') ? 1 : 0
}

function onInput(e: Event) {
  const el = e.target as HTMLInputElement
  const caret = el.selectionStart ?? el.value.length
  const digitsAfter = el.value.slice(caret).replace(/\D/g, '').length

  const { text, value } = sanitize(el.value)
  // Reset value + caret synchronously so the next keystroke sees the masked
  // (e.g. fraction-truncated) text — an async update would race fast typing.
  el.value = text
  display.value = text
  const pos = caretFromDigitsRight(text, digitsAfter)
  el.setSelectionRange(pos, pos)
  emit('update:modelValue', value)
}

// Tidy trailing separators on blur ("100," -> "100,00").
function onBlur() {
  display.value = formatValue(sanitize(display.value).value)
}

// Sync from outside without clobbering equal in-progress edits.
watch(
  () => props.modelValue,
  (v) => {
    const shown = sanitize(display.value).value
    const incoming = v === '' || v == null ? null : Number(v)
    if (incoming !== shown) display.value = formatValue(incoming)
  },
  { immediate: true },
)
</script>

<template>
  <input
    :value="display"
    type="text"
    inputmode="decimal"
    @input="onInput"
    @blur="onBlur"
  />
</template>

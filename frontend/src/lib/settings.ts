import { ref, watch } from 'vue'

// User display preferences, persisted to localStorage. These are read inside
// Money.format(), so toggling re-renders every money value reactively.

const HIDE_MINOR_KEY = 'pref:hideMinorUnits'

// When on, displayed money is rounded to whole units (no cents). Amount *inputs*
// are unaffected — they format via MoneyInput, not Money.format().
export const hideMinorUnits = ref(localStorage.getItem(HIDE_MINOR_KEY) === '1')

watch(hideMinorUnits, (v) => {
  localStorage.setItem(HIDE_MINOR_KEY, v ? '1' : '0')
})

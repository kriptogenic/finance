<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import { budgetsApi } from '../api/budgets'
import { errMessage } from '../api/client'
import { confirm } from '../lib/confirm'
import type { Budget, BudgetPeriod, Category, CreateBudgetRequest, UpdateBudgetRequest } from '../api/types'
import { toMinor, toMajor } from '../lib/format'
import Modal from './Modal.vue'
import MoneyInput from './MoneyInput.vue'

const props = withDefaults(
  defineProps<{
    budget?: Budget | null
    categories?: Category[]
    budgetedIds?: string[]
    base?: string
  }>(),
  { budget: null, categories: () => [], budgetedIds: () => [], base: 'UZS' },
)
const emit = defineEmits<{ close: []; saved: [] }>()

const editing = computed(() => !!props.budget)
const error = ref('')
const saving = ref(false)
const removing = ref(false)

async function del() {
  if (!props.budget) return
  if (!(await confirm({ title: 'Delete budget?', message: `The budget for "${props.budget.category_name}" will be deleted.` }))) return
  removing.value = true
  error.value = ''
  try {
    await budgetsApi.remove(props.budget.id)
    emit('saved')
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    removing.value = false
  }
}

const form = reactive({
  categoryId: props.budget?.category_id ?? '',
  period: (props.budget?.period ?? 'monthly') as BudgetPeriod,
  amount: props.budget ? toMajor(props.budget.amount.amount, props.budget.amount.currency) : ('' as number | string),
  rollover: props.budget?.rollover ?? false,
  startPeriod: props.budget?.start_period ?? '',
})

// expense categories without a budget yet (for the create dropdown)
const options = computed(() =>
  props.categories.filter((c) => c.type === 'expense' && !c.archived && !props.budgetedIds.includes(c.id)),
)

async function submit() {
  error.value = ''
  saving.value = true
  try {
    if (props.budget) {
      const body: UpdateBudgetRequest = {
        period: form.period,
        amount: toMinor(form.amount, props.base),
        rollover: form.rollover,
        ...(form.startPeriod ? { start_period: form.startPeriod } : {}),
      }
      await budgetsApi.update(props.budget.id, body)
    } else {
      const body: CreateBudgetRequest = {
        category_id: form.categoryId,
        period: form.period,
        amount: toMinor(form.amount, props.base),
        rollover: form.rollover,
        ...(form.startPeriod ? { start_period: form.startPeriod } : {}),
      }
      await budgetsApi.create(body)
    }
    emit('saved')
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Modal :title="editing ? 'Edit budget' : 'New budget'" @close="emit('close')">
    <form class="space-y-4" @submit.prevent="submit">
      <div v-if="!editing">
        <label class="lbl">Category</label>
        <select v-model="form.categoryId" class="field" required>
          <option value="" disabled>Select an expense category…</option>
          <option v-for="c in options" :key="c.id" :value="c.id">{{ c.parent_id ? '— ' : '' }}{{ c.name }}</option>
        </select>
      </div>
      <div v-else>
        <label class="lbl">Category</label>
        <p class="font-medium text-slate-800">{{ budget?.category_name }}</p>
      </div>

      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="lbl">Period</label>
          <select v-model="form.period" class="field">
            <option value="weekly">Weekly</option>
            <option value="monthly">Monthly</option>
            <option value="yearly">Yearly</option>
          </select>
        </div>
        <div>
          <label class="lbl">Limit ({{ base }})</label>
          <MoneyInput v-model="form.amount" :currency="base" class="field" required placeholder="0,00" />
        </div>
      </div>

      <div class="grid grid-cols-2 items-end gap-3">
        <div>
          <label class="lbl">Start (optional)</label>
          <input v-model="form.startPeriod" type="date" class="field" />
        </div>
        <label class="flex items-center gap-2 pb-2">
          <input v-model="form.rollover" type="checkbox" class="h-4 w-4 rounded border-slate-300" />
          <span class="text-sm text-slate-600">Roll over unused</span>
        </label>
      </div>

      <p v-if="error" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600">{{ error }}</p>

      <div class="flex items-center gap-2 pt-1">
        <button v-if="editing" type="button" class="btn btn-danger" :disabled="saving || removing" @click="del">Delete</button>
        <button type="button" class="btn btn-soft ml-auto" @click="emit('close')">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="saving || removing">{{ saving ? 'Saving…' : 'Save' }}</button>
      </div>
    </form>
  </Modal>
</template>

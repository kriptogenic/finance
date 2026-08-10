<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import type { Category } from '../api/types'
import Modal from './Modal.vue'
import CategoryIcon from './CategoryIcon.vue'

const props = withDefaults(
  defineProps<{ category: Category; subcategories?: Category[]; spent?: string | null }>(),
  { subcategories: () => [], spent: null },
)
const emit = defineEmits<{ edit: [Category]; remove: [Category]; addSub: [Category]; close: [] }>()

const tint = computed(() => (props.category.color ? { backgroundColor: props.category.color + '22' } : undefined))

const router = useRouter()
// Navigate first, then close: the modal's history sentinel is only safe to drop
// once the new entry is on the stack.
async function viewTransactions() {
  await router.push({ name: 'transactions', query: { category: props.category.id } })
  emit('close')
}
</script>

<template>
  <Modal :title="category.name" @close="emit('close')">
    <div class="space-y-5">
      <!-- header -->
      <div class="flex flex-col items-center gap-2 text-center">
        <span class="grid h-16 w-16 place-items-center rounded-2xl" :class="category.color ? '' : 'bg-slate-100'" :style="tint">
          <CategoryIcon :icon="category.icon" :color="category.color" :size="30" />
        </span>
        <p v-if="spent" class="tabular text-xl font-bold text-slate-900">{{ spent }}</p>
        <p class="text-xs text-slate-400">{{ category.type === 'expense' ? 'Spent this month' : 'Income category' }}</p>
      </div>

      <!-- subcategories -->
      <div v-if="subcategories.length">
        <p class="lbl">Subcategories</p>
        <ul class="divide-y divide-slate-100">
          <li v-for="c in subcategories" :key="c.id" class="group flex items-center gap-3 py-2.5">
            <CategoryIcon :icon="c.icon" :color="c.color" :size="18" />
            <span class="min-w-0 flex-1 truncate text-sm text-slate-700">{{ c.name }}</span>
            <button class="grid h-7 w-7 place-items-center rounded-lg text-slate-400 hover:bg-slate-100 hover:text-slate-700" title="Edit" @click="emit('edit', c)"><i class="ti ti-pencil" /></button>
            <button class="grid h-7 w-7 place-items-center rounded-lg text-slate-400 hover:bg-rose-100 hover:text-rose-600" title="Delete" @click="emit('remove', c)"><i class="ti ti-trash" /></button>
          </li>
        </ul>
      </div>

      <button class="btn btn-soft w-full" @click="emit('addSub', category)">
        <i class="ti ti-plus" /> Add subcategory
      </button>

      <button class="btn btn-soft w-full" @click="viewTransactions">
        <i class="ti ti-list-details" /> View transactions
      </button>

      <div class="flex gap-2 border-t border-slate-100 pt-4">
        <button class="btn btn-danger flex-1" @click="emit('remove', category)">Delete</button>
        <button class="btn btn-primary flex-1" @click="emit('edit', category)">Edit</button>
      </div>
    </div>
  </Modal>
</template>

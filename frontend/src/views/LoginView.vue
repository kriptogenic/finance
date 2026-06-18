<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { setCredentials, verifyCredentials } from '../api/auth'
import { errMessage } from '../api/client'

const router = useRouter()
const route = useRoute()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const ok = await verifyCredentials(username.value, password.value)
    if (!ok) {
      error.value = 'Invalid login or password'
      return
    }
    setCredentials(username.value, password.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.push(redirect)
  } catch (e) {
    error.value = errMessage(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-slate-100 px-4">
    <form
      class="w-full max-w-sm space-y-5 rounded-3xl bg-white p-8 shadow-sm ring-1 ring-slate-200/70"
      @submit.prevent="submit"
    >
      <div class="flex items-center gap-2">
        <img src="/favicon.svg" alt="" class="h-9 w-9" />
        <span class="text-lg font-semibold tracking-tight text-slate-900">Mullajiring</span>
      </div>

      <div>
        <h1 class="text-xl font-bold tracking-tight text-slate-900">Sign in</h1>
        <p class="text-sm text-slate-500">Enter your credentials to continue</p>
      </div>

      <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-100">{{ error }}</p>

      <div>
        <label class="lbl">Login</label>
        <input v-model="username" class="field" autocomplete="username" required autofocus />
      </div>

      <div>
        <label class="lbl">Password</label>
        <input v-model="password" type="password" class="field" autocomplete="current-password" required />
      </div>

      <button type="submit" class="btn btn-primary w-full" :disabled="loading">
        {{ loading ? 'Signing in…' : 'Sign in' }}
      </button>
    </form>
  </div>
</template>

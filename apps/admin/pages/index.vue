<template>
  <section class="min-h-screen bg-[#f4f5f1] px-6 py-8 text-[#20231f] dark:bg-[#171916] dark:text-[#f2f3ef]">
    <div class="mx-auto grid min-h-[calc(100vh-4rem)] w-full max-w-6xl items-center gap-8 lg:grid-cols-[minmax(0,0.95fr)_minmax(360px,440px)]">
      <div class="hidden lg:block">
        <div class="max-w-xl">
          <div class="inline-flex h-12 w-12 items-center justify-center rounded-lg bg-[#165a4a] text-white shadow-sm">
            <ShieldCheck class="h-6 w-6" />
          </div>
          <p class="mt-8 text-sm font-medium uppercase text-[#5d6a61] dark:text-[#aeb8b0]">SEO Blog CMS</p>
          <h1 class="mt-3 max-w-lg text-4xl font-semibold leading-tight tracking-normal">Administration workspace</h1>
          <div class="mt-8 grid max-w-md gap-3 text-sm text-[#4f5b54] dark:text-[#c5cec8]">
            <div class="flex items-center gap-3 rounded-lg border border-[#d7ded8] bg-white px-4 py-3 shadow-sm dark:border-[#343a38] dark:bg-[#202422]">
              <LockKeyhole class="h-4 w-4 text-[#3162a3]" />
              <span>Invite-only access</span>
            </div>
            <div class="flex items-center gap-3 rounded-lg border border-[#d7ded8] bg-white px-4 py-3 shadow-sm dark:border-[#343a38] dark:bg-[#202422]">
              <Server class="h-4 w-4 text-[#165a4a]" />
              <span>Project-scoped admin</span>
            </div>
          </div>
        </div>
      </div>

      <form class="w-full space-y-5 rounded-lg border border-[#d8d8d0] bg-white p-6 shadow-sm dark:border-[#3d403a] dark:bg-[#252823]" @submit.prevent="signIn">
        <div class="flex items-start justify-between gap-4">
          <div>
            <p class="text-sm text-[#666b60] dark:text-[#aeb8b0]">SEO Blog CMS</p>
            <h2 class="mt-1 text-2xl font-semibold tracking-normal">Sign in</h2>
          </div>
          <div class="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-[#eef5f1] text-[#165a4a] dark:bg-[#13261e] dark:text-[#aee4d0]">
            <ShieldCheck class="h-5 w-5" />
          </div>
        </div>

        <label class="block space-y-2">
          <span class="text-sm font-medium">Email</span>
          <span class="relative block">
            <Mail class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#667169] dark:text-[#aeb8b0]" />
            <input v-model.trim="email" class="h-11 w-full rounded-md border border-[#c9c9bf] bg-white pl-10 pr-3 text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 dark:border-[#555a50] dark:bg-[#1c1e1b] dark:text-[#f2f3ef]" name="email" type="email" autocomplete="email" required />
          </span>
        </label>

        <label class="block space-y-2">
          <span class="text-sm font-medium">Password</span>
          <span class="relative block">
            <LockKeyhole class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#667169] dark:text-[#aeb8b0]" />
            <input v-model="password" class="h-11 w-full rounded-md border border-[#c9c9bf] bg-white pl-10 pr-12 text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 dark:border-[#555a50] dark:bg-[#1c1e1b] dark:text-[#f2f3ef]" name="password" :type="passwordVisible ? 'text' : 'password'" autocomplete="current-password" required minlength="8" />
            <button
              class="absolute right-1.5 top-1/2 inline-flex h-8 w-8 -translate-y-1/2 items-center justify-center rounded-md text-[#4f5b54] hover:bg-[#eef2ef] dark:text-[#c5cec8] dark:hover:bg-[#2a302d]"
              type="button"
              :title="passwordVisible ? 'Hide password' : 'Show password'"
              :aria-label="passwordVisible ? 'Hide password' : 'Show password'"
              @click="passwordVisible = !passwordVisible"
            >
              <EyeOff v-if="passwordVisible" class="h-4 w-4" />
              <Eye v-else class="h-4 w-4" />
            </button>
          </span>
        </label>

        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-3 py-2 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>
        <p v-if="successMessage" class="rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-3 py-2 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]">
          {{ successMessage }}
        </p>

        <button class="inline-flex h-11 w-full items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 font-medium text-white transition hover:bg-[#10463a] disabled:opacity-60" type="submit" :disabled="pending">
          <LoaderCircle v-if="pending" class="h-4 w-4 animate-spin" />
          <ArrowRight v-else class="h-4 w-4" />
          {{ pending ? 'Signing in...' : 'Continue' }}
        </button>
      </form>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ArrowRight, Eye, EyeOff, LoaderCircle, LockKeyhole, Mail, Server, ShieldCheck } from 'lucide-vue-next'

const email = ref('')
const password = ref('')
const passwordVisible = ref(false)
const pending = ref(false)
const errorMessage = ref('')
const successMessage = ref('')

async function signIn() {
  pending.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    await $fetch('/api/v1/auth/login', {
      method: 'POST',
      body: { email: email.value, password: password.value },
      credentials: 'include'
    })
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Sign-in failed. Check your details and try again.')
    pending.value = false
    return
  }

  successMessage.value = 'Signed in. Opening dashboard...'
  try {
    await navigateTo('/dashboard', { replace: true })
  } catch {
    errorMessage.value = 'Signed in, but the dashboard did not open. Go to /dashboard or refresh the page.'
  } finally {
    pending.value = false
  }
}

function normalizeAPIError(error: unknown, fallback: string) {
  if (typeof error === 'object' && error !== null && 'data' in error) {
    const data = (error as { data?: { title?: string, detail?: string, statusCode?: number, statusMessage?: string } }).data
    if (data?.statusCode === 502) {
      return 'The admin API is unavailable. Start the Go API on the configured proxy port or set NUXT_API_BASE_URL to the running API.'
    }
    return data?.detail || data?.title || fallback
  }
  return fallback
}
</script>

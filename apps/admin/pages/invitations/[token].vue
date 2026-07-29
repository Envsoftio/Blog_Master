<template>
  <section class="min-h-screen grid place-items-center px-6 py-10">
    <div class="w-full max-w-md">
      <form
        v-if="!acceptance"
        class="space-y-5 rounded-lg border border-[#d8d8d0] bg-white p-6 shadow-sm dark:border-[#3d403a] dark:bg-[#252823]"
        @submit.prevent="acceptInvitation"
      >
        <div class="flex items-start gap-3">
          <div class="mt-0.5 grid h-9 w-9 place-items-center rounded-md bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]">
            <UserCheck class="h-5 w-5" />
          </div>
          <div>
            <p class="text-sm text-[#666b60] dark:text-[#b8c2bb]">SEO Blog CMS</p>
            <h1 class="mt-1 text-2xl font-semibold tracking-normal">Accept invitation</h1>
          </div>
        </div>

        <label class="block space-y-2">
          <span class="text-sm font-medium">Account password</span>
          <input
            v-model="password"
            class="w-full rounded-md border border-[#c9c9bf] px-3 py-2 dark:border-[#555a50] dark:bg-[#1c1e1b]"
            name="password"
            type="password"
            autocomplete="current-password"
            required
            minlength="8"
          />
        </label>

        <label class="block space-y-2">
          <span class="text-sm font-medium">Confirm password</span>
          <input
            v-model="confirmation"
            class="w-full rounded-md border border-[#c9c9bf] px-3 py-2 dark:border-[#555a50] dark:bg-[#1c1e1b]"
            name="password-confirmation"
            type="password"
            autocomplete="current-password"
            required
            minlength="8"
          />
        </label>

        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>

        <button
          class="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
          type="submit"
          :disabled="pending || !canSubmit"
        >
          <LoaderCircle v-if="pending" class="h-4 w-4 animate-spin" />
          <UserCheck v-else class="h-4 w-4" />
          Activate account
        </button>
      </form>

      <div v-else class="space-y-5 rounded-lg border border-[#b9dcc9] bg-white p-6 shadow-sm dark:border-[#2d644a] dark:bg-[#252823]">
        <div class="flex items-start gap-3">
          <div class="mt-0.5 grid h-9 w-9 place-items-center rounded-md bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]">
            <Check class="h-5 w-5" />
          </div>
          <div class="min-w-0">
            <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Invitation accepted</p>
            <h1 class="mt-1 truncate text-2xl font-semibold tracking-normal">{{ acceptance.email }}</h1>
          </div>
        </div>
        <dl class="grid gap-3 text-sm sm:grid-cols-2">
          <div>
            <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Role</dt>
            <dd class="mt-1">{{ roleLabel(acceptance.role) }}</dd>
          </div>
          <div>
            <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Project</dt>
            <dd class="mt-1 truncate">{{ acceptance.projectId }}</dd>
          </div>
        </dl>
        <NuxtLink class="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 font-medium text-white hover:bg-[#10463a]" to="/">
          <LogIn class="h-4 w-4" />
          Sign in
        </NuxtLink>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { Check, LoaderCircle, LogIn, UserCheck } from 'lucide-vue-next'

type APIEnvelope<T> = {
  data: T
}

type InvitationAcceptance = {
  projectId: string
  userId: string
  email: string
  role: string
}

const route = useRoute()
const token = computed(() => {
  const value = route.params.token
  return Array.isArray(value) ? String(value[0] || '') : String(value || '')
})
const password = ref('')
const confirmation = ref('')
const pending = ref(false)
const errorMessage = ref('')
const acceptance = ref<InvitationAcceptance | null>(null)
const canSubmit = computed(() => password.value.length >= 8 && password.value === confirmation.value && Boolean(token.value))

async function acceptInvitation() {
  if (!canSubmit.value) {
    errorMessage.value = password.value !== confirmation.value
      ? 'Passwords do not match.'
      : 'Password must be at least 8 characters.'
    return
  }
  pending.value = true
  errorMessage.value = ''
  try {
    const response = await $fetch<APIEnvelope<InvitationAcceptance>>(`/api/v1/invitations/${encodeURIComponent(token.value)}/accept`, {
      method: 'POST',
      body: { password: password.value }
    })
    acceptance.value = response.data
    password.value = ''
    confirmation.value = ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error)
  } finally {
    pending.value = false
  }
}

function roleLabel(role: string) {
  return role.replaceAll('_', ' ')
}

function normalizeAPIError(error: unknown) {
  if (typeof error === 'object' && error !== null && 'data' in error) {
    const data = (error as { data?: { title?: string, detail?: string } }).data
    return data?.detail || data?.title || 'Could not accept this invitation.'
  }
  return 'Could not accept this invitation.'
}
</script>

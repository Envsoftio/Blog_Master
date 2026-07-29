<template>
  <section class="min-h-screen grid place-items-center px-6">
    <form class="w-full max-w-sm space-y-5 rounded-lg border border-[#d8d8d0] bg-white p-6 shadow-sm dark:border-[#3d403a] dark:bg-[#252823]" @submit.prevent="signIn">
      <div>
        <p class="text-sm text-[#666b60]">SEO Blog CMS</p>
        <h1 class="mt-1 text-2xl font-semibold tracking-normal">Sign in</h1>
      </div>

      <label class="block space-y-2">
        <span class="text-sm font-medium">Email</span>
        <input v-model.trim="email" class="w-full rounded-md border border-[#c9c9bf] px-3 py-2 dark:border-[#555a50] dark:bg-[#1c1e1b]" name="email" type="email" autocomplete="email" required />
      </label>

      <label class="block space-y-2">
        <span class="text-sm font-medium">Password</span>
        <input v-model="password" class="w-full rounded-md border border-[#c9c9bf] px-3 py-2 dark:border-[#555a50] dark:bg-[#1c1e1b]" name="password" type="password" autocomplete="current-password" required minlength="8" />
      </label>

      <p v-if="errorMessage" class="text-sm text-red-700" role="alert">{{ errorMessage }}</p>

      <button class="inline-flex w-full items-center justify-center rounded-md bg-[#1f4d3a] px-4 py-2 font-medium text-white disabled:opacity-60" type="submit" :disabled="pending">
        {{ pending ? 'Signing in…' : 'Continue' }}
      </button>
    </form>
  </section>
</template>

<script setup lang="ts">
const email = ref('')
const password = ref('')
const pending = ref(false)
const errorMessage = ref('')

async function signIn() {
  pending.value = true
  errorMessage.value = ''
  try {
    await $fetch('/api/v1/auth/login', {
      method: 'POST',
      body: { email: email.value, password: password.value },
      credentials: 'include'
    })
    await navigateTo('/projects')
  } catch {
    errorMessage.value = 'Sign-in failed. Check your details and try again.'
  } finally {
    pending.value = false
  }
}
</script>

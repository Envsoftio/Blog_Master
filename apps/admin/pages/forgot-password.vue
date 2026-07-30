<template>
  <AuthPanel
    eyebrow="Account recovery"
    title="Reset your password"
    description="Enter the email address associated with your invited account."
  >
    <form v-if="!submitted" class="auth-form" @submit.prevent="requestReset">
      <label class="field">
        <span>Email address</span>
        <span class="auth-input">
          <Mail :size="16" />
          <input v-model.trim="email" type="email" autocomplete="email" required placeholder="you@example.com">
        </span>
      </label>
      <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
      <button class="button button--primary auth-submit" type="submit" :disabled="pending">
        <LoaderCircle v-if="pending" class="spin" :size="16" />
        <Send v-else :size="16" />
        Send reset link
      </button>
      <NuxtLink class="auth-back" to="/"><ArrowLeft :size="14" />Back to sign in</NuxtLink>
    </form>
    <div v-else class="auth-confirmation">
      <span><MailCheck :size="22" /></span>
      <h2>Check your email</h2>
      <p>If an account exists for <strong>{{ email }}</strong>, a reset link has been sent.</p>
      <NuxtLink class="button button--primary" to="/">Return to sign in</NuxtLink>
    </div>
  </AuthPanel>
</template>

<script setup lang="ts">
import { ArrowLeft, LoaderCircle, Mail, MailCheck, Send } from 'lucide-vue-next'

const api = useAdminApi()
const email = ref('')
const pending = ref(false)
const submitted = ref(false)
const errorMessage = ref('')

async function requestReset() {
  pending.value = true
  errorMessage.value = ''
  try {
    await api.forgotPassword(email.value)
    submitted.value = true
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not request a password reset.')
  } finally {
    pending.value = false
  }
}
</script>

<style scoped>
.auth-form { display: grid; gap: 16px; }
.auth-input { position: relative; display: flex; align-items: center; color: var(--text-soft); }
.auth-input svg { position: absolute; z-index: 1; left: 12px; }
.auth-input input { min-height: 44px; padding-left: 39px; }
.auth-submit { width: 100%; min-height: 44px; }
.auth-back { display: inline-flex; align-items: center; justify-content: center; gap: 5px; color: var(--text-soft); font-size: 9px; text-decoration: none; }
.auth-confirmation { display: grid; justify-items: center; text-align: center; }
.auth-confirmation > span { display: grid; width: 46px; height: 46px; place-items: center; border-radius: 8px; background: var(--primary-soft); color: var(--primary); }
.auth-confirmation h2 { margin: 15px 0 0; font-size: 18px; }
.auth-confirmation p { margin: 7px 0 18px; color: var(--text-soft); font-size: 10px; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>

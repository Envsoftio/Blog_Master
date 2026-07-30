<template>
  <AuthPanel
    eyebrow="Account recovery"
    title="Choose a new password"
    description="Set a strong password for your administrator account."
  >
    <form v-if="!complete" class="auth-form" @submit.prevent="resetPassword">
      <label class="field">
        <span>New password</span>
        <span class="auth-input">
          <LockKeyhole :size="16" />
          <input v-model="password" :type="passwordVisible ? 'text' : 'password'" autocomplete="new-password" required minlength="15" maxlength="128">
          <button type="button" :title="passwordVisible ? 'Hide password' : 'Show password'" :aria-label="passwordVisible ? 'Hide password' : 'Show password'" @click="passwordVisible = !passwordVisible">
            <EyeOff v-if="passwordVisible" :size="16" /><Eye v-else :size="16" />
          </button>
        </span>
      </label>
      <label class="field">
        <span>Confirm password</span>
        <span class="auth-input">
          <LockKeyhole :size="16" />
          <input v-model="confirmation" :type="passwordVisible ? 'text' : 'password'" autocomplete="new-password" required minlength="15" maxlength="128">
        </span>
      </label>
      <div class="password-strength">
        <span><i :style="{ width: `${strength}%` }" /></span>
        <small>{{ strengthLabel }}</small>
      </div>
      <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
      <button class="button button--primary auth-submit" type="submit" :disabled="pending || !canSubmit">
        <LoaderCircle v-if="pending" class="spin" :size="16" />
        <KeyRound v-else :size="16" />
        Update password
      </button>
      <NuxtLink class="auth-back" to="/"><ArrowLeft :size="14" />Back to sign in</NuxtLink>
    </form>
    <div v-else class="auth-confirmation">
      <span><CircleCheck :size="22" /></span>
      <h2>Password updated</h2>
      <p>Your new password is ready to use.</p>
      <NuxtLink class="button button--primary" to="/">Sign in</NuxtLink>
    </div>
  </AuthPanel>
</template>

<script setup lang="ts">
import { ArrowLeft, CircleCheck, Eye, EyeOff, KeyRound, LoaderCircle, LockKeyhole } from 'lucide-vue-next'

const route = useRoute()
const api = useAdminApi()
const token = computed(() => String(route.query.token || ''))
const password = ref('')
const confirmation = ref('')
const passwordVisible = ref(false)
const pending = ref(false)
const complete = ref(false)
const errorMessage = ref('')
const canSubmit = computed(() => token.value.length > 0 && password.value.length >= 15 && password.value === confirmation.value)
const strength = computed(() => {
  let score = 0
  if (password.value.length >= 15) score += 35
  if (password.value.length >= 24) score += 15
  if (/[a-z]/.test(password.value) && /[A-Z]/.test(password.value)) score += 15
  if (/\d/.test(password.value)) score += 15
  if (/[^a-zA-Z0-9]/.test(password.value)) score += 20
  return Math.min(score, 100)
})
const strengthLabel = computed(() => {
  if (!password.value) return 'At least 15 characters'
  if (strength.value < 50) return 'Weak'
  if (strength.value < 75) return 'Good'
  return 'Strong'
})

onMounted(() => {
  if (!token.value) errorMessage.value = 'This reset link is missing its token.'
})

async function resetPassword() {
  if (!canSubmit.value) {
    errorMessage.value = password.value !== confirmation.value ? 'Passwords do not match.' : 'Password must be at least 15 characters.'
    return
  }
  pending.value = true
  errorMessage.value = ''
  try {
    await api.resetPassword(token.value, password.value)
    complete.value = true
  } catch (error) {
    errorMessage.value = apiStatus(error) === 501
      ? 'Password reset is not enabled on this backend.'
      : normalizeAPIError(error, 'Could not reset the password.')
  } finally {
    pending.value = false
  }
}
</script>

<style scoped>
.auth-form { display: grid; gap: 15px; }
.auth-input { position: relative; display: flex; align-items: center; color: var(--text-soft); }
.auth-input > svg:first-child { position: absolute; z-index: 1; left: 12px; }
.auth-input input { min-height: 44px; padding-left: 39px; padding-right: 40px; }
.auth-input button { position: absolute; right: 6px; display: grid; width: 32px; height: 32px; place-items: center; border: 0; border-radius: 5px; background: transparent; color: var(--text-soft); cursor: pointer; }
.password-strength { display: flex; align-items: center; gap: 9px; }
.password-strength > span { height: 4px; flex: 1; overflow: hidden; border-radius: 2px; background: var(--border); }
.password-strength i { display: block; height: 100%; background: var(--primary); transition: width .2s ease; }
.password-strength small { min-width: 80px; color: var(--text-soft); font-size: 8px; text-align: right; }
.auth-submit { width: 100%; min-height: 44px; }
.auth-back { display: inline-flex; align-items: center; justify-content: center; gap: 5px; color: var(--text-soft); font-size: 9px; text-decoration: none; }
.auth-confirmation { display: grid; justify-items: center; text-align: center; }
.auth-confirmation > span { display: grid; width: 46px; height: 46px; place-items: center; border-radius: 8px; background: var(--primary-soft); color: var(--primary); }
.auth-confirmation h2 { margin: 15px 0 0; font-size: 18px; }
.auth-confirmation p { margin: 7px 0 18px; color: var(--text-soft); font-size: 10px; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>

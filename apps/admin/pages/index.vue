<template>
  <section class="login-page">
    <div class="login-page__top">
      <NuxtLink class="login-brand" to="/" aria-label="Article Content Hub">
        <span><PenLine :size="19" /></span>
        <strong>Editorial</strong>
      </NuxtLink>
      <div class="theme-switcher" role="group" aria-label="Color theme">
        <button
          v-for="option in themeOptions"
          :key="option.value"
          class="theme-switcher__option"
          :class="{ 'is-active': colorMode.preference === option.value }"
          type="button"
          :title="`${option.label} theme`"
          :aria-label="`${option.label} theme`"
          :aria-pressed="colorMode.preference === option.value"
          @click="colorMode.preference = option.value"
        >
          <component :is="option.icon" :size="15" />
        </button>
      </div>
    </div>

    <div class="login-layout">
      <aside class="login-context">
        <span class="login-context__mark"><BookOpenText :size="28" /></span>
        <p>Article Content Hub</p>
        <h1>Write and publish without the workflow overhead.</h1>
        <div class="login-context__features">
          <div><span><Layers3 :size="17" /></span><p><strong>Project isolation</strong><small>Content and access stay scoped to the selected project.</small></p></div>
          <div><span><UploadCloud :size="17" /></span><p><strong>Simple publishing</strong><small>Save a draft, publish now, or choose a future date.</small></p></div>
          <div><span><ShieldCheck :size="17" /></span><p><strong>Secure delivery</strong><small>Server-side sessions and project credentials protect every request.</small></p></div>
        </div>
      </aside>

      <main class="login-panel">
        <form class="login-form surface" @submit.prevent="signIn">
          <div class="login-form__heading">
            <span>Welcome back</span>
            <h2>Sign in to your workspace</h2>
            <p>Use your invite-only administrator account.</p>
          </div>

          <label class="field login-field">
            <span>Email address</span>
            <span class="login-input">
              <Mail :size="16" />
              <input v-model.trim="email" name="email" type="email" autocomplete="email" required placeholder="you@example.com">
            </span>
          </label>

          <label class="field login-field">
            <span class="password-label">Password <NuxtLink to="/forgot-password">Forgot password?</NuxtLink></span>
            <span class="login-input">
              <LockKeyhole :size="16" />
              <input
                v-model="password"
                name="password"
                :type="passwordVisible ? 'text' : 'password'"
                autocomplete="current-password"
                required
                minlength="8"
                placeholder="Enter your password"
              >
              <button type="button" :title="passwordVisible ? 'Hide password' : 'Show password'" :aria-label="passwordVisible ? 'Hide password' : 'Show password'" @click="passwordVisible = !passwordVisible">
                <EyeOff v-if="passwordVisible" :size="16" />
                <Eye v-else :size="16" />
              </button>
            </span>
          </label>

          <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
          <p v-if="successMessage" class="ui-alert ui-alert--success">{{ successMessage }}</p>

          <button class="button button--primary login-submit" type="submit" :disabled="pending">
            <LoaderCircle v-if="pending" class="spin" :size="17" />
            <LogIn v-else :size="17" />
            {{ pending ? 'Signing in' : 'Sign in' }}
          </button>

          <div class="login-security"><ShieldCheck :size="14" /><span>Invite-only access · Secure session</span></div>
        </form>
      </main>
    </div>
  </section>
</template>

<script setup lang="ts">
import {
  BookOpenText,
  Eye,
  EyeOff,
  Laptop,
  Layers3,
  LoaderCircle,
  LockKeyhole,
  LogIn,
  Mail,
  Moon,
  PenLine,
  ShieldCheck,
  Sun,
  UploadCloud
} from 'lucide-vue-next'

const api = useAdminApi()
const colorMode = useColorMode()
const email = ref('')
const password = ref('')
const passwordVisible = ref(false)
const pending = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const themeOptions = [
  { value: 'system', label: 'System', icon: Laptop },
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon }
]

async function signIn() {
  pending.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const response = await api.login(email.value, password.value)
    useState('admin-user').value = response.data.user
    successMessage.value = 'Signed in. Opening your workspace.'
    await navigateTo('/dashboard', { replace: true })
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Sign-in failed. Check your details and try again.')
  } finally {
    pending.value = false
  }
}
</script>

<style scoped>
.login-page { min-height: 100vh; background: var(--bg); color: var(--text); }
.login-page__top { position: fixed; z-index: 10; inset: 0 0 auto; display: flex; height: 72px; align-items: center; justify-content: space-between; padding: 0 32px; }
.login-brand { display: inline-flex; align-items: center; gap: 10px; color: var(--text); text-decoration: none; }
.login-brand > span { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 7px; background: var(--primary); color: white; }
.login-brand strong { font-size: 14px; }
.login-layout { display: grid; min-height: 100vh; grid-template-columns: minmax(0, 1fr) minmax(430px, .78fr); }
.login-context { display: flex; max-width: 760px; justify-self: center; flex-direction: column; justify-content: center; padding: 110px 64px 64px; }
.login-context__mark { display: grid; width: 54px; height: 54px; place-items: center; border: 1px solid color-mix(in srgb, var(--primary) 25%, var(--border)); border-radius: 8px; background: var(--primary-soft); color: var(--primary); }
.login-context > p { margin: 27px 0 0; color: var(--primary); font-size: 13px; font-weight: 700; text-transform: uppercase; }
.login-context h1 { max-width: 650px; margin: 8px 0 0; font-size: clamp(34px, 4vw, 54px); font-weight: 710; line-height: 1.09; letter-spacing: 0; }
.login-context__features { display: grid; max-width: 600px; gap: 15px; margin-top: 40px; }
.login-context__features > div { display: grid; grid-template-columns: 38px minmax(0, 1fr); gap: 12px; align-items: center; }
.login-context__features > div > span { display: grid; width: 38px; height: 38px; place-items: center; border: 1px solid var(--border); border-radius: 7px; background: var(--surface); color: var(--text-soft); box-shadow: var(--shadow-sm); }
.login-context__features p { display: flex; margin: 0; flex-direction: column; }
.login-context__features strong { font-size: 13px; }
.login-context__features small { margin-top: 3px; color: var(--text-soft); font-size: 12px; }
.login-panel { display: grid; place-items: center; padding: 96px 42px 42px; border-left: 1px solid var(--border); background: var(--surface-subtle); }
.login-form { width: 100%; max-width: 420px; padding: 30px; box-shadow: var(--shadow-md); }
.login-form__heading { margin-bottom: 25px; }
.login-form__heading span { color: var(--primary); font-size: 12px; font-weight: 700; text-transform: uppercase; }
.login-form__heading h2 { margin: 5px 0 0; font-size: 22px; }
.login-form__heading p { margin: 6px 0 0; color: var(--text-soft); font-size: 12px; }
.login-field { margin-top: 15px; }
.password-label { display: flex; align-items: center; justify-content: space-between; }
.password-label a { color: var(--primary); font-size: 12px; font-weight: 600; text-decoration: none; }
.login-input { position: relative; display: flex; align-items: center; color: var(--text-soft); }
.login-input > svg:first-child { position: absolute; z-index: 1; left: 12px; pointer-events: none; }
.login-input input { min-height: 44px; padding-left: 39px; padding-right: 40px; }
.login-input button { position: absolute; right: 6px; display: grid; width: 32px; height: 32px; place-items: center; border: 0; border-radius: 5px; background: transparent; color: var(--text-soft); cursor: pointer; }
.login-input button:hover { background: var(--surface-subtle); }
.login-submit { width: 100%; min-height: 44px; margin-top: 19px; }
.login-security { display: flex; align-items: center; justify-content: center; gap: 6px; margin-top: 18px; color: var(--text-faint); font-size: 12px; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 950px) { .login-layout { grid-template-columns: 1fr; } .login-context { display: none; } .login-panel { min-height: 100vh; border-left: 0; } }
@media (max-width: 560px) { .login-page__top { height: 64px; padding-inline: 16px; } .login-panel { padding: 88px 16px 24px; } .login-form { padding: 23px 18px; } }
</style>

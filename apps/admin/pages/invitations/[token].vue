<template>
  <AuthPanel
    :eyebrow="acceptance ? 'Invitation accepted' : 'Project invitation'"
    :title="acceptance ? 'Your account is ready' : 'Activate your account'"
    :description="acceptance ? 'Sign in to open your new project workspace.' : 'Choose a secure password to accept this invitation.'"
  >
    <form v-if="!acceptance" class="invite-form" @submit.prevent="acceptInvitation">
      <label class="field">
        <span>Account password</span>
        <span class="invite-input">
          <LockKeyhole :size="16" />
          <input v-model="password" :type="passwordVisible ? 'text' : 'password'" autocomplete="new-password" required minlength="15" maxlength="128">
          <button type="button" :title="passwordVisible ? 'Hide password' : 'Show password'" :aria-label="passwordVisible ? 'Hide password' : 'Show password'" @click="passwordVisible = !passwordVisible">
            <EyeOff v-if="passwordVisible" :size="16" /><Eye v-else :size="16" />
          </button>
        </span>
      </label>
      <label class="field">
        <span>Confirm password</span>
        <span class="invite-input">
          <LockKeyhole :size="16" />
          <input v-model="confirmation" :type="passwordVisible ? 'text' : 'password'" autocomplete="new-password" required minlength="15" maxlength="128">
        </span>
      </label>
      <div class="password-rule" :class="{ 'is-ready': password.length >= 15 && password === confirmation }">
        <CircleCheck :size="14" />
        <span>At least 15 characters and both entries match</span>
      </div>
      <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
      <button class="button button--primary invite-submit" type="submit" :disabled="pending || !canSubmit">
        <LoaderCircle v-if="pending" class="spin" :size="16" />
        <UserCheck v-else :size="16" />
        Activate account
      </button>
    </form>

    <div v-else class="invitation-complete">
      <span class="invitation-complete__icon"><CircleCheck :size="22" /></span>
      <dl>
        <div><dt>Email</dt><dd>{{ acceptance.email }}</dd></div>
        <div><dt>Role</dt><dd>{{ labelize(acceptance.role) }}</dd></div>
      </dl>
      <NuxtLink class="button button--primary" to="/"><LogIn :size="16" />Sign in</NuxtLink>
    </div>
  </AuthPanel>
</template>

<script setup lang="ts">
import { CircleCheck, Eye, EyeOff, LoaderCircle, LockKeyhole, LogIn, UserCheck } from 'lucide-vue-next'
import type { APIEnvelope } from '~/composables/useAdminApi'

type InvitationAcceptance = {
  projectId: string
  userId: string
  email: string
  role: string
}

const route = useRoute()
const api = useAdminApi()
const token = computed(() => String(route.params.token || ''))
const password = ref('')
const confirmation = ref('')
const passwordVisible = ref(false)
const pending = ref(false)
const errorMessage = ref('')
const acceptance = ref<InvitationAcceptance | null>(null)
const canSubmit = computed(() => token.value.length > 0 && password.value.length >= 15 && password.value === confirmation.value)

async function acceptInvitation() {
  if (!canSubmit.value) {
    errorMessage.value = password.value !== confirmation.value ? 'Passwords do not match.' : 'Password must be at least 15 characters.'
    return
  }
  pending.value = true
  errorMessage.value = ''
  try {
    const response = await api.request<APIEnvelope<InvitationAcceptance>>(`/api/v1/invitations/${encodeURIComponent(token.value)}/accept`, {
      method: 'POST',
      body: { password: password.value }
    })
    acceptance.value = response.data
    password.value = ''
    confirmation.value = ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not accept this invitation.')
  } finally {
    pending.value = false
  }
}
</script>

<style scoped>
.invite-form { display: grid; gap: 15px; }
.invite-input { position: relative; display: flex; align-items: center; color: var(--text-soft); }
.invite-input > svg:first-child { position: absolute; z-index: 1; left: 12px; }
.invite-input input { min-height: 44px; padding-left: 39px; padding-right: 40px; }
.invite-input button { position: absolute; right: 6px; display: grid; width: 32px; height: 32px; place-items: center; border: 0; border-radius: 5px; background: transparent; color: var(--text-soft); cursor: pointer; }
.password-rule { display: flex; align-items: center; gap: 6px; color: var(--text-faint); font-size: 12px; }
.password-rule.is-ready { color: var(--primary); }
.invite-submit { width: 100%; min-height: 44px; }
.invitation-complete { display: grid; justify-items: center; }
.invitation-complete__icon { display: grid; width: 46px; height: 46px; place-items: center; border-radius: 8px; background: var(--primary-soft); color: var(--primary); }
.invitation-complete dl { display: grid; width: 100%; grid-template-columns: 1fr 1fr; gap: 1px; margin: 20px 0; overflow: hidden; border: 1px solid var(--border); border-radius: 7px; background: var(--border); }
.invitation-complete dl > div { min-width: 0; padding: 11px; background: var(--surface); }
.invitation-complete dt { color: var(--text-soft); font-size: 12px; }
.invitation-complete dd { overflow: hidden; margin: 3px 0 0; font-size: 12px; font-weight: 600; text-overflow: ellipsis; text-transform: capitalize; white-space: nowrap; }
.invitation-complete .button { width: 100%; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>

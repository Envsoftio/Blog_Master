<template>
  <div class="page-stack">
    <div class="page-heading">
      <p>Create and manage server-side credentials for production sites, previews, and build pipelines.</p>
      <button class="button button--compact" type="button" :disabled="pending" @click="refresh">
        <RefreshCw :class="{ spin: pending }" :size="16" />Refresh
      </button>
    </div>

    <div class="metric-grid">
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Total keys</span><KeyRound :size="17" /></div>
        <p class="metric-card__value">{{ apiKeys.length }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Active</span><ShieldCheck :size="17" /></div>
        <p class="metric-card__value metric-card__value--success">{{ activeKeyCount }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Used in 30 days</span><Activity :size="17" /></div>
        <p class="metric-card__value metric-card__value--blue">{{ recentlyUsedCount }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Revoked</span><Ban :size="17" /></div>
        <p class="metric-card__value metric-card__value--danger">{{ revokedKeyCount }}</p>
      </article>
    </div>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success" role="status">{{ successMessage }}</p>

    <section v-if="oneTimeSecret" class="key-secret surface" aria-live="polite">
      <span class="key-secret__icon"><KeyRound :size="18" /></span>
      <div class="key-secret__content">
        <span>One-time secret · {{ oneTimeSecret.name }}</span>
        <p>{{ oneTimeSecret.note }}</p>
        <div class="key-secret__value">
          <code>{{ secretVisible ? oneTimeSecret.secret : maskedSecret }}</code>
          <button class="icon-button" type="button" :title="secretVisible ? 'Hide secret' : 'Reveal secret'" :aria-label="secretVisible ? 'Hide secret' : 'Reveal secret'" @click="secretVisible = !secretVisible">
            <EyeOff v-if="secretVisible" :size="16" /><Eye v-else :size="16" />
          </button>
        </div>
        <small><ShieldCheck :size="13" />Server-side use only. Never expose this value in browser code.</small>
      </div>
      <div class="key-secret__actions">
        <button class="button button--compact" type="button" @click="copySecret">
          <Check v-if="secretCopied" :size="15" /><Copy v-else :size="15" />{{ secretCopied ? 'Copied' : 'Copy key' }}
        </button>
        <button class="button button--primary button--compact" type="button" @click="dismissSecret">I’ve saved it</button>
      </div>
    </section>

    <div class="keys-layout">
      <section class="keys-directory">
        <div class="key-toolbar surface surface--subtle">
          <label class="key-search">
            <Search :size="16" />
            <input v-model.trim="filters.search" type="search" placeholder="Search keys by name or prefix" aria-label="Search API keys">
          </label>
          <label class="key-filter">
            <span>Environment</span>
            <select v-model="filters.environment" aria-label="Filter by environment">
              <option value="all">All environments</option>
              <option v-for="environment in environments" :key="environment.value" :value="environment.value">{{ environment.label }}</option>
            </select>
          </label>
          <label class="key-filter">
            <span>Status</span>
            <select v-model="filters.status" aria-label="Filter by status">
              <option value="all">All statuses</option>
              <option value="active">Active</option>
              <option value="expired">Expired</option>
              <option value="revoked">Revoked</option>
            </select>
          </label>
          <span class="key-toolbar__count">{{ filteredAPIKeys.length }} shown</span>
        </div>

        <div v-if="pending" class="loading-surface surface"><LoaderCircle class="spin" :size="18" />Loading API keys</div>
        <div v-else-if="filteredAPIKeys.length === 0" class="empty-state">
          <div>
            <span class="empty-state__icon"><KeyRound :size="20" /></span>
            <h3>{{ apiKeys.length ? 'No keys match these filters' : 'No API keys yet' }}</h3>
            <p>{{ apiKeys.length ? 'Try another search, environment, or status.' : 'Create a server credential for your website, SSR process, or build pipeline.' }}</p>
          </div>
        </div>

        <div v-else class="key-list surface">
          <article v-for="apiKey in filteredAPIKeys" :key="apiKey.id" class="key-row">
            <span class="key-row__icon" :class="{ 'key-row__icon--inactive': keyStatus(apiKey) !== 'active' }"><KeyRound :size="18" /></span>
            <div class="key-row__main">
              <div class="key-row__heading">
                <div><h3>{{ apiKey.name }}</h3><code>{{ apiKey.tokenPrefix }}…</code></div>
                <div class="key-row__pills">
                  <span class="environment-pill" :class="environmentClass(apiKey.environment)">{{ apiKey.environment }}</span>
                  <span class="status-pill" :class="keyStatusClass(apiKey)">{{ keyStatus(apiKey) }}</span>
                </div>
              </div>
              <dl class="key-row__meta">
                <div><CalendarClock :size="14" /><span><dt>Expires</dt><dd>{{ apiKey.expiresAt ? formatDate(apiKey.expiresAt) : 'No expiry' }}</dd></span></div>
                <div><Activity :size="14" /><span><dt>Last used</dt><dd>{{ apiKey.lastUsedAt ? formatDate(apiKey.lastUsedAt) : 'Never' }}</dd></span></div>
                <div><KeyRound :size="14" /><span><dt>Created</dt><dd>{{ formatDate(apiKey.createdAt) }}</dd></span></div>
              </dl>
              <div class="key-scopes">
                <span v-for="scope in apiKey.scopes" :key="scope">{{ scopeLabel(scope) }}</span>
              </div>
              <div class="key-row__actions">
                <button class="button button--compact" type="button" :disabled="Boolean(actionPending[apiKey.id]) || keyStatus(apiKey) !== 'active'" @click="openKeyConfirmation('rotate', apiKey)">
                  <RefreshCw :class="{ spin: actionPending[apiKey.id] === 'rotate' }" :size="15" />Rotate
                </button>
                <button class="button button--compact key-action--danger" type="button" :disabled="Boolean(actionPending[apiKey.id]) || Boolean(apiKey.revokedAt)" @click="openKeyConfirmation('revoke', apiKey)">
                  <LoaderCircle v-if="actionPending[apiKey.id] === 'revoke'" class="spin" :size="15" /><Ban v-else :size="15" />Revoke
                </button>
              </div>
            </div>
          </article>
        </div>
      </section>

      <aside class="key-sidebar">
        <form class="key-panel surface" @submit.prevent="createKey">
          <div class="key-panel__header">
            <span class="key-panel__icon"><Plus :size="18" /></span>
            <div><span>Project credential</span><h2>Create API key</h2></div>
          </div>
          <div class="key-panel__body">
            <label class="field">
              <span>Key name</span>
              <input v-model.trim="form.name" maxlength="100" placeholder="Production website" required>
              <small>Use the workload or deployment name.</small>
            </label>
            <label class="field">
              <span>Environment</span>
              <select v-model="form.environment">
                <option v-for="environment in environments" :key="environment.value" :value="environment.value">{{ environment.label }}</option>
              </select>
            </label>
            <fieldset class="key-fieldset">
              <legend>Expiration</legend>
              <div class="expiry-options">
                <label :class="{ 'is-selected': form.neverExpires }"><input v-model="form.neverExpires" type="radio" :value="true"><span><strong>No expiry</strong><small>Active until revoked</small></span></label>
                <label :class="{ 'is-selected': !form.neverExpires }"><input v-model="form.neverExpires" type="radio" :value="false"><span><strong>Set a date</strong><small>Expire automatically</small></span></label>
              </div>
              <label v-if="!form.neverExpires" class="field expiry-date">
                <span>Expires at</span>
                <input v-model="form.expiresAt" type="datetime-local" :min="minimumExpiry" required>
              </label>
            </fieldset>
            <fieldset class="key-fieldset">
              <legend>Permissions</legend>
              <p>Grant only what this workload needs.</p>
              <div class="scope-options">
                <label v-for="scope in availableScopes" :key="scope.value" :class="{ 'is-selected': form.scopes.includes(scope.value) }">
                  <input v-model="form.scopes" type="checkbox" :value="scope.value"><span>{{ scope.label }}</span>
                </label>
              </div>
            </fieldset>
          </div>
          <div class="key-panel__footer">
            <span><ShieldCheck :size="14" />Shown once</span>
            <button class="button button--primary" type="submit" :disabled="creating || !canCreate">
              <LoaderCircle v-if="creating" class="spin" :size="16" /><Plus v-else :size="16" />Create key
            </button>
          </div>
        </form>
        <section class="key-guide surface">
          <span class="key-guide__icon"><ShieldCheck :size="17" /></span>
          <div><h3>Keep keys private</h3><p>Store credentials in server environment variables or a secrets manager. Rotate them during team or infrastructure changes.</p></div>
        </section>
      </aside>
    </div>

    <div v-if="keyConfirmation" class="key-dialog-backdrop" @click.self="keyConfirmation = null">
      <div class="key-dialog surface" role="dialog" aria-modal="true" aria-labelledby="key-confirmation-title">
        <div class="key-dialog__header" :class="{ 'key-dialog__header--danger': keyConfirmation.action === 'revoke' }">
          <span><TriangleAlert :size="18" /></span>
          <div><small>Protected action</small><h2 id="key-confirmation-title">{{ keyConfirmation.action === 'revoke' ? 'Revoke API key?' : 'Rotate API key?' }}</h2></div>
        </div>
        <div class="key-dialog__body">
          <p v-if="keyConfirmation.action === 'revoke'"><strong>{{ keyConfirmation.key.name }}</strong> will stop working immediately. This cannot be undone.</p>
          <p v-else>A replacement for <strong>{{ keyConfirmation.key.name }}</strong> will be created. The current key remains active until you revoke it.</p>
          <code>{{ keyConfirmation.key.tokenPrefix }}…</code>
        </div>
        <div class="key-dialog__actions">
          <button class="button" type="button" @click="keyConfirmation = null">Cancel</button>
          <button class="button" :class="keyConfirmation.action === 'revoke' ? 'key-button--danger' : 'button--primary'" type="button" :disabled="Boolean(actionPending[keyConfirmation.key.id])" @click="confirmKeyAction">
            {{ keyConfirmation.action === 'revoke' ? 'Revoke key' : 'Create replacement' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="reauthenticationOpen" class="key-dialog-backdrop" @click.self="cancelReauthentication">
      <form class="key-dialog surface" role="dialog" aria-modal="true" aria-labelledby="reauthentication-title" @submit.prevent="confirmReauthentication">
        <div class="key-dialog__header">
          <span><LockKeyhole :size="18" /></span>
          <div><small>Protected action</small><h2 id="reauthentication-title">Confirm your identity</h2></div>
        </div>
        <div class="key-dialog__body">
          <p>Enter your current password to {{ pendingProtectedAction?.label || 'continue' }}.</p>
          <p v-if="reauthenticationError" class="ui-alert ui-alert--danger" role="alert">{{ reauthenticationError }}</p>
          <label class="field"><span>Current password</span><input v-model="reauthenticationPassword" type="password" autocomplete="current-password" required autofocus></label>
        </div>
        <div class="key-dialog__actions">
          <button class="button" type="button" :disabled="reauthenticating" @click="cancelReauthentication">Cancel</button>
          <button class="button button--primary" type="submit" :disabled="reauthenticating || !reauthenticationPassword"><LoaderCircle v-if="reauthenticating" class="spin" :size="16" />Confirm</button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Activity, Ban, CalendarClock, Check, Copy, Eye, EyeOff, KeyRound, LoaderCircle, LockKeyhole, Plus, RefreshCw, Search, ShieldCheck, TriangleAlert } from 'lucide-vue-next'
import type { AdminAPIKey } from '~/composables/useAdminApi'

type PendingProtectedAction = {
  label: string
  run: () => Promise<void>
}

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => {
  const value = route.params.projectId
  return Array.isArray(value) ? String(value[0] || '') : String(value || '')
})

const availableScopes = [
  { value: 'content:published:read', label: 'Published posts' },
  { value: 'taxonomy:published:read', label: 'Taxonomy' },
  { value: 'authors:published:read', label: 'Authors' },
  { value: 'discovery:read', label: 'Discovery' },
  { value: 'redirects:read', label: 'Redirects' }
]

const environments = [
  { value: 'production', label: 'Production' },
  { value: 'staging', label: 'Staging' },
  { value: 'development', label: 'Development' },
  { value: 'preview', label: 'Preview' }
]

const apiKeys = ref<AdminAPIKey[]>([])
const pending = ref(true)
const creating = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const oneTimeSecret = ref<{ name: string, secret: string, note?: string } | null>(null)
const secretVisible = ref(true)
const secretCopied = ref(false)
const actionPending = reactive<Record<string, string>>({})
const keyConfirmation = ref<{ action: 'rotate' | 'revoke', key: AdminAPIKey } | null>(null)
const reauthenticationOpen = ref(false)
const reauthenticationPassword = ref('')
const reauthenticationError = ref('')
const reauthenticating = ref(false)
const pendingProtectedAction = ref<PendingProtectedAction | null>(null)

const form = reactive({
  name: '',
  environment: 'production',
  neverExpires: true,
  expiresAt: '',
  scopes: availableScopes.map((scope) => scope.value)
})
const filters = reactive({ search: '', environment: 'all', status: 'all' })

const canCreate = computed(() => Boolean(
  form.name.trim() &&
  form.scopes.length > 0 &&
  (form.neverExpires || form.expiresAt)
))
const filteredAPIKeys = computed(() => {
  const query = filters.search.toLowerCase()
  return apiKeys.value.filter((key) => {
    if (filters.environment !== 'all' && key.environment !== filters.environment) return false
    if (filters.status !== 'all' && keyStatus(key) !== filters.status) return false
    return !query || key.name.toLowerCase().includes(query) || key.tokenPrefix.toLowerCase().includes(query)
  })
})
const activeKeyCount = computed(() => apiKeys.value.filter(key => keyStatus(key) === 'active').length)
const recentlyUsedCount = computed(() => apiKeys.value.filter(key => wasUsedRecently(key.lastUsedAt)).length)
const revokedKeyCount = computed(() => apiKeys.value.filter(key => keyStatus(key) === 'revoked').length)
const maskedSecret = computed(() => {
  const secret = oneTimeSecret.value?.secret || ''
  return `${secret.slice(0, 12)}${'•'.repeat(Math.max(12, secret.length - 12))}`
})
const minimumExpiry = ref('')

onMounted(() => {
  minimumExpiry.value = formatLocalDateTimeInput(new Date(Date.now() + 60_000))
  void refresh()
})

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    apiKeys.value = await loadAllAPIKeys()
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load API keys. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function loadAllAPIKeys() {
  const keys: AdminAPIKey[] = []
  const seenCursors = new Set<string>()
  let cursor = ''
  do {
    const response = await api.listAPIKeys(projectID.value, cursor, 100)
    keys.push(...apiListData(response))
    const nextCursor = response.meta?.nextCursor || ''
    if (!nextCursor || seenCursors.has(nextCursor)) break
    seenCursors.add(nextCursor)
    cursor = nextCursor
  } while (cursor)
  return keys.sort((left, right) => parseBackendUTC(right.createdAt).getTime() - parseBackendUTC(left.createdAt).getTime())
}

async function createKey() {
  creating.value = true
  clearMessages()
  try {
    const response = await api.createAPIKey(projectID.value, {
      name: form.name,
      environment: form.environment,
      expiresAt: !form.neverExpires && form.expiresAt ? new Date(form.expiresAt).toISOString() : undefined,
      scopes: form.scopes
    })
    upsertKey(response.data.key)
    oneTimeSecret.value = {
      name: response.data.key.name,
      secret: response.data.secret,
      note: 'Store this secret now. It cannot be shown again.'
    }
    secretVisible.value = true
    secretCopied.value = false
    form.name = ''
    form.environment = 'production'
    form.neverExpires = true
    form.expiresAt = ''
    form.scopes = availableScopes.map((scope) => scope.value)
    successMessage.value = 'API key created.'
  } catch (error) {
    if (queueReauthentication(error, 'create this API key', createKey)) return
    errorMessage.value = normalizeAPIError(error, 'Could not create API key.')
  } finally {
    creating.value = false
  }
}

async function rotateKey(apiKey: AdminAPIKey) {
  actionPending[apiKey.id] = 'rotate'
  clearMessages()
  try {
    const response = await api.rotateAPIKey(projectID.value, apiKey.id)
    upsertKey(response.data.key)
    oneTimeSecret.value = {
      name: response.data.key.name,
      secret: response.data.secret,
      note: `The previous key (${apiKey.tokenPrefix}) remains active. Revoke it after the replacement is deployed.`
    }
    secretVisible.value = true
    secretCopied.value = false
    successMessage.value = 'Replacement key created.'
  } catch (error) {
    if (queueReauthentication(error, `rotate ${apiKey.name}`, () => rotateKey(apiKey))) return
    errorMessage.value = normalizeAPIError(error, 'Could not rotate API key.')
  } finally {
    delete actionPending[apiKey.id]
  }
}

function openKeyConfirmation(action: 'rotate' | 'revoke', key: AdminAPIKey) {
  keyConfirmation.value = { action, key }
}

async function confirmKeyAction() {
  const confirmation = keyConfirmation.value
  if (!confirmation) return
  keyConfirmation.value = null
  if (confirmation.action === 'rotate') {
    await rotateKey(confirmation.key)
  } else {
    await performRevokeKey(confirmation.key)
  }
}

async function performRevokeKey(apiKey: AdminAPIKey) {
  actionPending[apiKey.id] = 'revoke'
  clearMessages()
  try {
    const response = await api.revokeAPIKey(projectID.value, apiKey.id)
    upsertKey(response.data)
    successMessage.value = 'API key revoked.'
  } catch (error) {
    if (queueReauthentication(error, `revoke ${apiKey.name}`, () => performRevokeKey(apiKey))) return
    errorMessage.value = normalizeAPIError(error, 'Could not revoke API key.')
  } finally {
    delete actionPending[apiKey.id]
  }
}

function queueReauthentication(error: unknown, label: string, run: () => Promise<void>) {
  const problem = apiProblem(error)
  if (problem?.title !== 'Recent reauthentication required') return false
  pendingProtectedAction.value = { label, run }
  reauthenticationPassword.value = ''
  reauthenticationError.value = ''
  reauthenticationOpen.value = true
  return true
}

async function confirmReauthentication() {
  if (!pendingProtectedAction.value || !reauthenticationPassword.value) return
  reauthenticating.value = true
  reauthenticationError.value = ''
  try {
    await api.reauthenticate(reauthenticationPassword.value)
    const action = pendingProtectedAction.value
    reauthenticationOpen.value = false
    reauthenticationPassword.value = ''
    pendingProtectedAction.value = null
    await action.run()
  } catch (error) {
    reauthenticationError.value = normalizeAPIError(error, 'Could not confirm your identity.')
  } finally {
    reauthenticating.value = false
  }
}

function cancelReauthentication() {
  if (reauthenticating.value) return
  reauthenticationOpen.value = false
  reauthenticationPassword.value = ''
  reauthenticationError.value = ''
  pendingProtectedAction.value = null
}

async function copySecret() {
  if (!oneTimeSecret.value) return
  try {
    await navigator.clipboard.writeText(oneTimeSecret.value.secret)
    secretCopied.value = true
    successMessage.value = 'Secret copied to the clipboard.'
  } catch {
    secretVisible.value = true
    errorMessage.value = 'Clipboard access was blocked. Select and copy the visible key manually.'
  }
}

function dismissSecret() {
  oneTimeSecret.value = null
  secretCopied.value = false
  secretVisible.value = true
}

function upsertKey(apiKey: AdminAPIKey) {
  const existing = apiKeys.value.findIndex((item) => item.id === apiKey.id)
  if (existing >= 0) {
    apiKeys.value.splice(existing, 1, apiKey)
  } else {
    apiKeys.value = [apiKey, ...apiKeys.value]
  }
}

function parseBackendUTC(value: string) {
  return new Date(value.includes('T') ? value : `${value.replace(' ', 'T')}Z`)
}

function wasUsedRecently(value?: string) {
  if (!value) return false
  const usedAt = parseBackendUTC(value).getTime()
  return Number.isFinite(usedAt) && usedAt >= Date.now() - 30 * 24 * 60 * 60 * 1000
}

function keyStatus(apiKey: AdminAPIKey) {
  return apiKey.status || (apiKey.revokedAt ? 'revoked' : apiKey.expiresAt && parseBackendUTC(apiKey.expiresAt).getTime() <= Date.now() ? 'expired' : 'active')
}

function keyStatusClass(apiKey: AdminAPIKey) {
  switch (keyStatus(apiKey)) {
    case 'revoked':
      return 'status-pill--warning'
    case 'expired':
      return 'status-pill--danger'
    default:
      return 'status-pill--success'
  }
}

function formatDate(value?: string) {
  if (!value) return 'Not set'
  const date = parseBackendUTC(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(date)
}

function formatLocalDateTimeInput(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0')
  return [
    date.getFullYear(),
    '-',
    pad(date.getMonth() + 1),
    '-',
    pad(date.getDate()),
    'T',
    pad(date.getHours()),
    ':',
    pad(date.getMinutes())
  ].join('')
}

function environmentClass(environment: string) {
  return `environment-pill--${environment}`
}

function scopeLabel(scope: string) {
  return availableScopes.find(option => option.value === scope)?.label || scope
}

function clearMessages() {
  errorMessage.value = ''
  successMessage.value = ''
}

function normalizeAPIError(error: unknown, fallback: string) {
  const problem = apiProblem(error)
  if (problem) return problem.detail || problem.title || fallback
  if (error instanceof Error && error.message) return error.message
  return fallback
}

function apiProblem(error: unknown) {
  if (typeof error !== 'object' || error === null || !('data' in error)) return null
  return (error as { data?: { title?: string, detail?: string, status?: number } }).data || null
}
</script>

<style scoped>
.metric-card__value--success { color: var(--primary); }
.metric-card__value--blue { color: var(--blue); }
.metric-card__value--danger { color: var(--danger); }
.key-secret { display: grid; grid-template-columns: 42px minmax(0, 1fr) auto; align-items: start; gap: 13px; padding: 15px; border-color: color-mix(in srgb, var(--amber) 38%, var(--border)); background: color-mix(in srgb, var(--amber-soft) 55%, var(--surface)); }
.key-secret__icon { display: grid; width: 42px; height: 42px; place-items: center; border-radius: 7px; background: var(--surface); color: var(--amber); box-shadow: var(--shadow-sm); }
.key-secret__content { min-width: 0; }
.key-secret__content > span { color: var(--amber); font-size: 12px; font-weight: 700; text-transform: uppercase; }
.key-secret__content > p { margin: 3px 0 0; color: var(--text-soft); font-size: 13px; }
.key-secret__content > small { display: inline-flex; align-items: center; gap: 5px; margin-top: 8px; color: var(--text-faint); font-size: 12px; }
.key-secret__value { display: grid; grid-template-columns: minmax(0, 1fr) 36px; margin-top: 10px; overflow: hidden; border-radius: 6px; background: var(--sidebar); }
.key-secret__value code { min-width: 0; overflow-x: auto; padding: 10px 12px; color: #e8fff8; font-size: 12px; white-space: nowrap; }
.key-secret__value .icon-button { width: 36px; height: 100%; border: 0; border-left: 1px solid rgb(255 255 255 / 0.14); border-radius: 0; color: #b8c5d8; }
.key-secret__value .icon-button:hover { background: rgb(255 255 255 / 0.08); color: white; }
.key-secret__actions { display: flex; flex-direction: column; gap: 7px; }
.keys-layout { display: grid; grid-template-columns: minmax(0, 1fr) 350px; align-items: start; gap: 16px; }
.keys-directory { display: grid; min-width: 0; gap: 14px; }
.key-toolbar { display: grid; grid-template-columns: minmax(220px, 1fr) 155px 140px auto; align-items: end; gap: 8px; padding: 8px; }
.key-search { display: flex; min-height: 36px; align-items: center; gap: 8px; padding: 0 10px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--text-soft); }
.key-search input { width: 100%; min-width: 0; min-height: 34px; padding: 0; border: 0; outline: 0; background: transparent; color: var(--text); font-size: 13px; }
.key-search:focus-within { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent); }
.key-filter { display: grid; gap: 3px; }
.key-filter > span { padding-left: 2px; color: var(--text-faint); font-size: 12px; font-weight: 650; }
.key-filter select { min-height: 36px; padding: 7px 28px 7px 10px; border: 1px solid var(--border); border-radius: 6px; outline: 0; background: var(--surface); color: var(--text); font-size: 13px; }
.key-filter select:focus { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent); }
.key-toolbar__count { align-self: center; padding: 12px 4px 0; color: var(--text-faint); font-size: 12px; white-space: nowrap; }
.loading-surface { display: flex; min-height: 150px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.key-list { overflow: hidden; }
.key-row { display: grid; grid-template-columns: 42px minmax(0, 1fr); gap: 13px; padding: 16px; border-bottom: 1px solid var(--border); }
.key-row:last-child { border-bottom: 0; }
.key-row:hover { background: var(--surface-subtle); }
.key-row__icon { display: grid; width: 42px; height: 42px; place-items: center; border-radius: 7px; background: var(--blue-soft); color: var(--blue); }
.key-row__icon--inactive { background: var(--surface-subtle); color: var(--text-faint); }
.key-row__main { min-width: 0; }
.key-row__heading { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 12px; }
.key-row__heading > div:first-child { min-width: 0; }
.key-row__heading h3 { overflow: hidden; margin: 0; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.key-row__heading code { display: block; overflow: hidden; margin-top: 4px; color: var(--text-soft); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.key-row__pills { display: flex; flex: 0 0 auto; flex-wrap: wrap; justify-content: flex-end; gap: 5px; }
.key-row__pills .status-pill { font-size: 12px; }
.environment-pill { display: inline-flex; min-height: 24px; align-items: center; padding: 3px 8px; border: 1px solid var(--border); border-radius: 999px; background: var(--surface-subtle); color: var(--text-soft); font-size: 12px; font-weight: 650; text-transform: capitalize; }
.environment-pill--production { border-color: color-mix(in srgb, var(--primary) 30%, var(--border)); background: var(--primary-soft); color: var(--primary); }
.environment-pill--staging { border-color: color-mix(in srgb, var(--blue) 30%, var(--border)); background: var(--blue-soft); color: var(--blue); }
.environment-pill--preview { border-color: color-mix(in srgb, var(--amber) 30%, var(--border)); background: var(--amber-soft); color: var(--amber); }
.key-row__meta { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; margin: 13px 0 0; }
.key-row__meta > div { display: flex; min-width: 0; align-items: flex-start; gap: 7px; color: var(--text-faint); }
.key-row__meta svg { flex: 0 0 auto; margin-top: 2px; }
.key-row__meta span { min-width: 0; }
.key-row__meta dt { color: var(--text-faint); font-size: 11px; font-weight: 650; text-transform: uppercase; }
.key-row__meta dd { overflow: hidden; margin: 2px 0 0; color: var(--text-soft); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.key-scopes { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 12px; }
.key-scopes span { padding: 4px 7px; border: 1px solid var(--border); border-radius: 5px; background: var(--surface-subtle); color: var(--text-soft); font-size: 11px; }
.key-row__actions { display: flex; flex-wrap: wrap; gap: 7px; margin-top: 13px; padding-top: 12px; border-top: 1px solid var(--border); }
.key-row__actions .button { min-height: 34px; font-size: 12px; }
.key-action--danger { border-color: color-mix(in srgb, var(--danger) 35%, var(--border)); color: var(--danger); }
.key-action--danger:hover { border-color: var(--danger); background: var(--danger-soft); }
.key-sidebar { position: sticky; top: 96px; display: grid; gap: 14px; }
.key-panel { overflow: hidden; }
.key-panel__header { display: flex; align-items: flex-start; gap: 10px; padding: 14px; border-bottom: 1px solid var(--border); }
.key-panel__icon { display: grid; width: 36px; height: 36px; flex: 0 0 36px; place-items: center; border-radius: 7px; background: var(--primary-soft); color: var(--primary); }
.key-panel__header div > span { color: var(--text-soft); font-size: 12px; font-weight: 700; text-transform: uppercase; }
.key-panel__header h2 { margin: 1px 0 0; font-size: 15px; }
.key-panel__body { display: grid; gap: 15px; padding: 14px; }
.key-panel .field small { color: var(--text-faint); font-size: 12px; line-height: 1.4; }
.key-fieldset { display: grid; gap: 8px; margin: 0; padding: 0; border: 0; }
.key-fieldset legend { margin-bottom: 1px; color: var(--text); font-size: 13px; font-weight: 650; }
.key-fieldset > p { margin: -3px 0 2px; color: var(--text-faint); font-size: 12px; }
.expiry-options { display: grid; grid-template-columns: 1fr 1fr; gap: 7px; }
.expiry-options > label { display: flex; align-items: flex-start; gap: 7px; padding: 9px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface-subtle); cursor: pointer; }
.expiry-options > label.is-selected { border-color: color-mix(in srgb, var(--primary) 55%, var(--border)); background: var(--primary-soft); }
.expiry-options input, .scope-options input { width: 14px; height: 14px; min-height: 0; margin: 2px 0 0; accent-color: var(--primary); }
.expiry-options span { display: grid; gap: 2px; }
.expiry-options strong { font-size: 12px; }
.expiry-options small { color: var(--text-faint); font-size: 10px; line-height: 1.25; }
.expiry-date { margin-top: 2px; }
.scope-options { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
.scope-options label { display: flex; align-items: center; gap: 7px; min-width: 0; padding: 8px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface-subtle); color: var(--text-soft); font-size: 11px; cursor: pointer; }
.scope-options label.is-selected { border-color: color-mix(in srgb, var(--primary) 35%, var(--border)); background: var(--primary-soft); color: var(--primary); }
.scope-options span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.key-panel__footer { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 12px 14px; border-top: 1px solid var(--border); background: var(--surface-subtle); }
.key-panel__footer > span { display: inline-flex; align-items: center; gap: 5px; color: var(--text-faint); font-size: 11px; }
.key-guide { display: flex; gap: 10px; padding: 13px; }
.key-guide__icon { display: grid; width: 34px; height: 34px; flex: 0 0 34px; place-items: center; border-radius: 7px; background: var(--blue-soft); color: var(--blue); }
.key-guide h3 { margin: 1px 0 0; font-size: 13px; }
.key-guide p { margin: 4px 0 0; color: var(--text-soft); font-size: 11px; line-height: 1.5; }
.key-dialog-backdrop { position: fixed; inset: 0; z-index: 70; display: grid; place-items: center; padding: 20px; background: rgb(15 23 42 / 0.52); backdrop-filter: blur(2px); }
.key-dialog { width: min(440px, 100%); overflow: hidden; box-shadow: var(--shadow-md); }
.key-dialog__header { display: flex; align-items: center; gap: 10px; padding: 14px; border-bottom: 1px solid var(--border); }
.key-dialog__header > span { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 7px; background: var(--amber-soft); color: var(--amber); }
.key-dialog__header--danger > span { background: var(--danger-soft); color: var(--danger); }
.key-dialog__header small { color: var(--text-soft); font-size: 11px; font-weight: 700; text-transform: uppercase; }
.key-dialog__header h2 { margin: 1px 0 0; font-size: 15px; }
.key-dialog__body { display: grid; gap: 13px; padding: 14px; }
.key-dialog__body > p { margin: 0; color: var(--text-soft); font-size: 13px; line-height: 1.55; }
.key-dialog__body > code { padding: 9px 10px; border-radius: 6px; background: var(--surface-subtle); color: var(--text); font-size: 12px; }
.key-dialog__body > .ui-alert { color: var(--danger); }
.key-dialog__actions { display: flex; justify-content: flex-end; gap: 8px; padding: 12px 14px; border-top: 1px solid var(--border); background: var(--surface-subtle); }
.key-button--danger { border-color: var(--danger); background: var(--danger); color: white; }
.key-button--danger:hover { border-color: var(--danger); background: color-mix(in srgb, var(--danger) 86%, black); }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1100px) {
  .keys-layout { grid-template-columns: minmax(0, 1fr) 320px; }
  .key-toolbar { grid-template-columns: minmax(180px, 1fr) 145px 130px; }
  .key-toolbar__count { display: none; }
}
@media (max-width: 900px) {
  .keys-layout { grid-template-columns: 1fr; }
  .key-sidebar { position: static; grid-template-columns: minmax(0, 1fr) 260px; }
}
@media (max-width: 680px) {
  .key-toolbar { grid-template-columns: 1fr 1fr; }
  .key-search { grid-column: 1 / -1; }
  .key-secret { grid-template-columns: 38px minmax(0, 1fr); }
  .key-secret__icon { width: 38px; height: 38px; }
  .key-secret__actions { grid-column: 2; flex-direction: row; }
  .key-sidebar { grid-template-columns: 1fr; }
  .key-row__meta { grid-template-columns: 1fr 1fr; }
}
@media (max-width: 500px) {
  .page-heading, .page-heading .button { width: 100%; }
  .key-toolbar { grid-template-columns: 1fr; }
  .key-search { grid-column: auto; }
  .key-row { grid-template-columns: 36px minmax(0, 1fr); gap: 10px; padding: 13px; }
  .key-row__icon { width: 36px; height: 36px; }
  .key-row__heading { flex-direction: column; }
  .key-row__pills { justify-content: flex-start; }
  .key-row__meta { grid-template-columns: 1fr; }
  .key-row__actions .button { flex: 1; }
  .key-secret { grid-template-columns: minmax(0, 1fr); }
  .key-secret__icon { display: none; }
  .key-secret__actions { grid-column: auto; flex-direction: column; }
  .expiry-options, .scope-options { grid-template-columns: 1fr; }
  .key-panel__footer { align-items: stretch; flex-direction: column; }
  .key-dialog-backdrop { padding: 10px; }
}
</style>

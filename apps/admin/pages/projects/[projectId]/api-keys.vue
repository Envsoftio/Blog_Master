<template>
  <section class="min-h-screen">
    <header class="border-b border-[#d7ded8] bg-white px-6 py-4 dark:border-[#343a38] dark:bg-[#202422]">
      <div class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4">
        <div class="flex min-w-0 items-center gap-3">
          <NuxtLink
            class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#eef5f1] dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]"
            to="/projects"
            title="Back to projects"
            aria-label="Back to projects"
          >
            <ArrowLeft class="h-4 w-4" />
          </NuxtLink>
          <div class="min-w-0">
            <p class="truncate text-sm text-[#5d6a61] dark:text-[#aeb8b0]">{{ project?.name || 'Project' }}</p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button
            class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#eef5f1] disabled:opacity-50 dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]"
            type="button"
            title="Refresh"
            aria-label="Refresh"
            :disabled="pending"
            @click="refresh"
          >
            <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': pending }" />
          </button>
          <button
            class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#fff4df] dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]"
            type="button"
            title="Log out"
            aria-label="Log out"
            @click="logout"
          >
            <LogOut class="h-4 w-4" />
          </button>
        </div>
      </div>
    </header>

    <div class="mx-auto grid max-w-7xl grid-cols-1 gap-6 px-6 py-6 lg:grid-cols-[220px_1fr]">
      <ProjectNav :project-id="projectID" :project="project" active="api-keys" />

      <div class="space-y-5">
        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>
        <p v-if="successMessage" class="rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-4 py-3 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]">
          {{ successMessage }}
        </p>

        <div v-if="oneTimeSecret" class="rounded-lg border-2 border-[#2f8068] bg-[#f5fcf8] p-5 shadow-sm dark:border-[#4caa8c] dark:bg-[#15251f]" role="status" aria-live="polite">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="min-w-0">
              <p class="inline-flex items-center gap-2 text-sm font-medium text-[#165a4a] dark:text-[#aee4d0]"><ShieldCheck class="h-4 w-4" /> One-time secret</p>
              <h2 class="mt-1 text-lg font-semibold tracking-normal">Save {{ oneTimeSecret.name }} now</h2>
            </div>
            <div class="flex items-center gap-2">
              <button
                class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] text-[#28342d] hover:bg-[#eef5f1] dark:border-[#414a45] dark:text-[#eef4ef] dark:hover:bg-[#2a302d]"
                type="button"
                :title="secretCopied ? 'Copied' : 'Copy secret'"
                :aria-label="secretCopied ? 'Secret copied' : 'Copy secret'"
                @click="copySecret"
              >
                <Check v-if="secretCopied" class="h-4 w-4" />
                <Copy v-else class="h-4 w-4" />
              </button>
            </div>
          </div>
          <p v-if="oneTimeSecret.note" class="mt-3 text-sm text-[#36594a] dark:text-[#b6d7c8]">{{ oneTimeSecret.note }}</p>
          <div class="mt-4 flex items-stretch overflow-hidden rounded-md bg-[#17201b] text-[#dff7ea]">
            <code class="min-w-0 flex-1 overflow-x-auto px-3 py-3 text-sm">{{ secretVisible ? oneTimeSecret.secret : maskedSecret }}</code>
            <button class="border-l border-white/20 px-3 hover:bg-white/10" type="button" :aria-label="secretVisible ? 'Hide secret' : 'Reveal secret'" @click="secretVisible = !secretVisible">
              <EyeOff v-if="secretVisible" class="h-4 w-4" />
              <Eye v-else class="h-4 w-4" />
            </button>
          </div>
          <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
            <p class="text-xs text-[#526158] dark:text-[#b6c4bb]">For server-side use only. Never expose this value in browser code.</p>
            <button class="rounded-md bg-[#165a4a] px-3 py-2 text-sm font-medium text-white hover:bg-[#10463a]" type="button" @click="dismissSecret">I’ve saved this key</button>
          </div>
        </div>

        <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
          <div class="space-y-4">
            <div>
              <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Project credentials</p>
              <h2 class="mt-1 text-xl font-semibold tracking-normal">Server keys</h2>
            </div>

            <div class="grid gap-3 sm:grid-cols-3">
              <div v-for="summary in keySummary" :key="summary.label" class="rounded-lg border border-[#cfd8d1] bg-white px-4 py-3 dark:border-[#3f4843] dark:bg-[#202522]">
                <p class="text-2xl font-semibold">{{ summary.value }}</p>
                <p class="text-xs uppercase tracking-wide text-[#667169] dark:text-[#aeb8b0]">{{ summary.label }}</p>
              </div>
            </div>

            <div class="grid gap-3 rounded-lg border border-[#cfd8d1] bg-white p-3 dark:border-[#3f4843] dark:bg-[#202522] sm:grid-cols-[1fr_150px_150px]">
              <label class="relative">
                <Search class="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-[#667169]" />
                <input v-model.trim="filters.search" class="w-full rounded-md border border-[#bfcac3] py-2 pl-9 pr-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" placeholder="Search name or prefix" aria-label="Search API keys" />
              </label>
              <select v-model="filters.environment" class="rounded-md border border-[#bfcac3] px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" aria-label="Filter by environment">
                <option value="all">All environments</option>
                <option v-for="environment in environments" :key="environment.value" :value="environment.value">{{ environment.label }}</option>
              </select>
              <select v-model="filters.status" class="rounded-md border border-[#bfcac3] px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" aria-label="Filter by status">
                <option value="all">All statuses</option>
                <option value="active">Active</option>
                <option value="expired">Expired</option>
                <option value="revoked">Revoked</option>
              </select>
            </div>

            <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
              <LoaderCircle class="h-4 w-4 animate-spin" />
              Loading API keys
            </div>

            <div v-else-if="filteredAPIKeys.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]">
              <h2 class="text-lg font-semibold">{{ apiKeys.length ? 'No keys match these filters' : 'No API keys yet' }}</h2>
              <p class="mt-2 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ apiKeys.length ? 'Clear or change the filters to see more keys.' : 'Create a server-side credential for a landing build, SSR or ISR process.' }}</p>
            </div>

            <article v-for="apiKey in filteredAPIKeys" :key="apiKey.id" class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div class="min-w-0">
                  <h3 class="truncate text-lg font-semibold">{{ apiKey.name }}</h3>
                  <p class="mt-1 truncate font-mono text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ apiKey.tokenPrefix }}</p>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="environmentClass(apiKey.environment)">
                    {{ apiKey.environment }}
                  </span>
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="keyStatusClass(apiKey)">
                    {{ keyStatus(apiKey) }}
                  </span>
                </div>
              </div>

              <dl class="mt-5 grid gap-3 text-sm md:grid-cols-3">
                <div class="flex items-center gap-2">
                  <CalendarClock class="h-4 w-4 text-[#3162a3]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Expires</dt>
                    <dd class="truncate">{{ formatDate(apiKey.expiresAt) }}</dd>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <Activity class="h-4 w-4 text-[#165a4a]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Last used</dt>
                    <dd class="truncate">{{ formatDate(apiKey.lastUsedAt) }}</dd>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <KeyRound class="h-4 w-4 text-[#8a5b00]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Created</dt>
                    <dd class="truncate">{{ formatDate(apiKey.createdAt) }}</dd>
                  </div>
                </div>
              </dl>

              <div class="mt-4 flex flex-wrap gap-2">
                <span v-for="scope in apiKey.scopes" :key="scope" class="rounded-md bg-[#f2f5f3] px-2.5 py-1 font-mono text-xs text-[#4f5b54] dark:bg-[#171b18] dark:text-[#c5cec8]">
                  {{ scope }}
                </span>
              </div>

              <div class="mt-5 flex flex-wrap items-center gap-2">
                <button
                  class="inline-flex items-center gap-2 rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                  type="button"
                  :disabled="Boolean(actionPending[apiKey.id]) || keyStatus(apiKey) !== 'active'"
                  @click="openKeyConfirmation('rotate', apiKey)"
                >
                  <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': actionPending[apiKey.id] === 'rotate' }" />
                  Rotate
                </button>
                <button
                  class="inline-flex items-center gap-2 rounded-md border border-[#d9b7aa] px-3 py-2 text-sm font-medium text-[#9b2d23] hover:bg-[#fff4f2] disabled:opacity-60 dark:border-[#6d352f] dark:text-[#ffc4bd] dark:hover:bg-[#2a1c1a]"
                  type="button"
                  :disabled="Boolean(actionPending[apiKey.id]) || Boolean(apiKey.revokedAt)"
                  @click="openKeyConfirmation('revoke', apiKey)"
                >
                  <Ban class="h-4 w-4" />
                  Revoke
                </button>
              </div>
            </article>
          </div>

          <form class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="createKey">
            <div class="flex items-start gap-3">
              <KeyRound class="mt-1 h-4 w-4 text-[#3162a3]" />
              <div>
                <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Create</p>
                <h2 class="mt-1 text-lg font-semibold tracking-normal">API key</h2>
              </div>
            </div>

            <label class="block space-y-2">
              <span class="text-sm font-medium">Name</span>
              <input v-model.trim="form.name" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" maxlength="100" required />
              <span class="block text-xs text-[#667169] dark:text-[#aeb8b0]">Use a workload name, such as “Production website”.</span>
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium">Environment</span>
              <select v-model="form.environment" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]">
                <option v-for="environment in environments" :key="environment.value" :value="environment.value">{{ environment.label }}</option>
              </select>
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium">Expires at</span>
              <input v-model="form.expiresAt" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" type="datetime-local" :min="minimumExpiry" />
            </label>

            <fieldset class="space-y-2">
              <legend class="text-sm font-medium">Scopes</legend>
              <p class="text-xs text-[#667169] dark:text-[#aeb8b0]">Grant only what this workload needs.</p>
              <label v-for="scope in availableScopes" :key="scope.value" class="flex items-center gap-2 text-sm">
                <input v-model="form.scopes" class="h-4 w-4 rounded border-[#bfcac3]" type="checkbox" :value="scope.value" />
                {{ scope.label }}
              </label>
            </fieldset>

            <button
              class="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
              type="submit"
              :disabled="creating || !canCreate"
            >
              <LoaderCircle v-if="creating" class="h-4 w-4 animate-spin" />
              <Plus v-else class="h-4 w-4" />
              Create key
            </button>
          </form>
        </div>
      </div>
    </div>

    <div v-if="keyConfirmation" class="fixed inset-0 z-50 flex items-center justify-center bg-black/45 px-4" @click.self="keyConfirmation = null">
      <div class="w-full max-w-md rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-xl dark:border-[#3f4843] dark:bg-[#202522]" role="dialog" aria-modal="true" aria-labelledby="key-confirmation-title">
        <div class="flex items-start gap-3">
          <TriangleAlert class="mt-1 h-5 w-5" :class="keyConfirmation.action === 'revoke' ? 'text-[#9b2d23]' : 'text-[#8a5b00]'" />
          <div>
            <h2 id="key-confirmation-title" class="text-lg font-semibold">{{ keyConfirmation.action === 'revoke' ? 'Revoke API key?' : 'Rotate API key?' }}</h2>
            <p class="mt-2 text-sm text-[#5d6a61] dark:text-[#aeb8b0]">
              <template v-if="keyConfirmation.action === 'revoke'">{{ keyConfirmation.key.name }} will stop working immediately. This cannot be undone.</template>
              <template v-else>A replacement for {{ keyConfirmation.key.name }} will be created. The current key stays active until you revoke it after deployment.</template>
            </p>
            <p class="mt-3 rounded-md bg-[#f2f5f3] px-3 py-2 font-mono text-xs dark:bg-[#171b18]">{{ keyConfirmation.key.tokenPrefix }}…</p>
          </div>
        </div>
        <div class="mt-5 flex justify-end gap-2">
          <button class="rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]" type="button" @click="keyConfirmation = null">Cancel</button>
          <button class="rounded-md px-4 py-2 text-sm font-medium text-white disabled:opacity-60" :class="keyConfirmation.action === 'revoke' ? 'bg-[#9b2d23] hover:bg-[#7d241c]' : 'bg-[#165a4a] hover:bg-[#10463a]'" type="button" :disabled="Boolean(actionPending[keyConfirmation.key.id])" @click="confirmKeyAction">
            {{ keyConfirmation.action === 'revoke' ? 'Revoke key' : 'Create replacement' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="reauthenticationOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/45 px-4" @click.self="cancelReauthentication">
      <form
        class="w-full max-w-md rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-xl dark:border-[#3f4843] dark:bg-[#202522]"
        role="dialog"
        aria-modal="true"
        aria-labelledby="reauthentication-title"
        @submit.prevent="confirmReauthentication"
      >
        <div class="flex items-start gap-3">
          <LockKeyhole class="mt-1 h-5 w-5 text-[#3162a3]" />
          <div>
            <h2 id="reauthentication-title" class="text-lg font-semibold tracking-normal">Confirm your identity</h2>
            <p class="mt-1 text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Enter your current password to {{ pendingProtectedAction?.label || 'continue' }}.</p>
          </div>
        </div>

        <p v-if="reauthenticationError" class="mt-4 rounded-md border border-[#edc6c2] bg-[#fff4f2] px-3 py-2 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ reauthenticationError }}
        </p>

        <label class="mt-5 block space-y-2">
          <span class="text-sm font-medium">Current password</span>
          <input
            v-model="reauthenticationPassword"
            class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]"
            type="password"
            autocomplete="current-password"
            required
            autofocus
          />
        </label>

        <div class="mt-5 flex justify-end gap-2">
          <button class="rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]" type="button" :disabled="reauthenticating" @click="cancelReauthentication">
            Cancel
          </button>
          <button class="inline-flex items-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60" type="submit" :disabled="reauthenticating || !reauthenticationPassword">
            <LoaderCircle v-if="reauthenticating" class="h-4 w-4 animate-spin" />
            Confirm
          </button>
        </div>
      </form>
    </div>
  </section>
</template>

<script setup lang="ts">
import { Activity, ArrowLeft, Ban, CalendarClock, Check, Copy, Eye, EyeOff, KeyRound, LoaderCircle, LockKeyhole, LogOut, Plus, RefreshCw, Search, ShieldCheck, TriangleAlert } from 'lucide-vue-next'
import type { AdminAPIKey, AdminProject } from '~/composables/useAdminApi'

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

const project = ref<AdminProject | null>(null)
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
  expiresAt: '',
  scopes: availableScopes.map((scope) => scope.value)
})
const filters = reactive({ search: '', environment: 'all', status: 'all' })

const canCreate = computed(() => Boolean(form.name.trim() && form.scopes.length > 0))
const filteredAPIKeys = computed(() => {
  const query = filters.search.toLowerCase()
  return apiKeys.value.filter((key) => {
    if (filters.environment !== 'all' && key.environment !== filters.environment) return false
    if (filters.status !== 'all' && keyStatus(key) !== filters.status) return false
    return !query || key.name.toLowerCase().includes(query) || key.tokenPrefix.toLowerCase().includes(query)
  })
})
const keySummary = computed(() => [
  { label: 'Active', value: apiKeys.value.filter(key => keyStatus(key) === 'active').length },
  { label: 'Used recently', value: apiKeys.value.filter(key => wasUsedRecently(key.lastUsedAt)).length },
  { label: 'Revoked', value: apiKeys.value.filter(key => keyStatus(key) === 'revoked').length }
])
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
    const [projectResponse, keys] = await Promise.all([
      api.getProject(projectID.value),
      loadAllAPIKeys()
    ])
    project.value = projectResponse.data
    apiKeys.value = keys
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
      expiresAt: form.expiresAt ? new Date(form.expiresAt).toISOString() : undefined,
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

async function logout() {
  try {
    await api.logout()
  } finally {
    await navigateTo('/')
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
      return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
    case 'expired':
      return 'bg-[#fbe4e1] text-[#8f3028] dark:bg-[#46231f] dark:text-[#ffc4bd]'
    default:
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
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
  switch (environment) {
    case 'production':
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'staging':
      return 'bg-[#e8f0ff] text-[#245b99] dark:bg-[#152944] dark:text-[#b8d5ff]'
    case 'preview':
      return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
    default:
      return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
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

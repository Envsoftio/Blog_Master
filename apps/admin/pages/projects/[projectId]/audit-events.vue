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
            <h1 class="mt-1 text-2xl font-semibold tracking-normal">Audit</h1>
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
      <aside class="flex gap-2 overflow-x-auto lg:block lg:space-y-2">
        <NuxtLink class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" to="/projects">Projects</NuxtLink>
        <NuxtLink class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/articles`">Articles</NuxtLink>
        <NuxtLink class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/categories`">Categories</NuxtLink>
        <NuxtLink v-if="canManageProject" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/members`">Members</NuxtLink>
        <NuxtLink class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/api-keys`">API keys</NuxtLink>
        <NuxtLink v-if="canManageProject" class="block rounded-md bg-white px-3 py-2 text-sm shadow-sm dark:bg-[#252b28]" :to="`/projects/${projectID}/audit-events`">Audit</NuxtLink>
      </aside>

      <div class="space-y-5">
        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>

        <div class="space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Security and editorial trail</p>
              <h2 class="mt-1 text-xl font-semibold tracking-normal">Project events</h2>
            </div>
            <span class="inline-flex items-center gap-2 rounded-md bg-[#eef5f1] px-3 py-2 text-sm text-[#36594a] dark:bg-[#18261f] dark:text-[#b6d7c8]">
              <ShieldCheck class="h-4 w-4" />
              {{ events.length }} loaded
            </span>
          </div>

          <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
            <LoaderCircle class="h-4 w-4 animate-spin" />
            Loading audit events
          </div>

          <div v-else-if="events.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]">
            <h2 class="text-lg font-semibold">No audit events yet</h2>
          </div>

          <article v-for="event in events" :key="event.id" class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div class="min-w-0">
                <h3 class="truncate text-lg font-semibold">{{ event.action }}</h3>
                <p class="mt-1 truncate font-mono text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ event.id }}</p>
              </div>
              <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="outcomeClass(event.outcome)">
                {{ event.outcome }}
              </span>
            </div>

            <dl class="mt-5 grid gap-3 text-sm md:grid-cols-4">
              <div class="flex items-center gap-2">
                <History class="h-4 w-4 text-[#3162a3]" />
                <div class="min-w-0">
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Time</dt>
                  <dd class="truncate">{{ formatDate(event.createdAt) }}</dd>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <UserRound class="h-4 w-4 text-[#165a4a]" />
                <div class="min-w-0">
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Actor</dt>
                  <dd class="truncate">{{ event.actorId || event.actorType }}</dd>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <Crosshair class="h-4 w-4 text-[#8a5b00]" />
                <div class="min-w-0">
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Target</dt>
                  <dd class="truncate">{{ targetLabel(event) }}</dd>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <ScrollText class="h-4 w-4 text-[#6b5797]" />
                <div class="min-w-0">
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Request</dt>
                  <dd class="truncate">{{ event.requestId || 'Not set' }}</dd>
                </div>
              </div>
            </dl>

            <pre v-if="hasMetadata(event)" class="mt-4 overflow-x-auto rounded-md bg-[#17201b] px-3 py-3 text-xs leading-5 text-[#dff7ea]">{{ metadataText(event) }}</pre>
          </article>

          <button
            v-if="nextCursor"
            class="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] bg-white px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:bg-[#202522] dark:hover:bg-[#2a302d]"
            type="button"
            :disabled="loadingMore"
            @click="loadMore"
          >
            <LoaderCircle v-if="loadingMore" class="h-4 w-4 animate-spin" />
            <RefreshCw v-else class="h-4 w-4" />
            Load more events
          </button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ArrowLeft, Crosshair, History, LoaderCircle, LogOut, RefreshCw, ScrollText, ShieldCheck, UserRound } from 'lucide-vue-next'

type APIEnvelope<T> = {
  data: T
}

type APIListEnvelope<T> = {
  data: T[]
  meta: {
    nextCursor?: string
    limit: number
  }
}

type AdminProject = {
  id: string
  slug: string
  name: string
  status: string
  role: string
}

type AuditEvent = {
  id: string
  projectId?: string
  actorType: string
  actorId?: string
  action: string
  targetType?: string
  targetId?: string
  outcome: string
  requestId?: string
  metadata: Record<string, unknown>
  createdAt: string
}

const route = useRoute()
const projectID = computed(() => {
  const value = route.params.projectId
  return Array.isArray(value) ? value[0] : String(value || '')
})

const project = ref<AdminProject | null>(null)
const events = ref<AuditEvent[]>([])
const pending = ref(true)
const loadingMore = ref(false)
const nextCursor = ref('')
const errorMessage = ref('')
const canManageProject = computed(() => project.value?.role === 'project_owner' || project.value?.role === 'project_admin')

onMounted(refresh)

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const [projectResponse, auditResponse] = await Promise.all([
      $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}`, { credentials: 'include' }),
      $fetch<APIListEnvelope<AuditEvent>>(`/api/v1/projects/${projectID.value}/audit-events`, { credentials: 'include' })
    ])
    project.value = projectResponse.data
    events.value = auditResponse.data
    nextCursor.value = auditResponse.meta.nextCursor || ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load audit events. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function loadMore() {
  if (!nextCursor.value || loadingMore.value) return
  loadingMore.value = true
  errorMessage.value = ''
  try {
    const response = await $fetch<APIListEnvelope<AuditEvent>>(`/api/v1/projects/${projectID.value}/audit-events`, {
      credentials: 'include',
      query: {
        cursor: nextCursor.value,
        limit: 50
      }
    })
    events.value = [...events.value, ...response.data]
    nextCursor.value = response.meta.nextCursor || ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load more audit events.')
  } finally {
    loadingMore.value = false
  }
}

async function logout() {
  try {
    const csrfToken = await getCSRFToken()
    await $fetch('/api/v1/auth/logout', {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken }
    })
  } finally {
    await navigateTo('/')
  }
}

async function getCSRFToken() {
  const response = await $fetch<APIEnvelope<{ csrfToken: string }>>('/api/v1/auth/csrf', {
    credentials: 'include'
  })
  return response.data.csrfToken
}

function targetLabel(event: AuditEvent) {
  if (!event.targetType && !event.targetId) return 'Not set'
  return [event.targetType, event.targetId].filter(Boolean).join(' ')
}

function hasMetadata(event: AuditEvent) {
  return Boolean(event.metadata && Object.keys(event.metadata).length > 0)
}

function metadataText(event: AuditEvent) {
  return JSON.stringify(event.metadata, null, 2)
}

function parseBackendUTC(value: string) {
  return new Date(value.includes('T') ? value : `${value.replace(' ', 'T')}Z`)
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

function outcomeClass(outcome: string) {
  switch (outcome) {
    case 'success':
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'failure':
      return 'bg-[#fbe4e1] text-[#8f3028] dark:bg-[#46231f] dark:text-[#ffc4bd]'
    default:
      return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
}

function normalizeAPIError(error: unknown, fallback: string) {
  if (typeof error === 'object' && error !== null && 'data' in error) {
    const data = (error as { data?: { title?: string, detail?: string } }).data
    return data?.detail || data?.title || fallback
  }
  return fallback
}
</script>

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
      <ProjectNav :project-id="projectID" :project="project" active="settings" />

      <div class="space-y-5">
        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>
        <p v-if="successMessage" class="rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-4 py-3 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]">
          {{ successMessage }}
        </p>

        <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
          <LoaderCircle class="h-4 w-4 animate-spin" />
          Loading project settings
        </div>

        <div v-else class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
          <form class="space-y-5 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="saveProject">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div>
                <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Project defaults</p>
                <h2 class="mt-1 text-xl font-semibold tracking-normal">Tenant and SEO configuration</h2>
              </div>
              <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(project?.status || '')">{{ project?.status || 'unknown' }}</span>
            </div>

            <div class="grid gap-4 md:grid-cols-2">
              <label class="block space-y-2">
                <span class="text-sm font-medium">Name</span>
                <input v-model.trim="form.name" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required :disabled="!canManageProject" />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Public key</span>
                <input :value="project?.publicProjectKey || ''" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 font-mono text-sm dark:border-[#4b5650] dark:bg-[#171b18]" disabled />
              </label>
            </div>

            <div class="grid gap-4 md:grid-cols-2">
              <label class="block space-y-2">
                <span class="text-sm font-medium">Primary domain</span>
                <input v-model.trim="form.primaryDomain" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" :disabled="!canManageProject" />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Blog base path</span>
                <input v-model.trim="form.blogBasePath" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required :disabled="!canManageProject" />
              </label>
            </div>

            <label class="block space-y-2">
              <span class="text-sm font-medium">Verified domains</span>
              <textarea v-model.trim="form.verifiedDomains" class="min-h-24 w-full rounded-md border border-[#bfcac3] px-3 py-2 font-mono text-sm dark:border-[#4b5650] dark:bg-[#171b18]" :disabled="!canManageProject" />
            </label>

            <div class="grid gap-4 md:grid-cols-1">
              <label class="block space-y-2">
                <span class="text-sm font-medium">Timezone</span>
                <input v-model.trim="form.timezone" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required :disabled="!canManageProject" />
              </label>
            </div>

            <div class="grid gap-4 md:grid-cols-2">
              <label class="block space-y-2">
                <span class="text-sm font-medium">Publisher name</span>
                <input v-model.trim="form.publisherName" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" :disabled="!canManageProject" />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Publisher URL</span>
                <input v-model.trim="form.publisherUrl" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" type="url" :disabled="!canManageProject" />
              </label>
            </div>

            <label class="block space-y-2">
              <span class="text-sm font-medium">Default robots policy</span>
              <select v-model="form.defaultRobotsPolicy" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" :disabled="!canManageProject">
                <option value="index,follow">index,follow</option>
                <option value="noindex,nofollow">noindex,nofollow</option>
                <option value="noindex,follow">noindex,follow</option>
              </select>
            </label>

            <label class="flex items-start gap-3 rounded-md border border-[#e1bd70] bg-[#fff8e7] p-3 text-sm text-[#6b4905] dark:border-[#665223] dark:bg-[#2b2415] dark:text-[#f5d992]">
              <input
                v-model="form.soloOwnerApprovalEnabled"
                class="mt-1 h-4 w-4"
                type="checkbox"
                :disabled="project?.role !== 'project_owner'"
              >
              <span>
                <strong class="block">Allow owner self-approval</strong>
                <span class="mt-1 block text-xs">
                  When enabled, a project owner may approve an exact revision they created. Other roles can never self-approve.
                </span>
              </span>
            </label>

            <button
              v-if="canManageProject"
              class="inline-flex items-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
              type="submit"
              :disabled="saving || !canSave"
            >
              <LoaderCircle v-if="saving" class="h-4 w-4 animate-spin" />
              <Save v-else class="h-4 w-4" />
              Save settings
            </button>
          </form>

          <div class="space-y-5">
            <section class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex items-start gap-3">
                <ShieldAlert class="mt-1 h-4 w-4 text-[#8a5b00]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Lifecycle</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Project status</h2>
                </div>
              </div>

              <div class="mt-5 grid gap-2">
                <button
                  class="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-[#d6bd7a] px-3 text-sm font-medium text-[#7a4f00] hover:bg-[#fff7e4] disabled:opacity-60 dark:border-[#6b572e] dark:text-[#ffd98a] dark:hover:bg-[#2b2415]"
                  type="button"
                  :disabled="statusPending || !canManageProject || project?.status === 'suspended'"
                  @click="setStatus('suspended')"
                >
                  <PauseCircle class="h-4 w-4" />
                  Suspend
                </button>
                <button
                  class="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-3 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                  type="button"
                  :disabled="statusPending || !canManageProject || project?.status === 'archived'"
                  @click="setStatus('archived')"
                >
                  <Archive class="h-4 w-4" />
                  Archive
                </button>
              </div>
            </section>

            <section class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div class="flex items-start gap-3">
                  <ListChecks class="mt-1 h-4 w-4 text-[#3162a3]" />
                  <div>
                    <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Deletion impact</p>
                    <h2 class="mt-1 text-lg font-semibold tracking-normal">Dependencies</h2>
                  </div>
                </div>
                <button
                  class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] hover:bg-[#eef5f1] disabled:opacity-50 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                  type="button"
                  title="Refresh deletion impact"
                  aria-label="Refresh deletion impact"
                  :disabled="impactPending || !canManageProject"
                  @click="loadDeletionImpact"
                >
                  <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': impactPending }" />
                </button>
              </div>

              <dl v-if="deletionImpact" class="mt-5 grid gap-3 text-sm">
                <div v-for="item in impactRows" :key="item.label" class="flex items-center justify-between gap-4 rounded-md bg-[#f2f5f3] px-3 py-2 dark:bg-[#171b18]">
                  <dt class="text-[#5d6a61] dark:text-[#aeb8b0]">{{ item.label }}</dt>
                  <dd class="font-mono">{{ item.value }}</dd>
                </div>
              </dl>

              <button
                class="mt-5 inline-flex w-full items-center justify-center gap-2 rounded-md border border-[#d9b7aa] px-4 py-2 text-sm font-medium text-[#9b2d23] hover:bg-[#fff4f2] disabled:opacity-60 dark:border-[#6d352f] dark:text-[#ffc4bd] dark:hover:bg-[#2a1c1a]"
                type="button"
                :disabled="deleting || !canDeleteProject"
                @click="deleteProject"
              >
                <LoaderCircle v-if="deleting" class="h-4 w-4 animate-spin" />
                <Trash2 v-else class="h-4 w-4" />
                Delete project
              </button>
            </section>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import {
  Archive,
  ArrowLeft,
  ListChecks,
  LoaderCircle,
  LogOut,
  PauseCircle,
  RefreshCw,
  Save,
  ShieldAlert,
  Trash2
} from 'lucide-vue-next'

type APIEnvelope<T> = {
  data: T
}

type AdminProject = {
  id: string
  slug: string
  name: string
  status: string
  publicProjectKey: string
  primaryDomain?: string
  verifiedDomains: string[]
  blogBasePath: string
  timezone: string
  publisherName?: string
  publisherUrl?: string
  defaultRobotsPolicy: string
  soloOwnerApprovalEnabled: boolean
  role: string
  createdAt: string
  updatedAt: string
}

type ProjectDeletionImpact = {
  projectId: string
  canDelete: boolean
  activeApiKeys: number
  activeMembers: number
  contentItems: number
  publishedPublications: number
  scheduledPublications: number
  redirects: number
  assets: number
  webhooks: number
  pendingJobs: number
}

const route = useRoute()
const projectID = computed(() => {
  const value = route.params.projectId
  return Array.isArray(value) ? String(value[0] || '') : String(value || '')
})

const project = ref<AdminProject | null>(null)
const deletionImpact = ref<ProjectDeletionImpact | null>(null)
const pending = ref(true)
const saving = ref(false)
const statusPending = ref(false)
const impactPending = ref(false)
const deleting = ref(false)
const errorMessage = ref('')
const successMessage = ref('')

const form = reactive({
  name: '',
  primaryDomain: '',
  verifiedDomains: '',
  blogBasePath: '/blog',
  timezone: 'UTC',
  publisherName: '',
  publisherUrl: '',
  defaultRobotsPolicy: 'index,follow',
  soloOwnerApprovalEnabled: false
})

const canManageProject = computed(() => project.value?.role === 'project_owner' || project.value?.role === 'project_admin')
const canDeleteProject = computed(() => project.value?.role === 'project_owner' && Boolean(deletionImpact.value?.canDelete))
const canSave = computed(() => Boolean(form.name.trim() && form.blogBasePath.trim() && form.timezone.trim()))
const impactRows = computed(() => {
  if (!deletionImpact.value) return []
  return [
    { label: 'Can delete', value: deletionImpact.value.canDelete ? 'yes' : 'no' },
    { label: 'Active API keys', value: deletionImpact.value.activeApiKeys },
    { label: 'Active members', value: deletionImpact.value.activeMembers },
    { label: 'Content items', value: deletionImpact.value.contentItems },
    { label: 'Published publications', value: deletionImpact.value.publishedPublications },
    { label: 'Scheduled publications', value: deletionImpact.value.scheduledPublications },
    { label: 'Redirects', value: deletionImpact.value.redirects },
    { label: 'Assets', value: deletionImpact.value.assets },
    { label: 'Webhooks', value: deletionImpact.value.webhooks },
    { label: 'Pending jobs', value: deletionImpact.value.pendingJobs }
  ]
})

onMounted(refresh)

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const response = await $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}`, {
      credentials: 'include'
    })
    setProject(response.data)
    if (canManageProject.value) await loadDeletionImpact()
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load project settings. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function saveProject() {
  saving.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}`, {
      method: 'PATCH',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: projectPatchBody()
    })
    setProject(response.data)
    successMessage.value = 'Project settings saved.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not save project settings.')
  } finally {
    saving.value = false
  }
}

async function setStatus(status: 'suspended' | 'archived') {
  if (!window.confirm(`${capitalize(status)} this project?`)) return
  statusPending.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}/${status === 'suspended' ? 'suspend' : 'archive'}`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {}
    })
    setProject(response.data)
    successMessage.value = `Project ${status}.`
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, `Could not ${status} project.`)
  } finally {
    statusPending.value = false
  }
}

async function loadDeletionImpact() {
  impactPending.value = true
  try {
    const response = await $fetch<APIEnvelope<ProjectDeletionImpact>>(`/api/v1/projects/${projectID.value}/deletion-impact`, {
      credentials: 'include'
    })
    deletionImpact.value = response.data
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load deletion impact.')
  } finally {
    impactPending.value = false
  }
}

async function deleteProject() {
  if (!window.confirm(`Delete ${project.value?.name || 'this project'}? This cannot be undone.`)) return
  deleting.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    await $fetch(`/api/v1/projects/${projectID.value}`, {
      method: 'DELETE',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken }
    })
    await navigateTo('/projects')
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not delete project.')
  } finally {
    deleting.value = false
  }
}

function setProject(value: AdminProject) {
  project.value = value
  form.name = value.name || ''
  form.primaryDomain = value.primaryDomain || ''
  form.verifiedDomains = (value.verifiedDomains || []).join('\n')
  form.blogBasePath = value.blogBasePath || '/blog'
  form.timezone = value.timezone || 'UTC'
  form.publisherName = value.publisherName || ''
  form.publisherUrl = value.publisherUrl || ''
  form.defaultRobotsPolicy = value.defaultRobotsPolicy || 'index,follow'
  form.soloOwnerApprovalEnabled = Boolean(value.soloOwnerApprovalEnabled)
}

function projectPatchBody() {
  const body: Record<string, unknown> = {
    name: form.name,
    primaryDomain: form.primaryDomain,
    verifiedDomains: splitLines(form.verifiedDomains),
    blogBasePath: form.blogBasePath,
    timezone: form.timezone,
    publisherName: form.publisherName,
    publisherUrl: form.publisherUrl,
    defaultRobotsPolicy: form.defaultRobotsPolicy
  }
  if (project.value?.role === 'project_owner') {
    body.soloOwnerApprovalEnabled = form.soloOwnerApprovalEnabled
  }
  return body
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

function splitLines(value: string) {
  return cleanList(value.split('\n'))
}

function cleanList(values: string[]) {
  const seen = new Set<string>()
  const out: string[] = []
  for (const value of values) {
    const item = value.trim()
    if (!item || seen.has(item)) continue
    seen.add(item)
    out.push(item)
  }
  return out
}

function statusClass(status: string) {
  switch (status) {
    case 'active':
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'suspended':
      return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
    case 'archived':
      return 'bg-[#e9edf4] text-[#40506a] dark:bg-[#252d3a] dark:text-[#c4d0e3]'
    default:
      return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
}

function capitalize(value: string) {
  return `${value.slice(0, 1).toUpperCase()}${value.slice(1)}`
}

function clearMessages() {
  errorMessage.value = ''
  successMessage.value = ''
}

function normalizeAPIError(error: unknown, fallback: string) {
  if (typeof error === 'object' && error !== null && 'data' in error) {
    const data = (error as { data?: { title?: string, detail?: string } }).data
    return data?.detail || data?.title || fallback
  }
  if (error instanceof Error && error.message) return error.message
  return fallback
}
</script>

<template>
  <section class="min-h-screen">
    <header class="border-b border-[#d7ded8] bg-white px-6 py-4 dark:border-[#343a38] dark:bg-[#202422]">
      <div class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4">
        <div>
          <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Workspace</p>
          <h1 class="mt-1 text-2xl font-semibold tracking-normal">Dashboard</h1>
        </div>
        <div class="flex items-center gap-2">
          <button
            class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#eef5f1] disabled:opacity-50 dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]"
            type="button"
            title="Refresh"
            aria-label="Refresh"
            :disabled="pending"
            @click="fetchProjects"
          >
            <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': pending }" />
          </button>
          <NuxtLink
            class="inline-flex h-10 w-10 items-center justify-center rounded-md bg-[#165a4a] text-white hover:bg-[#10463a]"
            to="/projects?new=1"
            title="New project"
            aria-label="New project"
          >
            <Plus class="h-4 w-4" />
          </NuxtLink>
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

    <main class="mx-auto grid max-w-7xl grid-cols-1 gap-6 px-6 py-6 lg:grid-cols-[220px_1fr]">
      <aside class="flex gap-2 overflow-x-auto lg:block lg:space-y-2">
        <NuxtLink class="block rounded-md bg-white px-3 py-2 text-sm shadow-sm dark:bg-[#252b28]" to="/dashboard">Dashboard</NuxtLink>
        <NuxtLink class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" to="/projects">Projects</NuxtLink>
        <NuxtLink v-if="selectedProjectID" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${selectedProjectID}/articles`">Articles</NuxtLink>
        <NuxtLink v-if="selectedProjectID" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${selectedProjectID}/categories`">Categories</NuxtLink>
        <NuxtLink v-if="selectedProjectID" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${selectedProjectID}/series`">Series</NuxtLink>
        <NuxtLink v-if="selectedProjectID" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${selectedProjectID}/authors`">Authors</NuxtLink>
        <NuxtLink v-if="selectedProjectID" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${selectedProjectID}/api-keys`">API keys</NuxtLink>
      </aside>

      <div class="space-y-6">
        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>

        <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
          <LoaderCircle class="h-4 w-4 animate-spin" />
          Loading dashboard
        </div>

        <template v-else>
          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <article v-for="metric in metrics" :key="metric.label" class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex items-center justify-between gap-3">
                <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">{{ metric.label }}</p>
                <component :is="metric.icon" class="h-4 w-4" :class="metric.color" />
              </div>
              <p class="mt-3 text-3xl font-semibold tracking-normal">{{ metric.value }}</p>
            </article>
          </div>

          <div v-if="projects.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]">
            <h2 class="text-lg font-semibold">No projects yet</h2>
            <NuxtLink class="mt-5 inline-flex items-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a]" to="/projects?new=1">
              <Plus class="h-4 w-4" />
              New project
            </NuxtLink>
          </div>

          <div v-else class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
            <section class="space-y-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Focus</p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">{{ selectedProject?.name || 'Project' }}</h2>
                </div>
                <select
                  v-model="selectedProjectID"
                  class="h-10 rounded-md border border-[#bfcac3] bg-white px-3 py-2 text-sm text-[#20231f] dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]"
                >
                  <option v-for="project in projects" :key="project.id" :value="project.id">{{ project.name }}</option>
                </select>
              </div>

              <article v-if="selectedProject" class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
                <div class="flex flex-wrap items-start justify-between gap-4">
                  <div class="min-w-0">
                    <h3 class="truncate text-lg font-semibold">{{ selectedProject.name }}</h3>
                    <p class="mt-1 truncate text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ selectedProject.slug }}</p>
                  </div>
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(selectedProject.status)">
                    {{ selectedProject.status }}
                  </span>
                </div>

                <dl class="mt-5 grid gap-3 text-sm sm:grid-cols-2">
                  <div class="flex items-center gap-2">
                    <Globe2 class="h-4 w-4 text-[#3162a3]" />
                    <div class="min-w-0">
                      <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Domain</dt>
                      <dd class="truncate">{{ selectedProject.primaryDomain || 'Not set' }}</dd>
                    </div>
                  </div>
                  <div class="flex items-center gap-2">
                    <ShieldCheck class="h-4 w-4 text-[#8a5b00]" />
                    <div class="min-w-0">
                      <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Role</dt>
                      <dd class="truncate">{{ roleLabel(selectedProject.role) }}</dd>
                    </div>
                  </div>
                  <div class="flex items-center gap-2">
                    <FolderKanban class="h-4 w-4 text-[#165a4a]" />
                    <div class="min-w-0">
                      <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Blog path</dt>
                      <dd class="truncate">{{ selectedProject.blogBasePath }}</dd>
                    </div>
                  </div>
                  <div class="flex items-center gap-2">
                    <Languages class="h-4 w-4 text-[#7b4f9d]" />
                    <div class="min-w-0">
                      <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Locale</dt>
                      <dd class="truncate">{{ selectedProject.defaultLocale }}</dd>
                    </div>
                  </div>
                </dl>

                <div class="mt-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                  <NuxtLink v-for="action in projectActions" :key="action.label" class="inline-flex items-center justify-between gap-3 rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]" :to="action.to">
                    <span class="inline-flex items-center gap-2">
                      <component :is="action.icon" class="h-4 w-4" />
                      {{ action.label }}
                    </span>
                    <ArrowRight class="h-4 w-4" />
                  </NuxtLink>
                </div>
              </article>
            </section>

            <section class="space-y-4">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Projects</p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">Recent</h2>
                </div>
                <NuxtLink class="inline-flex items-center gap-2 rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]" to="/projects">
                  Open
                  <ArrowRight class="h-4 w-4" />
                </NuxtLink>
              </div>

              <div class="space-y-3">
                <NuxtLink
                  v-for="project in projects.slice(0, 5)"
                  :key="project.id"
                  class="flex items-center justify-between gap-3 rounded-lg border border-[#cfd8d1] bg-white p-4 shadow-sm hover:bg-[#f7faf8] dark:border-[#3f4843] dark:bg-[#202522] dark:hover:bg-[#252b28]"
                  :to="`/projects/${project.id}/articles`"
                >
                  <span class="min-w-0">
                    <span class="block truncate text-sm font-medium">{{ project.name }}</span>
                    <span class="mt-1 block truncate text-xs text-[#667169] dark:text-[#aeb8b0]">{{ project.slug }}</span>
                  </span>
                  <ArrowRight class="h-4 w-4 shrink-0 text-[#667169]" />
                </NuxtLink>
              </div>
            </section>
          </div>
        </template>
      </div>
    </main>
  </section>
</template>

<script setup lang="ts">
import {
  ArrowRight,
  FileText,
  FolderKanban,
  FolderTree,
  Globe2,
  KeyRound,
  Languages,
  LayoutGrid,
  LoaderCircle,
  LogOut,
  PanelsTopLeft,
  Plus,
  RefreshCw,
  ScrollText,
  Settings,
  ShieldCheck,
  UsersRound
} from 'lucide-vue-next'

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
  publicProjectKey: string
  primaryDomain?: string
  blogBasePath: string
  defaultLocale: string
  supportedLocales: string[]
  timezone: string
  role: string
}

const projects = ref<AdminProject[]>([])
const selectedProjectID = ref('')
const pending = ref(true)
const errorMessage = ref('')

const selectedProject = computed(() => projects.value.find(project => project.id === selectedProjectID.value) || projects.value[0] || null)
const manageableProjects = computed(() => projects.value.filter(project => project.role === 'project_owner' || project.role === 'project_admin'))
const selectedManaged = computed(() => Boolean(selectedProject.value && (selectedProject.value.role === 'project_owner' || selectedProject.value.role === 'project_admin')))
const metrics = computed(() => [
  { label: 'Projects', value: projects.value.length, icon: LayoutGrid, color: 'text-[#165a4a]' },
  { label: 'Active', value: projects.value.filter(project => project.status === 'active').length, icon: FolderKanban, color: 'text-[#3162a3]' },
  { label: 'Managed', value: manageableProjects.value.length, icon: ShieldCheck, color: 'text-[#8a5b00]' },
  { label: 'Archived', value: projects.value.filter(project => project.status === 'archived').length, icon: ScrollText, color: 'text-[#6b7280]' }
])
const projectActions = computed(() => {
  const project = selectedProject.value
  if (!project) return []
  const base = `/projects/${project.id}`
  const actions = [
    { label: 'Articles', to: `${base}/articles`, icon: FileText },
    { label: 'Categories', to: `${base}/categories`, icon: FolderTree },
    { label: 'Series', to: `${base}/series`, icon: PanelsTopLeft },
    { label: 'Authors', to: `${base}/authors`, icon: UsersRound },
    { label: 'API keys', to: `${base}/api-keys`, icon: KeyRound }
  ]
  if (selectedManaged.value) {
    actions.push(
      { label: 'Members', to: `${base}/members`, icon: ShieldCheck },
      { label: 'Settings', to: `${base}/settings`, icon: Settings }
    )
  }
  return actions
})

onMounted(fetchProjects)

async function fetchProjects() {
  pending.value = true
  errorMessage.value = ''
  try {
    const response = await $fetch<APIListEnvelope<AdminProject>>('/api/v1/projects', {
      credentials: 'include',
      query: { limit: 100 }
    })
    projects.value = response.data
    if (!selectedProjectID.value || !projects.value.some(project => project.id === selectedProjectID.value)) {
      selectedProjectID.value = projects.value[0]?.id || ''
    }
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load dashboard. Sign in again if your session has expired.')
  } finally {
    pending.value = false
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

function roleLabel(role: string) {
  return role.replaceAll('_', ' ')
}

function normalizeAPIError(error: unknown, fallback: string) {
  if (typeof error === 'object' && error !== null && 'data' in error) {
    const data = (error as { data?: { title?: string, detail?: string, statusCode?: number, statusMessage?: string, message?: string } }).data
    if (data?.statusCode === 502) {
      return 'The admin API is unavailable. Start the Go API on the configured proxy port or set NUXT_API_BASE_URL to the running API.'
    }
    return data?.detail || data?.title || data?.message || fallback
  }
  return fallback
}
</script>

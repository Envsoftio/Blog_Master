<template>
  <section class="min-h-screen">
    <header class="border-b border-[#d7ded8] bg-white px-6 py-4 dark:border-[#343a38] dark:bg-[#202422]">
      <div class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4">
        <div>
          <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Workspace</p>
          <h1 class="mt-1 text-2xl font-semibold tracking-normal">Projects</h1>
        </div>
        <div class="flex items-center gap-2">
          <button
            class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#eef5f1] disabled:opacity-50 dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]"
            type="button"
            title="Refresh projects"
            aria-label="Refresh projects"
            :disabled="pending"
            @click="fetchProjects"
          >
            <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': pending }" />
          </button>
          <button
            class="inline-flex items-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a]"
            type="button"
            @click="formOpen = !formOpen"
          >
            <Plus class="h-4 w-4" />
            New project
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
        <NuxtLink class="block rounded-md bg-white px-3 py-2 text-sm shadow-sm dark:bg-[#252b28]" to="/projects">Projects</NuxtLink>
        <NuxtLink v-if="firstProjectID" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${firstProjectID}/articles`">Articles</NuxtLink>
        <NuxtLink v-if="firstProjectID" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${firstProjectID}/categories`">Categories</NuxtLink>
        <NuxtLink v-if="firstProjectID" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${firstProjectID}/api-keys`">API keys</NuxtLink>
      </aside>

      <div class="space-y-5">
        <form
          v-if="formOpen"
          class="grid gap-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522] md:grid-cols-2"
          @submit.prevent="createProject"
        >
          <label class="block space-y-2">
            <span class="text-sm font-medium">Name</span>
            <input v-model.trim="form.name" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
          </label>
          <label class="block space-y-2">
            <span class="text-sm font-medium">Slug</span>
            <input v-model.trim="form.slug" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
          </label>
          <label class="block space-y-2">
            <span class="text-sm font-medium">Primary domain</span>
            <input v-model.trim="form.primaryDomain" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" placeholder="example.com" />
          </label>
          <label class="block space-y-2">
            <span class="text-sm font-medium">Blog path</span>
            <input v-model.trim="form.blogBasePath" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
          </label>
          <div class="flex items-end gap-2 md:col-span-2">
            <button
              class="inline-flex items-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
              type="submit"
              :disabled="creating"
            >
              <LoaderCircle v-if="creating" class="h-4 w-4 animate-spin" />
              <Plus v-else class="h-4 w-4" />
              Create
            </button>
            <button class="rounded-md px-4 py-2 text-sm text-[#58625c] hover:bg-[#eef2ef] dark:text-[#bec7c1] dark:hover:bg-[#2a302d]" type="button" @click="formOpen = false">
              Cancel
            </button>
          </div>
        </form>

        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>

        <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
          <LoaderCircle class="h-4 w-4 animate-spin" />
          Loading projects
        </div>

        <div v-else-if="projects.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]">
          <h2 class="text-lg font-semibold">No projects yet</h2>
          <p class="mt-2 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Create the first tenant project to begin setting up content, authors and API keys.</p>
        </div>

        <div v-else class="grid gap-4 xl:grid-cols-2">
          <article v-for="project in projects" :key="project.id" class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
            <div class="flex items-start justify-between gap-4">
              <div class="min-w-0">
                <h2 class="truncate text-lg font-semibold">{{ project.name }}</h2>
                <p class="mt-1 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ project.slug }}</p>
              </div>
              <span class="shrink-0 rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(project.status)">
                {{ project.status }}
              </span>
            </div>

            <dl class="mt-5 grid gap-3 text-sm sm:grid-cols-2">
              <div class="flex items-center gap-2">
                <Globe2 class="h-4 w-4 text-[#3162a3]" />
                <div class="min-w-0">
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Domain</dt>
                  <dd class="truncate">{{ project.primaryDomain || 'Not set' }}</dd>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <ShieldCheck class="h-4 w-4 text-[#8a5b00]" />
                <div class="min-w-0">
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Role</dt>
                  <dd class="truncate">{{ roleLabel(project.role) }}</dd>
                </div>
              </div>
            </dl>

            <NuxtLink class="mt-5 inline-flex items-center gap-2 rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]" :to="`/projects/${project.id}/articles`">
              Open
              <ArrowRight class="h-4 w-4" />
            </NuxtLink>
          </article>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ArrowRight, Globe2, LoaderCircle, LogOut, Plus, RefreshCw, ShieldCheck } from 'lucide-vue-next'

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
const pending = ref(true)
const creating = ref(false)
const formOpen = ref(false)
const errorMessage = ref('')
const firstProjectID = computed(() => projects.value[0]?.id || '')

const form = reactive({
  name: '',
  slug: '',
  primaryDomain: '',
  blogBasePath: '/blog',
  defaultLocale: 'en',
  timezone: 'UTC'
})

watch(() => form.name, (value) => {
  if (!form.slug) form.slug = slugify(value)
})

onMounted(fetchProjects)

async function fetchProjects() {
  pending.value = true
  errorMessage.value = ''
  try {
    const response = await $fetch<APIListEnvelope<AdminProject>>('/api/v1/projects', {
      credentials: 'include'
    })
    projects.value = response.data
  } catch {
    errorMessage.value = 'Could not load projects. Sign in again if your session has expired.'
  } finally {
    pending.value = false
  }
}

async function createProject() {
  creating.value = true
  errorMessage.value = ''
  try {
    const csrfToken = await getCSRFToken()
    await $fetch<APIEnvelope<AdminProject>>('/api/v1/projects', {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        name: form.name,
        slug: form.slug,
        primaryDomain: form.primaryDomain,
        blogBasePath: form.blogBasePath,
        defaultLocale: form.defaultLocale,
        supportedLocales: [form.defaultLocale],
        timezone: form.timezone
      }
    })
    form.name = ''
    form.slug = ''
    form.primaryDomain = ''
    form.blogBasePath = '/blog'
    formOpen.value = false
    await fetchProjects()
  } catch {
    errorMessage.value = 'Could not create project. Check the fields and try again.'
  } finally {
    creating.value = false
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

function slugify(value: string) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}
</script>

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
            <h1 class="mt-1 text-2xl font-semibold tracking-normal">Categories</h1>
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
        <NuxtLink class="block rounded-md bg-white px-3 py-2 text-sm shadow-sm dark:bg-[#252b28]" :to="`/projects/${projectID}/categories`">Categories</NuxtLink>
        <NuxtLink class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/authors`">Authors</NuxtLink>
        <NuxtLink v-if="project?.role === 'project_owner' || project?.role === 'project_admin'" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/members`">Members</NuxtLink>
        <NuxtLink class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/api-keys`">API keys</NuxtLink>
        <NuxtLink v-if="project?.role === 'project_owner' || project?.role === 'project_admin'" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/audit-events`">Audit</NuxtLink>
      </aside>

      <div class="space-y-5">
        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>
        <p v-if="successMessage" class="rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-4 py-3 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]">
          {{ successMessage }}
        </p>

        <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
          <div class="space-y-4">
            <div>
              <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Taxonomy</p>
              <h2 class="mt-1 text-xl font-semibold tracking-normal">Primary categories</h2>
            </div>

            <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
              <LoaderCircle class="h-4 w-4 animate-spin" />
              Loading categories
            </div>

            <div v-else-if="categories.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]">
              <h2 class="text-lg font-semibold">No categories yet</h2>
              <p class="mt-2 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Create one before drafting articles.</p>
            </div>

            <article v-for="category in categories" :key="category.id" class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex items-start justify-between gap-4">
                <div class="min-w-0">
                  <h3 class="truncate text-lg font-semibold">{{ category.name }}</h3>
                  <p class="mt-1 truncate text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ category.slug }}</p>
                </div>
                <span class="shrink-0 rounded-full px-2.5 py-1 text-xs font-medium" :class="category.indexable ? 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]' : 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'">
                  {{ category.indexable ? 'index' : 'noindex' }}
                </span>
              </div>
              <p v-if="category.description" class="mt-4 text-sm text-[#4f5b54] dark:text-[#c5cec8]">
                {{ category.description }}
              </p>
            </article>
          </div>

          <form
            class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]"
            @submit.prevent="createCategory"
          >
            <div class="flex items-start gap-3">
              <FolderTree class="mt-1 h-4 w-4 text-[#3162a3]" />
              <div>
                <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Create</p>
                <h2 class="mt-1 text-lg font-semibold tracking-normal">Category</h2>
              </div>
            </div>
            <label class="block space-y-2">
              <span class="text-sm font-medium">Name</span>
              <input v-model.trim="form.name" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
            </label>
            <label class="block space-y-2">
              <span class="text-sm font-medium">Slug</span>
              <input v-model.trim="form.slug" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
            </label>
            <label class="block space-y-2">
              <span class="text-sm font-medium">Description</span>
              <textarea v-model.trim="form.description" class="min-h-24 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
            </label>
            <label class="flex items-center gap-2 text-sm">
              <input v-model="form.indexable" class="h-4 w-4 rounded border-[#bfcac3]" type="checkbox" />
              Indexable
            </label>
            <button
              class="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
              type="submit"
              :disabled="creating || !canCreate"
            >
              <LoaderCircle v-if="creating" class="h-4 w-4 animate-spin" />
              <Plus v-else class="h-4 w-4" />
              Create category
            </button>
          </form>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ArrowLeft, FolderTree, LoaderCircle, LogOut, Plus, RefreshCw } from 'lucide-vue-next'

type APIEnvelope<T> = {
  data: T
}

type APIListEnvelope<T> = {
  data: T[]
}

type AdminProject = {
  id: string
  slug: string
  name: string
  status: string
  role: string
}

type TaxonomyTerm = {
  id: string
  type: string
  slug: string
  name: string
  description?: string
  parentId?: string
  indexable: boolean
}

const route = useRoute()
const projectID = computed(() => {
  const value = route.params.projectId
  return Array.isArray(value) ? value[0] : String(value || '')
})

const project = ref<AdminProject | null>(null)
const categories = ref<TaxonomyTerm[]>([])
const pending = ref(true)
const creating = ref(false)
const errorMessage = ref('')
const successMessage = ref('')

const form = reactive({
  name: '',
  slug: '',
  description: '',
  indexable: true
})

const canCreate = computed(() => Boolean(form.name.trim() && form.slug.trim()))

watch(() => form.name, (value) => {
  if (!form.slug) form.slug = slugify(value)
})

onMounted(refresh)

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const [projectResponse, categoryResponse] = await Promise.all([
      $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}`, { credentials: 'include' }),
      $fetch<APIListEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID.value}/categories`, { credentials: 'include' })
    ])
    project.value = projectResponse.data
    categories.value = categoryResponse.data
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load categories. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function createCategory() {
  creating.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID.value}/categories`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        name: form.name,
        slug: form.slug,
        description: form.description,
        indexable: form.indexable
      }
    })
    categories.value = [...categories.value, response.data].sort((left, right) => left.name.localeCompare(right.name))
    form.name = ''
    form.slug = ''
    form.description = ''
    form.indexable = true
    successMessage.value = 'Category created.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create category.')
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

function slugify(value: string) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
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
  return fallback
}
</script>

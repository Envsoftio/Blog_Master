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
            <h1 class="mt-1 text-2xl font-semibold tracking-normal">Series</h1>
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
      <ProjectNav :project-id="projectID" :project="project" active="series" />

      <div class="space-y-5">
        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>
        <p v-if="successMessage" class="rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-4 py-3 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]">
          {{ successMessage }}
        </p>

        <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
          <div class="space-y-6">
            <section class="space-y-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Topic clusters</p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">Editorial series</h2>
                </div>
                <span class="rounded-md bg-[#eef5f1] px-3 py-2 text-sm text-[#36594a] dark:bg-[#18261f] dark:text-[#b6d7c8]">{{ seriesItems.length }} series</span>
              </div>

              <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
                <LoaderCircle class="h-4 w-4 animate-spin" />
                Loading series
              </div>

              <div v-else-if="seriesItems.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]">
                <h2 class="text-lg font-semibold">No series yet</h2>
                <p class="mt-2 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Group pillar and supporting articles into stable public collections.</p>
              </div>

              <article v-for="item in seriesItems" :key="item.id" class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
                <div class="flex flex-wrap items-start justify-between gap-4">
                  <div class="min-w-0">
                    <h3 class="truncate text-lg font-semibold">{{ item.name }}</h3>
                    <p class="mt-1 truncate text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ item.slug }}</p>
                  </div>
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="indexableClass(item.indexable)">
                    {{ item.indexable ? 'index' : 'noindex' }}
                  </span>
                </div>
                <p v-if="item.description" class="mt-4 text-sm text-[#4f5b54] dark:text-[#c5cec8]">{{ item.description }}</p>
              </article>
            </section>

            <section class="space-y-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Flat labels</p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">Tags</h2>
                </div>
                <span class="rounded-md bg-[#eef5f1] px-3 py-2 text-sm text-[#36594a] dark:bg-[#18261f] dark:text-[#b6d7c8]">{{ tags.length }} tags</span>
              </div>

              <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
                <LoaderCircle class="h-4 w-4 animate-spin" />
                Loading tags
              </div>

              <div v-else-if="tags.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]">
                <h2 class="text-lg font-semibold">No tags yet</h2>
              </div>

              <div v-else class="flex flex-wrap gap-2">
                <span v-for="tag in tags" :key="tag.id" class="inline-flex items-center gap-2 rounded-md border border-[#cfd8d1] bg-white px-3 py-2 text-sm shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
                  <Tag class="h-4 w-4 text-[#3162a3]" />
                  <span>{{ tag.name }}</span>
                  <span class="font-mono text-xs text-[#667169] dark:text-[#aeb8b0]">{{ tag.slug }}</span>
                </span>
              </div>
            </section>
          </div>

          <div v-if="canManageTaxonomy" class="space-y-5">
            <form class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="createSeries">
              <div class="flex items-start gap-3">
                <PanelsTopLeft class="mt-1 h-4 w-4 text-[#3162a3]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Create</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Series</h2>
                </div>
              </div>

              <label class="block space-y-2">
                <span class="text-sm font-medium">Name</span>
                <input v-model.trim="seriesForm.name" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Slug</span>
                <input v-model.trim="seriesForm.slug" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Description</span>
                <textarea v-model.trim="seriesForm.description" class="min-h-24 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>
              <label class="flex items-center gap-2 text-sm">
                <input v-model="seriesForm.indexable" class="h-4 w-4 rounded border-[#bfcac3]" type="checkbox" />
                Indexable
              </label>
              <button
                class="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                type="submit"
                :disabled="savingSeries || !canCreateSeries"
              >
                <LoaderCircle v-if="savingSeries" class="h-4 w-4 animate-spin" />
                <Plus v-else class="h-4 w-4" />
                Create series
              </button>
            </form>

            <form class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="createTag">
              <div class="flex items-start gap-3">
                <Tag class="mt-1 h-4 w-4 text-[#3162a3]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Create</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Tag</h2>
                </div>
              </div>

              <label class="block space-y-2">
                <span class="text-sm font-medium">Name</span>
                <input v-model.trim="tagForm.name" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Slug</span>
                <input v-model.trim="tagForm.slug" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Description</span>
                <textarea v-model.trim="tagForm.description" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>
              <label class="flex items-center gap-2 text-sm">
                <input v-model="tagForm.indexable" class="h-4 w-4 rounded border-[#bfcac3]" type="checkbox" />
                Indexable
              </label>
              <button
                class="inline-flex w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-4 py-2 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                type="submit"
                :disabled="savingTag || !canCreateTag"
              >
                <LoaderCircle v-if="savingTag" class="h-4 w-4 animate-spin" />
                <Plus v-else class="h-4 w-4" />
                Create tag
              </button>
            </form>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ArrowLeft, LoaderCircle, LogOut, PanelsTopLeft, Plus, RefreshCw, Tag } from 'lucide-vue-next'

type APIEnvelope<T> = {
  data: T
}

type APIListEnvelope<T> = {
  data: T[]
  meta?: {
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

type Series = {
  id: string
  slug: string
  name: string
  description?: string
  indexable: boolean
}

type TaxonomyTerm = {
  id: string
  type: string
  slug: string
  name: string
  description?: string
  indexable: boolean
}

const route = useRoute()
const projectID = computed(() => {
  const value = route.params.projectId
  return Array.isArray(value) ? String(value[0] || '') : String(value || '')
})

const project = ref<AdminProject | null>(null)
const seriesItems = ref<Series[]>([])
const tags = ref<TaxonomyTerm[]>([])
const pending = ref(true)
const savingSeries = ref(false)
const savingTag = ref(false)
const errorMessage = ref('')
const successMessage = ref('')

const seriesForm = reactive({
  name: '',
  slug: '',
  description: '',
  indexable: true
})

const tagForm = reactive({
  name: '',
  slug: '',
  description: '',
  indexable: true
})

const canManageProject = computed(() => project.value?.role === 'project_owner' || project.value?.role === 'project_admin')
const canManageTaxonomy = computed(() => canManageProject.value || project.value?.role === 'editor')
const canCreateSeries = computed(() => Boolean(seriesForm.name.trim() && seriesForm.slug.trim()))
const canCreateTag = computed(() => Boolean(tagForm.name.trim() && tagForm.slug.trim()))

watch(() => seriesForm.name, (value) => {
  if (!seriesForm.slug) seriesForm.slug = slugify(value)
})

watch(() => tagForm.name, (value) => {
  if (!tagForm.slug) tagForm.slug = slugify(value)
})

onMounted(refresh)

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const [projectResponse, seriesResponse, tagResponse] = await Promise.all([
      $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}`, { credentials: 'include' }),
      $fetch<APIListEnvelope<Series>>(`/api/v1/projects/${projectID.value}/series`, {
        credentials: 'include',
        query: { limit: 100 }
      }),
      $fetch<APIListEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID.value}/tags`, {
        credentials: 'include',
        query: { limit: 100 }
      })
    ])
    project.value = projectResponse.data
    seriesItems.value = sortByName(apiListData(seriesResponse))
    tags.value = sortByName(apiListData(tagResponse))
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load series and tags. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function createSeries() {
  savingSeries.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<Series>>(`/api/v1/projects/${projectID.value}/series`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        name: seriesForm.name,
        slug: seriesForm.slug,
        description: seriesForm.description,
        indexable: seriesForm.indexable
      }
    })
    seriesItems.value = sortByName([...seriesItems.value, response.data])
    seriesForm.name = ''
    seriesForm.slug = ''
    seriesForm.description = ''
    seriesForm.indexable = true
    successMessage.value = 'Series created.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create series.')
  } finally {
    savingSeries.value = false
  }
}

async function createTag() {
  savingTag.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID.value}/tags`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        name: tagForm.name,
        slug: tagForm.slug,
        description: tagForm.description,
        indexable: tagForm.indexable
      }
    })
    tags.value = sortByName([...tags.value, response.data])
    tagForm.name = ''
    tagForm.slug = ''
    tagForm.description = ''
    tagForm.indexable = true
    successMessage.value = 'Tag created.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create tag.')
  } finally {
    savingTag.value = false
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

function sortByName<T extends { name: string, id: string }>(values: T[]) {
  return [...values].sort((left, right) => {
    const nameOrder = left.name.localeCompare(right.name)
    if (nameOrder !== 0) return nameOrder
    return left.id.localeCompare(right.id)
  })
}

function indexableClass(indexable: boolean) {
  return indexable
    ? 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    : 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
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
  if (error instanceof Error && error.message) return error.message
  return fallback
}
</script>

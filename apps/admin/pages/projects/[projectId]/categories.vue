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
        <NuxtLink v-if="canManageProject" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/members`">Members</NuxtLink>
        <NuxtLink class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/api-keys`">API keys</NuxtLink>
        <NuxtLink v-if="canManageProject" class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/audit-events`">Audit</NuxtLink>
      </aside>

      <div class="space-y-5">
        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>
        <p v-if="successMessage" class="rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-4 py-3 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]">
          {{ successMessage }}
        </p>

        <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
          <div class="space-y-4">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Taxonomy</p>
                <h2 class="mt-1 text-xl font-semibold tracking-normal">Category tree</h2>
              </div>
              <button
                v-if="canManageTaxonomy"
                class="inline-flex items-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                type="button"
                @click="resetForm"
              >
                <Plus class="h-4 w-4" />
                New category
              </button>
            </div>

            <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
              <LoaderCircle class="h-4 w-4 animate-spin" />
              Loading categories
            </div>

            <div v-else-if="categories.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]">
              <h2 class="text-lg font-semibold">No categories yet</h2>
              <p class="mt-2 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Create one before drafting articles.</p>
            </div>

            <article
              v-for="row in categoryRows"
              :key="row.category.id"
              class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]"
              :style="rowStyle(row)"
            >
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div class="min-w-0">
                  <h3 class="truncate text-lg font-semibold">{{ row.category.name }}</h3>
                  <p class="mt-1 truncate text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ row.path }}</p>
                </div>
                <div class="flex items-center gap-2">
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="row.category.indexable ? 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]' : 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'">
                    {{ row.category.indexable ? 'index' : 'noindex' }}
                  </span>
                  <button
                    v-if="canManageTaxonomy"
                    class="inline-flex h-9 w-9 items-center justify-center rounded-md border border-[#c9d4cc] hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                    type="button"
                    title="Edit category"
                    aria-label="Edit category"
                    @click="startEdit(row.category)"
                  >
                    <Pencil class="h-4 w-4" />
                  </button>
                </div>
              </div>
              <p v-if="row.category.description" class="mt-4 text-sm text-[#4f5b54] dark:text-[#c5cec8]">
                {{ row.category.description }}
              </p>
              <dl class="mt-5 grid gap-3 text-sm sm:grid-cols-2">
                <div>
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Slug</dt>
                  <dd class="truncate">{{ row.category.slug }}</dd>
                </div>
                <div>
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Parent</dt>
                  <dd class="truncate">{{ parentName(row.category.parentId) }}</dd>
                </div>
              </dl>
            </article>
          </div>

          <form
            v-if="canManageTaxonomy"
            class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]"
            @submit.prevent="saveCategory"
          >
            <div class="flex items-start gap-3">
              <FolderTree class="mt-1 h-4 w-4 text-[#3162a3]" />
              <div>
                <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">{{ editingCategoryID ? 'Edit' : 'Create' }}</p>
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
              <span class="text-sm font-medium">Parent category</span>
              <select v-model="form.parentId" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]">
                <option value="">Root category</option>
                <option v-for="option in parentOptions" :key="option.id" :value="option.id">
                  {{ option.label }}
                </option>
              </select>
            </label>
            <label class="block space-y-2">
              <span class="text-sm font-medium">Description</span>
              <textarea v-model.trim="form.description" class="min-h-24 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
            </label>
            <label class="flex items-center gap-2 text-sm">
              <input v-model="form.indexable" class="h-4 w-4 rounded border-[#bfcac3]" type="checkbox" />
              Indexable
            </label>
            <div class="flex flex-wrap gap-2">
              <button
                class="inline-flex flex-1 items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                type="submit"
                :disabled="saving || !canSave"
              >
                <LoaderCircle v-if="saving" class="h-4 w-4 animate-spin" />
                <Check v-else-if="editingCategoryID" class="h-4 w-4" />
                <Plus v-else class="h-4 w-4" />
                {{ editingCategoryID ? 'Save category' : 'Create category' }}
              </button>
              <button
                v-if="editingCategoryID"
                class="inline-flex items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-4 py-2 text-sm font-medium hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                type="button"
                @click="resetForm"
              >
                <X class="h-4 w-4" />
                Cancel
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ArrowLeft, Check, FolderTree, LoaderCircle, LogOut, Pencil, Plus, RefreshCw, X } from 'lucide-vue-next'

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

type TaxonomyTerm = {
  id: string
  type: string
  slug: string
  name: string
  description?: string
  parentId?: string
  ancestors?: TaxonomyTerm[]
  children?: TaxonomyTerm[]
  indexable: boolean
}

type CategoryRow = {
  category: TaxonomyTerm
  depth: number
  path: string
}

const route = useRoute()
const projectID = computed(() => {
  const value = route.params.projectId
  return Array.isArray(value) ? value[0] : String(value || '')
})

const project = ref<AdminProject | null>(null)
const categories = ref<TaxonomyTerm[]>([])
const pending = ref(true)
const saving = ref(false)
const editingCategoryID = ref('')
const errorMessage = ref('')
const successMessage = ref('')

const form = reactive({
  name: '',
  slug: '',
  parentId: '',
  description: '',
  indexable: true
})

const canManageProject = computed(() => project.value?.role === 'project_owner' || project.value?.role === 'project_admin')
const canManageTaxonomy = computed(() => canManageProject.value || project.value?.role === 'editor')
const canSave = computed(() => Boolean(form.name.trim() && form.slug.trim()))
const categoriesByID = computed(() => new Map(categories.value.map(category => [category.id, category])))
const categoryRows = computed(() => buildCategoryRows(categories.value))
const parentOptions = computed(() => {
  return categoryRows.value
    .filter((row) => {
      if (!editingCategoryID.value) return true
      return row.category.id !== editingCategoryID.value && !isDescendantOf(row.category.id, editingCategoryID.value)
    })
    .map(row => ({ id: row.category.id, label: row.path }))
})

watch(() => form.name, (value) => {
  if (!editingCategoryID.value && !form.slug) form.slug = slugify(value)
})

onMounted(refresh)

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const [projectResponse, categoryResponse] = await Promise.all([
      $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}`, { credentials: 'include' }),
      fetchAllCategories()
    ])
    project.value = projectResponse.data
    categories.value = sortCategories(categoryResponse)
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load categories. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function fetchAllCategories() {
  const allCategories = new Map<string, TaxonomyTerm>()
  const seenCursors = new Set<string>()
  let cursor = ''

  do {
    const response = await $fetch<APIListEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID.value}/categories`, {
      credentials: 'include',
      query: {
        limit: 100,
        ...(cursor ? { cursor } : {})
      }
    })
    for (const category of response.data) allCategories.set(category.id, category)

    const nextCursor = response.meta?.nextCursor || ''
    if (nextCursor && seenCursors.has(nextCursor)) throw new Error('Category pagination returned a repeated cursor')
    if (nextCursor) seenCursors.add(nextCursor)
    cursor = nextCursor
  } while (cursor)

  return [...allCategories.values()]
}

function startEdit(category: TaxonomyTerm) {
  editingCategoryID.value = category.id
  form.name = category.name
  form.slug = category.slug
  form.parentId = category.parentId || ''
  form.description = category.description || ''
  form.indexable = category.indexable
  clearMessages()
}

function resetForm() {
  editingCategoryID.value = ''
  form.name = ''
  form.slug = ''
  form.parentId = ''
  form.description = ''
  form.indexable = true
  clearMessages()
}

async function saveCategory() {
  saving.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const body = categoryBody()
    if (editingCategoryID.value) {
      const response = await $fetch<APIEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID.value}/categories/${editingCategoryID.value}`, {
        method: 'PATCH',
        credentials: 'include',
        headers: { 'X-CSRF-Token': csrfToken },
        body
      })
      categories.value = sortCategories(categories.value.map(category => category.id === response.data.id ? response.data : category))
      resetForm()
      successMessage.value = 'Category updated.'
    } else {
      const response = await $fetch<APIEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID.value}/categories`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'X-CSRF-Token': csrfToken },
        body
      })
      categories.value = sortCategories([...categories.value, response.data])
      resetForm()
      successMessage.value = 'Category created.'
    }
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, editingCategoryID.value ? 'Could not update category.' : 'Could not create category.')
  } finally {
    saving.value = false
  }
}

function categoryBody() {
  return {
    name: form.name,
    slug: form.slug,
    parentId: form.parentId,
    description: form.description,
    indexable: form.indexable
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

function buildCategoryRows(values: TaxonomyTerm[]) {
  const byID = new Map(values.map(category => [category.id, category]))
  const byParent = new Map<string, TaxonomyTerm[]>()
  for (const category of values) {
    const parentKey = category.parentId && byID.has(category.parentId) ? category.parentId : ''
    byParent.set(parentKey, [...(byParent.get(parentKey) || []), category])
  }
  const sort = (items: TaxonomyTerm[]) => [...items].sort(compareCategories)
  const rows: CategoryRow[] = []
  const visit = (category: TaxonomyTerm, depth: number, ancestors: string[]) => {
    const path = [...ancestors, category.name].join(' / ')
    rows.push({ category, depth, path })
    for (const child of sort(byParent.get(category.id) || [])) {
      visit(child, depth + 1, [...ancestors, category.name])
    }
  }
  for (const root of sort(byParent.get('') || [])) {
    visit(root, 0, [])
  }
  return rows
}

function isDescendantOf(categoryID: string, ancestorID: string) {
  let current = categoriesByID.value.get(categoryID)
  const seen = new Set<string>()
  while (current?.parentId && !seen.has(current.parentId)) {
    if (current.parentId === ancestorID) return true
    seen.add(current.parentId)
    current = categoriesByID.value.get(current.parentId)
  }
  return false
}

function rowStyle(row: CategoryRow) {
  if (row.depth <= 0) return {}
  return { 'margin-left': `${Math.min(row.depth, 2) * 1.25}rem` }
}

function parentName(parentID?: string) {
  if (!parentID) return 'Root'
  return categoriesByID.value.get(parentID)?.name || 'Unknown'
}

function sortCategories(values: TaxonomyTerm[]) {
  return [...values].sort(compareCategories)
}

function compareCategories(left: TaxonomyTerm, right: TaxonomyTerm) {
  const nameOrder = left.name.localeCompare(right.name)
  if (nameOrder !== 0) return nameOrder
  return left.id.localeCompare(right.id)
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

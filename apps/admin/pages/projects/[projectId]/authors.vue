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
            <h1 class="mt-1 text-2xl font-semibold tracking-normal">Authors</h1>
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
        <NuxtLink class="block rounded-md bg-white px-3 py-2 text-sm shadow-sm dark:bg-[#252b28]" :to="`/projects/${projectID}/authors`">Authors</NuxtLink>
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
                <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Profiles</p>
                <h2 class="mt-1 text-xl font-semibold tracking-normal">Public bylines</h2>
              </div>
              <button
                v-if="canManageAuthors"
                class="inline-flex items-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                type="button"
                @click="resetForm"
              >
                <Plus class="h-4 w-4" />
                New author
              </button>
            </div>

            <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
              <LoaderCircle class="h-4 w-4 animate-spin" />
              Loading authors
            </div>

            <div v-else-if="authors.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]">
              <h2 class="text-lg font-semibold">No authors yet</h2>
              <p class="mt-2 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Create public profiles before assigning bylines.</p>
            </div>

            <article v-for="author in authors" :key="author.id" class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div class="min-w-0">
                  <h3 class="truncate text-lg font-semibold">{{ author.displayName }}</h3>
                  <p class="mt-1 truncate text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ author.slug }}</p>
                </div>
                <div class="flex items-center gap-2">
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(author.status)">
                    {{ author.status }}
                  </span>
                  <button
                    v-if="canManageAuthors"
                    class="inline-flex h-9 w-9 items-center justify-center rounded-md border border-[#c9d4cc] hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                    type="button"
                    title="Edit author"
                    aria-label="Edit author"
                    @click="startEdit(author)"
                  >
                    <Pencil class="h-4 w-4" />
                  </button>
                </div>
              </div>

              <p v-if="author.shortBio" class="mt-4 text-sm text-[#4f5b54] dark:text-[#c5cec8]">
                {{ author.shortBio }}
              </p>

              <dl class="mt-5 grid gap-3 text-sm md:grid-cols-3">
                <div class="flex items-center gap-2">
                  <Briefcase class="h-4 w-4 text-[#3162a3]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Title</dt>
                    <dd class="truncate">{{ author.jobTitle || 'Not set' }}</dd>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <BadgeCheck class="h-4 w-4 text-[#165a4a]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Organization</dt>
                    <dd class="truncate">{{ author.organization || 'Not set' }}</dd>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <Link2 class="h-4 w-4 text-[#8a5b00]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Profile</dt>
                    <dd class="truncate">{{ author.profileUrl || 'Not set' }}</dd>
                  </div>
                </div>
              </dl>

              <div v-if="author.credentials?.length || author.expertise?.length" class="mt-5 flex flex-wrap gap-2">
                <span v-for="credential in author.credentials" :key="`credential-${author.id}-${credential}`" class="rounded-full bg-[#e8f0ff] px-2.5 py-1 text-xs font-medium text-[#245b99] dark:bg-[#152944] dark:text-[#b8d5ff]">
                  {{ credential }}
                </span>
                <span v-for="item in author.expertise" :key="`expertise-${author.id}-${item}`" class="rounded-full bg-[#eef2ef] px-2.5 py-1 text-xs font-medium text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]">
                  {{ item }}
                </span>
              </div>
            </article>

            <button
              v-if="nextCursor"
              class="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] bg-white px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:bg-[#202522] dark:hover:bg-[#2a302d]"
              type="button"
              :disabled="loadingMore"
              @click="loadMoreAuthors"
            >
              <LoaderCircle v-if="loadingMore" class="h-4 w-4 animate-spin" />
              <RefreshCw v-else class="h-4 w-4" />
              Load more authors
            </button>
          </div>

          <form
            v-if="canManageAuthors"
            class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]"
            @submit.prevent="saveAuthor"
          >
            <div class="flex items-start gap-3">
              <UserRound class="mt-1 h-4 w-4 text-[#3162a3]" />
              <div>
                <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">{{ editingAuthorID ? 'Edit' : 'Create' }}</p>
                <h2 class="mt-1 text-lg font-semibold tracking-normal">Author profile</h2>
              </div>
            </div>

            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block space-y-2">
                <span class="text-sm font-medium">Name</span>
                <input v-model.trim="form.displayName" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Slug</span>
                <input v-model.trim="form.slug" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
            </div>

            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block space-y-2">
                <span class="text-sm font-medium">Status</span>
                <select v-model="form.status" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]">
                  <option value="active">Active</option>
                  <option value="inactive">Inactive</option>
                </select>
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Profile URL</span>
                <input v-model.trim="form.profileUrl" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" type="url" />
              </label>
            </div>

            <label class="block space-y-2">
              <span class="text-sm font-medium">Photo asset ID</span>
              <input v-model.trim="form.photoAssetId" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" autocomplete="off" />
            </label>

            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block space-y-2">
                <span class="text-sm font-medium">Job title</span>
                <input v-model.trim="form.jobTitle" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Organization</span>
                <input v-model.trim="form.organization" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>
            </div>

            <label class="block space-y-2">
              <span class="text-sm font-medium">Short bio</span>
              <textarea v-model.trim="form.shortBio" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
            </label>
            <label class="block space-y-2">
              <span class="text-sm font-medium">Full bio</span>
              <textarea v-model.trim="form.fullBio" class="min-h-28 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
            </label>

            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block space-y-2">
                <span class="text-sm font-medium">Credentials</span>
                <input v-model.trim="form.credentials" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Expertise</span>
                <input v-model.trim="form.expertise" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>
            </div>

            <label class="block space-y-2">
              <span class="text-sm font-medium">External profiles</span>
              <textarea v-model.trim="form.externalProfiles" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
            </label>
            <label class="block space-y-2">
              <span class="text-sm font-medium">SameAs links</span>
              <textarea v-model.trim="form.sameAs" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
            </label>

            <div class="flex flex-wrap gap-2">
              <button
                class="inline-flex flex-1 items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                type="submit"
                :disabled="saving || !canSave"
              >
                <LoaderCircle v-if="saving" class="h-4 w-4 animate-spin" />
                <Check v-else class="h-4 w-4" />
                {{ editingAuthorID ? 'Save author' : 'Create author' }}
              </button>
              <button
                v-if="editingAuthorID"
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
import { ArrowLeft, BadgeCheck, Briefcase, Check, Link2, LoaderCircle, LogOut, Pencil, Plus, RefreshCw, UserRound, X } from 'lucide-vue-next'

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

type Author = {
  id: string
  slug: string
  displayName: string
  shortBio?: string
  fullBio?: string
  photoAssetId?: string
  jobTitle?: string
  organization?: string
  credentials?: string[]
  expertise?: string[]
  profileUrl?: string
  externalProfiles?: string[]
  sameAs?: string[]
  status: string
  createdAt?: string
  updatedAt?: string
}

const route = useRoute()
const projectID = computed(() => {
  const value = route.params.projectId
  return Array.isArray(value) ? value[0] : String(value || '')
})

const project = ref<AdminProject | null>(null)
const authors = ref<Author[]>([])
const pending = ref(true)
const loadingMore = ref(false)
const saving = ref(false)
const editingAuthorID = ref('')
const nextCursor = ref('')
const errorMessage = ref('')
const successMessage = ref('')

const form = reactive({
  displayName: '',
  slug: '',
  status: 'active',
  shortBio: '',
  fullBio: '',
  photoAssetId: '',
  jobTitle: '',
  organization: '',
  profileUrl: '',
  credentials: '',
  expertise: '',
  externalProfiles: '',
  sameAs: ''
})

const canManageProject = computed(() => project.value?.role === 'project_owner' || project.value?.role === 'project_admin')
const canManageAuthors = computed(() => canManageProject.value || project.value?.role === 'editor')
const canSave = computed(() => Boolean(form.displayName.trim() && form.slug.trim()))

watch(() => form.displayName, (value) => {
  if (!editingAuthorID.value && !form.slug) form.slug = slugify(value)
})

onMounted(refresh)

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const [projectResponse, authorResponse] = await Promise.all([
      $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}`, { credentials: 'include' }),
      $fetch<APIListEnvelope<Author>>(`/api/v1/projects/${projectID.value}/authors`, {
        credentials: 'include',
        query: { limit: 100 }
      })
    ])
    project.value = projectResponse.data
    authors.value = sortAuthors(authorResponse.data)
    nextCursor.value = authorResponse.meta?.nextCursor || ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load authors. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function loadMoreAuthors() {
  if (!nextCursor.value || loadingMore.value) return
  loadingMore.value = true
  clearMessages()
  try {
    const response = await $fetch<APIListEnvelope<Author>>(`/api/v1/projects/${projectID.value}/authors`, {
      credentials: 'include',
      query: {
        limit: 100,
        cursor: nextCursor.value
      }
    })
    const merged = new Map(authors.value.map(author => [author.id, author]))
    for (const author of response.data) merged.set(author.id, author)
    authors.value = sortAuthors([...merged.values()])
    nextCursor.value = response.meta?.nextCursor || ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load more authors.')
  } finally {
    loadingMore.value = false
  }
}

function startEdit(author: Author) {
  editingAuthorID.value = author.id
  form.displayName = author.displayName
  form.slug = author.slug
  form.status = author.status || 'active'
  form.shortBio = author.shortBio || ''
  form.fullBio = author.fullBio || ''
  form.photoAssetId = author.photoAssetId || ''
  form.jobTitle = author.jobTitle || ''
  form.organization = author.organization || ''
  form.profileUrl = author.profileUrl || ''
  form.credentials = (author.credentials || []).join(', ')
  form.expertise = (author.expertise || []).join(', ')
  form.externalProfiles = (author.externalProfiles || []).join('\n')
  form.sameAs = (author.sameAs || []).join('\n')
  clearMessages()
}

function resetForm() {
  editingAuthorID.value = ''
  form.displayName = ''
  form.slug = ''
  form.status = 'active'
  form.shortBio = ''
  form.fullBio = ''
  form.photoAssetId = ''
  form.jobTitle = ''
  form.organization = ''
  form.profileUrl = ''
  form.credentials = ''
  form.expertise = ''
  form.externalProfiles = ''
  form.sameAs = ''
  clearMessages()
}

async function saveAuthor() {
  saving.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const body = authorBody()
    if (editingAuthorID.value) {
      const response = await $fetch<APIEnvelope<Author>>(`/api/v1/projects/${projectID.value}/authors/${editingAuthorID.value}`, {
        method: 'PATCH',
        credentials: 'include',
        headers: { 'X-CSRF-Token': csrfToken },
        body
      })
      authors.value = sortAuthors(authors.value.map(author => author.id === response.data.id ? response.data : author))
      resetForm()
      successMessage.value = 'Author updated.'
    } else {
      const response = await $fetch<APIEnvelope<Author>>(`/api/v1/projects/${projectID.value}/authors`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'X-CSRF-Token': csrfToken },
        body
      })
      authors.value = sortAuthors([...authors.value, response.data])
      resetForm()
      successMessage.value = 'Author created.'
    }
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, editingAuthorID.value ? 'Could not update author.' : 'Could not create author.')
  } finally {
    saving.value = false
  }
}

function authorBody() {
  return {
    displayName: form.displayName,
    slug: form.slug,
    status: form.status,
    shortBio: form.shortBio,
    fullBio: form.fullBio,
    photoAssetId: form.photoAssetId,
    jobTitle: form.jobTitle,
    organization: form.organization,
    profileUrl: form.profileUrl,
    credentials: splitCSV(form.credentials),
    expertise: splitCSV(form.expertise),
    externalProfiles: splitLines(form.externalProfiles),
    sameAs: splitLines(form.sameAs)
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

function splitCSV(value: string) {
  return cleanList(value.split(','))
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

function sortAuthors(values: Author[]) {
  return [...values].sort((left, right) => left.displayName.localeCompare(right.displayName))
}

function statusClass(status: string) {
  if (status === 'active') {
    return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
  }
  return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
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

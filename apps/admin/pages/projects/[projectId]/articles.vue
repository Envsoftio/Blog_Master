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
            <h1 class="mt-1 text-2xl font-semibold tracking-normal">Articles</h1>
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
        <NuxtLink class="block rounded-md bg-white px-3 py-2 text-sm shadow-sm dark:bg-[#252b28]" :to="`/projects/${projectID}/articles`">Articles</NuxtLink>
        <NuxtLink class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/categories`">Categories</NuxtLink>
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

        <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
          <div class="space-y-4">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Workflow</p>
                <h2 class="mt-1 text-xl font-semibold tracking-normal">Draft, approve, schedule</h2>
              </div>
              <button
                class="inline-flex items-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                type="button"
                :disabled="categories.length === 0"
                @click="articleFormOpen = !articleFormOpen"
              >
                <Plus class="h-4 w-4" />
                New article
              </button>
            </div>

            <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
              <LoaderCircle class="h-4 w-4 animate-spin" />
              Loading articles
            </div>

            <div v-else-if="articles.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]">
              <h2 class="text-lg font-semibold">No articles yet</h2>
              <p class="mt-2 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Create a category, then draft the first article for this project.</p>
            </div>

            <article v-for="article in articles" :key="article.id" class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div class="min-w-0">
                  <h3 class="truncate text-lg font-semibold">{{ article.title }}</h3>
                  <p class="mt-1 truncate text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ article.slug }}</p>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="editorialClass(article.editorialState)">
                    {{ labelize(article.editorialState) }}
                  </span>
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="publicationClass(article.publicationState)">
                    {{ labelize(article.publicationState) }}
                  </span>
                </div>
              </div>

              <p v-if="article.latestRevision?.excerpt" class="mt-4 text-sm text-[#4f5b54] dark:text-[#c5cec8]">
                {{ article.latestRevision.excerpt }}
              </p>

              <dl class="mt-5 grid gap-3 text-sm md:grid-cols-3">
                <div class="flex items-center gap-2">
                  <FileText class="h-4 w-4 text-[#3162a3]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Revision</dt>
                    <dd class="truncate">{{ article.latestRevision ? `#${article.latestRevision.revisionNumber}` : 'None' }}</dd>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <CalendarClock class="h-4 w-4 text-[#8a5b00]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Scheduled</dt>
                    <dd class="truncate">{{ formatDate(article.scheduledForUtc) }}</dd>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <UploadCloud class="h-4 w-4 text-[#165a4a]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Published</dt>
                    <dd class="truncate">{{ formatDate(article.publishedAt) }}</dd>
                  </div>
                </div>
              </dl>

              <div v-if="article.canonicalUrl" class="mt-4 truncate rounded-md bg-[#f2f5f3] px-3 py-2 text-sm text-[#4f5b54] dark:bg-[#171b18] dark:text-[#c5cec8]">
                {{ article.canonicalUrl }}
              </div>

              <div class="mt-5 grid gap-3 lg:grid-cols-[1fr_auto]">
                <div class="flex flex-wrap items-center gap-2">
                  <button
                    v-if="article.editorialState === 'draft' || article.editorialState === 'changes_requested'"
                    class="inline-flex items-center gap-2 rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                    type="button"
                    :disabled="Boolean(actionPending[article.id])"
                    @click="submitRevision(article)"
                  >
                    <Send class="h-4 w-4" />
                    Submit
                  </button>
                  <button
                    v-if="article.editorialState !== 'approved'"
                    class="inline-flex items-center gap-2 rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                    type="button"
                    :disabled="Boolean(actionPending[article.id])"
                    @click="approveRevision(article)"
                  >
                    <CheckCircle2 class="h-4 w-4" />
                    Approve
                  </button>
                  <button
                    class="inline-flex items-center gap-2 rounded-md bg-[#165a4a] px-3 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                    type="button"
                    :disabled="Boolean(actionPending[article.id]) || article.editorialState !== 'approved'"
                    @click="publishArticle(article)"
                  >
                    <UploadCloud class="h-4 w-4" />
                    Publish
                  </button>
                  <button
                    v-if="article.publicationState !== 'unpublished'"
                    class="inline-flex items-center gap-2 rounded-md border border-[#d9b7aa] px-3 py-2 text-sm font-medium text-[#9b2d23] hover:bg-[#fff4f2] disabled:opacity-60 dark:border-[#6d352f] dark:text-[#ffc4bd] dark:hover:bg-[#2a1c1a]"
                    type="button"
                    :disabled="Boolean(actionPending[article.id])"
                    @click="unpublishArticle(article)"
                  >
                    <XCircle class="h-4 w-4" />
                    Unpublish
                  </button>
                </div>

                <form class="grid gap-2 sm:grid-cols-[minmax(190px,1fr)_auto]" @submit.prevent="scheduleArticle(article)">
                  <input
                    v-model="scheduleDrafts[article.id]"
                    class="h-10 rounded-md border border-[#bfcac3] px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#171b18]"
                    type="datetime-local"
                    :disabled="Boolean(actionPending[article.id]) || article.editorialState !== 'approved'"
                    required
                  />
                  <button
                    class="inline-flex items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                    type="submit"
                    :disabled="Boolean(actionPending[article.id]) || article.editorialState !== 'approved' || !scheduleDrafts[article.id]"
                  >
                    <CalendarClock class="h-4 w-4" />
                    Schedule
                  </button>
                </form>
              </div>
            </article>
          </div>

          <div class="space-y-5">
            <form
              v-if="articleFormOpen"
              class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]"
              @submit.prevent="createArticle"
            >
              <div>
                <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Create</p>
                <h2 class="mt-1 text-lg font-semibold tracking-normal">Article</h2>
              </div>

              <label class="block space-y-2">
                <span class="text-sm font-medium">Title</span>
                <input v-model.trim="articleForm.title" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Slug</span>
                <input v-model.trim="articleForm.slug" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Type</span>
                <select v-model="articleForm.articleType" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]">
                  <option v-for="type in articleTypes" :key="type" :value="type">{{ labelize(type) }}</option>
                </select>
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Primary category</span>
                <select v-model="articleForm.primaryCategoryId" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required>
                  <option value="" disabled>Select category</option>
                  <option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }}</option>
                </select>
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Excerpt</span>
                <textarea v-model.trim="articleForm.excerpt" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">HTML</span>
                <textarea v-model.trim="articleForm.html" class="min-h-40 w-full rounded-md border border-[#bfcac3] px-3 py-2 font-mono text-sm dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>

              <button
                class="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                type="submit"
                :disabled="creatingArticle || !canCreateArticle"
              >
                <LoaderCircle v-if="creatingArticle" class="h-4 w-4 animate-spin" />
                <Plus v-else class="h-4 w-4" />
                Create article
              </button>
            </form>

            <form
              class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]"
              @submit.prevent="createCategory"
            >
              <div class="flex items-start gap-3">
                <FolderTree class="mt-1 h-4 w-4 text-[#3162a3]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Taxonomy</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Category</h2>
                </div>
              </div>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Name</span>
                <input v-model.trim="categoryForm.name" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Slug</span>
                <input v-model.trim="categoryForm.slug" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Description</span>
                <textarea v-model.trim="categoryForm.description" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" />
              </label>
              <label class="flex items-center gap-2 text-sm">
                <input v-model="categoryForm.indexable" class="h-4 w-4 rounded border-[#bfcac3]" type="checkbox" />
                Indexable
              </label>
              <button
                class="inline-flex w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-4 py-2 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                type="submit"
                :disabled="creatingCategory || !canCreateCategory"
              >
                <LoaderCircle v-if="creatingCategory" class="h-4 w-4 animate-spin" />
                <Plus v-else class="h-4 w-4" />
                Create category
              </button>
            </form>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import {
  ArrowLeft,
  CalendarClock,
  CheckCircle2,
  FileText,
  FolderTree,
  LoaderCircle,
  LogOut,
  Plus,
  RefreshCw,
  Send,
  UploadCloud,
  XCircle
} from 'lucide-vue-next'

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
  primaryDomain?: string
  blogBasePath: string
  defaultLocale: string
}

type AdminRevision = {
  id: string
  articleId: string
  revisionNumber: number
  title: string
  excerpt?: string
  locale: string
  editorialState: string
}

type AdminArticle = {
  id: string
  projectId: string
  articleType: string
  slug: string
  locale: string
  title: string
  editorialState: string
  publicationState: string
  scheduledForUtc?: string
  publishedAt?: string
  canonicalUrl?: string
  latestRevision?: AdminRevision
  createdAt: string
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

const articleTypes = [
  'standard',
  'guide',
  'tutorial',
  'comparison',
  'case_study',
  'research',
  'listicle',
  'news_update',
  'opinion',
  'reference',
  'glossary',
  'release_note'
]

const project = ref<AdminProject | null>(null)
const articles = ref<AdminArticle[]>([])
const categories = ref<TaxonomyTerm[]>([])
const pending = ref(true)
const creatingArticle = ref(false)
const creatingCategory = ref(false)
const articleFormOpen = ref(true)
const errorMessage = ref('')
const successMessage = ref('')
const actionPending = reactive<Record<string, string>>({})
const scheduleDrafts = reactive<Record<string, string>>({})

const articleForm = reactive({
  articleType: 'standard',
  title: '',
  slug: '',
  primaryCategoryId: '',
  excerpt: '',
  html: ''
})

const categoryForm = reactive({
  name: '',
  slug: '',
  description: '',
  indexable: true
})

const canCreateArticle = computed(() => Boolean(
  articleForm.title.trim() &&
  articleForm.slug.trim() &&
  articleForm.primaryCategoryId
))
const canCreateCategory = computed(() => Boolean(categoryForm.name.trim() && categoryForm.slug.trim()))

watch(() => articleForm.title, (value) => {
  if (!articleForm.slug) articleForm.slug = slugify(value)
})

watch(() => categoryForm.name, (value) => {
  if (!categoryForm.slug) categoryForm.slug = slugify(value)
})

onMounted(refresh)

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const [projectResponse, categoryResponse, articleResponse] = await Promise.all([
      $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}`, { credentials: 'include' }),
      $fetch<APIListEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID.value}/categories`, { credentials: 'include' }),
      $fetch<APIListEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles`, { credentials: 'include' })
    ])
    project.value = projectResponse.data
    categories.value = categoryResponse.data
    articles.value = articleResponse.data
    if (!articleForm.primaryCategoryId && categories.value[0]) {
      articleForm.primaryCategoryId = categories.value[0].id
    }
    seedScheduleDrafts()
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load this project. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function createCategory() {
  creatingCategory.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID.value}/categories`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        name: categoryForm.name,
        slug: categoryForm.slug,
        description: categoryForm.description,
        indexable: categoryForm.indexable
      }
    })
    categories.value = [...categories.value, response.data].sort((left, right) => left.name.localeCompare(right.name))
    if (!articleForm.primaryCategoryId) {
      articleForm.primaryCategoryId = response.data.id
    }
    categoryForm.name = ''
    categoryForm.slug = ''
    categoryForm.description = ''
    categoryForm.indexable = true
    successMessage.value = 'Category created.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create category.')
  } finally {
    creatingCategory.value = false
  }
}

async function createArticle() {
  creatingArticle.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        articleType: articleForm.articleType,
        title: articleForm.title,
        slug: articleForm.slug,
        primaryCategoryId: articleForm.primaryCategoryId,
        excerpt: articleForm.excerpt,
        html: articleForm.html || `<p>${escapeHTML(articleForm.title)}</p>`
      }
    })
    articles.value = [response.data, ...articles.value]
    scheduleDrafts[response.data.id] = toLocalInputValue(new Date(Date.now() + 15 * 60 * 1000))
    articleForm.title = ''
    articleForm.slug = ''
    articleForm.excerpt = ''
    articleForm.html = ''
    articleFormOpen.value = false
    successMessage.value = 'Article created.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create article.')
  } finally {
    creatingArticle.value = false
  }
}

async function submitRevision(article: AdminArticle) {
  await mutateArticle(article, 'submit', async (csrfToken) => {
    await $fetch<APIEnvelope<AdminRevision>>(`/api/v1/projects/${projectID.value}/revisions/${latestRevisionID(article)}/submit`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {}
    })
    successMessage.value = 'Revision submitted.'
  })
}

async function approveRevision(article: AdminArticle) {
  await mutateArticle(article, 'approve', async (csrfToken) => {
    await $fetch<APIEnvelope<AdminRevision>>(`/api/v1/projects/${projectID.value}/revisions/${latestRevisionID(article)}/approve`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {}
    })
    successMessage.value = 'Revision approved.'
  })
}

async function publishArticle(article: AdminArticle) {
  await mutateArticle(article, 'publish', async (csrfToken) => {
    await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${article.id}/publish`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: publicationBody(article)
    })
    successMessage.value = 'Article published.'
  })
}

async function scheduleArticle(article: AdminArticle) {
  await mutateArticle(article, 'schedule', async (csrfToken) => {
    const scheduledAt = scheduleDrafts[article.id]
    if (!scheduledAt) {
      throw new Error('Scheduled time is required.')
    }
    const scheduledForUtc = new Date(scheduledAt).toISOString()
    await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${article.id}/schedule`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        ...publicationBody(article),
        scheduledForUtc
      }
    })
    successMessage.value = 'Article scheduled.'
  })
}

async function unpublishArticle(article: AdminArticle) {
  await mutateArticle(article, 'unpublish', async (csrfToken) => {
    await $fetch<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles/${article.id}/unpublish`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {}
    })
    successMessage.value = 'Article unpublished.'
  })
}

async function mutateArticle(article: AdminArticle, action: string, operation: (csrfToken: string) => Promise<void>) {
  if (!article.latestRevision) {
    errorMessage.value = 'This article has no revision.'
    return
  }
  actionPending[article.id] = action
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    await operation(csrfToken)
    await fetchArticles()
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, `Could not ${action} article.`)
  } finally {
    delete actionPending[article.id]
  }
}

async function fetchArticles() {
  const response = await $fetch<APIListEnvelope<AdminArticle>>(`/api/v1/projects/${projectID.value}/articles`, {
    credentials: 'include'
  })
  articles.value = response.data
  seedScheduleDrafts()
}

function publicationBody(article: AdminArticle) {
  return {
    revisionId: latestRevisionID(article),
    slug: article.slug,
    locale: article.locale,
    canonicalUrl: article.canonicalUrl || undefined
  }
}

function latestRevisionID(article: AdminArticle) {
  return article.latestRevision?.id || ''
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

function seedScheduleDrafts() {
  for (const article of articles.value) {
    if (scheduleDrafts[article.id]) continue
    const scheduledAt = article.scheduledForUtc ? parseBackendUTC(article.scheduledForUtc) : new Date(Date.now() + 15 * 60 * 1000)
    scheduleDrafts[article.id] = toLocalInputValue(scheduledAt)
  }
}

function parseBackendUTC(value: string) {
  return new Date(value.includes('T') ? value : `${value.replace(' ', 'T')}Z`)
}

function toLocalInputValue(date: Date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
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

function editorialClass(state: string) {
  switch (state) {
    case 'approved':
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'in_review':
      return 'bg-[#e8f0ff] text-[#245b99] dark:bg-[#152944] dark:text-[#b8d5ff]'
    case 'changes_requested':
      return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
    default:
      return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
}

function publicationClass(state: string) {
  switch (state) {
    case 'published':
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'scheduled':
      return 'bg-[#e8f0ff] text-[#245b99] dark:bg-[#152944] dark:text-[#b8d5ff]'
    default:
      return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
}

function labelize(value: string) {
  return value.replaceAll('_', ' ')
}

function slugify(value: string) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}

function escapeHTML(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}

function clearMessages() {
  errorMessage.value = ''
  successMessage.value = ''
}

function normalizeAPIError(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) {
    return error.message
  }
  if (typeof error === 'object' && error !== null && 'data' in error) {
    const data = (error as { data?: { title?: string, detail?: string } }).data
    return data?.detail || data?.title || fallback
  }
  return fallback
}
</script>

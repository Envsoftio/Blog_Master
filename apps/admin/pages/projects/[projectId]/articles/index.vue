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
            <h1 class="truncate text-lg font-semibold tracking-normal">Articles</h1>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button
            class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#eef5f1] disabled:opacity-50 dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]"
            type="button"
            title="Refresh articles"
            aria-label="Refresh articles"
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
      <ProjectNav :project-id="projectID" :project="project" active="articles" />

      <main class="min-w-0 space-y-5">
        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>
        <p v-if="successMessage" class="rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-4 py-3 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]" role="status">
          {{ successMessage }}
        </p>

        <section class="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Editorial workspace</p>
            <h2 class="mt-1 text-2xl font-semibold tracking-tight">Plan, review, and publish</h2>
            <p class="mt-2 max-w-2xl text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Searches and workflow filters run across the project on the server. Every action is scoped to this project and your current role.</p>
          </div>
          <NuxtLink
            v-if="canWriteArticles"
            class="inline-flex h-10 items-center gap-2 rounded-md bg-[#165a4a] px-4 text-sm font-medium text-white hover:bg-[#10463a]"
            :to="`/projects/${projectID}/articles/create`"
          >
            <Plus class="h-4 w-4" />
            New article
          </NuxtLink>
        </section>

        <dl class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <div class="rounded-lg border border-[#cfd8d1] bg-white p-4 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
            <dt class="text-xs uppercase tracking-wide text-[#667169] dark:text-[#aeb8b0]">Loaded results</dt>
            <dd class="mt-2 text-2xl font-semibold">{{ articles.length }}</dd>
          </div>
          <div class="rounded-lg border border-[#cfd8d1] bg-white p-4 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
            <dt class="text-xs uppercase tracking-wide text-[#667169] dark:text-[#aeb8b0]">Needs review</dt>
            <dd class="mt-2 text-2xl font-semibold text-[#245b99] dark:text-[#b8d5ff]">{{ articleStats.inReview }}</dd>
          </div>
          <div class="rounded-lg border border-[#cfd8d1] bg-white p-4 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
            <dt class="text-xs uppercase tracking-wide text-[#667169] dark:text-[#aeb8b0]">Published</dt>
            <dd class="mt-2 text-2xl font-semibold text-[#165a4a] dark:text-[#aee4d0]">{{ articleStats.published }}</dd>
          </div>
          <div class="rounded-lg border border-[#cfd8d1] bg-white p-4 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
            <dt class="text-xs uppercase tracking-wide text-[#667169] dark:text-[#aeb8b0]">Scheduled</dt>
            <dd class="mt-2 text-2xl font-semibold text-[#7a4f00] dark:text-[#ffd98a]">{{ articleStats.scheduled }}</dd>
          </div>
        </dl>

        <form class="grid gap-3 rounded-lg border border-[#cfd8d1] bg-white p-3 shadow-sm dark:border-[#3f4843] dark:bg-[#202522] lg:grid-cols-[minmax(240px,1fr)_180px_180px_auto]" role="search" @submit.prevent="applyFilters">
          <label class="relative block">
            <span class="sr-only">Search articles</span>
            <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#667169] dark:text-[#aeb8b0]" />
            <input
              v-model="filterDraft.search"
              class="h-10 w-full rounded-md border border-[#bfcac3] bg-white pl-10 pr-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]"
              type="search"
              maxlength="100"
              placeholder="Search title, slug, or type"
            />
          </label>
          <label>
            <span class="sr-only">Editorial state</span>
            <select v-model="filterDraft.editorialState" class="h-10 w-full rounded-md border border-[#bfcac3] bg-white px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]">
              <option value="">All workflow states</option>
              <option value="draft">Draft</option>
              <option value="in_review">In review</option>
              <option value="changes_requested">Changes requested</option>
              <option value="approved">Approved</option>
            </select>
          </label>
          <label>
            <span class="sr-only">Publication state</span>
            <select v-model="filterDraft.publicationState" class="h-10 w-full rounded-md border border-[#bfcac3] bg-white px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]">
              <option value="">All publication states</option>
              <option value="unpublished">Unpublished</option>
              <option value="scheduled">Scheduled</option>
              <option value="published">Published</option>
              <option value="archived">Archived</option>
            </select>
          </label>
          <button class="inline-flex h-10 items-center justify-center gap-2 rounded-md bg-[#25352c] px-4 text-sm font-medium text-white hover:bg-[#18251e] disabled:opacity-60 dark:bg-[#dce8df] dark:text-[#17201b]" type="submit" :disabled="pending">
            <SlidersHorizontal class="h-4 w-4" />
            Apply
          </button>
          <div class="flex flex-wrap items-center justify-between gap-3 lg:col-span-4">
            <label class="flex items-center gap-2 text-sm">
              <input v-model="filterDraft.includeArchived" class="h-4 w-4 rounded border-[#bfcac3]" type="checkbox" />
              Include archived articles
            </label>
            <button v-if="filtersActive" class="text-sm font-medium text-[#245b99] underline-offset-2 hover:underline dark:text-[#b8d5ff]" type="button" @click="clearFilters">
              Clear filters
            </button>
          </div>
        </form>

        <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]" aria-live="polite">
          <LoaderCircle class="h-4 w-4 animate-spin" />
          Loading articles
        </div>

        <section v-else-if="articles.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-10 text-center dark:border-[#4b5650] dark:bg-[#202522]">
          <FileSearch class="mx-auto h-8 w-8 text-[#667169] dark:text-[#aeb8b0]" />
          <h2 class="mt-3 text-lg font-semibold">{{ filtersActive ? 'No matching articles' : 'No articles yet' }}</h2>
          <p class="mt-2 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ filtersActive ? 'Adjust or clear the project-wide filters.' : 'Create a category and start the first draft.' }}</p>
        </section>

        <section v-else class="space-y-4" aria-label="Article results" aria-live="polite">
          <article v-for="article in articles" :key="article.id" class="rounded-lg border bg-white p-5 shadow-sm dark:bg-[#202522]" :class="article.archivedAt ? 'border-[#d9b7aa] dark:border-[#6d352f]' : 'border-[#cfd8d1] dark:border-[#3f4843]'">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2 text-xs uppercase tracking-wide text-[#667169] dark:text-[#aeb8b0]">
                  <span>{{ labelize(article.articleType) }}</span>
                  <span aria-hidden="true">•</span>
                  <span>{{ article.locale }}</span>
                  <span aria-hidden="true">•</span>
                  <span>Created {{ formatDate(article.createdAt) }}</span>
                </div>
                <h3 class="mt-2 truncate text-lg font-semibold">{{ article.title }}</h3>
                <p class="mt-1 truncate font-mono text-xs text-[#5f6a63] dark:text-[#b8c2bb]">/{{ article.slug }}</p>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="editorialClass(article.editorialState)">{{ labelize(article.editorialState) }}</span>
                <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="publicationClass(article.publicationState)">{{ labelize(article.publicationState) }}</span>
              </div>
            </div>

            <p v-if="article.latestRevision?.excerpt" class="mt-4 line-clamp-2 text-sm text-[#4f5b54] dark:text-[#c5cec8]">{{ article.latestRevision.excerpt }}</p>

            <dl class="mt-5 grid gap-3 text-sm sm:grid-cols-2 xl:grid-cols-4">
              <div class="flex items-center gap-2">
                <FileText class="h-4 w-4 text-[#3162a3]" />
                <div class="min-w-0"><dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Revision</dt><dd class="truncate">{{ article.latestRevision ? `#${article.latestRevision.revisionNumber}` : 'None' }}</dd></div>
              </div>
              <div class="flex items-center gap-2">
                <CalendarClock class="h-4 w-4 text-[#8a5b00]" />
                <div class="min-w-0"><dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Scheduled</dt><dd class="truncate">{{ formatDate(article.scheduledForUtc) }}</dd></div>
              </div>
              <div class="flex items-center gap-2">
                <UploadCloud class="h-4 w-4 text-[#165a4a]" />
                <div class="min-w-0"><dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Published</dt><dd class="truncate">{{ formatDate(article.publishedAt) }}</dd></div>
              </div>
              <div class="flex items-center gap-2">
                <Archive class="h-4 w-4 text-[#9b2d23]" />
                <div class="min-w-0"><dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Archived</dt><dd class="truncate">{{ formatDate(article.archivedAt) }}</dd></div>
              </div>
            </dl>

            <a v-if="article.canonicalUrl" class="mt-4 block truncate rounded-md bg-[#f2f5f3] px-3 py-2 text-sm text-[#245b99] hover:underline dark:bg-[#171b18] dark:text-[#b8d5ff]" :href="article.canonicalUrl" target="_blank" rel="noopener noreferrer">{{ article.canonicalUrl }}</a>

            <p v-if="actionPending[article.id]" class="mt-4 flex items-center gap-2 text-sm text-[#5d6a61] dark:text-[#aeb8b0]" role="status">
              <LoaderCircle class="h-4 w-4 animate-spin" />
              {{ actionLabel(actionPending[article.id] || '') }}
            </p>

            <div class="mt-5 flex flex-wrap items-center gap-2">
              <NuxtLink v-if="!article.archivedAt" class="inline-flex h-10 items-center gap-2 rounded-md border border-[#c9d4cc] px-3 text-sm font-medium hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]" :to="`/projects/${projectID}/articles/${article.id}`">
                <FilePenLine class="h-4 w-4" />
                Open workspace
              </NuxtLink>
              <button v-if="article.archivedAt && canPublishArticles" class="inline-flex h-10 items-center gap-2 rounded-md bg-[#165a4a] px-3 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60" type="button" :disabled="Boolean(actionPending[article.id])" @click="restoreArticle(article)">
                <ArchiveRestore class="h-4 w-4" />
                Restore unpublished
              </button>
              <button v-if="!article.archivedAt && canWriteArticles && ['draft', 'changes_requested'].includes(article.editorialState)" class="inline-flex h-10 items-center gap-2 rounded-md border border-[#c9d4cc] px-3 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]" type="button" :disabled="Boolean(actionPending[article.id])" @click="submitRevision(article)">
                <Send class="h-4 w-4" />
                Submit
              </button>
              <button v-if="!article.archivedAt && canReviewArticles && article.editorialState === 'in_review'" class="inline-flex h-10 items-center gap-2 rounded-md border border-[#d6bd7a] px-3 text-sm font-medium text-[#7a4f00] hover:bg-[#fff7e4] disabled:opacity-60 dark:border-[#6b572e] dark:text-[#ffd98a]" type="button" :disabled="Boolean(actionPending[article.id])" @click="requestChanges(article)">
                <RotateCcw class="h-4 w-4" />
                Request changes
              </button>
              <button v-if="!article.archivedAt && canReviewArticles && article.editorialState === 'in_review'" class="inline-flex h-10 items-center gap-2 rounded-md border border-[#b9dcc9] px-3 text-sm font-medium text-[#165a4a] hover:bg-[#edf9f1] disabled:opacity-60 dark:border-[#2d644a] dark:text-[#aee4d0]" type="button" :disabled="Boolean(actionPending[article.id])" @click="approveRevision(article)">
                <CheckCircle2 class="h-4 w-4" />
                Approve
              </button>
              <button v-if="!article.archivedAt && canPublishArticles && article.editorialState === 'approved'" class="inline-flex h-10 items-center gap-2 rounded-md bg-[#165a4a] px-3 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60" type="button" :disabled="Boolean(actionPending[article.id])" @click="publishArticle(article)">
                <UploadCloud class="h-4 w-4" />
                {{ article.publicationState === 'published' ? 'Republish' : 'Publish' }}
              </button>
              <button v-if="!article.archivedAt && canPublishArticles && ['published', 'scheduled'].includes(article.publicationState)" class="inline-flex h-10 items-center gap-2 rounded-md border border-[#d9b7aa] px-3 text-sm font-medium text-[#9b2d23] hover:bg-[#fff4f2] disabled:opacity-60 dark:border-[#6d352f] dark:text-[#ffc4bd]" type="button" :disabled="Boolean(actionPending[article.id])" @click="unpublishArticle(article)">
                <XCircle class="h-4 w-4" />
                Unpublish
              </button>
              <button v-if="!article.archivedAt && canPublishArticles" class="inline-flex h-10 items-center gap-2 rounded-md border border-[#d9b7aa] px-3 text-sm font-medium text-[#9b2d23] hover:bg-[#fff4f2] disabled:opacity-60 dark:border-[#6d352f] dark:text-[#ffc4bd]" type="button" :disabled="Boolean(actionPending[article.id])" @click="archiveArticle(article)">
                <Archive class="h-4 w-4" />
                Archive
              </button>
            </div>

            <form v-if="!article.archivedAt && canPublishArticles && article.editorialState === 'approved'" class="mt-4 grid gap-2 rounded-md bg-[#f5f7f5] p-3 dark:bg-[#171b18] sm:grid-cols-[minmax(220px,1fr)_auto]" @submit.prevent="scheduleArticle(article)">
              <label>
                <span class="sr-only">Schedule {{ article.title }}</span>
                <input v-model="scheduleDrafts[article.id]" class="h-10 w-full rounded-md border border-[#bfcac3] bg-white px-3 text-sm dark:border-[#4b5650] dark:bg-[#202522]" type="datetime-local" :min="minimumSchedule" :disabled="Boolean(actionPending[article.id])" required />
              </label>
              <button class="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-3 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]" type="submit" :disabled="Boolean(actionPending[article.id]) || !scheduleDrafts[article.id]">
                <CalendarClock class="h-4 w-4" />
                Schedule
              </button>
            </form>
          </article>

          <button v-if="nextCursor" class="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] bg-white px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:bg-[#202522] dark:hover:bg-[#2a302d]" type="button" :disabled="loadingMore" @click="loadMoreArticles">
            <LoaderCircle v-if="loadingMore" class="h-4 w-4 animate-spin" />
            <ChevronDown v-else class="h-4 w-4" />
            Load more articles
          </button>
        </section>

        <section class="grid gap-4 md:grid-cols-3">
          <NuxtLink class="rounded-lg border border-[#cfd8d1] bg-white p-4 shadow-sm hover:bg-[#f5f8f6] dark:border-[#3f4843] dark:bg-[#202522] dark:hover:bg-[#252b28]" :to="`/projects/${projectID}/review`">
            <CheckCircle2 class="h-5 w-5 text-[#3162a3]" />
            <h2 class="mt-3 font-semibold">Review queue</h2>
            <p class="mt-1 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Assignments, due dates, and pending reviews.</p>
          </NuxtLink>
          <NuxtLink class="rounded-lg border border-[#cfd8d1] bg-white p-4 shadow-sm hover:bg-[#f5f8f6] dark:border-[#3f4843] dark:bg-[#202522] dark:hover:bg-[#252b28]" :to="`/projects/${projectID}/calendar`">
            <CalendarDays class="h-5 w-5 text-[#8a5b00]" />
            <h2 class="mt-3 font-semibold">Editorial calendar</h2>
            <p class="mt-1 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">See the project publication schedule.</p>
          </NuxtLink>
          <NuxtLink class="rounded-lg border border-[#cfd8d1] bg-white p-4 shadow-sm hover:bg-[#f5f8f6] dark:border-[#3f4843] dark:bg-[#202522] dark:hover:bg-[#252b28]" :to="`/projects/${projectID}/categories`">
            <FolderTree class="h-5 w-5 text-[#165a4a]" />
            <h2 class="mt-3 font-semibold">{{ categories.length }} categories loaded</h2>
            <p class="mt-1 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Manage hierarchy and archive visibility.</p>
          </NuxtLink>
        </section>
      </main>
    </div>
  </section>
</template>

<script setup lang="ts">
import {
  Archive,
  ArchiveRestore,
  ArrowLeft,
  CalendarClock,
  CalendarDays,
  CheckCircle2,
  ChevronDown,
  FilePenLine,
  FileSearch,
  FileText,
  FolderTree,
  LoaderCircle,
  LogOut,
  Plus,
  RefreshCw,
  RotateCcw,
  Search,
  Send,
  SlidersHorizontal,
  UploadCloud,
  XCircle
} from 'lucide-vue-next'
import type { AdminArticle, AdminProject, ArticleListOptions, TaxonomyTerm } from '~/composables/useAdminApi'

type EditorialFilter = NonNullable<ArticleListOptions['editorialState']>
type PublicationFilter = NonNullable<ArticleListOptions['publicationState']>

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => {
  const value = route.params.projectId
  return Array.isArray(value) ? String(value[0] || '') : String(value || '')
})

const project = ref<AdminProject | null>(null)
const articles = ref<AdminArticle[]>([])
const categories = ref<TaxonomyTerm[]>([])
const pending = ref(true)
const loadingMore = ref(false)
const nextCursor = ref('')
const errorMessage = ref('')
const successMessage = ref('')
const actionPending = reactive<Record<string, string>>({})
const scheduleDrafts = reactive<Record<string, string>>({})
const minimumSchedule = ref('')
const filterDraft = reactive({
  search: '',
  editorialState: '' as EditorialFilter,
  publicationState: '' as PublicationFilter,
  includeArchived: false
})
const appliedFilters = reactive({ ...filterDraft })

const projectIsActive = computed(() => project.value?.status === 'active')
const canWriteArticles = computed(() => projectIsActive.value && ['project_owner', 'project_admin', 'editor', 'writer'].includes(project.value?.role || ''))
const canReviewArticles = computed(() => projectIsActive.value && ['project_owner', 'project_admin', 'editor', 'reviewer'].includes(project.value?.role || ''))
const canPublishArticles = computed(() => projectIsActive.value && ['project_owner', 'project_admin', 'editor'].includes(project.value?.role || ''))
const filtersActive = computed(() => Boolean(appliedFilters.search || appliedFilters.editorialState || appliedFilters.publicationState || appliedFilters.includeArchived))
const articleStats = computed(() => ({
  inReview: articles.value.filter(article => article.editorialState === 'in_review').length,
  published: articles.value.filter(article => article.publicationState === 'published').length,
  scheduled: articles.value.filter(article => article.publicationState === 'scheduled').length
}))

onMounted(() => {
  minimumSchedule.value = toLocalInputValue(new Date(Date.now() + 60_000))
  void refresh()
})

async function refresh() {
  pending.value = true
  clearMessages()
  try {
    const [projectResponse, categoryResponse, articleResponse] = await Promise.all([
      api.getProject(projectID.value),
      api.listCategories(projectID.value),
      api.listArticles(projectID.value, articleQuery())
    ])
    project.value = projectResponse.data
    categories.value = categoryResponse.data
    articles.value = articleResponse.data
    nextCursor.value = articleResponse.meta?.nextCursor || ''
    seedScheduleDrafts()
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load this project. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function applyFilters() {
  appliedFilters.search = filterDraft.search.trim()
  appliedFilters.editorialState = filterDraft.editorialState
  appliedFilters.publicationState = filterDraft.publicationState
  appliedFilters.includeArchived = filterDraft.includeArchived || filterDraft.publicationState === 'archived'
  await refresh()
}

async function clearFilters() {
  filterDraft.search = ''
  filterDraft.editorialState = ''
  filterDraft.publicationState = ''
  filterDraft.includeArchived = false
  appliedFilters.search = ''
  appliedFilters.editorialState = ''
  appliedFilters.publicationState = ''
  appliedFilters.includeArchived = false
  await refresh()
}

async function loadMoreArticles() {
  if (!nextCursor.value || loadingMore.value) return
  loadingMore.value = true
  clearMessages()
  try {
    const response = await api.listArticles(projectID.value, articleQuery(nextCursor.value))
    const merged = new Map(articles.value.map(article => [article.id, article]))
    for (const article of response.data) merged.set(article.id, article)
    articles.value = [...merged.values()]
    nextCursor.value = response.meta?.nextCursor || ''
    seedScheduleDrafts()
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load more articles.')
  } finally {
    loadingMore.value = false
  }
}

function articleQuery(cursor = ''): ArticleListOptions {
  return {
    cursor,
    limit: 25,
    search: appliedFilters.search,
    editorialState: appliedFilters.editorialState,
    publicationState: appliedFilters.publicationState,
    includeArchived: appliedFilters.includeArchived
  }
}

async function submitRevision(article: AdminArticle) {
  await mutateArticle(article, 'submit', async () => {
    await api.revisionAction(projectID.value, latestRevisionID(article), 'submit')
    successMessage.value = 'Revision submitted for review.'
  })
}

async function requestChanges(article: AdminArticle) {
  await mutateArticle(article, 'request-changes', async () => {
    await api.revisionAction(projectID.value, latestRevisionID(article), 'request-changes')
    successMessage.value = 'Changes requested.'
  })
}

async function approveRevision(article: AdminArticle) {
  await mutateArticle(article, 'approve', async () => {
    await api.revisionAction(projectID.value, latestRevisionID(article), 'approve')
    successMessage.value = 'Exact revision approved.'
  })
}

async function publishArticle(article: AdminArticle) {
  await mutateArticle(article, 'publish', async () => {
    await api.articleAction(projectID.value, article.id, 'publish', publicationBody(article))
    successMessage.value = 'Article published.'
  })
}

async function scheduleArticle(article: AdminArticle) {
  const scheduledAt = scheduleDrafts[article.id]
  if (!scheduledAt) return
  const parsed = new Date(scheduledAt)
  if (Number.isNaN(parsed.getTime()) || parsed.getTime() <= Date.now()) {
    errorMessage.value = 'Choose a valid future publication time.'
    return
  }
  await mutateArticle(article, 'schedule', async () => {
    await api.articleAction(projectID.value, article.id, 'schedule', {
      ...publicationBody(article),
      scheduledForUtc: parsed.toISOString()
    })
    successMessage.value = 'Article scheduled.'
  })
}

async function unpublishArticle(article: AdminArticle) {
  if (!window.confirm(`Unpublish “${article.title}”? The published JSON route will stop resolving.`)) return
  await mutateArticle(article, 'unpublish', async () => {
    await api.articleAction(projectID.value, article.id, 'unpublish')
    successMessage.value = 'Article unpublished.'
  })
}

async function archiveArticle(article: AdminArticle) {
  const warning = article.publicationState === 'published'
    ? 'It will also be removed from the Content API. You can restore it later as an unpublished article.'
    : 'Its immutable revisions will be retained and it can be restored later.'
  if (!window.confirm(`Archive “${article.title}”? ${warning}`)) return
  await mutateArticle(article, 'archive', async () => {
    await api.deleteArticle(projectID.value, article.id)
    successMessage.value = 'Article archived. Enable “Include archived articles” to restore it.'
  })
}

async function restoreArticle(article: AdminArticle) {
  if (!window.confirm(`Restore “${article.title}” as an unpublished article?`)) return
  await mutateArticle(article, 'restore', async () => {
    await api.articleAction(projectID.value, article.id, 'restore')
    successMessage.value = 'Article restored as unpublished.'
  })
}

async function mutateArticle(article: AdminArticle, action: string, operation: () => Promise<void>) {
  if (!article.latestRevision && action !== 'restore' && action !== 'archive') {
    errorMessage.value = 'This article has no revision.'
    return
  }
  actionPending[article.id] = action
  clearMessages()
  try {
    await operation()
    await fetchArticles()
  } catch (error) {
    successMessage.value = ''
    errorMessage.value = normalizeAPIError(error, 'Could not complete the article action.')
  } finally {
    delete actionPending[article.id]
  }
}

async function fetchArticles() {
  const response = await api.listArticles(projectID.value, articleQuery())
  articles.value = response.data
  nextCursor.value = response.meta?.nextCursor || ''
  seedScheduleDrafts()
}

function publicationBody(article: AdminArticle) {
  return {
    revisionId: latestRevisionID(article),
    slug: article.slug,
    locale: article.locale,
    ...(article.canonicalUrl ? { canonicalUrl: article.canonicalUrl } : {})
  }
}

function latestRevisionID(article: AdminArticle) {
  return article.latestRevision?.id || ''
}

function seedScheduleDrafts() {
  for (const article of articles.value) {
    if (scheduleDrafts[article.id]) continue
    const date = article.scheduledForUtc ? parseBackendUTC(article.scheduledForUtc) : new Date(Date.now() + 30 * 60 * 1000)
    scheduleDrafts[article.id] = toLocalInputValue(date)
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

function toLocalInputValue(date: Date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

function formatDate(value?: string) {
  if (!value) return 'Not set'
  const date = parseBackendUTC(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

function editorialClass(state: string) {
  switch (state) {
    case 'approved': return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'in_review': return 'bg-[#e8f0ff] text-[#245b99] dark:bg-[#152944] dark:text-[#b8d5ff]'
    case 'changes_requested': return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
    default: return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
}

function publicationClass(state: string) {
  switch (state) {
    case 'published': return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'scheduled': return 'bg-[#e8f0ff] text-[#245b99] dark:bg-[#152944] dark:text-[#b8d5ff]'
    case 'archived': return 'bg-[#fbe4e1] text-[#8f3028] dark:bg-[#46231f] dark:text-[#ffc4bd]'
    default: return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
}

function actionLabel(action: string) {
  const labels: Record<string, string> = {
    submit: 'Submitting revision…',
    'request-changes': 'Requesting changes…',
    approve: 'Approving revision…',
    publish: 'Publishing article…',
    schedule: 'Scheduling article…',
    unpublish: 'Unpublishing article…',
    archive: 'Archiving article…',
    restore: 'Restoring article…'
  }
  return labels[action] || 'Updating article…'
}

function labelize(value: string) {
  return value.replaceAll('_', ' ')
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

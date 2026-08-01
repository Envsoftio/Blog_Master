<template>
  <div class="page-stack content-page">
    <div class="page-heading">
      <div>
        <p>Plan, review, and publish every article in {{ project?.name || 'this project' }}.</p>
      </div>
      <button class="button button--compact" type="button" :disabled="pending" @click="refresh">
        <RefreshCw :class="{ spin: pending }" :size="16" />
        Refresh
      </button>
    </div>

    <dl class="metric-grid">
      <div class="metric-card surface">
        <div class="metric-card__top"><dt>Current results</dt><FileText :size="17" /></div>
        <dd class="metric-card__value">{{ articles.length }}</dd>
      </div>
      <div class="metric-card surface">
        <div class="metric-card__top"><dt>Needs review</dt><BookOpenCheck :size="17" /></div>
        <dd class="metric-card__value">{{ articleStats.inReview }}</dd>
      </div>
      <div class="metric-card surface">
        <div class="metric-card__top"><dt>Published</dt><CheckCircle2 :size="17" /></div>
        <dd class="metric-card__value">{{ articleStats.published }}</dd>
      </div>
      <div class="metric-card surface">
        <div class="metric-card__top"><dt>Scheduled</dt><CalendarClock :size="17" /></div>
        <dd class="metric-card__value">{{ articleStats.scheduled }}</dd>
      </div>
    </dl>

    <form class="content-toolbar surface surface--subtle" role="search" @submit.prevent="applyFilters">
      <label class="content-search">
        <Search :size="16" />
        <input v-model="filterDraft.search" type="search" maxlength="100" placeholder="Search title, slug, or type" aria-label="Search articles">
      </label>
      <select v-model="filterDraft.editorialState" class="input" aria-label="Editorial state">
        <option value="">All workflows</option>
        <option value="draft">Draft</option>
        <option value="in_review">In review</option>
        <option value="changes_requested">Changes requested</option>
        <option value="approved">Approved</option>
      </select>
      <select v-model="filterDraft.publicationState" class="input" aria-label="Publication state">
        <option value="">All publication</option>
        <option value="unpublished">Unpublished</option>
        <option value="scheduled">Scheduled</option>
        <option value="published">Published</option>
        <option value="archived">Archived</option>
      </select>
      <button class="button button--primary button--compact" type="submit" :disabled="pending">
        <SlidersHorizontal :size="15" />
        Apply
      </button>
      <div class="content-toolbar__options">
        <label class="archive-toggle">
          <input v-model="filterDraft.includeArchived" type="checkbox">
          <span>Include archived</span>
        </label>
        <button v-if="filtersActive" class="clear-filters" type="button" @click="clearFilters">Clear filters</button>
      </div>
    </form>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success" role="status">{{ successMessage }}</p>

    <div v-if="pending" class="loading-surface surface" aria-live="polite">
      <LoaderCircle class="spin" :size="18" />
      Loading content
    </div>

    <div v-else-if="articles.length === 0" class="empty-state">
      <div>
        <span class="empty-state__icon"><FileSearch :size="20" /></span>
        <h3>{{ filtersActive ? 'No matching content' : 'No content yet' }}</h3>
        <p>{{ filtersActive ? 'Try a broader search or clear the active filters.' : 'Create the first article to start your editorial workflow.' }}</p>
        <NuxtLink v-if="canWriteArticles && !filtersActive" class="button button--primary" :to="`/projects/${projectID}/articles/create`">
          Create article
        </NuxtLink>
        <button v-else-if="filtersActive" class="button" type="button" @click="clearFilters">Clear filters</button>
      </div>
    </div>

    <section v-else class="content-list surface" aria-label="Content results" aria-live="polite">
      <header class="content-list__heading">
        <div>
          <p>Content library</p>
          <h2>{{ articles.length }} {{ articles.length === 1 ? 'article' : 'articles' }}</h2>
        </div>
        <span v-if="filtersActive" class="filter-indicator"><SlidersHorizontal :size="13" />Filtered</span>
      </header>

      <div class="content-list__columns" aria-hidden="true">
        <span>Article</span>
        <span>Status</span>
        <span>Last activity</span>
        <span>Actions</span>
      </div>

      <article v-for="article in articles" :key="article.id" class="content-item" :class="{ 'content-item--archived': article.archivedAt }">
        <div class="content-item__row">
          <div class="content-item__main">
            <span class="article-icon" :class="{ 'article-icon--archived': article.archivedAt }">
              <Archive v-if="article.archivedAt" :size="17" />
              <FileText v-else :size="17" />
            </span>
            <div class="content-item__copy">
              <NuxtLink v-if="!article.archivedAt" :to="`/projects/${projectID}/articles/${article.id}`">{{ article.title }}</NuxtLink>
              <strong v-else>{{ article.title }}</strong>
              <p>{{ article.latestRevision?.excerpt || `/${article.slug}` }}</p>
              <div class="content-item__meta">
                <span>{{ labelize(article.articleType) }}</span>
                <span>{{ article.latestRevision ? `Revision ${article.latestRevision.revisionNumber}` : 'No revision' }}</span>
              </div>
            </div>
          </div>

          <div class="content-item__status">
            <span class="status-pill" :class="editorialClass(article.editorialState)">{{ labelize(article.editorialState) }}</span>
            <span class="status-pill" :class="publicationClass(article.publicationState)">{{ labelize(article.publicationState) }}</span>
          </div>

          <div class="content-item__activity">
            <strong>{{ activityLabel(article) }}</strong>
            <span>{{ formatDate(activityDate(article)) }}</span>
          </div>

          <div class="content-item__actions">
            <span v-if="actionPending[article.id]" class="action-progress" role="status">
              <LoaderCircle class="spin" :size="15" />
              {{ actionLabel(actionPending[article.id] || '') }}
            </span>
            <template v-else>
              <NuxtLink v-if="!article.archivedAt" class="button button--compact" :to="`/projects/${projectID}/articles/${article.id}`">
                <FilePenLine :size="15" />
                Open
              </NuxtLink>
              <button v-if="article.archivedAt && canPublishArticles" class="button button--primary button--compact" type="button" @click="restoreArticle(article)">
                <ArchiveRestore :size="15" />
                Restore
              </button>
              <button v-else-if="canWriteArticles && ['draft', 'changes_requested'].includes(article.editorialState)" class="button button--compact" type="button" @click="submitRevision(article)">
                <Send :size="15" />
                Submit
              </button>
              <button v-else-if="canReviewArticles && article.editorialState === 'in_review'" class="button button--primary button--compact" type="button" @click="approveRevision(article)">
                <CheckCircle2 :size="15" />
                Approve
              </button>
              <button v-else-if="canPublishArticles && article.editorialState === 'approved'" class="button button--primary button--compact" type="button" @click="publishArticle(article)">
                <UploadCloud :size="15" />
                {{ article.publicationState === 'published' ? 'Republish' : 'Publish' }}
              </button>

              <details v-if="articleMenuVisible(article)" class="article-menu">
                <summary class="icon-button" title="More actions" :aria-label="`More actions for ${article.title}`">
                  <Ellipsis :size="17" />
                </summary>
                <div class="article-menu__panel">
                  <a v-if="article.canonicalUrl" :href="article.canonicalUrl" target="_blank" rel="noopener noreferrer">
                    <ExternalLink :size="14" />View live URL
                  </a>
                  <button v-if="!article.archivedAt && canReviewArticles && article.editorialState === 'in_review'" type="button" @click="requestChanges(article)">
                    <RotateCcw :size="14" />Request changes
                  </button>
                  <button v-if="!article.archivedAt && canPublishArticles && ['published', 'scheduled'].includes(article.publicationState)" type="button" @click="unpublishArticle(article)">
                    <XCircle :size="14" />Unpublish
                  </button>
                  <button v-if="!article.archivedAt && canPublishArticles" class="article-menu__danger" type="button" @click="archiveArticle(article)">
                    <Archive :size="14" />Archive
                  </button>
                </div>
              </details>
            </template>
          </div>
        </div>

        <form v-if="!article.archivedAt && canPublishArticles && article.editorialState === 'approved'" class="schedule-row" @submit.prevent="scheduleArticle(article)">
          <span><CalendarClock :size="15" />Schedule publication</span>
          <label>
            <span class="sr-only">Schedule {{ article.title }}</span>
            <input v-model="scheduleDrafts[article.id]" type="datetime-local" :min="minimumSchedule" :disabled="Boolean(actionPending[article.id])" required>
          </label>
          <button class="button button--compact" type="submit" :disabled="Boolean(actionPending[article.id]) || !scheduleDrafts[article.id]">Schedule</button>
        </form>
      </article>

      <div v-if="nextCursor" class="content-list__footer">
        <button class="button" type="button" :disabled="loadingMore" @click="loadMoreArticles">
          <LoaderCircle v-if="loadingMore" class="spin" :size="16" />
          <ChevronDown v-else :size="16" />
          Load more content
        </button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import {
  Archive,
  ArchiveRestore,
  BookOpenCheck,
  CalendarClock,
  CheckCircle2,
  ChevronDown,
  Ellipsis,
  ExternalLink,
  FilePenLine,
  FileSearch,
  FileText,
  LoaderCircle,
  RefreshCw,
  RotateCcw,
  Search,
  Send,
  SlidersHorizontal,
  UploadCloud,
  XCircle
} from 'lucide-vue-next'
import type { AdminArticle, AdminProject, ArticleListOptions } from '~/composables/useAdminApi'

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
    const [projectResponse, articleResponse] = await Promise.all([
      api.getProject(projectID.value),
      api.listArticles(projectID.value, articleQuery())
    ])
    project.value = projectResponse.data
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

function activityLabel(article: AdminArticle) {
  if (article.archivedAt) return 'Archived'
  if (article.publicationState === 'published') return 'Published'
  if (article.publicationState === 'scheduled') return 'Scheduled'
  if (article.latestRevision) return 'Last edited'
  return 'Created'
}

function activityDate(article: AdminArticle) {
  if (article.archivedAt) return article.archivedAt
  if (article.publicationState === 'published' && article.publishedAt) return article.publishedAt
  if (article.publicationState === 'scheduled' && article.scheduledForUtc) return article.scheduledForUtc
  return article.latestRevision?.createdAt || article.createdAt
}

function articleMenuVisible(article: AdminArticle) {
  return Boolean(
    article.canonicalUrl
    || (!article.archivedAt && canReviewArticles.value && article.editorialState === 'in_review')
    || (!article.archivedAt && canPublishArticles.value)
  )
}

function editorialClass(state: string) {
  switch (state) {
    case 'approved': return 'status-pill--success'
    case 'in_review': return 'status-pill--info'
    case 'changes_requested': return 'status-pill--warning'
    default: return ''
  }
}

function publicationClass(state: string) {
  switch (state) {
    case 'published': return 'status-pill--success'
    case 'scheduled': return 'status-pill--warning'
    case 'archived': return 'status-pill--danger'
    default: return ''
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

<style scoped>
.content-page .metric-grid { margin: 0; }
.metric-card:nth-child(1) .metric-card__top svg { color: var(--blue); }
.metric-card:nth-child(2) .metric-card__top svg { color: var(--amber); }
.metric-card:nth-child(3) .metric-card__top svg { color: var(--primary); }
.metric-card:nth-child(4) .metric-card__top svg { color: #7454c0; }
.dark .metric-card:nth-child(4) .metric-card__top svg { color: #b7a0f3; }
.metric-card dt { font-style: normal; }
.metric-card dd { margin-inline: 0; }
.content-toolbar { display: grid; grid-template-columns: minmax(260px, 1fr) 160px 160px auto; gap: 8px; padding: 8px; }
.content-search { display: flex; min-width: 0; align-items: center; gap: 8px; padding: 0 10px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--text-soft); }
.content-search input { width: 100%; min-width: 0; min-height: 34px; padding: 0; border: 0 !important; box-shadow: none !important; background: transparent !important; font-size: 13px; }
.content-toolbar > .input { min-height: 36px; padding-block: 6px; font-size: 12px; }
.content-toolbar__options { display: flex; grid-column: 1 / -1; align-items: center; justify-content: space-between; gap: 12px; padding: 3px 3px 1px; }
.archive-toggle { display: inline-flex; align-items: center; gap: 7px; color: var(--text-soft); font-size: 12px; cursor: pointer; }
.archive-toggle input { width: 14px; height: 14px; margin: 0; accent-color: var(--primary); }
.clear-filters { padding: 2px 4px; border: 0; background: transparent; color: var(--primary); font-size: 12px; font-weight: 650; cursor: pointer; }
.clear-filters:hover { text-decoration: underline; }
.content-list { position: relative; }
.content-list__heading { display: flex; min-height: 62px; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 16px; border-bottom: 1px solid var(--border); }
.content-list__heading p, .content-list__heading h2 { margin: 0; }
.content-list__heading p { color: var(--text-soft); font-size: 12px; }
.content-list__heading h2 { margin-top: 1px; font-size: 14px; }
.filter-indicator { display: inline-flex; align-items: center; gap: 5px; color: var(--primary); font-size: 12px; font-weight: 650; }
.content-list__columns, .content-item__row { display: grid; grid-template-columns: minmax(280px, 1.5fr) minmax(185px, .72fr) minmax(135px, .5fr) minmax(210px, auto); gap: 14px; align-items: center; }
.content-list__columns { min-height: 34px; padding: 7px 16px; border-bottom: 1px solid var(--border); background: var(--surface-subtle); color: var(--text-soft); font-size: 12px; font-weight: 650; text-transform: uppercase; }
.content-list__columns span:last-child { text-align: right; }
.content-item { border-bottom: 1px solid var(--border); }
.content-item:last-of-type { border-bottom: 0; }
.content-item:hover { background: var(--surface-subtle); }
.content-item--archived { background: color-mix(in srgb, var(--danger-soft) 42%, var(--surface)); }
.content-item__row { min-height: 86px; padding: 12px 16px; }
.content-item__main { display: flex; min-width: 0; align-items: flex-start; gap: 11px; }
.article-icon { display: grid; width: 36px; height: 36px; flex: 0 0 36px; place-items: center; border-radius: 7px; background: var(--blue-soft); color: var(--blue); }
.article-icon--archived { background: var(--danger-soft); color: var(--danger); }
.content-item__copy { min-width: 0; }
.content-item__copy > a, .content-item__copy > strong { display: block; overflow: hidden; color: var(--text); font-size: 12px; font-weight: 680; text-decoration: none; text-overflow: ellipsis; white-space: nowrap; }
.content-item__copy > a:hover { color: var(--primary); }
.content-item__copy > p { overflow: hidden; max-width: 600px; margin: 3px 0 0; color: var(--text-soft); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.content-item__meta { display: flex; flex-wrap: wrap; gap: 0; margin-top: 7px; color: var(--text-faint); font-size: 12px; text-transform: capitalize; }
.content-item__meta span + span::before { margin-inline: 6px; content: "·"; }
.content-item__status { display: flex; flex-wrap: wrap; gap: 5px; }
.status-pill--info { border-color: color-mix(in srgb, var(--blue) 35%, var(--border)); background: var(--blue-soft); color: var(--blue); }
.content-item__activity { display: flex; min-width: 0; flex-direction: column; }
.content-item__activity strong, .content-item__activity span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.content-item__activity strong { font-size: 12px; font-weight: 650; }
.content-item__activity span { margin-top: 3px; color: var(--text-soft); font-size: 12px; }
.content-item__actions { position: relative; display: flex; min-width: 0; align-items: center; justify-content: flex-end; gap: 6px; }
.content-item__actions .button { white-space: nowrap; }
.action-progress { display: inline-flex; align-items: center; gap: 6px; color: var(--text-soft); font-size: 12px; white-space: nowrap; }
.article-menu { position: relative; flex: 0 0 auto; }
.article-menu > summary { list-style: none; border-color: var(--border); background: var(--surface); }
.article-menu > summary::-webkit-details-marker { display: none; }
.article-menu[open] > summary { border-color: var(--primary); color: var(--primary); }
.article-menu__panel { position: absolute; z-index: 20; top: calc(100% + 5px); right: 0; display: grid; width: 178px; padding: 5px; border: 1px solid var(--border); border-radius: 7px; background: var(--surface-raised); box-shadow: var(--shadow-md); }
.article-menu__panel a, .article-menu__panel button { display: flex; width: 100%; min-height: 34px; align-items: center; gap: 8px; padding: 7px 9px; border: 0; border-radius: 5px; background: transparent; color: var(--text); font-size: 12px; text-align: left; text-decoration: none; cursor: pointer; }
.article-menu__panel a:hover, .article-menu__panel button:hover { background: var(--surface-subtle); }
.article-menu__panel .article-menu__danger { color: var(--danger); }
.schedule-row { display: grid; grid-template-columns: minmax(170px, 1fr) 220px auto; gap: 8px; align-items: center; padding: 9px 16px 9px 63px; border-top: 1px solid var(--border); background: color-mix(in srgb, var(--amber-soft) 36%, var(--surface)); }
.schedule-row > span { display: inline-flex; align-items: center; gap: 7px; color: var(--amber); font-size: 12px; font-weight: 650; }
.schedule-row input { width: 100%; min-height: 34px; padding: 5px 9px; border: 1px solid var(--border-strong); border-radius: 6px; background: var(--surface); color: var(--text); font-size: 12px; }
.content-list__footer { display: flex; justify-content: center; padding: 12px; border-top: 1px solid var(--border); background: var(--surface-subtle); }
.loading-surface { display: flex; min-height: 150px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.empty-state .button { margin-top: 14px; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1180px) {
  .content-list__columns { display: none; }
  .content-item__row { grid-template-columns: minmax(0, 1fr) auto; align-items: start; }
  .content-item__main { grid-column: 1; }
  .content-item__status { grid-column: 1; }
  .content-item__activity { display: none; }
  .content-item__actions { grid-column: 2; grid-row: 1 / span 2; align-self: center; }
}
@media (max-width: 760px) {
  .content-toolbar { grid-template-columns: 1fr 1fr; }
  .content-search { grid-column: 1 / -1; }
  .content-toolbar > .button { grid-column: 1 / -1; }
  .content-item__row { display: block; }
  .content-item__status { margin: 11px 0; }
  .content-item__actions { justify-content: flex-start; }
  .schedule-row { grid-template-columns: 1fr auto; padding-left: 16px; }
  .schedule-row > span { grid-column: 1 / -1; }
}
@media (max-width: 520px) {
  .page-heading { align-items: stretch; flex-direction: column; }
  .page-heading .button { align-self: flex-start; }
  .content-toolbar { grid-template-columns: 1fr; }
  .content-search, .content-toolbar > .button, .content-toolbar__options { grid-column: 1; }
  .schedule-row { grid-template-columns: 1fr; }
  .content-item__actions { flex-wrap: wrap; }
  .content-item__actions .button { flex: 1; }
}
</style>

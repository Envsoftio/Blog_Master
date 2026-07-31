<template>
  <div class="page-stack">
    <div class="page-heading">
      <div>
        <p>Revisions waiting for editorial decisions, requested changes, and publication readiness.</p>
      </div>
      <button class="button button--compact" type="button" :disabled="pending" @click="loadArticles">
        <RefreshCw :class="{ spin: pending }" :size="16" />
        Refresh
      </button>
    </div>

    <div class="metric-grid">
      <article v-for="metric in metrics" :key="metric.label" class="metric-card surface">
        <div class="metric-card__top">
          <span>{{ metric.label }}</span>
          <component :is="metric.icon" :size="17" />
        </div>
        <p class="metric-card__value">{{ metric.value }}</p>
      </article>
    </div>

    <div class="review-toolbar surface surface--subtle">
      <div class="segmented-control" role="tablist" aria-label="Review state">
        <button
          v-for="filter in filters"
          :key="filter.value"
          type="button"
          :class="{ 'is-active': activeFilter === filter.value }"
          @click="activeFilter = filter.value"
        >
          {{ filter.label }}
          <span>{{ filter.count }}</span>
        </button>
      </div>
      <label class="review-search">
        <Search :size="16" />
        <input v-model.trim="search" type="search" placeholder="Search review queue" aria-label="Search review queue">
      </label>
    </div>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success">{{ successMessage }}</p>

    <div v-if="pending" class="loading-surface surface">
      <LoaderCircle class="spin" :size="18" />
      Loading review queue
    </div>

    <div v-else-if="filteredArticles.length === 0" class="empty-state">
      <div>
        <span class="empty-state__icon"><ListChecks :size="20" /></span>
        <h3>Queue is clear</h3>
        <p>No articles match the selected review state.</p>
      </div>
    </div>

    <div v-else class="review-list surface">
      <article v-for="article in filteredArticles" :key="article.id" class="review-item">
        <div class="review-item__main">
          <span class="article-type-icon"><FileText :size="17" /></span>
          <div class="review-item__copy">
            <div class="review-item__title">
              <NuxtLink :to="`/projects/${projectID}/articles/${article.id}`">{{ article.title }}</NuxtLink>
              <span class="status-pill" :class="statusClass(article.editorialState)">{{ labelize(article.editorialState) }}</span>
            </div>
            <p>{{ article.latestRevision?.excerpt || article.slug }}</p>
            <div class="review-item__meta">
              <span><Languages :size="13" />{{ article.locale.toUpperCase() }}</span>
              <span><GitCommitHorizontal :size="13" />Revision {{ article.latestRevision?.revisionNumber || '–' }}</span>
              <span><Clock3 :size="13" />{{ relativeDate(article.latestRevision?.createdAt || article.createdAt) }}</span>
              <span>{{ labelize(article.articleType) }}</span>
            </div>
          </div>
        </div>
        <div class="review-item__actions">
          <NuxtLink class="button button--compact" :to="`/projects/${projectID}/articles/${article.id}`">
            Open
            <ArrowUpRight :size="15" />
          </NuxtLink>
          <button
            v-if="article.editorialState === 'in_review'"
            class="button button--compact"
            type="button"
            :disabled="actionPending === article.id"
            @click="requestChanges(article)"
          >
            <Undo2 :size="15" />
            Changes
          </button>
          <button
            v-if="article.editorialState === 'in_review'"
            class="button button--primary button--compact"
            type="button"
            :disabled="actionPending === article.id"
            @click="approve(article)"
          >
            <Check :size="15" />
            Approve
          </button>
        </div>
      </article>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  ArrowUpRight,
  Check,
  CheckCircle2,
  Clock3,
  FileText,
  GitCommitHorizontal,
  Languages,
  ListChecks,
  LoaderCircle,
  RefreshCw,
  Search,
  Send,
  Undo2
} from 'lucide-vue-next'
import type { AdminArticle } from '~/composables/useAdminApi'

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => String(route.params.projectId || ''))
const articles = ref<AdminArticle[]>([])
const pending = ref(true)
const actionPending = ref('')
const activeFilter = ref('needs_review')
const search = ref('')
const errorMessage = ref('')
const successMessage = ref('')

const reviewArticles = computed(() => articles.value.filter(article => ['in_review', 'changes_requested', 'approved'].includes(article.editorialState)))
const metrics = computed(() => [
  { label: 'Needs review', value: articles.value.filter(article => article.editorialState === 'in_review').length, icon: ListChecks },
  { label: 'Changes requested', value: articles.value.filter(article => article.editorialState === 'changes_requested').length, icon: Undo2 },
  { label: 'Approved', value: articles.value.filter(article => article.editorialState === 'approved').length, icon: CheckCircle2 },
  { label: 'Ready to publish', value: articles.value.filter(article => article.editorialState === 'approved' && article.publicationState === 'unpublished').length, icon: Send }
])
const filters = computed(() => [
  { value: 'needs_review', label: 'Needs review', count: articles.value.filter(article => article.editorialState === 'in_review').length },
  { value: 'changes_requested', label: 'Changes requested', count: articles.value.filter(article => article.editorialState === 'changes_requested').length },
  { value: 'approved', label: 'Approved', count: articles.value.filter(article => article.editorialState === 'approved').length },
  { value: 'all', label: 'All', count: reviewArticles.value.length }
])
const filteredArticles = computed(() => {
  const term = search.value.toLowerCase()
  return reviewArticles.value.filter(article => {
    const stateMatches = activeFilter.value === 'all'
      || (activeFilter.value === 'needs_review' ? article.editorialState === 'in_review' : article.editorialState === activeFilter.value)
    const searchMatches = !term || `${article.title} ${article.slug} ${article.articleType}`.toLowerCase().includes(term)
    return stateMatches && searchMatches
  })
})

onMounted(loadArticles)

async function loadArticles() {
  pending.value = true
  errorMessage.value = ''
  try {
    articles.value = (await api.listArticles(projectID.value)).data
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load the review queue.')
  } finally {
    pending.value = false
  }
}

async function approve(article: AdminArticle) {
  const revisionID = article.latestRevision?.id
  if (!revisionID) return
  await runAction(article.id, async () => {
    await api.revisionAction(projectID.value, revisionID, 'approve')
    successMessage.value = `${article.title} was approved.`
  })
}

async function requestChanges(article: AdminArticle) {
  const revisionID = article.latestRevision?.id
  if (!revisionID) return
  await runAction(article.id, async () => {
    await api.revisionAction(projectID.value, revisionID, 'request-changes')
    successMessage.value = `Changes were requested for ${article.title}.`
  })
}

async function runAction(articleID: string, action: () => Promise<void>) {
  actionPending.value = articleID
  errorMessage.value = ''
  successMessage.value = ''
  try {
    await action()
    await loadArticles()
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not update the review state.')
  } finally {
    actionPending.value = ''
  }
}

function statusClass(state: string) {
  if (state === 'approved') return 'status-pill--success'
  if (state === 'changes_requested') return 'status-pill--warning'
  return ''
}

function relativeDate(value?: string) {
  if (!value) return 'No timestamp'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'No timestamp'
  const diff = Date.now() - date.getTime()
  const days = Math.floor(diff / 86400000)
  if (days < 1) return 'Today'
  if (days === 1) return 'Yesterday'
  return `${days} days ago`
}
</script>

<style scoped>
.review-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 8px; }
.segmented-control { display: flex; gap: 3px; overflow-x: auto; }
.segmented-control button { display: inline-flex; min-height: 34px; align-items: center; gap: 7px; padding: 6px 10px; border: 0; border-radius: 5px; background: transparent; color: var(--text-soft); font-size: 13px; font-weight: 600; white-space: nowrap; cursor: pointer; }
.segmented-control button span { display: grid; min-width: 19px; height: 19px; place-items: center; border-radius: 10px; background: var(--surface); font-size: 12px; }
.segmented-control button.is-active { background: var(--surface); color: var(--text); box-shadow: var(--shadow-sm); }
.review-search { display: flex; width: min(260px, 100%); align-items: center; gap: 8px; padding: 0 10px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--text-soft); }
.review-search input { width: 100%; min-height: 34px; padding: 0; border: 0 !important; box-shadow: none !important; background: transparent !important; font-size: 13px; }
.review-list { overflow: hidden; }
.review-item { display: flex; align-items: center; justify-content: space-between; gap: 18px; padding: 15px 17px; border-bottom: 1px solid var(--border); }
.review-item:last-child { border-bottom: 0; }
.review-item:hover { background: var(--surface-subtle); }
.review-item__main { display: flex; min-width: 0; align-items: flex-start; gap: 12px; }
.article-type-icon { display: grid; width: 36px; height: 36px; flex: 0 0 36px; place-items: center; border-radius: 6px; background: var(--blue-soft); color: var(--blue); }
.review-item__copy { min-width: 0; }
.review-item__title { display: flex; min-width: 0; align-items: center; gap: 9px; }
.review-item__title a { overflow: hidden; color: var(--text); font-size: 13px; font-weight: 650; text-decoration: none; text-overflow: ellipsis; white-space: nowrap; }
.review-item__copy > p { overflow: hidden; max-width: 720px; margin: 4px 0 0; color: var(--text-soft); font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.review-item__meta { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 8px; color: var(--text-faint); font-size: 12px; text-transform: capitalize; }
.review-item__meta span { display: inline-flex; align-items: center; gap: 4px; }
.review-item__actions { display: flex; flex: 0 0 auto; gap: 6px; }
.loading-surface { display: flex; min-height: 130px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 900px) { .review-item { align-items: stretch; flex-direction: column; } .review-item__actions { justify-content: flex-end; } }
@media (max-width: 680px) { .review-toolbar { align-items: stretch; flex-direction: column; } .review-search { width: 100%; } .review-item__actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>

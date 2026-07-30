<template>
  <div class="page-stack">
    <div class="dashboard-welcome">
      <div>
        <p>{{ greeting }}</p>
        <h2>Editorial overview</h2>
        <span>Content progress, review work, and publishing activity across your projects.</span>
      </div>
      <div class="dashboard-welcome__actions">
        <button class="button button--compact" type="button" :disabled="pending" @click="loadDashboard">
          <RefreshCw :class="{ spin: pending }" :size="16" />
          Refresh
        </button>
        <NuxtLink class="button button--primary button--compact" to="/projects?new=1">
          <Plus :size="16" />
          New project
        </NuxtLink>
      </div>
    </div>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>

    <div class="metric-grid">
      <article v-for="metric in metrics" :key="metric.label" class="metric-card surface">
        <div class="metric-card__top">
          <span>{{ metric.label }}</span>
          <span class="metric-icon" :class="metric.tone"><component :is="metric.icon" :size="16" /></span>
        </div>
        <p class="metric-card__value">{{ metric.value }}</p>
        <small>{{ metric.detail }}</small>
      </article>
    </div>

    <div v-if="pending" class="loading-surface surface">
      <LoaderCircle class="spin" :size="18" />
      Loading workspace
    </div>

    <div v-else-if="projects.length === 0" class="empty-state">
      <div>
        <span class="empty-state__icon"><PanelsTopLeft :size="20" /></span>
        <h3>Create your first project</h3>
        <p>Projects isolate content, people, credentials, and publishing settings.</p>
        <NuxtLink class="button button--primary" to="/projects?new=1"><Plus :size="16" />New project</NuxtLink>
      </div>
    </div>

    <template v-else>
      <div class="dashboard-grid">
        <section class="surface overview-panel">
          <div class="panel-heading">
            <div>
              <p>Content activity</p>
              <h3>Recent articles</h3>
            </div>
            <label class="compact-select">
              <span class="sr-only">Project</span>
              <select v-model="selectedProjectID">
                <option value="all">All projects</option>
                <option v-for="project in projects" :key="project.id" :value="project.id">{{ project.name }}</option>
              </select>
              <ChevronDown :size="14" />
            </label>
          </div>

          <div v-if="recentArticles.length" class="content-table">
            <div class="content-row content-row--header">
              <span>Article</span>
              <span>Project</span>
              <span>Workflow</span>
              <span>Publication</span>
              <span></span>
            </div>
            <div v-for="item in recentArticles" :key="item.article.id" class="content-row">
              <span class="content-title">
                <span class="content-title__icon"><FileText :size="15" /></span>
                <span>
                  <strong>{{ item.article.title }}</strong>
                  <small>{{ labelize(item.article.articleType) }} · {{ item.article.locale.toUpperCase() }}</small>
                </span>
              </span>
              <span>{{ item.project.name }}</span>
              <span><i class="status-pill" :class="editorialStatusClass(item.article.editorialState)">{{ labelize(item.article.editorialState) }}</i></span>
              <span><i class="status-pill" :class="publicationStatusClass(item.article.publicationState)">{{ labelize(item.article.publicationState) }}</i></span>
              <NuxtLink class="icon-button" :to="`/projects/${item.project.id}/articles/${item.article.id}`" title="Open article" aria-label="Open article"><ArrowUpRight :size="16" /></NuxtLink>
            </div>
          </div>
          <div v-else class="empty-state empty-state--embedded">
            <div><span class="empty-state__icon"><FileText :size="20" /></span><h3>No content yet</h3><p>Create the first article in this project.</p></div>
          </div>
        </section>

        <aside class="dashboard-rail">
          <section class="surface focus-panel">
            <div class="panel-heading">
              <div><p>Current focus</p><h3>{{ selectedProject?.name || 'All projects' }}</h3></div>
              <span v-if="selectedProject" class="project-avatar">{{ initials(selectedProject.name) }}</span>
            </div>
            <template v-if="selectedProject">
              <dl class="project-details">
                <div><dt>Domain</dt><dd>{{ selectedProject.primaryDomain || 'Not configured' }}</dd></div>
                <div><dt>Locale</dt><dd>{{ selectedProject.defaultLocale.toUpperCase() }}</dd></div>
                <div><dt>Role</dt><dd>{{ labelize(selectedProject.role) }}</dd></div>
                <div><dt>Status</dt><dd><span class="status-pill" :class="{ 'status-pill--success': selectedProject.status === 'active' }">{{ selectedProject.status }}</span></dd></div>
              </dl>
              <div class="focus-actions">
                <NuxtLink :to="`/projects/${selectedProject.id}/articles/create`"><Plus :size="15" /><span>New article</span><ChevronRight :size="14" /></NuxtLink>
                <NuxtLink :to="`/projects/${selectedProject.id}/review`"><ListChecks :size="15" /><span>Review queue</span><ChevronRight :size="14" /></NuxtLink>
                <NuxtLink :to="`/projects/${selectedProject.id}/calendar`"><CalendarDays :size="15" /><span>Calendar</span><ChevronRight :size="14" /></NuxtLink>
                <NuxtLink :to="`/projects/${selectedProject.id}/settings`"><Settings2 :size="15" /><span>Project settings</span><ChevronRight :size="14" /></NuxtLink>
              </div>
            </template>
            <div v-else class="project-picker">
              <button v-for="project in projects" :key="project.id" type="button" @click="selectedProjectID = project.id">
                <span class="project-avatar">{{ initials(project.name) }}</span>
                <span><strong>{{ project.name }}</strong><small>{{ project.primaryDomain || project.slug }}</small></span>
                <ChevronRight :size="14" />
              </button>
            </div>
          </section>

          <section class="surface attention-panel">
            <div class="panel-heading">
              <div><p>Workflow</p><h3>Needs attention</h3></div>
              <span class="status-pill" :class="{ 'status-pill--warning': attentionItems.length }">{{ attentionItems.length }}</span>
            </div>
            <div v-if="attentionItems.length" class="attention-list">
              <NuxtLink v-for="item in attentionItems.slice(0, 5)" :key="item.article.id" :to="item.to">
                <span class="attention-dot" :class="item.tone" />
                <span><strong>{{ item.article.title }}</strong><small>{{ item.label }} · {{ item.project.name }}</small></span>
                <ChevronRight :size="14" />
              </NuxtLink>
            </div>
            <p v-else class="rail-empty">Nothing needs immediate attention</p>
          </section>
        </aside>
      </div>

      <section class="surface projects-strip">
        <div class="panel-heading">
          <div><p>Workspace</p><h3>Your projects</h3></div>
          <NuxtLink class="button button--compact" to="/projects">View all<ArrowUpRight :size="14" /></NuxtLink>
        </div>
        <div class="project-cards">
          <button
            v-for="project in projects.slice(0, 6)"
            :key="project.id"
            type="button"
            :class="{ 'is-selected': selectedProjectID === project.id }"
            @click="selectedProjectID = project.id"
          >
            <span class="project-avatar">{{ initials(project.name) }}</span>
            <span><strong>{{ project.name }}</strong><small>{{ project.primaryDomain || project.slug }}</small></span>
            <span class="status-pill" :class="{ 'status-pill--success': project.status === 'active' }">{{ project.status }}</span>
          </button>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import {
  ArrowUpRight,
  CalendarDays,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Clock3,
  FileText,
  Layers3,
  ListChecks,
  LoaderCircle,
  PanelsTopLeft,
  Plus,
  RefreshCw,
  Settings2
} from 'lucide-vue-next'
import type { AdminArticle, AdminProject } from '~/composables/useAdminApi'

type ProjectArticle = { project: AdminProject, article: AdminArticle }

const api = useAdminApi()
const projects = useAdminProjectsState()
const projectArticles = ref<ProjectArticle[]>([])
const selectedProjectID = ref('all')
const pending = ref(true)
const errorMessage = ref('')

const greeting = computed(() => {
  const hour = new Date().getHours()
  if (hour < 12) return 'Good morning'
  if (hour < 18) return 'Good afternoon'
  return 'Good evening'
})
const scopedArticles = computed(() => selectedProjectID.value === 'all'
  ? projectArticles.value
  : projectArticles.value.filter(item => item.project.id === selectedProjectID.value))
const selectedProject = computed(() => projects.value.find(project => project.id === selectedProjectID.value) || null)
const recentArticles = computed(() => [...scopedArticles.value]
  .sort((a, b) => dateValue(b.article.latestRevision?.createdAt || b.article.createdAt) - dateValue(a.article.latestRevision?.createdAt || a.article.createdAt))
  .slice(0, 8))
const metrics = computed(() => [
  { label: 'Projects', value: projects.value.length, detail: `${projects.value.filter(project => project.status === 'active').length} active`, icon: PanelsTopLeft, tone: 'metric-icon--green' },
  { label: 'Articles', value: projectArticles.value.length, detail: `${projectArticles.value.filter(item => item.article.publicationState === 'published').length} published`, icon: Layers3, tone: 'metric-icon--blue' },
  { label: 'In review', value: projectArticles.value.filter(item => item.article.editorialState === 'in_review').length, detail: 'Awaiting a decision', icon: ListChecks, tone: 'metric-icon--amber' },
  { label: 'Scheduled', value: projectArticles.value.filter(item => item.article.publicationState === 'scheduled').length, detail: 'Upcoming releases', icon: Clock3, tone: 'metric-icon--violet' }
])
const attentionItems = computed(() => projectArticles.value.flatMap(item => {
  if (item.article.editorialState === 'in_review') return [{ ...item, label: 'Ready for review', tone: 'attention-dot--amber', to: `/projects/${item.project.id}/articles/${item.article.id}` }]
  if (item.article.editorialState === 'changes_requested') return [{ ...item, label: 'Changes requested', tone: 'attention-dot--danger', to: `/projects/${item.project.id}/articles/${item.article.id}` }]
  if (item.article.editorialState === 'approved' && item.article.publicationState === 'unpublished') return [{ ...item, label: 'Approved, not published', tone: 'attention-dot--green', to: `/projects/${item.project.id}/articles/${item.article.id}` }]
  return []
}))

watch(projects, value => {
  if (selectedProjectID.value !== 'all' && !value.some(project => project.id === selectedProjectID.value)) selectedProjectID.value = 'all'
})

onMounted(loadDashboard)

async function loadDashboard() {
  pending.value = true
  errorMessage.value = ''
  try {
    const projectResponse = await api.listProjects()
    projects.value = projectResponse.data
    const articleResults = await Promise.allSettled(projects.value.map(async project => ({
      project,
      articles: (await api.listArticles(project.id)).data
    })))
    projectArticles.value = articleResults.flatMap(result => result.status === 'fulfilled'
      ? result.value.articles.map(article => ({ project: result.value.project, article }))
      : [])
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load the workspace dashboard.')
  } finally {
    pending.value = false
  }
}

function initials(value: string) {
  return value.split(/\s+/).filter(Boolean).slice(0, 2).map(part => part[0]?.toUpperCase()).join('')
}

function dateValue(value?: string) {
  if (!value) return 0
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? 0 : date.getTime()
}

function editorialStatusClass(state: string) {
  if (state === 'approved') return 'status-pill--success'
  if (state === 'changes_requested') return 'status-pill--warning'
  return ''
}

function publicationStatusClass(state: string) {
  if (state === 'published') return 'status-pill--success'
  if (state === 'scheduled') return 'status-pill--warning'
  return ''
}
</script>

<style scoped>
.dashboard-welcome { display: flex; align-items: flex-end; justify-content: space-between; gap: 18px; }
.dashboard-welcome p, .dashboard-welcome h2, .dashboard-welcome span { margin: 0; }
.dashboard-welcome p { color: var(--primary); font-size: 11px; font-weight: 650; }
.dashboard-welcome h2 { margin-top: 2px; font-size: 24px; }
.dashboard-welcome > div > span { display: block; margin-top: 5px; color: var(--text-soft); font-size: 12px; }
.dashboard-welcome__actions { display: flex; gap: 7px; }
.metric-card small { display: block; margin-top: 8px; color: var(--text-faint); font-size: 9px; }
.metric-icon { display: grid; width: 30px; height: 30px; place-items: center; border-radius: 6px; background: var(--surface-subtle); color: var(--text-soft); }
.metric-icon--green { background: var(--primary-soft); color: var(--primary); }
.metric-icon--blue { background: var(--blue-soft); color: var(--blue); }
.metric-icon--amber { background: var(--amber-soft); color: var(--amber); }
.metric-icon--violet { background: #f1eefe; color: #7454c0; }
.dark .metric-icon--violet { background: #27203d; color: #b7a0f3; }
.dashboard-grid { display: grid; grid-template-columns: minmax(0, 1fr) 310px; gap: 16px; align-items: start; }
.overview-panel, .focus-panel, .attention-panel, .projects-strip { overflow: hidden; }
.panel-heading { display: flex; min-height: 61px; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 15px; border-bottom: 1px solid var(--border); }
.panel-heading p, .panel-heading h3 { margin: 0; }
.panel-heading p { color: var(--text-soft); font-size: 9px; }
.panel-heading h3 { margin-top: 1px; font-size: 14px; }
.compact-select { position: relative; display: flex; align-items: center; }
.compact-select select { min-height: 32px; padding: 5px 28px 5px 9px; appearance: none; border: 1px solid var(--border) !important; border-radius: 5px; background: var(--surface) !important; color: var(--text); font-size: 9px; }
.compact-select svg { position: absolute; right: 8px; pointer-events: none; color: var(--text-soft); }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0, 0, 0, 0); }
.content-table { overflow-x: auto; }
.content-row { display: grid; min-width: 720px; grid-template-columns: minmax(250px, 1.5fr) minmax(110px, .7fr) 115px 110px 30px; gap: 12px; align-items: center; min-height: 58px; padding: 8px 14px; border-bottom: 1px solid var(--border); font-size: 9px; }
.content-row:last-child { border-bottom: 0; }
.content-row--header { min-height: 34px; background: var(--surface-subtle); color: var(--text-soft); font-weight: 650; text-transform: uppercase; }
.content-title { display: flex; min-width: 0; align-items: center; gap: 9px; }
.content-title__icon { display: grid; width: 32px; height: 32px; flex: 0 0 32px; place-items: center; border-radius: 6px; background: var(--blue-soft); color: var(--blue); }
.content-title > span:last-child { display: flex; min-width: 0; flex-direction: column; }
.content-title strong, .content-title small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.content-title strong { color: var(--text); font-size: 10px; }
.content-title small { margin-top: 3px; color: var(--text-soft); font-size: 8px; text-transform: capitalize; }
.content-row i { font-style: normal; }
.dashboard-rail { display: grid; gap: 14px; }
.project-avatar { display: grid; width: 34px; height: 34px; flex: 0 0 34px; place-items: center; border-radius: 7px; background: #e8b95a; color: #2a2112; font-size: 10px; font-weight: 750; }
.project-details { display: grid; grid-template-columns: 1fr 1fr; gap: 1px; margin: 0; background: var(--border); }
.project-details > div { min-width: 0; padding: 11px 14px; background: var(--surface); }
.project-details dt { color: var(--text-soft); font-size: 8px; }
.project-details dd { overflow: hidden; margin: 3px 0 0; font-size: 9px; font-weight: 600; text-overflow: ellipsis; text-transform: capitalize; white-space: nowrap; }
.focus-actions { display: grid; }
.focus-actions a { display: grid; grid-template-columns: 18px minmax(0, 1fr) 15px; align-items: center; gap: 8px; min-height: 39px; padding: 8px 14px; border-top: 1px solid var(--border); color: var(--text-soft); font-size: 10px; text-decoration: none; }
.focus-actions a:hover { background: var(--surface-subtle); color: var(--text); }
.project-picker button, .project-cards button { border: 0; background: transparent; color: var(--text); cursor: pointer; }
.project-picker button { display: grid; width: 100%; grid-template-columns: 34px minmax(0, 1fr) 15px; align-items: center; gap: 9px; padding: 10px 14px; border-top: 1px solid var(--border); text-align: left; }
.project-picker button > span:nth-child(2), .attention-list a > span:nth-child(2), .project-cards button > span:nth-child(2) { display: flex; min-width: 0; flex-direction: column; }
.project-picker strong, .project-picker small, .attention-list strong, .attention-list small, .project-cards strong, .project-cards small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.project-picker strong, .attention-list strong, .project-cards strong { font-size: 10px; }
.project-picker small, .attention-list small, .project-cards small { margin-top: 2px; color: var(--text-soft); font-size: 8px; }
.attention-list a { display: grid; grid-template-columns: 8px minmax(0, 1fr) 15px; align-items: center; gap: 9px; padding: 10px 14px; border-top: 1px solid var(--border); color: var(--text); text-decoration: none; }
.attention-list a:hover { background: var(--surface-subtle); }
.attention-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--amber); }
.attention-dot--danger { background: var(--danger); }
.attention-dot--green { background: var(--primary); }
.rail-empty { margin: 0; padding: 16px 14px; color: var(--text-soft); font-size: 10px; }
.project-cards { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 1px; background: var(--border); }
.project-cards button { display: grid; grid-template-columns: 34px minmax(0, 1fr); align-items: center; gap: 9px; padding: 13px 14px; background: var(--surface); text-align: left; }
.project-cards button:hover, .project-cards button.is-selected { background: var(--surface-subtle); box-shadow: inset 0 2px 0 var(--primary); }
.project-cards .status-pill { grid-column: 2; justify-self: start; margin-top: -2px; }
.empty-state--embedded { min-height: 260px; border: 0; border-radius: 0; box-shadow: none; }
.empty-state .button { margin-top: 14px; }
.loading-surface { display: flex; min-height: 140px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1100px) { .dashboard-grid { grid-template-columns: 1fr; } .dashboard-rail { grid-template-columns: 1fr 1fr; } }
@media (max-width: 760px) { .dashboard-welcome { align-items: stretch; flex-direction: column; } .dashboard-welcome__actions { justify-content: flex-end; } .dashboard-rail { grid-template-columns: 1fr; } .project-cards { grid-template-columns: 1fr 1fr; } }
@media (max-width: 520px) { .project-cards { grid-template-columns: 1fr; } }
</style>

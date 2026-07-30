<template>
  <div class="page-stack">
    <div class="page-heading">
      <div>
        <p>Tenant workspaces, domains, access roles, and lifecycle state.</p>
      </div>
      <button class="button button--primary button--compact" type="button" @click="formOpen = !formOpen">
        <Plus :size="16" />
        New project
      </button>
    </div>

    <form v-if="formOpen" class="surface project-form" @submit.prevent="createProject">
      <div class="project-form__heading">
        <span class="project-form__icon"><PanelsTopLeft :size="18" /></span>
        <div><p>New project</p><h3>Project details</h3></div>
        <button class="icon-button" type="button" title="Close" aria-label="Close" @click="formOpen = false"><X :size="17" /></button>
      </div>
      <div class="project-form__body">
        <label class="field">
          <span>Project name</span>
          <input v-model.trim="form.name" required placeholder="Acme editorial">
        </label>
        <label class="field">
          <span>Project slug</span>
          <input v-model.trim="form.slug" required placeholder="acme-editorial" @input="slugTouched = true">
        </label>
        <label class="field">
          <span>Primary domain</span>
          <input v-model.trim="form.primaryDomain" placeholder="www.example.com">
        </label>
        <label class="field">
          <span>Blog path</span>
          <input v-model.trim="form.blogBasePath" required placeholder="/blog">
        </label>
        <label class="field">
          <span>Default locale</span>
          <input v-model.trim="form.defaultLocale" required placeholder="en">
        </label>
        <label class="field">
          <span>Timezone</span>
          <input v-model.trim="form.timezone" required placeholder="UTC">
        </label>
      </div>
      <div class="project-form__footer">
        <button class="button" type="button" @click="formOpen = false">Cancel</button>
        <button class="button button--primary" type="submit" :disabled="creating || !canCreate">
          <LoaderCircle v-if="creating" class="spin" :size="16" />
          <Plus v-else :size="16" />
          Create project
        </button>
      </div>
    </form>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success">{{ successMessage }}</p>

    <div class="projects-toolbar surface surface--subtle">
      <label class="project-search">
        <Search :size="16" />
        <input v-model.trim="search" type="search" placeholder="Search projects" aria-label="Search projects">
      </label>
      <div class="project-filters">
        <select v-model="statusFilter" class="input" aria-label="Project status">
          <option value="all">All statuses</option>
          <option value="active">Active</option>
          <option value="suspended">Suspended</option>
          <option value="archived">Archived</option>
        </select>
        <button class="icon-button surface" type="button" title="Refresh projects" aria-label="Refresh projects" :disabled="pending" @click="loadProjects">
          <RefreshCw :class="{ spin: pending }" :size="16" />
        </button>
      </div>
    </div>

    <div v-if="pending" class="loading-surface surface"><LoaderCircle class="spin" :size="18" />Loading projects</div>

    <div v-else-if="filteredProjects.length === 0" class="empty-state">
      <div>
        <span class="empty-state__icon"><PanelsTopLeft :size="20" /></span>
        <h3>{{ projects.length ? 'No matching projects' : 'No projects yet' }}</h3>
        <p>{{ projects.length ? 'Try another search or status.' : 'Create the first project in this workspace.' }}</p>
      </div>
    </div>

    <div v-else class="projects-grid">
      <article v-for="project in filteredProjects" :key="project.id" class="project-card surface">
        <div class="project-card__top">
          <span class="project-avatar">{{ initials(project.name) }}</span>
          <div>
            <h3>{{ project.name }}</h3>
            <p>{{ project.slug }}</p>
          </div>
          <button class="icon-button" type="button" title="Project options" aria-label="Project options"><Ellipsis :size="17" /></button>
        </div>
        <div class="project-card__status">
          <span class="status-pill" :class="{ 'status-pill--success': project.status === 'active', 'status-pill--warning': project.status === 'suspended' }">{{ project.status }}</span>
          <span>{{ labelize(project.role) }}</span>
        </div>
        <dl>
          <div><dt><Globe2 :size="14" />Domain</dt><dd>{{ project.primaryDomain || 'Not configured' }}</dd></div>
          <div><dt><Languages :size="14" />Locale</dt><dd>{{ project.defaultLocale.toUpperCase() }}</dd></div>
          <div><dt><FolderKanban :size="14" />Blog path</dt><dd>{{ project.blogBasePath }}</dd></div>
          <div><dt><Clock3 :size="14" />Timezone</dt><dd>{{ project.timezone || 'UTC' }}</dd></div>
        </dl>
        <div class="project-card__actions">
          <NuxtLink class="button" :to="`/projects/${project.id}/articles`">Open project<ArrowRight :size="15" /></NuxtLink>
          <NuxtLink class="icon-button surface" :to="`/projects/${project.id}/articles/create`" title="New article" aria-label="New article"><Plus :size="16" /></NuxtLink>
          <NuxtLink class="icon-button surface" :to="`/projects/${project.id}/settings`" title="Project settings" aria-label="Project settings"><Settings2 :size="16" /></NuxtLink>
        </div>
      </article>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  ArrowRight,
  Clock3,
  Ellipsis,
  FolderKanban,
  Globe2,
  Languages,
  LoaderCircle,
  PanelsTopLeft,
  Plus,
  RefreshCw,
  Search,
  Settings2,
  X
} from 'lucide-vue-next'
import type { AdminProject } from '~/composables/useAdminApi'

const route = useRoute()
const api = useAdminApi()
const projects = useAdminProjectsState()
const pending = ref(true)
const creating = ref(false)
const formOpen = ref(false)
const slugTouched = ref(false)
const search = ref('')
const statusFilter = ref('all')
const errorMessage = ref('')
const successMessage = ref('')
const form = reactive({
  name: '',
  slug: '',
  primaryDomain: '',
  blogBasePath: '/blog',
  defaultLocale: 'en',
  timezone: 'UTC'
})
const canCreate = computed(() => form.name.length >= 2 && form.slug.length >= 2 && Boolean(form.blogBasePath) && Boolean(form.defaultLocale))
const filteredProjects = computed(() => {
  const term = search.value.toLowerCase()
  return projects.value.filter(project => {
    const statusMatches = statusFilter.value === 'all' || project.status === statusFilter.value
    const searchMatches = !term || `${project.name} ${project.slug} ${project.primaryDomain || ''}`.toLowerCase().includes(term)
    return statusMatches && searchMatches
  })
})

watch(() => form.name, value => {
  if (!slugTouched.value) form.slug = slugify(value)
})

onMounted(() => {
  formOpen.value = route.query.new === '1'
  loadProjects()
})

async function loadProjects() {
  pending.value = true
  errorMessage.value = ''
  try {
    projects.value = (await api.listProjects()).data
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load projects.')
  } finally {
    pending.value = false
  }
}

async function createProject() {
  if (!canCreate.value) return
  creating.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const response = await api.createProject({
      name: form.name,
      slug: form.slug,
      primaryDomain: form.primaryDomain,
      blogBasePath: normalizeBlogPath(form.blogBasePath),
      defaultLocale: form.defaultLocale,
      supportedLocales: [form.defaultLocale],
      timezone: form.timezone
    })
    projects.value = [response.data, ...projects.value.filter(project => project.id !== response.data.id)]
    successMessage.value = 'Project created.'
    formOpen.value = false
    await navigateTo(`/projects/${response.data.id}/articles`)
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create the project.')
  } finally {
    creating.value = false
  }
}

function initials(value: string) {
  return value.split(/\s+/).filter(Boolean).slice(0, 2).map(part => part[0]?.toUpperCase()).join('')
}

function normalizeBlogPath(value: string) {
  const path = value.trim() || '/blog'
  return path.startsWith('/') ? path : `/${path}`
}
</script>

<style scoped>
.project-form { overflow: hidden; }
.project-form__heading { display: flex; align-items: center; gap: 11px; min-height: 66px; padding: 13px 16px; border-bottom: 1px solid var(--border); }
.project-form__icon { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 7px; background: var(--primary-soft); color: var(--primary); }
.project-form__heading p, .project-form__heading h3 { margin: 0; }
.project-form__heading p { color: var(--text-soft); font-size: 9px; }
.project-form__heading h3 { margin-top: 1px; font-size: 14px; }
.project-form__heading .icon-button { margin-left: auto; }
.project-form__body { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; padding: 18px; }
.project-form__footer { display: flex; justify-content: flex-end; gap: 7px; padding: 12px 18px; border-top: 1px solid var(--border); background: var(--surface-subtle); }
.projects-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px; }
.project-search { display: flex; width: min(360px, 100%); align-items: center; gap: 8px; padding: 0 10px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--text-soft); }
.project-search input { width: 100%; min-height: 34px; padding: 0; border: 0 !important; box-shadow: none !important; background: transparent !important; font-size: 11px; }
.project-filters { display: flex; gap: 7px; }
.project-filters .input { width: 132px; min-height: 34px; padding-block: 5px; font-size: 10px; }
.projects-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.project-card { overflow: hidden; }
.project-card__top { display: grid; grid-template-columns: 40px minmax(0, 1fr) 34px; align-items: center; gap: 10px; padding: 15px; }
.project-avatar { display: grid; width: 40px; height: 40px; place-items: center; border-radius: 7px; background: #e8b95a; color: #2a2112; font-size: 11px; font-weight: 750; }
.project-card__top > div { min-width: 0; }
.project-card h3, .project-card p { overflow: hidden; margin: 0; text-overflow: ellipsis; white-space: nowrap; }
.project-card h3 { font-size: 13px; }
.project-card p { margin-top: 3px; color: var(--text-soft); font-size: 9px; }
.project-card__status { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 9px 15px; border-block: 1px solid var(--border); background: var(--surface-subtle); }
.project-card__status > span:last-child { color: var(--text-soft); font-size: 9px; text-transform: capitalize; }
.project-card dl { display: grid; grid-template-columns: 1fr 1fr; gap: 1px; margin: 0; background: var(--border); }
.project-card dl > div { min-width: 0; padding: 11px 14px; background: var(--surface); }
.project-card dt { display: flex; align-items: center; gap: 5px; color: var(--text-soft); font-size: 8px; }
.project-card dd { overflow: hidden; margin: 4px 0 0; font-size: 9px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.project-card__actions { display: flex; gap: 7px; padding: 12px 14px; border-top: 1px solid var(--border); }
.project-card__actions .button { flex: 1; justify-content: space-between; }
.loading-surface { display: flex; min-height: 140px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1180px) { .projects-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 780px) { .project-form__body { grid-template-columns: 1fr 1fr; } .projects-grid { grid-template-columns: 1fr; } }
@media (max-width: 580px) { .projects-toolbar { align-items: stretch; flex-direction: column; } .project-search { width: 100%; } .project-filters { justify-content: space-between; } .project-form__body { grid-template-columns: 1fr; } }
</style>

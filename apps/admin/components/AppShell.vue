<template>
  <div class="app-shell">
    <a class="skip-link" href="#main-content">Skip to content</a>

    <div v-if="mobileOpen" class="app-shell__backdrop" aria-hidden="true" @click="mobileOpen = false" />

    <aside class="app-sidebar" :class="{ 'app-sidebar--open': mobileOpen }">
      <div class="app-sidebar__brand">
        <NuxtLink class="brand-mark" to="/dashboard" aria-label="SEO Blog CMS dashboard">
          <span class="brand-mark__icon"><PenLine :size="19" /></span>
          <span class="brand-mark__text">
            <strong>Editorial</strong>
            <small>SEO Blog CMS</small>
          </span>
        </NuxtLink>
        <button class="icon-button app-sidebar__close" type="button" title="Close navigation" aria-label="Close navigation" @click="mobileOpen = false">
          <X :size="18" />
        </button>
      </div>

      <div v-if="projects.length" class="project-switcher">
        <span class="project-switcher__label">Current project</span>
        <div class="project-switcher__control">
          <span class="project-switcher__avatar">{{ projectInitials }}</span>
          <select v-model="selectedProjectID" aria-label="Current project" @change="switchProject">
            <option v-for="project in projects" :key="project.id" :value="project.id">
              {{ project.name }}
            </option>
          </select>
          <ChevronsUpDown :size="15" aria-hidden="true" />
        </div>
      </div>

      <nav class="app-navigation" aria-label="Primary navigation" @click="mobileOpen = false">
        <div class="nav-group">
          <p class="nav-group__label">Workspace</p>
          <NuxtLink class="nav-item" :class="{ 'is-active': route.path === '/dashboard' }" to="/dashboard">
            <LayoutDashboard :size="18" />
            <span>Dashboard</span>
          </NuxtLink>
          <NuxtLink class="nav-item" :class="{ 'is-active': route.path === '/projects' }" to="/projects">
            <PanelsTopLeft :size="18" />
            <span>Projects</span>
          </NuxtLink>
          <NuxtLink class="nav-item" :class="{ 'is-active': route.path === '/workspaces' }" to="/workspaces">
            <Building2 :size="18" />
            <span>Workspaces</span>
          </NuxtLink>
        </div>

        <template v-if="activeProjectID">
          <div class="nav-group">
            <p class="nav-group__label">Publishing</p>
            <NuxtLink class="nav-item" :class="{ 'is-active': isProjectSection('articles') }" :to="projectPath('articles')">
              <Files :size="18" />
              <span>Content</span>
            </NuxtLink>
            <NuxtLink class="nav-item" :class="{ 'is-active': isProjectSection('calendar') }" :to="projectPath('calendar')">
              <CalendarDays :size="18" />
              <span>Calendar</span>
            </NuxtLink>
            <NuxtLink class="nav-item" :class="{ 'is-active': isProjectSection('categories') || isProjectSection('tags') }" :to="projectPath('categories')">
              <Tags :size="18" />
              <span>Taxonomy</span>
            </NuxtLink>
            <NuxtLink class="nav-item" :class="{ 'is-active': isProjectSection('series') }" :to="projectPath('series')">
              <GalleryVerticalEnd :size="18" />
              <span>Series</span>
            </NuxtLink>
            <NuxtLink class="nav-item" :class="{ 'is-active': isProjectSection('media') }" :to="projectPath('media')">
              <Images :size="18" />
              <span>Media</span>
            </NuxtLink>
          </div>

          <div class="nav-group">
            <p class="nav-group__label">Editorial</p>
            <NuxtLink class="nav-item" :class="{ 'is-active': isProjectSection('review') }" :to="projectPath('review')">
              <ListChecks :size="18" />
              <span>Review queue</span>
            </NuxtLink>
            <NuxtLink class="nav-item" :class="{ 'is-active': isProjectSection('authors') }" :to="projectPath('authors')">
              <UsersRound :size="18" />
              <span>Authors</span>
            </NuxtLink>
            <NuxtLink class="nav-item" :class="{ 'is-active': isProjectSection('ai') }" :to="projectPath('ai')">
              <Sparkles :size="18" />
              <span>AI workspace</span>
            </NuxtLink>
          </div>

          <div class="nav-group">
            <p class="nav-group__label">Project</p>
            <NuxtLink v-if="canManageProject" class="nav-item" :class="{ 'is-active': isProjectSection('members') }" :to="projectPath('members')">
              <UserRoundCog :size="18" />
              <span>Members</span>
            </NuxtLink>
            <NuxtLink class="nav-item" :class="{ 'is-active': isProjectSection('api-keys') }" :to="projectPath('api-keys')">
              <KeyRound :size="18" />
              <span>API keys</span>
            </NuxtLink>
            <NuxtLink v-if="canManageProject" class="nav-item" :class="{ 'is-active': isProjectSection('integrations') }" :to="projectPath('integrations')">
              <Webhook :size="18" />
              <span>Integrations</span>
            </NuxtLink>
            <NuxtLink v-if="canManageProject" class="nav-item" :class="{ 'is-active': isProjectSection('operations') }" :to="projectPath('operations')">
              <Activity :size="18" />
              <span>Operations</span>
            </NuxtLink>
            <NuxtLink v-if="canManageProject" class="nav-item" :class="{ 'is-active': isProjectSection('audit-events') }" :to="projectPath('audit-events')">
              <ScrollText :size="18" />
              <span>Audit log</span>
            </NuxtLink>
            <NuxtLink v-if="canManageProject" class="nav-item" :class="{ 'is-active': isProjectSection('settings') }" :to="projectPath('settings')">
              <Settings2 :size="18" />
              <span>Settings</span>
            </NuxtLink>
          </div>
        </template>
      </nav>

      <div class="app-sidebar__footer">
        <div class="sidebar-user">
          <span class="sidebar-user__avatar">{{ userInitials }}</span>
          <span class="sidebar-user__copy">
            <strong>{{ currentUser?.email || 'Signed in' }}</strong>
            <small>{{ currentProject?.role ? labelize(currentProject.role) : 'Workspace access' }}</small>
          </span>
          <button class="icon-button" type="button" title="Log out" aria-label="Log out" @click="logout">
            <LogOut :size="17" />
          </button>
        </div>
      </div>
    </aside>

    <div class="app-workspace">
      <header class="app-topbar">
        <div class="app-topbar__leading">
          <button class="icon-button app-topbar__menu" type="button" title="Open navigation" aria-label="Open navigation" @click="mobileOpen = true">
            <Menu :size="19" />
          </button>
          <div class="page-context">
            <p>{{ eyebrow }}</p>
            <h1>{{ pageTitle }}</h1>
          </div>
        </div>

        <div class="app-topbar__actions">
          <NuxtLink v-if="activeProjectID && !route.path.endsWith('/articles/create')" class="button button--primary button--compact" :to="projectPath('articles/create')">
            <Plus :size="17" />
            <span>New article</span>
          </NuxtLink>
          <div class="theme-switcher" role="group" aria-label="Color theme">
            <button
              v-for="option in themeOptions"
              :key="option.value"
              class="theme-switcher__option"
              :class="{ 'is-active': colorMode.preference === option.value }"
              type="button"
              :title="`${option.label} theme`"
              :aria-label="`${option.label} theme`"
              :aria-pressed="colorMode.preference === option.value"
              @click="colorMode.preference = option.value"
            >
              <component :is="option.icon" :size="15" />
            </button>
          </div>
        </div>
      </header>

      <main id="main-content" class="app-content" tabindex="-1">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  Activity,
  Building2,
  CalendarDays,
  ChevronsUpDown,
  Files,
  GalleryVerticalEnd,
  Images,
  KeyRound,
  Laptop,
  LayoutDashboard,
  ListChecks,
  LogOut,
  Menu,
  Moon,
  PanelsTopLeft,
  PenLine,
  Plus,
  ScrollText,
  Settings2,
  Sparkles,
  Sun,
  Tags,
  UserRoundCog,
  UsersRound,
  Webhook,
  X
} from 'lucide-vue-next'
import type { AdminProject } from '~/composables/useAdminApi'

type AdminUser = {
  id: string
  email: string
  status: string
}

const route = useRoute()
const api = useAdminApi()
const colorMode = useColorMode()
const mobileOpen = ref(false)
const projects = useAdminProjectsState()
const currentUser = useState<AdminUser | null>('admin-user', () => null)
const selectedProjectID = ref('')

const routeProjectID = computed(() => String(route.params.projectId || ''))
const activeProjectID = computed(() => routeProjectID.value || selectedProjectID.value || projects.value[0]?.id || '')
const currentProject = computed(() => projects.value.find(project => project.id === activeProjectID.value) || null)
const canManageProject = computed(() => ['project_owner', 'project_admin'].includes(currentProject.value?.role || ''))
const projectInitials = computed(() => initials(currentProject.value?.name || 'Project'))
const userInitials = computed(() => initials(currentUser.value?.email?.split('@')[0] || 'User'))

const section = computed(() => {
  const path = route.path
  if (!routeProjectID.value) return ''
  const remainder = path.split(`/projects/${routeProjectID.value}/`)[1] || ''
  return remainder.split('/')[0] || 'articles'
})

const pageTitle = computed(() => {
  if (route.path === '/dashboard') return 'Dashboard'
  if (route.path === '/projects') return 'Projects'
  if (route.path === '/workspaces') return 'Workspaces'
  const titles: Record<string, string> = {
    articles: route.path.endsWith('/create') ? 'Create article' : route.params.articleId ? 'Article editor' : 'Content',
    calendar: 'Editorial calendar',
    categories: 'Taxonomy',
    tags: 'Tags',
    series: 'Series',
    media: 'Media library',
    review: 'Review queue',
    authors: 'Authors',
    ai: 'AI workspace',
    members: 'Members',
    'api-keys': 'API keys',
    integrations: 'Integrations',
    operations: 'Operations',
    'audit-events': 'Audit log',
    settings: 'Project settings'
  }
  return titles[section.value] || 'Editorial'
})

const eyebrow = computed(() => currentProject.value?.name || (['/projects', '/workspaces'].includes(route.path) ? 'Workspace' : 'Overview'))
const themeOptions = [
  { value: 'system', label: 'System', icon: Laptop },
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon }
]

watch(routeProjectID, value => {
  if (value) selectedProjectID.value = value
}, { immediate: true })

watch(() => route.fullPath, () => {
  mobileOpen.value = false
})

onMounted(loadShellData)

async function loadShellData() {
  try {
    const [projectResponse, userResponse] = await Promise.all([
      api.listProjects(),
      api.currentUser()
    ])
    projects.value = projectResponse.data
    currentUser.value = userResponse.data
    if (!selectedProjectID.value) selectedProjectID.value = projects.value[0]?.id || ''
  } catch (error) {
    const status = (error as { status?: number, statusCode?: number })?.status || (error as { statusCode?: number })?.statusCode
    if (status === 401) await navigateTo('/', { replace: true })
  }
}

async function switchProject() {
  const projectID = selectedProjectID.value
  if (!projectID) return
  const destination = section.value ? `/projects/${projectID}/${section.value}` : `/projects/${projectID}/articles`
  await navigateTo(destination)
}

async function logout() {
  try {
    await api.logout()
  } finally {
    projects.value = []
    currentUser.value = null
    await navigateTo('/', { replace: true })
  }
}

function projectPath(child: string) {
  return `/projects/${activeProjectID.value}/${child}`
}

function isProjectSection(value: string) {
  return section.value === value
}

function initials(value: string) {
  const parts = value.trim().split(/[\s._-]+/).filter(Boolean)
  return parts.slice(0, 2).map(part => part[0]?.toUpperCase()).join('') || 'SB'
}
</script>

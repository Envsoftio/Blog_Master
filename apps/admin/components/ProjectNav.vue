<template>
  <aside class="space-y-3">
    <label class="block space-y-2 rounded-lg border border-[#cfd8d1] bg-white p-3 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
      <span class="text-xs font-medium uppercase text-[#667169] dark:text-[#aeb8b0]">Project</span>
      <select
        v-model="selectedProjectID"
        class="h-10 w-full rounded-md border border-[#bfcac3] bg-white px-3 py-2 text-sm text-[#20231f] dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]"
        @change="switchProject"
      >
        <option v-if="!loadedProjectIDs.has(projectId)" :value="projectId">{{ project?.name || 'Selected project' }}</option>
        <option v-for="item in projects" :key="item.id" :value="item.id">{{ item.name }}</option>
      </select>
    </label>

    <nav class="flex gap-2 overflow-x-auto lg:block lg:space-y-2" aria-label="Project navigation">
      <NuxtLink :class="linkClass('dashboard')" to="/dashboard">
        <LayoutDashboard class="h-4 w-4 shrink-0" />
        <span>Dashboard</span>
      </NuxtLink>
      <NuxtLink :class="linkClass('projects')" to="/projects">
        <LayoutGrid class="h-4 w-4 shrink-0" />
        <span>Projects</span>
      </NuxtLink>
      <NuxtLink :class="linkClass('articles')" :to="`/projects/${projectId}/articles`">
        <FileText class="h-4 w-4 shrink-0" />
        <span>Articles</span>
      </NuxtLink>
      <NuxtLink :class="linkClass('categories')" :to="`/projects/${projectId}/categories`">
        <FolderTree class="h-4 w-4 shrink-0" />
        <span>Categories</span>
      </NuxtLink>
      <NuxtLink :class="linkClass('series')" :to="`/projects/${projectId}/series`">
        <PanelsTopLeft class="h-4 w-4 shrink-0" />
        <span>Series</span>
      </NuxtLink>
      <NuxtLink :class="linkClass('authors')" :to="`/projects/${projectId}/authors`">
        <UsersRound class="h-4 w-4 shrink-0" />
        <span>Authors</span>
      </NuxtLink>
      <NuxtLink :class="linkClass('api-keys')" :to="`/projects/${projectId}/api-keys`">
        <KeyRound class="h-4 w-4 shrink-0" />
        <span>API keys</span>
      </NuxtLink>
      <NuxtLink v-if="canManageProject" :class="linkClass('members')" :to="`/projects/${projectId}/members`">
        <ShieldCheck class="h-4 w-4 shrink-0" />
        <span>Members</span>
      </NuxtLink>
      <NuxtLink v-if="canManageProject" :class="linkClass('audit')" :to="`/projects/${projectId}/audit-events`">
        <ScrollText class="h-4 w-4 shrink-0" />
        <span>Audit</span>
      </NuxtLink>
      <NuxtLink v-if="canManageProject" :class="linkClass('settings')" :to="`/projects/${projectId}/settings`">
        <Settings class="h-4 w-4 shrink-0" />
        <span>Settings</span>
      </NuxtLink>
    </nav>
  </aside>
</template>

<script setup lang="ts">
import {
  FileText,
  FolderTree,
  KeyRound,
  LayoutDashboard,
  LayoutGrid,
  PanelsTopLeft,
  ScrollText,
  Settings,
  ShieldCheck,
  UsersRound
} from 'lucide-vue-next'

type APIListEnvelope<T> = {
  data: T[]
}

type AdminProject = {
  id: string
  name: string
  role?: string
}

const props = defineProps<{
  projectId: string
  project?: AdminProject | null
  active: string
}>()

const route = useRoute()
const projects = ref<AdminProject[]>([])
const selectedProjectID = ref(props.projectId)

const loadedProjectIDs = computed(() => new Set(projects.value.map(project => project.id)))
const currentRole = computed(() => props.project?.role || projects.value.find(project => project.id === props.projectId)?.role || '')
const canManageProject = computed(() => currentRole.value === 'project_owner' || currentRole.value === 'project_admin')

watch(() => props.projectId, (value) => {
  selectedProjectID.value = value
})

onMounted(fetchProjects)

async function fetchProjects() {
  try {
    const response = await $fetch<APIListEnvelope<AdminProject>>('/api/v1/projects', {
      credentials: 'include',
      query: { limit: 100 }
    })
    projects.value = apiListData(response)
  } catch {
    projects.value = []
  }
}

async function switchProject() {
  const nextProjectID = selectedProjectID.value
  if (!nextProjectID || nextProjectID === props.projectId) return
  await navigateTo(sectionPath(nextProjectID, props.active))
}

function sectionPath(projectID: string, section: string) {
  const articleID = String(route.params.articleId || '')
  if (section === 'article-detail' && articleID) return `/projects/${projectID}/articles`
  if (section === 'categories') return `/projects/${projectID}/categories`
  if (section === 'series') return `/projects/${projectID}/series`
  if (section === 'authors') return `/projects/${projectID}/authors`
  if (section === 'api-keys') return `/projects/${projectID}/api-keys`
  if (section === 'members') return `/projects/${projectID}/members`
  if (section === 'audit') return `/projects/${projectID}/audit-events`
  if (section === 'settings') return `/projects/${projectID}/settings`
  return `/projects/${projectID}/articles`
}

function linkClass(section: string) {
  const isActive = props.active === section || (props.active === 'article-detail' && section === 'articles')
  return [
    'inline-flex min-h-10 items-center gap-2 whitespace-nowrap rounded-md px-3 py-2 text-sm transition lg:flex',
    isActive
      ? 'bg-white font-medium text-[#20231f] shadow-sm dark:bg-[#252b28] dark:text-[#f2f3ef]'
      : 'text-[#555f58] hover:bg-white/70 dark:text-[#b8c2bb] dark:hover:bg-[#252b28]/70'
  ]
}
</script>

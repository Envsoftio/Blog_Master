<template>
  <div class="page-stack">
    <div class="page-heading">
      <div>
        <p>Public byline profiles, credentials, affiliations, and account links for this project.</p>
      </div>
      <div class="author-heading-actions">
        <button class="button button--compact" type="button" :disabled="pending" @click="refresh">
          <RefreshCw :class="{ spin: pending }" :size="16" />
          Refresh
        </button>
        <button v-if="canManageAuthors" class="button button--primary button--compact" type="button" @click="openNewAuthor">
          <Plus :size="16" />
          New author
        </button>
      </div>
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

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success">{{ successMessage }}</p>

    <div class="authors-layout">
      <section class="authors-directory">
        <div class="author-toolbar surface surface--subtle">
          <label class="author-search">
            <Search :size="16" />
            <input v-model.trim="search" type="search" placeholder="Search authors" aria-label="Search authors">
          </label>
          <div class="segmented-control" role="tablist" aria-label="Author view">
            <button
              v-for="filter in filterOptions"
              :key="filter.value"
              type="button"
              :class="{ 'is-active': activeFilter === filter.value }"
              @click="activeFilter = filter.value"
            >
              {{ filter.label }}
              <span>{{ filter.count }}</span>
            </button>
          </div>
        </div>

        <div v-if="pending" class="loading-surface surface">
          <LoaderCircle class="spin" :size="18" />
          Loading authors
        </div>

        <div v-else-if="filteredAuthors.length === 0" class="empty-state">
          <div>
            <span class="empty-state__icon"><UsersRound :size="20" /></span>
            <h3>{{ authors.length ? 'No authors match' : 'No authors yet' }}</h3>
            <p>{{ authors.length ? 'Try another search or status view.' : 'Create a profile before assigning public bylines.' }}</p>
          </div>
        </div>

        <div v-else class="author-list surface">
          <article v-for="author in filteredAuthors" :key="author.id" class="author-row">
            <span class="author-avatar" :class="{ 'author-avatar--inactive': normalizedStatus(author) !== 'active' }">
              {{ initials(author.displayName) }}
            </span>

            <div class="author-row__main">
              <div class="author-row__title">
                <h3>{{ author.displayName }}</h3>
                <span class="status-pill" :class="authorStatusClass(author)">{{ labelize(normalizedStatus(author)) }}</span>
              </div>
              <p>{{ author.shortBio || author.fullBio || 'No biography added.' }}</p>

              <div class="author-row__meta">
                <span><BadgeCheck :size="13" />/{{ author.slug }}</span>
                <span><Briefcase :size="13" />{{ affiliation(author) }}</span>
                <a v-if="author.profileUrl" :href="author.profileUrl" rel="noreferrer" target="_blank">
                  <Link2 :size="13" />
                  {{ profileLabel(author.profileUrl) }}
                </a>
                <span v-if="canManageProject">
                  <UserCheck :size="13" />
                  {{ author.loginEmail || 'No login linked' }}
                </span>
                <span><Clock3 :size="13" />{{ relativeDate(author.updatedAt || author.createdAt) }}</span>
              </div>

              <div v-if="author.credentials?.length || author.expertise?.length" class="author-chips">
                <span v-for="credential in author.credentials" :key="`credential-${author.id}-${credential}`" class="chip chip--blue">
                  {{ credential }}
                </span>
                <span v-for="item in author.expertise" :key="`expertise-${author.id}-${item}`" class="chip">
                  {{ item }}
                </span>
              </div>
            </div>

            <div v-if="canManageAuthors" class="author-row__actions">
              <button class="icon-button author-action" type="button" title="Edit author" aria-label="Edit author" @click="startEdit(author)">
                <Pencil :size="16" />
              </button>
              <button
                v-if="normalizedStatus(author) !== 'inactive'"
                class="icon-button author-action author-action--danger"
                type="button"
                title="Deactivate author"
                aria-label="Deactivate author"
                :disabled="deletingAuthorID === author.id"
                @click="deleteAuthor(author)"
              >
                <LoaderCircle v-if="deletingAuthorID === author.id" class="spin" :size="16" />
                <Trash2 v-else :size="16" />
              </button>
            </div>
          </article>
        </div>

        <button v-if="nextCursor" class="button load-more" type="button" :disabled="loadingMore" @click="loadMoreAuthors">
          <LoaderCircle v-if="loadingMore" class="spin" :size="16" />
          <RefreshCw v-else :size="16" />
          Load more authors
        </button>
      </section>

      <div v-if="canManageAuthors && authorDialogOpen" class="author-dialog-backdrop" @click.self="closeAuthorDialog">
        <aside
          class="author-editor author-dialog surface"
          role="dialog"
          aria-modal="true"
          aria-labelledby="author-dialog-title"
        >
          <div class="author-editor__header">
            <span class="author-editor__icon"><UserRound :size="18" /></span>
            <div>
              <span>{{ editingAuthorID ? 'Editing profile' : 'New profile' }}</span>
              <h2 id="author-dialog-title">{{ editingAuthorID ? form.displayName || 'Author profile' : 'Author profile' }}</h2>
            </div>
            <button class="icon-button author-dialog__close" type="button" title="Close" aria-label="Close author dialog" :disabled="saving" @click="closeAuthorDialog">
              <X :size="16" />
            </button>
          </div>

          <form class="author-form" @submit.prevent="saveAuthor">
          <section class="author-form__section">
            <h3><IdCard :size="14" />Identity</h3>
            <div class="author-form__grid">
              <label class="field">
                <span>Name</span>
                <input v-model.trim="form.displayName" required>
              </label>
              <label class="field">
                <span>Slug</span>
                <input v-model.trim="form.slug" required>
              </label>
            </div>
            <div class="author-form__grid">
              <label class="field">
                <span>Status</span>
                <select v-model="form.status">
                  <option value="active">Active</option>
                  <option value="inactive">Inactive</option>
                </select>
              </label>
              <label class="field">
                <span>Photo asset ID</span>
                <input v-model.trim="form.photoAssetId" autocomplete="off">
              </label>
            </div>
          </section>

          <section class="author-form__section">
            <h3><Briefcase :size="14" />Affiliation</h3>
            <div class="author-form__grid">
              <label class="field">
                <span>Job title</span>
                <input v-model.trim="form.jobTitle">
              </label>
              <label class="field">
                <span>Organization</span>
                <input v-model.trim="form.organization">
              </label>
            </div>
            <label class="field">
              <span>Profile URL</span>
              <input v-model.trim="form.profileUrl" type="url">
            </label>
            <label v-if="canManageProject" class="field">
              <span>Login account</span>
              <select v-model="form.loginUserId">
                <option value="">Not linked</option>
                <option v-for="member in loginMembers" :key="member.userId" :value="member.userId">
                  {{ member.email }} - {{ labelize(member.role) }}
                </option>
              </select>
            </label>
          </section>

          <section class="author-form__section">
            <h3><FileText :size="14" />Biography</h3>
            <label class="field">
              <span>Short bio</span>
              <textarea v-model.trim="form.shortBio" class="textarea--short" />
            </label>
            <label class="field">
              <span>Full bio</span>
              <textarea v-model.trim="form.fullBio" />
            </label>
          </section>

          <section class="author-form__section">
            <h3><Sparkles :size="14" />Authority</h3>
            <div class="author-form__grid">
              <label class="field">
                <span>Credentials</span>
                <input v-model.trim="form.credentials">
              </label>
              <label class="field">
                <span>Expertise</span>
                <input v-model.trim="form.expertise">
              </label>
            </div>
            <label class="field">
              <span>External profiles</span>
              <textarea v-model.trim="form.externalProfiles" class="textarea--short" />
            </label>
            <label class="field">
              <span>SameAs links</span>
              <textarea v-model.trim="form.sameAs" class="textarea--short" />
            </label>
          </section>

          <div class="author-form__actions">
            <button class="button button--primary" type="submit" :disabled="saving || !canSave">
              <LoaderCircle v-if="saving" class="spin" :size="16" />
              <Check v-else :size="16" />
              {{ editingAuthorID ? 'Save author' : 'Create author' }}
            </button>
            <button class="button" type="button" :disabled="saving" @click="closeAuthorDialog">
              <X :size="16" />
              Cancel
            </button>
          </div>
          </form>
        </aside>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  BadgeCheck,
  Briefcase,
  Check,
  CircleCheck,
  Clock3,
  FileText,
  IdCard,
  Link2,
  LoaderCircle,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Sparkles,
  Trash2,
  UserCheck,
  UserRound,
  UsersRound,
  X
} from 'lucide-vue-next'
import type { AdminAuthor, AdminProject, AdminProjectMember, APIListEnvelope, AuthorPayload } from '~/composables/useAdminApi'
import { apiListData, labelize, normalizeAPIError, slugify, useAdminApi } from '~/composables/useAdminApi'

type AuthorFilter = 'all' | 'active' | 'inactive' | 'linked' | 'missing_bio'

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => String(route.params.projectId || ''))

const project = ref<AdminProject | null>(null)
const authors = ref<AdminAuthor[]>([])
const members = ref<AdminProjectMember[]>([])
const pending = ref(true)
const loadingMore = ref(false)
const saving = ref(false)
const deletingAuthorID = ref('')
const editingAuthorID = ref('')
const authorDialogOpen = ref(false)
const nextCursor = ref('')
const search = ref('')
const activeFilter = ref<AuthorFilter>('all')
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
  loginUserId: '',
  credentials: '',
  expertise: '',
  externalProfiles: '',
  sameAs: ''
})

const canManageProject = computed(() => ['project_owner', 'project_admin'].includes(project.value?.role || ''))
const canManageAuthors = computed(() => canManageProject.value || project.value?.role === 'editor')
const canSave = computed(() => Boolean(form.displayName.trim() && form.slug.trim()))
const loginMembers = computed(() => members.value.filter(member => member.status === 'active' || member.status === 'invited'))

const activeAuthors = computed(() => authors.value.filter(author => normalizedStatus(author) === 'active'))
const inactiveAuthors = computed(() => authors.value.filter(author => normalizedStatus(author) === 'inactive'))
const linkedAuthors = computed(() => authors.value.filter(author => Boolean(author.loginUserId || author.loginEmail)))
const missingBioAuthors = computed(() => authors.value.filter(author => isMissingBio(author)))
const authorityAuthors = computed(() => authors.value.filter(author => Boolean(author.credentials?.length || author.expertise?.length)))

const metrics = computed(() => [
  { label: 'Authors', value: authors.value.length, icon: UsersRound },
  { label: 'Active', value: activeAuthors.value.length, icon: CircleCheck },
  canManageProject.value
    ? { label: 'Linked logins', value: linkedAuthors.value.length, icon: UserCheck }
    : { label: 'With authority', value: authorityAuthors.value.length, icon: Sparkles },
  { label: 'Need bio', value: missingBioAuthors.value.length, icon: FileText }
])

const filterOptions = computed(() => {
  const filters: Array<{ value: AuthorFilter, label: string, count: number }> = [
    { value: 'all', label: 'All', count: authors.value.length },
    { value: 'active', label: 'Active', count: activeAuthors.value.length },
    { value: 'inactive', label: 'Inactive', count: inactiveAuthors.value.length },
    { value: 'missing_bio', label: 'Need bio', count: missingBioAuthors.value.length }
  ]
  if (canManageProject.value) {
    filters.splice(3, 0, { value: 'linked', label: 'Linked', count: linkedAuthors.value.length })
  }
  return filters
})

const filteredAuthors = computed(() => {
  const term = search.value.toLowerCase()
  return sortAuthors(authors.value.filter(author => {
    const matchesFilter = activeFilter.value === 'all'
      || (activeFilter.value === 'active' && normalizedStatus(author) === 'active')
      || (activeFilter.value === 'inactive' && normalizedStatus(author) === 'inactive')
      || (activeFilter.value === 'linked' && Boolean(author.loginUserId || author.loginEmail))
      || (activeFilter.value === 'missing_bio' && isMissingBio(author))
    if (!matchesFilter) return false
    if (!term) return true
    return authorSearchText(author).includes(term)
  }))
})

watch(() => form.displayName, (value) => {
  if (!editingAuthorID.value && !form.slug) form.slug = slugify(value)
})

watch(canManageProject, (value) => {
  if (!value && activeFilter.value === 'linked') activeFilter.value = 'all'
})

watch(canManageAuthors, (value) => {
  if (!value) closeAuthorDialog()
})

onMounted(refresh)

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const [projectResponse, authorResponse] = await Promise.all([
      api.getProject(projectID.value),
      api.request<APIListEnvelope<AdminAuthor>>(`/api/v1/projects/${projectID.value}/authors`, {
        query: { limit: 100 }
      })
    ])
    project.value = projectResponse.data
    authors.value = sortAuthors(apiListData(authorResponse))
    nextCursor.value = authorResponse.meta?.nextCursor || ''
    members.value = canManageProject.value ? await loadProjectMembers() : []
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
    const response = await api.request<APIListEnvelope<AdminAuthor>>(`/api/v1/projects/${projectID.value}/authors`, {
      query: {
        limit: 100,
        cursor: nextCursor.value
      }
    })
    const merged = new Map(authors.value.map(author => [author.id, author]))
    for (const author of apiListData(response)) merged.set(author.id, author)
    authors.value = sortAuthors([...merged.values()])
    nextCursor.value = response.meta?.nextCursor || ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load more authors.')
  } finally {
    loadingMore.value = false
  }
}

function startEdit(author: AdminAuthor) {
  editingAuthorID.value = author.id
  form.displayName = author.displayName
  form.slug = author.slug
  form.status = normalizedStatus(author)
  form.shortBio = author.shortBio || ''
  form.fullBio = author.fullBio || ''
  form.photoAssetId = author.photoAssetId || ''
  form.jobTitle = author.jobTitle || ''
  form.organization = author.organization || ''
  form.profileUrl = author.profileUrl || ''
  form.loginUserId = author.loginUserId || ''
  form.credentials = (author.credentials || []).join(', ')
  form.expertise = (author.expertise || []).join(', ')
  form.externalProfiles = (author.externalProfiles || []).join('\n')
  form.sameAs = (author.sameAs || []).join('\n')
  authorDialogOpen.value = true
  clearMessages()
}

function openNewAuthor() {
  resetForm()
  authorDialogOpen.value = true
}

function closeAuthorDialog() {
  if (saving.value) return
  resetForm()
  authorDialogOpen.value = false
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
  form.loginUserId = ''
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
    const body = authorBody()
    if (editingAuthorID.value) {
      const response = await api.updateAuthor(projectID.value, editingAuthorID.value, body)
      authors.value = sortAuthors(authors.value.map(author => author.id === response.data.id ? response.data : author))
      resetForm()
      authorDialogOpen.value = false
      successMessage.value = 'Author updated.'
    } else {
      const response = await api.createAuthor(projectID.value, body)
      authors.value = sortAuthors([...authors.value, response.data])
      resetForm()
      authorDialogOpen.value = false
      successMessage.value = 'Author created.'
    }
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, editingAuthorID.value ? 'Could not update author.' : 'Could not create author.')
  } finally {
    saving.value = false
  }
}

function authorBody(): AuthorPayload {
  const body: AuthorPayload = {
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
  if (canManageProject.value) {
    body.loginUserId = form.loginUserId
  }
  return body
}

async function deleteAuthor(author: AdminAuthor) {
  if (deletingAuthorID.value) return
  if (!confirm(`Deactivate ${author.displayName}?`)) return
  deletingAuthorID.value = author.id
  clearMessages()
  try {
    const response = await api.deleteAuthor(projectID.value, author.id)
    authors.value = sortAuthors(authors.value.map(candidate => candidate.id === response.data.id ? response.data : candidate))
    if (editingAuthorID.value === author.id) closeAuthorDialog()
    successMessage.value = 'Author deactivated.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not deactivate author.')
  } finally {
    deletingAuthorID.value = ''
  }
}

async function loadProjectMembers() {
  const response = await api.request<APIListEnvelope<AdminProjectMember>>(`/api/v1/projects/${projectID.value}/members`, {
    query: { limit: 100 }
  })
  return apiListData(response)
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

function sortAuthors(values: AdminAuthor[]) {
  return [...values].sort((left, right) => left.displayName.localeCompare(right.displayName))
}

function normalizedStatus(author: AdminAuthor) {
  return author.status || 'active'
}

function authorStatusClass(author: AdminAuthor) {
  return {
    'status-pill--success': normalizedStatus(author) === 'active'
  }
}

function isMissingBio(author: AdminAuthor) {
  return !author.shortBio?.trim() && !author.fullBio?.trim()
}

function authorSearchText(author: AdminAuthor) {
  return [
    author.displayName,
    author.slug,
    author.shortBio,
    author.fullBio,
    author.jobTitle,
    author.organization,
    author.profileUrl,
    author.loginEmail,
    ...(author.credentials || []),
    ...(author.expertise || []),
    ...(author.externalProfiles || []),
    ...(author.sameAs || [])
  ].filter(Boolean).join(' ').toLowerCase()
}

function affiliation(author: AdminAuthor) {
  const values = [author.jobTitle, author.organization].filter(Boolean)
  return values.length ? values.join(' at ') : 'No affiliation set'
}

function initials(value: string) {
  const parts = value.trim().split(/\s+/).filter(Boolean)
  if (!parts.length) return 'AU'
  const first = parts[0]?.[0] || ''
  const second = parts[1]?.[0] || parts[0]?.[1] || ''
  return `${first}${second}`.toUpperCase()
}

function profileLabel(value: string) {
  try {
    return new URL(value).hostname.replace(/^www\./, '')
  } catch {
    return value
  }
}

function relativeDate(value?: string) {
  if (!value) return 'No date'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'No date'
  const elapsed = Date.now() - date.getTime()
  const minutes = Math.round(elapsed / 60000)
  if (minutes < 1) return 'Just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.round(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.round(hours / 24)
  if (days < 30) return `${days}d ago`
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
}

function clearMessages() {
  errorMessage.value = ''
  successMessage.value = ''
}
</script>

<style scoped>
.author-heading-actions { display: flex; flex-wrap: wrap; gap: 7px; }
.authors-layout { display: grid; grid-template-columns: 1fr; align-items: start; gap: 16px; }
.authors-directory { display: grid; min-width: 0; gap: 14px; }
.author-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px; }
.author-search { display: flex; width: min(320px, 100%); align-items: center; gap: 8px; padding: 0 10px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--text-soft); }
.author-search input { width: 100%; min-height: 34px; padding: 0; border: 0 !important; box-shadow: none !important; background: transparent !important; font-size: 11px; }
.segmented-control { display: flex; gap: 3px; overflow-x: auto; }
.segmented-control button { display: inline-flex; min-height: 34px; align-items: center; gap: 7px; padding: 6px 10px; border: 0; border-radius: 5px; background: transparent; color: var(--text-soft); font-size: 11px; font-weight: 600; white-space: nowrap; cursor: pointer; }
.segmented-control button span { display: grid; min-width: 19px; height: 19px; place-items: center; border-radius: 10px; background: var(--surface); font-size: 9px; }
.segmented-control button.is-active { background: var(--surface); color: var(--text); box-shadow: var(--shadow-sm); }
.author-list { overflow: hidden; }
.author-row { display: grid; grid-template-columns: 44px minmax(0, 1fr) auto; gap: 14px; padding: 16px; border-bottom: 1px solid var(--border); }
.author-row:last-child { border-bottom: 0; }
.author-row:hover { background: var(--surface-subtle); }
.author-avatar { display: grid; width: 44px; height: 44px; place-items: center; border-radius: 7px; background: var(--blue-soft); color: var(--blue); font-size: 12px; font-weight: 750; }
.author-avatar--inactive { background: var(--surface-subtle); color: var(--text-faint); }
.author-row__main { min-width: 0; }
.author-row__title { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: 8px; }
.author-row__title h3 { min-width: 0; max-width: 100%; overflow: hidden; margin: 0; color: var(--text); font-size: 14px; font-weight: 680; text-overflow: ellipsis; white-space: nowrap; }
.author-row__main > p { display: -webkit-box; overflow: hidden; margin: 5px 0 0; color: var(--text-soft); font-size: 12px; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.author-row__meta { display: flex; flex-wrap: wrap; gap: 9px 12px; margin-top: 9px; color: var(--text-faint); font-size: 10px; }
.author-row__meta span,
.author-row__meta a { display: inline-flex; min-width: 0; max-width: 260px; align-items: center; gap: 4px; overflow: hidden; color: inherit; text-decoration: none; text-overflow: ellipsis; white-space: nowrap; }
.author-row__meta a:hover { color: var(--primary); }
.author-chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
.chip { display: inline-flex; min-height: 23px; align-items: center; padding: 3px 8px; border-radius: 999px; background: var(--surface-subtle); color: var(--text-soft); font-size: 10px; font-weight: 650; }
.chip--blue { background: var(--blue-soft); color: var(--blue); }
.author-row__actions { display: flex; gap: 6px; }
.author-action { border-color: var(--border); color: var(--text-soft); }
.author-action:hover { color: var(--text); }
.author-action--danger { color: var(--danger); }
.author-action--danger:hover { background: var(--danger-soft); color: var(--danger); }
.author-dialog-backdrop { position: fixed; inset: 0; z-index: 70; display: flex; align-items: center; justify-content: center; padding: 24px; background: rgb(15 23 42 / 0.48); }
.author-editor { padding: 16px; }
.author-dialog { width: min(760px, 100%); max-height: min(90vh, 920px); overflow-y: auto; box-shadow: var(--shadow-md); }
.author-editor__header { display: flex; align-items: flex-start; gap: 11px; padding-bottom: 14px; border-bottom: 1px solid var(--border); }
.author-editor__icon { display: grid; width: 36px; height: 36px; flex: 0 0 36px; place-items: center; border-radius: 7px; background: var(--primary-soft); color: var(--primary); }
.author-editor__header span { color: var(--text-soft); font-size: 10px; font-weight: 650; text-transform: uppercase; }
.author-editor__header h2 { overflow: hidden; margin: 1px 0 0; font-size: 16px; text-overflow: ellipsis; white-space: nowrap; }
.author-dialog__close { margin-left: auto; border-color: var(--border); color: var(--text-soft); }
.author-dialog__close:hover { color: var(--text); }
.author-form { display: grid; gap: 16px; padding-top: 16px; }
.author-form__section { display: grid; gap: 12px; }
.author-form__section + .author-form__section { padding-top: 14px; border-top: 1px solid var(--border); }
.author-form__section h3 { display: inline-flex; align-items: center; gap: 6px; margin: 0; color: var(--text-soft); font-size: 11px; font-weight: 700; text-transform: uppercase; }
.author-form__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.textarea--short { min-height: 76px !important; }
.author-form__actions { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; padding-top: 4px; }
.author-form__actions .button { width: 100%; }
.load-more { width: 100%; }
.loading-surface { display: flex; min-height: 130px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1180px) {
  .authors-layout { grid-template-columns: 1fr; }
}
@media (max-width: 760px) {
  .author-toolbar { align-items: stretch; flex-direction: column; }
  .author-search { width: 100%; }
  .segmented-control { width: 100%; }
  .author-row { grid-template-columns: 38px minmax(0, 1fr); }
  .author-avatar { width: 38px; height: 38px; }
  .author-row__actions { grid-column: 2; justify-content: flex-end; }
}
@media (max-width: 560px) {
  .author-dialog-backdrop { align-items: stretch; padding: 10px; }
  .author-dialog { max-height: calc(100vh - 20px); }
  .author-heading-actions,
  .author-heading-actions .button { width: 100%; }
  .author-form__grid,
  .author-form__actions { grid-template-columns: 1fr; }
  .author-row__meta span,
  .author-row__meta a { max-width: 100%; }
}
</style>

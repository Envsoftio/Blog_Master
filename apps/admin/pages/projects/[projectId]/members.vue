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
            <h1 class="mt-1 text-2xl font-semibold tracking-normal">Members</h1>
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
        <NuxtLink class="block rounded-md bg-white px-3 py-2 text-sm shadow-sm dark:bg-[#252b28]" :to="`/projects/${projectID}/members`">Members</NuxtLink>
        <NuxtLink class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/api-keys`">API keys</NuxtLink>
        <NuxtLink class="block rounded-md px-3 py-2 text-sm text-[#555f58] dark:text-[#b8c2bb]" :to="`/projects/${projectID}/audit-events`">Audit</NuxtLink>
      </aside>

      <div class="space-y-5">
        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>
        <p v-if="successMessage" class="rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-4 py-3 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]">
          {{ successMessage }}
        </p>

        <div v-if="invitationToken" class="rounded-lg border border-[#b9dcc9] bg-white p-5 shadow-sm dark:border-[#2d644a] dark:bg-[#202522]">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="min-w-0">
              <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">One-time invitation token</p>
              <h2 class="mt-1 truncate text-lg font-semibold tracking-normal">{{ invitationToken.email }}</h2>
            </div>
            <div class="flex items-center gap-2">
              <button
                class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] text-[#28342d] hover:bg-[#eef5f1] dark:border-[#414a45] dark:text-[#eef4ef] dark:hover:bg-[#2a302d]"
                type="button"
                title="Copy invitation link"
                aria-label="Copy invitation link"
                @click="copyInvitationToken"
              >
                <Copy class="h-4 w-4" />
              </button>
              <button
                class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] text-[#28342d] hover:bg-[#eef5f1] dark:border-[#414a45] dark:text-[#eef4ef] dark:hover:bg-[#2a302d]"
                type="button"
                title="Dismiss"
                aria-label="Dismiss"
                @click="invitationToken = null"
              >
                <X class="h-4 w-4" />
              </button>
            </div>
          </div>
          <dl class="mt-4 grid gap-3 text-sm sm:grid-cols-2">
            <div class="flex items-center gap-2">
              <Clock3 class="h-4 w-4 text-[#3162a3]" />
              <div class="min-w-0">
                <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Expires</dt>
                <dd class="truncate">{{ formatDate(invitationToken.expiresAt) }}</dd>
              </div>
            </div>
          </dl>
          <code class="mt-4 block overflow-x-auto rounded-md bg-[#17201b] px-3 py-3 text-sm text-[#dff7ea]">{{ invitationURL }}</code>
        </div>

        <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
          <div class="space-y-4">
            <div>
              <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Access</p>
              <h2 class="mt-1 text-xl font-semibold tracking-normal">Project membership</h2>
            </div>

            <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
              <LoaderCircle class="h-4 w-4 animate-spin" />
              Loading members
            </div>

            <div v-else-if="members.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]">
              <h2 class="text-lg font-semibold">No members yet</h2>
            </div>

            <article v-for="member in members" :key="member.userId" class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div class="min-w-0">
                  <h3 class="truncate text-lg font-semibold">{{ member.email }}</h3>
                  <p class="mt-1 truncate text-sm text-[#5f6a63] dark:text-[#b8c2bb]">{{ member.userId }}</p>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="roleClass(member.role)">
                    {{ roleLabel(member.role) }}
                  </span>
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(member.status)">
                    {{ member.status }}
                  </span>
                </div>
              </div>

              <dl class="mt-5 grid gap-3 text-sm md:grid-cols-3">
                <div class="flex items-center gap-2">
                  <Mail class="h-4 w-4 text-[#3162a3]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Invited</dt>
                    <dd class="truncate">{{ formatDate(member.invitedAt) }}</dd>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <ShieldCheck class="h-4 w-4 text-[#165a4a]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Joined</dt>
                    <dd class="truncate">{{ formatDate(member.joinedAt) }}</dd>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <CalendarClock class="h-4 w-4 text-[#8a5b00]" />
                  <div class="min-w-0">
                    <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Removed</dt>
                    <dd class="truncate">{{ formatDate(member.removedAt) }}</dd>
                  </div>
                </div>
              </dl>

              <div class="mt-5 grid gap-3 sm:grid-cols-[minmax(160px,240px)_auto_auto]">
                <select
                  v-model="roleDrafts[member.userId]"
                  class="h-10 rounded-md border border-[#bfcac3] px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#171b18]"
                  :disabled="member.status === 'removed' || Boolean(actionPending[member.userId]) || (!canManageOwnership && member.role === 'project_owner')"
                >
                  <option v-for="role in roleOptions" :key="role.value" :value="role.value" :disabled="role.value === 'project_owner' && !canManageOwnership">{{ role.label }}</option>
                </select>
                <button
                  class="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-3 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                  type="button"
                  :disabled="member.status === 'removed' || actionPending[member.userId] === 'role' || roleDrafts[member.userId] === member.role || (!canManageOwnership && member.role === 'project_owner')"
                  @click="saveRole(member)"
                >
                  <LoaderCircle v-if="actionPending[member.userId] === 'role'" class="h-4 w-4 animate-spin" />
                  <Check v-else class="h-4 w-4" />
                  Save
                </button>
                <button
                  class="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-[#d9b7aa] px-3 text-sm font-medium text-[#9b2d23] hover:bg-[#fff4f2] disabled:opacity-60 dark:border-[#6d352f] dark:text-[#ffc4bd] dark:hover:bg-[#2a1c1a]"
                  type="button"
                  :disabled="member.status === 'removed' || actionPending[member.userId] === 'remove' || (!canManageOwnership && member.role === 'project_owner')"
                  @click="removeMember(member)"
                >
                  <LoaderCircle v-if="actionPending[member.userId] === 'remove'" class="h-4 w-4 animate-spin" />
                  <Trash2 v-else class="h-4 w-4" />
                  Remove
                </button>
              </div>
            </article>

            <button
              v-if="nextCursor"
              class="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] bg-white px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:bg-[#202522] dark:hover:bg-[#2a302d]"
              type="button"
              :disabled="loadingMore"
              @click="loadMoreMembers"
            >
              <LoaderCircle v-if="loadingMore" class="h-4 w-4 animate-spin" />
              <RefreshCw v-else class="h-4 w-4" />
              Load more members
            </button>
          </div>

          <form class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="inviteMember">
            <div class="flex items-start gap-3">
              <UserPlus class="mt-1 h-4 w-4 text-[#3162a3]" />
              <div>
                <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Invite</p>
                <h2 class="mt-1 text-lg font-semibold tracking-normal">Project member</h2>
              </div>
            </div>

            <label class="block space-y-2">
              <span class="text-sm font-medium">Email</span>
              <input v-model.trim="form.email" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" type="email" autocomplete="email" required />
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium">Role</span>
              <select v-model="form.role" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]">
                <option v-for="role in roleOptions" :key="role.value" :value="role.value" :disabled="role.value === 'project_owner' && !canManageOwnership">{{ role.label }}</option>
              </select>
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium">Expires at</span>
              <input v-model="form.expiresAt" class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]" type="datetime-local" :min="minimumExpiry" />
            </label>

            <button
              class="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
              type="submit"
              :disabled="creating || !canInvite"
            >
              <LoaderCircle v-if="creating" class="h-4 w-4 animate-spin" />
              <UserPlus v-else class="h-4 w-4" />
              Invite member
            </button>
          </form>
        </div>
      </div>
    </div>

    <div v-if="reauthenticationOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/45 px-4" @click.self="cancelReauthentication">
      <form
        class="w-full max-w-md rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-xl dark:border-[#3f4843] dark:bg-[#202522]"
        role="dialog"
        aria-modal="true"
        aria-labelledby="member-reauthentication-title"
        @submit.prevent="confirmReauthentication"
      >
        <div class="flex items-start gap-3">
          <LockKeyhole class="mt-1 h-5 w-5 text-[#3162a3]" />
          <div>
            <h2 id="member-reauthentication-title" class="text-lg font-semibold tracking-normal">Confirm your identity</h2>
            <p class="mt-1 text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Enter your current password to {{ pendingProtectedAction?.label || 'continue' }}.</p>
          </div>
        </div>

        <p v-if="reauthenticationError" class="mt-4 rounded-md border border-[#edc6c2] bg-[#fff4f2] px-3 py-2 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ reauthenticationError }}
        </p>

        <label class="mt-5 block space-y-2">
          <span class="text-sm font-medium">Current password</span>
          <input
            v-model="reauthenticationPassword"
            class="w-full rounded-md border border-[#bfcac3] px-3 py-2 dark:border-[#4b5650] dark:bg-[#171b18]"
            type="password"
            autocomplete="current-password"
            required
            autofocus
          />
        </label>

        <div class="mt-5 flex justify-end gap-2">
          <button class="rounded-md border border-[#c9d4cc] px-3 py-2 text-sm font-medium hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]" type="button" :disabled="reauthenticating" @click="cancelReauthentication">
            Cancel
          </button>
          <button class="inline-flex items-center gap-2 rounded-md bg-[#165a4a] px-4 py-2 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60" type="submit" :disabled="reauthenticating || !reauthenticationPassword">
            <LoaderCircle v-if="reauthenticating" class="h-4 w-4 animate-spin" />
            Confirm
          </button>
        </div>
      </form>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ArrowLeft, CalendarClock, Check, Clock3, Copy, LoaderCircle, LockKeyhole, LogOut, Mail, RefreshCw, ShieldCheck, Trash2, UserPlus, X } from 'lucide-vue-next'

type APIEnvelope<T> = {
  data: T
}

type APIListEnvelope<T> = {
  data: T[]
  meta: {
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

type AdminProjectMember = {
  projectId: string
  userId: string
  email: string
  role: string
  status: string
  invitedBy?: string
  invitedAt?: string
  joinedAt?: string
  updatedAt: string
  removedAt?: string
}

type ProjectMemberInvitation = {
  member: AdminProjectMember
  token: string
  expiresAt: string
}

type PendingProtectedAction = {
  label: string
  run: () => Promise<void>
}

const route = useRoute()
const projectID = computed(() => {
  const value = route.params.projectId
  return Array.isArray(value) ? value[0] : String(value || '')
})

const roleOptions = [
  { value: 'project_owner', label: 'Project owner' },
  { value: 'project_admin', label: 'Project admin' },
  { value: 'editor', label: 'Editor' },
  { value: 'reviewer', label: 'Reviewer' },
  { value: 'writer', label: 'Writer' }
]

const project = ref<AdminProject | null>(null)
const members = ref<AdminProjectMember[]>([])
const pending = ref(true)
const loadingMore = ref(false)
const nextCursor = ref('')
const creating = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const invitationToken = ref<{ email: string, token: string, expiresAt: string } | null>(null)
const roleDrafts = reactive<Record<string, string>>({})
const actionPending = reactive<Record<string, string>>({})
const minimumExpiry = ref('')
const reauthenticationOpen = ref(false)
const reauthenticationPassword = ref('')
const reauthenticationError = ref('')
const reauthenticating = ref(false)
const pendingProtectedAction = ref<PendingProtectedAction | null>(null)

const form = reactive({
  email: '',
  role: 'writer',
  expiresAt: ''
})

const canInvite = computed(() => Boolean(form.email.trim() && form.role))
const canManageOwnership = computed(() => project.value?.role === 'project_owner')
const invitationURL = computed(() => {
  if (!invitationToken.value || !import.meta.client) return ''
  return `${window.location.origin}/invitations/${encodeURIComponent(invitationToken.value.token)}`
})

onMounted(() => {
  minimumExpiry.value = formatLocalDateTimeInput(new Date(Date.now() + 60_000))
  void refresh()
})

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const [projectResponse, memberResponse] = await Promise.all([
      $fetch<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID.value}`, { credentials: 'include' }),
      $fetch<APIListEnvelope<AdminProjectMember>>(`/api/v1/projects/${projectID.value}/members`, { credentials: 'include' })
    ])
    project.value = projectResponse.data
    members.value = sortMembers(memberResponse.data)
    nextCursor.value = memberResponse.meta.nextCursor || ''
    syncRoleDrafts()
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load members. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function loadMoreMembers() {
  if (!nextCursor.value || loadingMore.value) return
  loadingMore.value = true
  clearMessages()
  try {
    const response = await $fetch<APIListEnvelope<AdminProjectMember>>(`/api/v1/projects/${projectID.value}/members`, {
      credentials: 'include',
      query: {
        limit: 50,
        cursor: nextCursor.value
      }
    })
    const merged = new Map(members.value.map(member => [member.userId, member]))
    for (const member of response.data) {
      merged.set(member.userId, member)
      roleDrafts[member.userId] = member.role
    }
    members.value = sortMembers([...merged.values()])
    nextCursor.value = response.meta.nextCursor || ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load more members.')
  } finally {
    loadingMore.value = false
  }
}

async function inviteMember() {
  creating.value = true
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<ProjectMemberInvitation>>(`/api/v1/projects/${projectID.value}/invitations`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: {
        email: form.email,
        role: form.role,
        expiresAt: form.expiresAt ? new Date(form.expiresAt).toISOString() : ''
      }
    })
    upsertMember(response.data.member)
    invitationToken.value = {
      email: response.data.member.email,
      token: response.data.token,
      expiresAt: response.data.expiresAt
    }
    form.email = ''
    form.role = 'writer'
    form.expiresAt = ''
    successMessage.value = 'Invitation created.'
  } catch (error) {
    if (queueReauthentication(error, 'invite a project owner', inviteMember)) return
    errorMessage.value = normalizeAPIError(error, 'Could not create invitation.')
  } finally {
    creating.value = false
  }
}

async function saveRole(member: AdminProjectMember) {
  const role = roleDrafts[member.userId]
  if (!role || role === member.role) return
  actionPending[member.userId] = 'role'
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    const response = await $fetch<APIEnvelope<AdminProjectMember>>(`/api/v1/projects/${projectID.value}/members/${member.userId}`, {
      method: 'PATCH',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: { role }
    })
    upsertMember(response.data)
    successMessage.value = 'Member role updated.'
  } catch (error) {
    if (queueReauthentication(error, `change ${member.email}'s ownership`, () => saveRole(member))) return
    roleDrafts[member.userId] = member.role
    errorMessage.value = normalizeAPIError(error, 'Could not update member role.')
  } finally {
    delete actionPending[member.userId]
  }
}

async function removeMember(member: AdminProjectMember) {
  if (!window.confirm(`Remove ${member.email} from this project?`)) return
  await performRemoveMember(member)
}

async function performRemoveMember(member: AdminProjectMember) {
  actionPending[member.userId] = 'remove'
  clearMessages()
  try {
    const csrfToken = await getCSRFToken()
    await $fetch(`/api/v1/projects/${projectID.value}/members/${member.userId}`, {
      method: 'DELETE',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken }
    })
    const removedAt = new Date().toISOString()
    upsertMember({
      ...member,
      status: 'removed',
      updatedAt: removedAt,
      removedAt
    })
    successMessage.value = 'Member removed.'
  } catch (error) {
    if (queueReauthentication(error, `remove owner ${member.email}`, () => performRemoveMember(member))) return
    errorMessage.value = normalizeAPIError(error, 'Could not remove member.')
  } finally {
    delete actionPending[member.userId]
  }
}

function queueReauthentication(error: unknown, label: string, run: () => Promise<void>) {
  const problem = apiProblem(error)
  if (problem?.title !== 'Recent reauthentication required') return false
  pendingProtectedAction.value = { label, run }
  reauthenticationPassword.value = ''
  reauthenticationError.value = ''
  reauthenticationOpen.value = true
  return true
}

async function confirmReauthentication() {
  if (!pendingProtectedAction.value || !reauthenticationPassword.value) return
  reauthenticating.value = true
  reauthenticationError.value = ''
  try {
    const csrfToken = await getCSRFToken()
    await $fetch('/api/v1/auth/reauthenticate', {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfToken },
      body: { password: reauthenticationPassword.value }
    })
    const action = pendingProtectedAction.value
    reauthenticationOpen.value = false
    reauthenticationPassword.value = ''
    pendingProtectedAction.value = null
    await action.run()
  } catch (error) {
    reauthenticationError.value = normalizeAPIError(error, 'Could not confirm your identity.')
  } finally {
    reauthenticating.value = false
  }
}

function cancelReauthentication() {
  if (reauthenticating.value) return
  reauthenticationOpen.value = false
  reauthenticationPassword.value = ''
  reauthenticationError.value = ''
  pendingProtectedAction.value = null
}

async function copyInvitationToken() {
  if (!invitationToken.value) return
  await navigator.clipboard.writeText(invitationURL.value)
  successMessage.value = 'Invitation link copied.'
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

function upsertMember(member: AdminProjectMember) {
  const existing = members.value.findIndex((item) => item.userId === member.userId)
  if (existing >= 0) {
    members.value.splice(existing, 1, member)
  } else {
    members.value.push(member)
  }
  members.value = sortMembers(members.value)
  roleDrafts[member.userId] = member.role
}

function syncRoleDrafts() {
  for (const member of members.value) {
    roleDrafts[member.userId] = member.role
  }
}

function sortMembers(values: AdminProjectMember[]) {
  return [...values].sort((left, right) => {
    const statusOrder = statusRank(left.status) - statusRank(right.status)
    if (statusOrder !== 0) return statusOrder
    return left.email.localeCompare(right.email)
  })
}

function statusRank(status: string) {
  switch (status) {
    case 'active':
      return 0
    case 'invited':
      return 1
    default:
      return 2
  }
}

function parseBackendUTC(value: string) {
  return new Date(value.includes('T') ? value : `${value.replace(' ', 'T')}Z`)
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

function formatLocalDateTimeInput(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0')
  return [
    date.getFullYear(),
    '-',
    pad(date.getMonth() + 1),
    '-',
    pad(date.getDate()),
    'T',
    pad(date.getHours()),
    ':',
    pad(date.getMinutes())
  ].join('')
}

function roleLabel(role: string) {
  return role.replaceAll('_', ' ')
}

function roleClass(role: string) {
  switch (role) {
    case 'project_owner':
      return 'bg-[#e8f0ff] text-[#245b99] dark:bg-[#152944] dark:text-[#b8d5ff]'
    case 'project_admin':
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'editor':
      return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
    default:
      return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
}

function statusClass(status: string) {
  switch (status) {
    case 'active':
      return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
    case 'invited':
      return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
    case 'removed':
      return 'bg-[#fbe4e1] text-[#8f3028] dark:bg-[#46231f] dark:text-[#ffc4bd]'
    default:
      return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
  }
}

function clearMessages() {
  errorMessage.value = ''
  successMessage.value = ''
}

function normalizeAPIError(error: unknown, fallback: string) {
  const problem = apiProblem(error)
  if (problem) return problem.detail || problem.title || fallback
  if (error instanceof Error && error.message) return error.message
  return fallback
}

function apiProblem(error: unknown) {
  if (typeof error !== 'object' || error === null || !('data' in error)) return null
  return (error as { data?: { title?: string, detail?: string, status?: number } }).data || null
}
</script>

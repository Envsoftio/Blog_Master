<template>
  <section class="min-h-screen">
    <header class="border-b border-[#d7ded8] bg-white px-6 py-4 dark:border-[#343a38] dark:bg-[#202422]">
      <div class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4">
        <div class="flex min-w-0 items-center gap-3">
          <NuxtLink class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#eef5f1] dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]" to="/projects" title="Back to projects" aria-label="Back to projects">
            <ArrowLeft class="h-4 w-4" />
          </NuxtLink>
          <div class="min-w-0">
            <p class="truncate text-sm text-[#5d6a61] dark:text-[#aeb8b0]">{{ project?.name || 'Project' }}</p>
            <h1 class="truncate text-lg font-semibold tracking-normal">Members</h1>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#eef5f1] disabled:opacity-50 dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]" type="button" title="Refresh members" aria-label="Refresh members" :disabled="pending" @click="refresh">
            <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': pending }" />
          </button>
          <button class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#fff4df] dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]" type="button" title="Log out" aria-label="Log out" @click="logout">
            <LogOut class="h-4 w-4" />
          </button>
        </div>
      </div>
    </header>

    <div class="mx-auto grid max-w-7xl grid-cols-1 gap-6 px-6 py-6 lg:grid-cols-[220px_1fr]">
      <ProjectNav :project-id="projectID" :project="project" active="members" />

      <main class="min-w-0 space-y-5">
        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">{{ errorMessage }}</p>
        <p v-if="successMessage" class="rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-4 py-3 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]" role="status">{{ successMessage }}</p>

        <section v-if="invitationToken" class="rounded-lg border border-[#b9dcc9] bg-white p-5 shadow-sm dark:border-[#2d644a] dark:bg-[#202522]" aria-labelledby="invitation-token-title">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="min-w-0">
              <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Shown once</p>
              <h2 id="invitation-token-title" class="mt-1 truncate text-lg font-semibold">Invitation for {{ invitationToken.email }}</h2>
              <p class="mt-2 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Copy this private link now. The server stores only a verifier and cannot show it again.</p>
            </div>
            <button class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]" type="button" title="Dismiss invitation link" aria-label="Dismiss invitation link" @click="invitationToken = null"><X class="h-4 w-4" /></button>
          </div>
          <div class="mt-4 flex flex-wrap items-center gap-3">
            <code class="min-w-0 flex-1 overflow-x-auto rounded-md bg-[#17201b] px-3 py-3 text-sm text-[#dff7ea]">{{ invitationURL }}</code>
            <button class="inline-flex h-10 items-center gap-2 rounded-md bg-[#165a4a] px-4 text-sm font-medium text-white hover:bg-[#10463a]" type="button" @click="copyInvitationToken"><Copy class="h-4 w-4" />Copy link</button>
          </div>
          <p class="mt-3 flex items-center gap-2 text-xs text-[#5d6a61] dark:text-[#aeb8b0]"><Clock3 class="h-4 w-4" />Expires {{ formatDate(invitationToken.expiresAt) }}</p>
        </section>

        <section class="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Project access</p>
            <h2 class="mt-1 text-2xl font-semibold tracking-tight">Membership and account access</h2>
            <p class="mt-2 max-w-2xl text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Membership controls access to this project. Disabling login is owner-only and revokes all sessions while preserving project history.</p>
          </div>
          <span class="rounded-full bg-[#eef2ef] px-3 py-1.5 text-sm font-medium text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]">Your role: {{ roleLabel(project?.role || '') }}</span>
        </section>

        <dl class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <div class="rounded-lg border border-[#cfd8d1] bg-white p-4 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]"><dt class="text-xs uppercase tracking-wide text-[#667169] dark:text-[#aeb8b0]">Active members</dt><dd class="mt-2 text-2xl font-semibold text-[#165a4a] dark:text-[#aee4d0]">{{ memberStats.active }}</dd></div>
          <div class="rounded-lg border border-[#cfd8d1] bg-white p-4 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]"><dt class="text-xs uppercase tracking-wide text-[#667169] dark:text-[#aeb8b0]">Pending invites</dt><dd class="mt-2 text-2xl font-semibold text-[#7a4f00] dark:text-[#ffd98a]">{{ memberStats.invited }}</dd></div>
          <div class="rounded-lg border border-[#cfd8d1] bg-white p-4 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]"><dt class="text-xs uppercase tracking-wide text-[#667169] dark:text-[#aeb8b0]">Owners</dt><dd class="mt-2 text-2xl font-semibold text-[#245b99] dark:text-[#b8d5ff]">{{ memberStats.owners }}</dd></div>
          <div class="rounded-lg border border-[#cfd8d1] bg-white p-4 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]"><dt class="text-xs uppercase tracking-wide text-[#667169] dark:text-[#aeb8b0]">Login disabled</dt><dd class="mt-2 text-2xl font-semibold text-[#9b2d23] dark:text-[#ffc4bd]">{{ memberStats.disabled }}</dd></div>
        </dl>
        <p v-if="nextCursor" class="text-xs text-[#667169] dark:text-[#aeb8b0]">Summary counts currently reflect loaded members. Load the remaining pages for a complete project count.</p>

        <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
          <section class="min-w-0 space-y-4">
            <div class="grid gap-3 rounded-lg border border-[#cfd8d1] bg-white p-3 shadow-sm dark:border-[#3f4843] dark:bg-[#202522] sm:grid-cols-[minmax(220px,1fr)_170px_170px]">
              <label class="relative block">
                <span class="sr-only">Search loaded members</span>
                <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#667169] dark:text-[#aeb8b0]" />
                <input v-model.trim="search" class="h-10 w-full rounded-md border border-[#bfcac3] bg-white pl-10 pr-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" type="search" placeholder="Search loaded members" />
              </label>
              <label><span class="sr-only">Membership status</span><select v-model="statusFilter" class="h-10 w-full rounded-md border border-[#bfcac3] bg-white px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]"><option value="all">All statuses</option><option value="active">Active</option><option value="invited">Invited</option><option value="removed">Removed</option></select></label>
              <label><span class="sr-only">Project role</span><select v-model="roleFilter" class="h-10 w-full rounded-md border border-[#bfcac3] bg-white px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]"><option value="all">All roles</option><option v-for="role in roleOptions" :key="role.value" :value="role.value">{{ role.label }}</option></select></label>
            </div>

            <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]" aria-live="polite"><LoaderCircle class="h-4 w-4 animate-spin" />Loading members</div>
            <div v-else-if="members.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]"><Users class="mx-auto h-8 w-8 text-[#667169]" /><h2 class="mt-3 text-lg font-semibold">No members found</h2></div>
            <div v-else-if="filteredMembers.length === 0" class="rounded-lg border border-dashed border-[#bfcac3] bg-white p-8 text-center dark:border-[#4b5650] dark:bg-[#202522]"><Search class="mx-auto h-8 w-8 text-[#667169]" /><h2 class="mt-3 text-lg font-semibold">No matching loaded members</h2><p class="mt-2 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Clear the filters or load the next page.</p></div>

            <article v-for="member in filteredMembers" :key="member.userId" class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <h3 class="truncate text-lg font-semibold">{{ member.email }}</h3>
                    <span v-if="member.userId === currentUserID" class="rounded-full bg-[#eef2ef] px-2 py-0.5 text-xs text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]">You</span>
                  </div>
                  <p class="mt-1 truncate font-mono text-xs text-[#5f6a63] dark:text-[#b8c2bb]">{{ member.userId }}</p>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="roleClass(member.role)">{{ roleLabel(member.role) }}</span>
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(member.status)">{{ member.status }}</span>
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="accountStatusClass(member.userStatus)">login {{ member.userStatus }}</span>
                </div>
              </div>

              <dl class="mt-5 grid gap-3 text-sm sm:grid-cols-3">
                <div class="flex items-center gap-2"><Mail class="h-4 w-4 text-[#3162a3]" /><div class="min-w-0"><dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Invited</dt><dd class="truncate">{{ formatDate(member.invitedAt) }}</dd></div></div>
                <div class="flex items-center gap-2"><ShieldCheck class="h-4 w-4 text-[#165a4a]" /><div class="min-w-0"><dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Joined</dt><dd class="truncate">{{ formatDate(member.joinedAt) }}</dd></div></div>
                <div class="flex items-center gap-2"><CalendarClock class="h-4 w-4 text-[#8a5b00]" /><div class="min-w-0"><dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Updated</dt><dd class="truncate">{{ formatDate(member.updatedAt) }}</dd></div></div>
              </dl>

              <p v-if="member.status === 'removed'" class="mt-4 rounded-md bg-[#f5f7f5] px-3 py-2 text-sm text-[#5f6a63] dark:bg-[#171b18] dark:text-[#b8c2bb]">Removed {{ formatDate(member.removedAt) }}. Historical authorship and audit records remain intact.</p>

              <div v-else-if="canManageMembers" class="mt-5 grid gap-3 sm:grid-cols-[minmax(170px,240px)_auto_auto]">
                <select v-model="roleDrafts[member.userId]" class="h-10 rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" :aria-label="`Role for ${member.email}`" :disabled="!canEditMember(member) || Boolean(actionPending[member.userId])">
                  <option v-for="role in roleOptions" :key="role.value" :value="role.value" :disabled="role.value === 'project_owner' && !canManageOwnership">{{ role.label }}</option>
                </select>
                <button class="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-3 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]" type="button" :disabled="!canEditMember(member) || Boolean(actionPending[member.userId]) || roleDrafts[member.userId] === member.role" @click="saveRole(member)">
                  <LoaderCircle v-if="actionPending[member.userId] === 'role'" class="h-4 w-4 animate-spin" /><Save v-else class="h-4 w-4" />Save role
                </button>
                <button class="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-[#d9b7aa] px-3 text-sm font-medium text-[#9b2d23] hover:bg-[#fff4f2] disabled:opacity-60 dark:border-[#6d352f] dark:text-[#ffc4bd]" type="button" :disabled="!canEditMember(member) || Boolean(actionPending[member.userId])" @click="removeMember(member)">
                  <LoaderCircle v-if="actionPending[member.userId] === 'remove'" class="h-4 w-4 animate-spin" /><UserMinus v-else class="h-4 w-4" />Remove
                </button>
                <button v-if="canShowLoginAction(member)" class="inline-flex h-10 items-center justify-center gap-2 rounded-md px-3 text-sm font-medium disabled:opacity-60 sm:col-span-3" :class="member.userStatus === 'disabled' ? 'border border-[#b9dcc9] text-[#165a4a] hover:bg-[#edf9f1] dark:border-[#2d644a] dark:text-[#aee4d0]' : 'border border-[#d9b7aa] text-[#9b2d23] hover:bg-[#fff4f2] dark:border-[#6d352f] dark:text-[#ffc4bd]'" type="button" :disabled="Boolean(actionPending[member.userId])" @click="toggleMemberLogin(member)">
                  <LoaderCircle v-if="actionPending[member.userId] === 'login'" class="h-4 w-4 animate-spin" /><UserCheck v-else-if="member.userStatus === 'disabled'" class="h-4 w-4" /><UserX v-else class="h-4 w-4" />{{ member.userStatus === 'disabled' ? 'Enable account login' : 'Disable account login and revoke sessions' }}
                </button>
              </div>
            </article>

            <button v-if="nextCursor" class="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] bg-white px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:bg-[#202522] dark:hover:bg-[#2a302d]" type="button" :disabled="loadingMore" @click="loadMoreMembers"><LoaderCircle v-if="loadingMore" class="h-4 w-4 animate-spin" /><ChevronDown v-else class="h-4 w-4" />Load more members</button>
          </section>

          <aside class="space-y-5">
            <form v-if="canManageMembers" class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="inviteMember">
              <div class="flex items-start gap-3"><UserPlus class="mt-1 h-4 w-4 text-[#3162a3]" /><div><p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Invite</p><h2 class="mt-1 text-lg font-semibold">Project member</h2></div></div>
              <label class="block space-y-2"><span class="text-sm font-medium">Email</span><input v-model.trim="form.email" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 dark:border-[#4b5650] dark:bg-[#171b18]" type="email" autocomplete="email" required /></label>
              <label class="block space-y-2"><span class="text-sm font-medium">Role</span><select v-model="form.role" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 dark:border-[#4b5650] dark:bg-[#171b18]"><option v-for="role in roleOptions" :key="role.value" :value="role.value" :disabled="role.value === 'project_owner' && !canManageOwnership">{{ role.label }}</option></select><span class="block text-xs text-[#667169] dark:text-[#aeb8b0]">{{ roleDescription(form.role) }}</span></label>
              <label class="block space-y-2"><span class="text-sm font-medium">Expires at</span><input v-model="form.expiresAt" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 dark:border-[#4b5650] dark:bg-[#171b18]" type="datetime-local" :min="minimumExpiry" /><span class="block text-xs text-[#667169] dark:text-[#aeb8b0]">Leave blank to use the server default.</span></label>
              <button class="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md bg-[#165a4a] px-4 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60" type="submit" :disabled="creating || !canInvite"><LoaderCircle v-if="creating" class="h-4 w-4 animate-spin" /><UserPlus v-else class="h-4 w-4" />Create invitation</button>
            </form>

            <section v-else class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]"><ShieldCheck class="h-5 w-5 text-[#3162a3]" /><h2 class="mt-3 font-semibold">Read-only membership</h2><p class="mt-2 text-sm text-[#5f6a63] dark:text-[#b8c2bb]">Only project owners and administrators can invite, change, or remove members.</p></section>

            <section class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex items-start gap-3"><KeyRound class="mt-1 h-4 w-4 text-[#6b5797]" /><div><p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Role guide</p><h2 class="mt-1 text-lg font-semibold">Least privilege</h2></div></div>
              <dl class="mt-4 space-y-3 text-sm"><div v-for="role in roleOptions" :key="role.value" class="rounded-md bg-[#f5f7f5] p-3 dark:bg-[#171b18]"><dt class="font-medium">{{ role.label }}</dt><dd class="mt-1 text-[#5f6a63] dark:text-[#b8c2bb]">{{ roleDescription(role.value) }}</dd></div></dl>
            </section>
          </aside>
        </div>
      </main>
    </div>

    <div v-if="reauthenticationOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/45 px-4" @click.self="cancelReauthentication">
      <form class="w-full max-w-md rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-xl dark:border-[#3f4843] dark:bg-[#202522]" role="dialog" aria-modal="true" aria-labelledby="member-reauthentication-title" @submit.prevent="confirmReauthentication">
        <div class="flex items-start gap-3"><LockKeyhole class="mt-1 h-5 w-5 text-[#3162a3]" /><div><h2 id="member-reauthentication-title" class="text-lg font-semibold">Confirm your identity</h2><p class="mt-1 text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Enter your current password to {{ pendingProtectedAction?.label || 'continue' }}.</p></div></div>
        <p v-if="reauthenticationError" class="mt-4 rounded-md border border-[#edc6c2] bg-[#fff4f2] px-3 py-2 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">{{ reauthenticationError }}</p>
        <label class="mt-5 block space-y-2"><span class="text-sm font-medium">Current password</span><input ref="reauthenticationInput" v-model="reauthenticationPassword" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 dark:border-[#4b5650] dark:bg-[#171b18]" type="password" autocomplete="current-password" required /></label>
        <div class="mt-5 flex justify-end gap-2"><button class="h-10 rounded-md border border-[#c9d4cc] px-3 text-sm font-medium hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]" type="button" :disabled="reauthenticating" @click="cancelReauthentication">Cancel</button><button class="inline-flex h-10 items-center gap-2 rounded-md bg-[#165a4a] px-4 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60" type="submit" :disabled="reauthenticating || !reauthenticationPassword"><LoaderCircle v-if="reauthenticating" class="h-4 w-4 animate-spin" />Confirm</button></div>
      </form>
    </div>
  </section>
</template>

<script setup lang="ts">
import {
  ArrowLeft,
  CalendarClock,
  ChevronDown,
  Clock3,
  Copy,
  KeyRound,
  LoaderCircle,
  LockKeyhole,
  LogOut,
  Mail,
  RefreshCw,
  Save,
  Search,
  ShieldCheck,
  UserCheck,
  UserMinus,
  UserPlus,
  Users,
  UserX,
  X
} from 'lucide-vue-next'
import type { AdminProject, AdminProjectMember } from '~/composables/useAdminApi'
import { normalizeAPIError, useAdminApi } from '~/composables/useAdminApi'

type PendingProtectedAction = { label: string, run: () => Promise<void> }
type InvitationToken = { email: string, token: string, expiresAt: string }

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => {
  const value = route.params.projectId
  return Array.isArray(value) ? String(value[0] || '') : String(value || '')
})
const roleOptions = [
  { value: 'project_owner', label: 'Project owner' },
  { value: 'project_admin', label: 'Project admin' },
  { value: 'editor', label: 'Editor' },
  { value: 'reviewer', label: 'Reviewer' },
  { value: 'writer', label: 'Writer' }
]

const project = ref<AdminProject | null>(null)
const currentUserID = ref('')
const members = ref<AdminProjectMember[]>([])
const pending = ref(true)
const loadingMore = ref(false)
const nextCursor = ref('')
const creating = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const invitationToken = ref<InvitationToken | null>(null)
const roleDrafts = reactive<Record<string, string>>({})
const actionPending = reactive<Record<string, string>>({})
const minimumExpiry = ref('')
const search = ref('')
const statusFilter = ref('all')
const roleFilter = ref('all')
const reauthenticationOpen = ref(false)
const reauthenticationPassword = ref('')
const reauthenticationError = ref('')
const reauthenticating = ref(false)
const reauthenticationInput = ref<HTMLInputElement | null>(null)
const pendingProtectedAction = ref<PendingProtectedAction | null>(null)
const form = reactive({ email: '', role: 'writer', expiresAt: '' })

const canManageMembers = computed(() => project.value?.status === 'active' && ['project_owner', 'project_admin'].includes(project.value?.role || ''))
const canManageOwnership = computed(() => project.value?.role === 'project_owner')
const canInvite = computed(() => canManageMembers.value && Boolean(form.email.trim() && form.role) && (form.role !== 'project_owner' || canManageOwnership.value))
const invitationURL = computed(() => invitationToken.value && import.meta.client ? `${window.location.origin}/invitations/${encodeURIComponent(invitationToken.value.token)}` : '')
const filteredMembers = computed(() => {
  const term = search.value.toLowerCase()
  return members.value.filter(member => {
    const matchesSearch = !term || `${member.email} ${member.userId} ${member.role}`.toLowerCase().includes(term)
    return matchesSearch && (statusFilter.value === 'all' || member.status === statusFilter.value) && (roleFilter.value === 'all' || member.role === roleFilter.value)
  })
})
const memberStats = computed(() => ({
  active: members.value.filter(member => member.status === 'active').length,
  invited: members.value.filter(member => member.status === 'invited').length,
  owners: members.value.filter(member => member.status === 'active' && member.role === 'project_owner').length,
  disabled: members.value.filter(member => member.status === 'active' && member.userStatus === 'disabled').length
}))

onMounted(() => {
  minimumExpiry.value = formatLocalDateTimeInput(new Date(Date.now() + 60_000))
  void refresh()
})

watch(reauthenticationOpen, async (open) => {
  if (!open) return
  await nextTick()
  reauthenticationInput.value?.focus()
})

async function refresh() {
  pending.value = true
  clearMessages()
  try {
    const [projectResponse, memberResponse, currentUserResponse] = await Promise.all([
      api.getProject(projectID.value),
      api.listMembers(projectID.value, '', 50),
      api.currentUser()
    ])
    project.value = projectResponse.data
    currentUserID.value = currentUserResponse.data.id
    members.value = sortMembers(memberResponse.data)
    nextCursor.value = memberResponse.meta?.nextCursor || ''
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
    const response = await api.listMembers(projectID.value, nextCursor.value, 50)
    const merged = new Map(members.value.map(member => [member.userId, member]))
    for (const member of response.data) merged.set(member.userId, member)
    members.value = sortMembers([...merged.values()])
    nextCursor.value = response.meta?.nextCursor || ''
    syncRoleDrafts()
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load more members.')
  } finally {
    loadingMore.value = false
  }
}

async function inviteMember() {
  if (!canInvite.value) return
  creating.value = true
  clearMessages()
  try {
    const response = await api.inviteMember(projectID.value, {
      email: form.email,
      role: form.role,
      ...(form.expiresAt ? { expiresAt: new Date(form.expiresAt).toISOString() } : {})
    })
    upsertMember(response.data.member)
    invitationToken.value = { email: response.data.member.email, token: response.data.token, expiresAt: response.data.expiresAt }
    form.email = ''
    form.role = 'writer'
    form.expiresAt = ''
    successMessage.value = 'Invitation created. Copy the one-time link now.'
  } catch (error) {
    if (queueReauthentication(error, 'invite a project owner', inviteMember)) return
    errorMessage.value = normalizeAPIError(error, 'Could not create invitation.')
  } finally {
    creating.value = false
  }
}

function canEditMember(member: AdminProjectMember) {
  if (!canManageMembers.value || member.status === 'removed') return false
  return canManageOwnership.value || member.role !== 'project_owner'
}

async function saveRole(member: AdminProjectMember) {
  const role = roleDrafts[member.userId]
  if (!canEditMember(member) || !role || role === member.role || (role === 'project_owner' && !canManageOwnership.value)) return
  actionPending[member.userId] = 'role'
  clearMessages()
  try {
    const response = await api.updateMember(projectID.value, member.userId, role)
    upsertMember(response.data)
    successMessage.value = `${member.email} is now ${roleLabel(response.data.role)}.`
  } catch (error) {
    if (queueReauthentication(error, `change ${member.email}'s ownership`, () => saveRole(member))) return
    roleDrafts[member.userId] = member.role
    errorMessage.value = normalizeAPIError(error, 'Could not update member role.')
  } finally {
    delete actionPending[member.userId]
  }
}

async function removeMember(member: AdminProjectMember) {
  if (!canEditMember(member)) return
  const selfMessage = member.userId === currentUserID.value ? ' You will immediately lose access to this project.' : ''
  if (!window.confirm(`Remove ${member.email} from this project?${selfMessage}`)) return
  await performRemoveMember(member)
}

async function performRemoveMember(member: AdminProjectMember) {
  actionPending[member.userId] = 'remove'
  clearMessages()
  try {
    await api.removeMember(projectID.value, member.userId)
    const removedAt = new Date().toISOString()
    upsertMember({ ...member, status: 'removed', updatedAt: removedAt, removedAt })
    successMessage.value = 'Member removed. Historical records were retained.'
    if (member.userId === currentUserID.value) await navigateTo('/projects')
  } catch (error) {
    if (queueReauthentication(error, `remove owner ${member.email}`, () => performRemoveMember(member))) return
    errorMessage.value = normalizeAPIError(error, 'Could not remove member.')
  } finally {
    delete actionPending[member.userId]
  }
}

function canShowLoginAction(member: AdminProjectMember) {
  return canManageOwnership.value && member.status === 'active' && member.userId !== currentUserID.value && ['active', 'disabled'].includes(member.userStatus)
}

async function toggleMemberLogin(member: AdminProjectMember) {
  const action = member.userStatus === 'disabled' ? 'enable' : 'disable'
  if (action === 'disable' && !window.confirm(`Disable login for ${member.email}? All active sessions will be revoked, while memberships and history remain.`)) return
  await performMemberLoginAction(member, action)
}

async function performMemberLoginAction(member: AdminProjectMember, action: 'disable' | 'enable') {
  actionPending[member.userId] = 'login'
  clearMessages()
  try {
    const response = await api.memberLoginAction(projectID.value, member.userId, action)
    upsertMember(response.data)
    successMessage.value = action === 'disable' ? 'Member login disabled and active sessions revoked.' : 'Member login enabled.'
  } catch (error) {
    if (queueReauthentication(error, `${action} login for ${member.email}`, () => performMemberLoginAction(member, action))) return
    errorMessage.value = normalizeAPIError(error, `Could not ${action} member login.`)
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
    await api.reauthenticate(reauthenticationPassword.value)
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
  if (!invitationURL.value) return
  try {
    await navigator.clipboard.writeText(invitationURL.value)
    successMessage.value = 'Invitation link copied.'
  } catch {
    errorMessage.value = 'Your browser blocked clipboard access. Select and copy the link manually.'
  }
}

async function logout() {
  try { await api.logout() } finally { await navigateTo('/') }
}

function upsertMember(member: AdminProjectMember) {
  const index = members.value.findIndex(item => item.userId === member.userId)
  if (index >= 0) members.value.splice(index, 1, member)
  else members.value.push(member)
  members.value = sortMembers(members.value)
  roleDrafts[member.userId] = member.role
}

function syncRoleDrafts() {
  for (const member of members.value) roleDrafts[member.userId] = member.role
}

function sortMembers(values: AdminProjectMember[]) {
  return [...values].sort((left, right) => statusRank(left.status) - statusRank(right.status) || left.email.localeCompare(right.email))
}

function statusRank(status: string) {
  if (status === 'active') return 0
  if (status === 'invited') return 1
  return 2
}

function parseBackendUTC(value: string) { return new Date(value.includes('T') ? value : `${value.replace(' ', 'T')}Z`) }
function formatDate(value?: string) {
  if (!value) return 'Not set'
  const date = parseBackendUTC(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}
function formatLocalDateTimeInput(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}
function roleLabel(role: string) { return role ? role.replaceAll('_', ' ') : 'unknown' }
function roleDescription(role: string) {
  const descriptions: Record<string, string> = {
    project_owner: 'Full project control, ownership transfer, account login controls, and publishing.',
    project_admin: 'Manage members except owners, settings, content, review, and publishing.',
    editor: 'Create, review, approve, schedule, publish, and manage taxonomy.',
    reviewer: 'Review exact revisions, request changes, approve, and verify claims.',
    writer: 'Create and edit drafts, submit revisions, comment, and use writing assistance.'
  }
  return descriptions[role] || 'Project-scoped access.'
}
function roleClass(role: string) {
  if (role === 'project_owner') return 'bg-[#e8f0ff] text-[#245b99] dark:bg-[#152944] dark:text-[#b8d5ff]'
  if (role === 'project_admin') return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
  if (role === 'editor') return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
  return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
}
function statusClass(status: string) {
  if (status === 'active') return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
  if (status === 'invited') return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
  if (status === 'removed') return 'bg-[#fbe4e1] text-[#8f3028] dark:bg-[#46231f] dark:text-[#ffc4bd]'
  return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
}
function accountStatusClass(status: string) {
  if (status === 'active') return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
  if (status === 'disabled') return 'bg-[#fbe4e1] text-[#8f3028] dark:bg-[#46231f] dark:text-[#ffc4bd]'
  if (status === 'invited') return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
  return 'bg-[#eef2ef] text-[#58625c] dark:bg-[#2a302d] dark:text-[#bec7c1]'
}
function clearMessages() { errorMessage.value = ''; successMessage.value = '' }
function apiProblem(error: unknown) {
  if (typeof error !== 'object' || error === null || !('data' in error)) return null
  return (error as { data?: { title?: string, detail?: string } }).data || null
}
</script>

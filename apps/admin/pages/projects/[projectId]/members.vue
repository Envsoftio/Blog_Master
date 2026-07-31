<template>
  <div class="page-stack">
    <div class="page-heading">
      <div>
        <p>Control project access, editorial roles, invitations, and account availability.</p>
      </div>
      <div class="member-heading-actions">
        <span class="status-pill member-current-role">Your role: {{ roleLabel(project?.role || '') }}</span>
        <button class="button button--compact" type="button" :disabled="pending" @click="refresh">
          <RefreshCw :class="{ spin: pending }" :size="16" />
          Refresh
        </button>
      </div>
    </div>

    <div class="metric-grid">
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Active members</span><Users :size="17" /></div>
        <p class="metric-card__value">{{ memberStats.active }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Pending invites</span><Mail :size="17" /></div>
        <p class="metric-card__value metric-card__value--warning">{{ memberStats.invited }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Project owners</span><ShieldCheck :size="17" /></div>
        <p class="metric-card__value metric-card__value--blue">{{ memberStats.owners }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Login disabled</span><UserX :size="17" /></div>
        <p class="metric-card__value metric-card__value--danger">{{ memberStats.disabled }}</p>
      </article>
    </div>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success" role="status">{{ successMessage }}</p>

    <section v-if="invitationToken" class="invitation-result surface" aria-labelledby="invitation-token-title">
      <span class="invitation-result__icon"><Copy :size="18" /></span>
      <div class="invitation-result__main">
        <span>One-time invitation link</span>
        <h2 id="invitation-token-title">Invitation for {{ invitationToken.email }}</h2>
        <p>Copy this private link now. The server cannot display it again.</p>
        <div class="invitation-result__link">
          <code>{{ invitationURL }}</code>
          <button class="button button--primary button--compact" type="button" @click="copyInvitationToken">
            <Copy :size="15" />Copy link
          </button>
        </div>
        <small><Clock3 :size="13" />Expires {{ formatDate(invitationToken.expiresAt) }}</small>
      </div>
      <button class="icon-button invitation-result__close" type="button" title="Dismiss invitation link" aria-label="Dismiss invitation link" @click="invitationToken = null">
        <X :size="16" />
      </button>
    </section>

    <p v-if="nextCursor" class="member-page-note">Summary cards reflect loaded members. Load all pages for complete totals.</p>

    <div class="members-layout">
      <section class="members-directory">
        <div class="member-toolbar surface surface--subtle">
          <label class="member-search">
            <Search :size="16" />
            <input v-model.trim="search" type="search" placeholder="Search members" aria-label="Search loaded members">
          </label>
          <label class="member-filter">
            <span>Membership status</span>
            <select v-model="statusFilter" aria-label="Membership status">
              <option value="all">All statuses</option>
              <option value="active">Active</option>
              <option value="invited">Invited</option>
              <option value="removed">Removed</option>
            </select>
          </label>
          <label class="member-filter">
            <span>Project role</span>
            <select v-model="roleFilter" aria-label="Project role">
              <option value="all">All roles</option>
              <option v-for="role in roleOptions" :key="role.value" :value="role.value">{{ role.label }}</option>
            </select>
          </label>
          <span class="member-toolbar__count">{{ filteredMembers.length }} shown</span>
        </div>

        <div v-if="pending" class="loading-surface surface" aria-live="polite">
          <LoaderCircle class="spin" :size="18" />Loading members
        </div>
        <div v-else-if="members.length === 0" class="empty-state">
          <div>
            <span class="empty-state__icon"><Users :size="20" /></span>
            <h3>No members found</h3>
            <p>Invite a teammate to start collaborating on this project.</p>
          </div>
        </div>
        <div v-else-if="filteredMembers.length === 0" class="empty-state">
          <div>
            <span class="empty-state__icon"><Search :size="20" /></span>
            <h3>No members match</h3>
            <p>Try another search, role, or membership status.</p>
          </div>
        </div>

        <div v-else class="member-list surface">
          <article v-for="member in filteredMembers" :key="member.userId" class="member-row">
            <span class="member-avatar" :class="{ 'member-avatar--inactive': member.status === 'removed' }">{{ initials(member.email) }}</span>

            <div class="member-row__main">
              <div class="member-row__title">
                <h3>{{ member.email }}</h3>
                <span v-if="member.userId === currentUserID" class="member-you">You</span>
              </div>

              <div class="member-row__pills">
                <span class="status-pill" :class="roleClass(member.role)">{{ roleLabel(member.role) }}</span>
                <span class="status-pill" :class="statusClass(member.status)">{{ member.status }}</span>
                <span class="status-pill" :class="accountStatusClass(member.userStatus)">Login {{ member.userStatus }}</span>
              </div>

              <div class="member-row__meta">
                <span><Mail :size="13" />Invited {{ formatDate(member.invitedAt) }}</span>
                <span><ShieldCheck :size="13" />Joined {{ formatDate(member.joinedAt) }}</span>
                <span><CalendarClock :size="13" />Updated {{ formatDate(member.updatedAt) }}</span>
              </div>

              <p v-if="member.status === 'removed'" class="member-removed-note">
                Removed {{ formatDate(member.removedAt) }}. Historical authorship and audit records remain intact.
              </p>

              <div v-else-if="canManageMembers" class="member-row__actions">
                <select v-model="roleDrafts[member.userId]" :aria-label="`Role for ${member.email}`" :disabled="!canEditMember(member) || Boolean(actionPending[member.userId])">
                  <option v-for="role in roleOptions" :key="role.value" :value="role.value" :disabled="role.value === 'project_owner' && !canManageOwnership">{{ role.label }}</option>
                </select>
                <button class="button button--compact" type="button" :disabled="!canEditMember(member) || Boolean(actionPending[member.userId]) || roleDrafts[member.userId] === member.role" @click="saveRole(member)">
                  <LoaderCircle v-if="actionPending[member.userId] === 'role'" class="spin" :size="15" />
                  <Save v-else :size="15" />Save role
                </button>
                <button class="button button--compact member-action--danger" type="button" :disabled="!canEditMember(member) || Boolean(actionPending[member.userId])" @click="removeMember(member)">
                  <LoaderCircle v-if="actionPending[member.userId] === 'remove'" class="spin" :size="15" />
                  <UserMinus v-else :size="15" />Remove
                </button>
                <button v-if="canShowLoginAction(member)" class="button button--compact member-login-action" :class="member.userStatus === 'disabled' ? 'member-action--success' : 'member-action--danger'" type="button" :disabled="Boolean(actionPending[member.userId])" @click="toggleMemberLogin(member)">
                  <LoaderCircle v-if="actionPending[member.userId] === 'login'" class="spin" :size="15" />
                  <UserCheck v-else-if="member.userStatus === 'disabled'" :size="15" />
                  <UserX v-else :size="15" />
                  {{ member.userStatus === 'disabled' ? 'Enable login' : 'Disable login' }}
                </button>
              </div>
            </div>
          </article>
        </div>

        <button v-if="nextCursor" class="button member-load-more" type="button" :disabled="loadingMore" @click="loadMoreMembers">
          <LoaderCircle v-if="loadingMore" class="spin" :size="16" />
          <RefreshCw v-else :size="16" />
          Load more members
        </button>
      </section>

      <aside class="member-sidebar">
        <form v-if="canManageMembers" class="member-panel surface" @submit.prevent="inviteMember">
          <div class="member-panel__header">
            <span class="member-panel__icon"><UserPlus :size="18" /></span>
            <div><span>Project access</span><h2>Invite a member</h2></div>
          </div>
          <div class="member-panel__body">
            <label class="field">
              <span>Email</span>
              <input v-model.trim="form.email" type="email" autocomplete="email" placeholder="name@company.com" required>
            </label>
            <label class="field">
              <span>Role</span>
              <select v-model="form.role">
                <option v-for="role in roleOptions" :key="role.value" :value="role.value" :disabled="role.value === 'project_owner' && !canManageOwnership">{{ role.label }}</option>
              </select>
              <small>{{ roleDescription(form.role) }}</small>
            </label>
            <label class="field">
              <span>Expires at <em>Optional</em></span>
              <input v-model="form.expiresAt" type="datetime-local" :min="minimumExpiry">
              <small>Leave blank to use the server default.</small>
            </label>
            <button class="button button--primary member-invite-button" type="submit" :disabled="creating || !canInvite">
              <LoaderCircle v-if="creating" class="spin" :size="16" />
              <UserPlus v-else :size="16" />Create invitation
            </button>
          </div>
        </form>

        <section v-else class="member-panel surface">
          <div class="member-panel__header">
            <span class="member-panel__icon"><ShieldCheck :size="18" /></span>
            <div><span>Project access</span><h2>Read-only membership</h2></div>
          </div>
          <p class="member-panel__copy">Only project owners and administrators can invite, change, or remove members.</p>
        </section>

        <section class="member-panel surface">
          <div class="member-panel__header">
            <span class="member-panel__icon member-panel__icon--blue"><KeyRound :size="18" /></span>
            <div><span>Permissions</span><h2>Role guide</h2></div>
          </div>
          <dl class="role-guide">
            <div v-for="role in roleOptions" :key="role.value">
              <dt>{{ role.label }}</dt>
              <dd>{{ roleDescription(role.value) }}</dd>
            </div>
          </dl>
        </section>
      </aside>
    </div>

    <div v-if="reauthenticationOpen" class="member-dialog-backdrop" @click.self="cancelReauthentication">
      <form class="member-dialog surface" role="dialog" aria-modal="true" aria-labelledby="member-reauthentication-title" @submit.prevent="confirmReauthentication">
        <div class="member-panel__header">
          <span class="member-panel__icon member-panel__icon--blue"><LockKeyhole :size="18" /></span>
          <div>
            <span>Protected action</span>
            <h2 id="member-reauthentication-title">Confirm your identity</h2>
          </div>
        </div>
        <p>Enter your current password to {{ pendingProtectedAction?.label || 'continue' }}.</p>
        <p v-if="reauthenticationError" class="ui-alert ui-alert--danger" role="alert">{{ reauthenticationError }}</p>
        <label class="field">
          <span>Current password</span>
          <input ref="reauthenticationInput" v-model="reauthenticationPassword" type="password" autocomplete="current-password" required>
        </label>
        <div class="member-dialog__actions">
          <button class="button" type="button" :disabled="reauthenticating" @click="cancelReauthentication">Cancel</button>
          <button class="button button--primary" type="submit" :disabled="reauthenticating || !reauthenticationPassword">
            <LoaderCircle v-if="reauthenticating" class="spin" :size="16" />Confirm
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  CalendarClock,
  Clock3,
  Copy,
  KeyRound,
  LoaderCircle,
  LockKeyhole,
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
function roleLabel(role: string) { return roleOptions.find(option => option.value === role)?.label || 'Unknown' }
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
  if (role === 'project_owner') return 'member-pill--blue'
  if (role === 'project_admin') return 'status-pill--success'
  if (role === 'editor') return 'status-pill--warning'
  return ''
}
function statusClass(status: string) {
  if (status === 'active') return 'status-pill--success'
  if (status === 'invited') return 'status-pill--warning'
  if (status === 'removed') return 'status-pill--danger'
  return ''
}
function accountStatusClass(status: string) {
  if (status === 'active') return 'status-pill--success'
  if (status === 'disabled') return 'status-pill--danger'
  if (status === 'invited') return 'status-pill--warning'
  return ''
}
function initials(email: string) {
  const localPart = email.split('@')[0] || email
  const parts = localPart.split(/[._\-\s]+/).filter(Boolean)
  return (parts.length > 1 ? `${parts[0]?.[0] || ''}${parts[1]?.[0] || ''}` : localPart.slice(0, 2)).toUpperCase()
}
function clearMessages() { errorMessage.value = ''; successMessage.value = '' }
function apiProblem(error: unknown) {
  if (typeof error !== 'object' || error === null || !('data' in error)) return null
  return (error as { data?: { title?: string, detail?: string } }).data || null
}
</script>

<style scoped>
.member-heading-actions { display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: 8px; }
.member-current-role { min-height: 36px; padding-inline: 11px; }
.metric-card__value--warning { color: var(--amber); }
.metric-card__value--blue { color: var(--blue); }
.metric-card__value--danger { color: var(--danger); }
.member-page-note { margin: -10px 0 0; color: var(--text-faint); font-size: 12px; }
.invitation-result { display: grid; grid-template-columns: 40px minmax(0, 1fr) 34px; gap: 12px; padding: 15px; border-color: color-mix(in srgb, var(--primary) 38%, var(--border)); background: color-mix(in srgb, var(--primary-soft) 48%, var(--surface)); }
.invitation-result__icon { display: grid; width: 40px; height: 40px; place-items: center; border-radius: 7px; background: var(--primary-soft); color: var(--primary); }
.invitation-result__main { min-width: 0; }
.invitation-result__main > span { color: var(--primary); font-size: 12px; font-weight: 700; text-transform: uppercase; }
.invitation-result h2 { overflow: hidden; margin: 1px 0 0; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.invitation-result p { margin: 4px 0 0; color: var(--text-soft); font-size: 13px; }
.invitation-result__link { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; margin-top: 11px; }
.invitation-result code { display: block; min-width: 0; overflow-x: auto; padding: 9px 11px; border-radius: 6px; background: var(--sidebar); color: #e8fff8; font-size: 12px; white-space: nowrap; }
.invitation-result small { display: inline-flex; align-items: center; gap: 5px; margin-top: 8px; color: var(--text-faint); font-size: 12px; }
.invitation-result__close { border-color: var(--border); color: var(--text-soft); }
.members-layout { display: grid; grid-template-columns: minmax(0, 1fr) 330px; align-items: start; gap: 16px; }
.members-directory { display: grid; min-width: 0; gap: 14px; }
.member-toolbar { display: grid; grid-template-columns: minmax(220px, 1fr) 160px 160px auto; align-items: end; gap: 8px; padding: 8px; }
.member-search { display: flex; min-height: 36px; align-items: center; gap: 8px; padding: 0 10px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--text-soft); }
.member-search input { width: 100%; min-width: 0; min-height: 34px; padding: 0; border: 0; outline: 0; background: transparent; color: var(--text); font-size: 13px; }
.member-search:focus-within { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent); }
.member-filter { display: grid; gap: 3px; }
.member-filter > span { padding-left: 2px; color: var(--text-faint); font-size: 12px; font-weight: 650; }
.member-filter select,
.member-row__actions select { min-height: 36px; padding: 7px 28px 7px 10px; border: 1px solid var(--border); border-radius: 6px; outline: 0; background: var(--surface); color: var(--text); font-size: 13px; }
.member-filter select:focus,
.member-row__actions select:focus { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent); }
.member-toolbar__count { align-self: center; padding: 12px 4px 0; color: var(--text-faint); font-size: 12px; white-space: nowrap; }
.member-list { overflow: hidden; }
.member-row { display: grid; grid-template-columns: 44px minmax(0, 1fr); gap: 14px; padding: 16px; border-bottom: 1px solid var(--border); }
.member-row:last-child { border-bottom: 0; }
.member-row:hover { background: var(--surface-subtle); }
.member-avatar { display: grid; width: 44px; height: 44px; place-items: center; border-radius: 7px; background: var(--blue-soft); color: var(--blue); font-size: 12px; font-weight: 750; }
.member-avatar--inactive { background: var(--surface-subtle); color: var(--text-faint); }
.member-row__main { min-width: 0; }
.member-row__title { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: 7px; }
.member-row__title h3 { min-width: 0; overflow: hidden; margin: 0; font-size: 14px; font-weight: 680; text-overflow: ellipsis; white-space: nowrap; }
.member-you { padding: 2px 6px; border-radius: 999px; background: var(--surface-subtle); color: var(--text-soft); font-size: 12px; font-weight: 700; }
.member-row__pills { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 7px; }
.member-pill--blue { border-color: color-mix(in srgb, var(--blue) 35%, var(--border)); background: var(--blue-soft); color: var(--blue); }
.member-row__meta { display: flex; flex-wrap: wrap; gap: 8px 12px; margin-top: 9px; color: var(--text-faint); font-size: 12px; }
.member-row__meta span { display: inline-flex; align-items: center; gap: 4px; }
.member-removed-note { margin: 10px 0 0; padding: 8px 10px; border-radius: 6px; background: var(--surface-subtle); color: var(--text-soft); font-size: 12px; }
.member-row__actions { display: flex; flex-wrap: wrap; align-items: center; gap: 7px; margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--border); }
.member-row__actions select { min-width: 170px; }
.member-row__actions .button { min-height: 36px; font-size: 13px; }
.member-login-action { margin-left: auto; }
.member-action--danger { border-color: color-mix(in srgb, var(--danger) 35%, var(--border)); color: var(--danger); }
.member-action--danger:hover { border-color: var(--danger); background: var(--danger-soft); }
.member-action--success { border-color: color-mix(in srgb, var(--primary) 35%, var(--border)); color: var(--primary); }
.member-action--success:hover { border-color: var(--primary); background: var(--primary-soft); }
.member-load-more { width: 100%; }
.member-sidebar { position: sticky; top: 96px; display: grid; gap: 14px; }
.member-panel { overflow: hidden; }
.member-panel__header { display: flex; align-items: flex-start; gap: 10px; padding: 14px; border-bottom: 1px solid var(--border); }
.member-panel__icon { display: grid; width: 36px; height: 36px; flex: 0 0 36px; place-items: center; border-radius: 7px; background: var(--primary-soft); color: var(--primary); }
.member-panel__icon--blue { background: var(--blue-soft); color: var(--blue); }
.member-panel__header div > span { color: var(--text-soft); font-size: 12px; font-weight: 700; text-transform: uppercase; }
.member-panel__header h2 { margin: 1px 0 0; font-size: 15px; }
.member-panel__body { display: grid; gap: 13px; padding: 14px; }
.member-panel .field small { color: var(--text-faint); font-size: 12px; line-height: 1.45; }
.member-panel .field em { margin-left: 4px; color: var(--text-faint); font-size: 12px; font-style: normal; font-weight: 500; }
.member-invite-button { width: 100%; }
.member-panel__copy { margin: 0; padding: 14px; color: var(--text-soft); font-size: 13px; }
.role-guide { margin: 0; padding: 4px 14px 10px; }
.role-guide div { padding: 10px 0; border-bottom: 1px solid var(--border); }
.role-guide div:last-child { border-bottom: 0; }
.role-guide dt { font-size: 13px; font-weight: 680; }
.role-guide dd { margin: 3px 0 0; color: var(--text-soft); font-size: 12px; line-height: 1.45; }
.loading-surface { display: flex; min-height: 130px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.member-dialog-backdrop { position: fixed; inset: 0; z-index: 70; display: grid; place-items: center; padding: 20px; background: rgb(15 23 42 / 0.48); }
.member-dialog { width: min(440px, 100%); padding-bottom: 14px; box-shadow: var(--shadow-md); }
.member-dialog > p { margin: 14px 14px 0; color: var(--text-soft); font-size: 13px; }
.member-dialog > .ui-alert { color: var(--danger); }
.member-dialog > .field { margin: 14px 14px 0; }
.member-dialog__actions { display: flex; justify-content: flex-end; gap: 8px; margin: 16px 14px 0; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1180px) {
  .members-layout { grid-template-columns: minmax(0, 1fr) 300px; }
  .member-toolbar { grid-template-columns: minmax(180px, 1fr) 145px 145px; }
  .member-toolbar__count { display: none; }
}
@media (max-width: 980px) {
  .members-layout { grid-template-columns: 1fr; }
  .member-sidebar { position: static; grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .member-sidebar > :only-child { grid-column: 1 / -1; }
}
@media (max-width: 720px) {
  .member-toolbar { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .member-search { grid-column: 1 / -1; }
  .invitation-result { grid-template-columns: 36px minmax(0, 1fr) 34px; }
  .invitation-result__icon { width: 36px; height: 36px; }
  .invitation-result__link { grid-template-columns: 1fr; }
  .member-login-action { margin-left: 0; }
}
@media (max-width: 560px) {
  .member-heading-actions,
  .member-heading-actions .button { width: 100%; }
  .member-current-role { flex: 1; }
  .member-sidebar { grid-template-columns: 1fr; }
  .member-row { grid-template-columns: 38px minmax(0, 1fr); gap: 10px; padding: 13px; }
  .member-avatar { width: 38px; height: 38px; }
  .member-row__actions select,
  .member-row__actions .button { width: 100%; }
  .invitation-result { grid-template-columns: minmax(0, 1fr) 34px; }
  .invitation-result__icon { display: none; }
  .member-dialog-backdrop { align-items: stretch; padding: 10px; }
  .member-dialog { align-self: center; max-height: calc(100vh - 20px); overflow-y: auto; }
}
</style>

<template>
  <div class="page-stack">
    <div class="page-heading">
      <div><p>Create and name the organizations that group your projects.</p></div>
      <button class="button button--primary button--compact" type="button" @click="openCreateForm">
        <Plus :size="16" />New workspace
      </button>
    </div>

    <form v-if="formOpen" class="surface workspace-form" @submit.prevent="saveWorkspace">
      <div class="workspace-form__heading">
        <span class="workspace-form__icon"><Building2 :size="18" /></span>
        <div><p>{{ editingID ? 'Workspace settings' : 'New workspace' }}</p><h3>{{ editingID ? 'Rename workspace' : 'Workspace details' }}</h3></div>
        <button class="icon-button" type="button" title="Close" aria-label="Close" @click="closeForm"><X :size="17" /></button>
      </div>
      <div class="workspace-form__body">
        <label class="field">
          <span>Workspace name</span>
          <input v-model.trim="form.name" required maxlength="120" placeholder="Acme publishing">
        </label>
        <label class="field">
          <span>Workspace slug</span>
          <input v-model.trim="form.slug" required :disabled="Boolean(editingID)" placeholder="acme-publishing" @input="slugTouched = true">
          <small v-if="editingID">The stable workspace slug cannot be changed.</small>
        </label>
      </div>
      <div class="workspace-form__footer">
        <button class="button" type="button" @click="closeForm">Cancel</button>
        <button class="button button--primary" type="submit" :disabled="saving || !canSave">
          <LoaderCircle v-if="saving" class="spin" :size="16" /><Save v-else-if="editingID" :size="16" /><Plus v-else :size="16" />
          {{ editingID ? 'Save changes' : 'Create workspace' }}
        </button>
      </div>
    </form>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success">{{ successMessage }}</p>

    <div v-if="pending" class="loading-surface surface"><LoaderCircle class="spin" :size="18" />Loading workspaces</div>
    <div v-else-if="!workspaces.length" class="empty-state">
      <div><span class="empty-state__icon"><Building2 :size="20" /></span><h3>No workspaces yet</h3><p>Create a workspace before organizing projects.</p></div>
    </div>
    <div v-else class="workspace-grid">
      <article v-for="workspace in workspaces" :key="workspace.id" class="surface workspace-card">
        <span class="workspace-card__avatar">{{ initials(workspace.name) }}</span>
        <div class="workspace-card__copy">
          <h3>{{ workspace.name }}</h3>
          <p>{{ workspace.slug }}</p>
          <div><span class="status-pill status-pill--success">{{ labelize(workspace.role) }}</span><span>{{ workspace.projectCount }} {{ workspace.projectCount === 1 ? 'project' : 'projects' }}</span></div>
        </div>
        <div class="workspace-card__actions">
          <NuxtLink class="button button--compact" :to="`/projects?workspace=${workspace.id}`">View projects</NuxtLink>
          <button class="icon-button surface" type="button" title="Rename workspace" aria-label="Rename workspace" @click="openEditForm(workspace)"><Pencil :size="16" /></button>
          <button class="icon-button surface" type="button" title="Delete workspace" aria-label="Delete workspace" :disabled="workspace.projectCount > 0 || deletingID === workspace.id" @click="removeWorkspace(workspace)"><Trash2 :size="16" /></button>
        </div>
      </article>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Building2, LoaderCircle, Pencil, Plus, Save, Trash2, X } from 'lucide-vue-next'
import type { AdminWorkspace } from '~/composables/useAdminApi'

const api = useAdminApi()
const workspaces = ref<AdminWorkspace[]>([])
const pending = ref(true)
const saving = ref(false)
const deletingID = ref('')
const formOpen = ref(false)
const editingID = ref('')
const slugTouched = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const form = reactive({ name: '', slug: '' })
const canSave = computed(() => form.name.length >= 2 && form.slug.length >= 2)

watch(() => form.name, value => {
  if (!slugTouched.value && !editingID.value) form.slug = slugify(value)
})
onMounted(loadWorkspaces)

async function loadWorkspaces() {
  pending.value = true
  errorMessage.value = ''
  try { workspaces.value = (await api.listWorkspaces()).data } catch (error) { errorMessage.value = normalizeAPIError(error, 'Could not load workspaces.') } finally { pending.value = false }
}

function openCreateForm() {
  editingID.value = ''
  slugTouched.value = false
  Object.assign(form, { name: '', slug: '' })
  formOpen.value = true
}

function openEditForm(workspace: AdminWorkspace) {
  editingID.value = workspace.id
  slugTouched.value = true
  Object.assign(form, { name: workspace.name, slug: workspace.slug })
  formOpen.value = true
}

function closeForm() { formOpen.value = false }

async function saveWorkspace() {
  if (!canSave.value) return
  saving.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const response = editingID.value
      ? await api.updateWorkspace(editingID.value, form.name)
      : await api.createWorkspace({ name: form.name, slug: form.slug })
    workspaces.value = [...workspaces.value.filter(item => item.id !== response.data.id), response.data].sort((a, b) => a.name.localeCompare(b.name))
    successMessage.value = editingID.value ? 'Workspace renamed.' : 'Workspace created.'
    closeForm()
  } catch (error) { errorMessage.value = normalizeAPIError(error, 'Could not save the workspace.') } finally { saving.value = false }
}

async function removeWorkspace(workspace: AdminWorkspace) {
  if (workspace.projectCount || !confirm(`Delete the empty workspace “${workspace.name}”?`)) return
  deletingID.value = workspace.id
  errorMessage.value = ''
  try {
    await api.deleteWorkspace(workspace.id)
    workspaces.value = workspaces.value.filter(item => item.id !== workspace.id)
    successMessage.value = 'Workspace deleted.'
  } catch (error) { errorMessage.value = normalizeAPIError(error, 'Could not delete the workspace.') } finally { deletingID.value = '' }
}

function initials(value: string) { return value.split(/\s+/).filter(Boolean).slice(0, 2).map(part => part[0]?.toUpperCase()).join('') }
</script>

<style scoped>
.workspace-form { overflow: hidden; }
.workspace-form__heading { display: flex; align-items: center; gap: 11px; padding: 13px 16px; border-bottom: 1px solid var(--border); }
.workspace-form__heading p, .workspace-form__heading h3 { margin: 0; }
.workspace-form__heading p { color: var(--text-soft); font-size: 12px; }
.workspace-form__heading h3 { font-size: 14px; }
.workspace-form__heading .icon-button { margin-left: auto; }
.workspace-form__icon, .workspace-card__avatar { display: grid; place-items: center; border-radius: 7px; background: var(--primary-soft); color: var(--primary); }
.workspace-form__icon { width: 36px; height: 36px; }
.workspace-form__body { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; padding: 18px; }
.workspace-form__footer { display: flex; justify-content: flex-end; gap: 7px; padding: 12px 18px; border-top: 1px solid var(--border); background: var(--surface-subtle); }
.workspace-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.workspace-card { display: grid; grid-template-columns: 44px minmax(0, 1fr); gap: 12px; padding: 16px; }
.workspace-card__avatar { width: 44px; height: 44px; font-weight: 750; }
.workspace-card h3, .workspace-card p { margin: 0; }
.workspace-card p { margin-top: 3px; color: var(--text-soft); font-size: 12px; }
.workspace-card__copy > div { display: flex; align-items: center; gap: 10px; margin-top: 12px; color: var(--text-soft); font-size: 12px; }
.workspace-card__actions { display: flex; grid-column: 1 / -1; gap: 7px; padding-top: 12px; border-top: 1px solid var(--border); }
.workspace-card__actions .button { margin-right: auto; }
.loading-surface { display: flex; min-height: 140px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 780px) { .workspace-grid, .workspace-form__body { grid-template-columns: 1fr; } }
</style>

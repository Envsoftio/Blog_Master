<template>
  <fieldset class="contributors-editor" :disabled="disabled">
    <legend>Authors and contributors</legend>
    <p class="contributors-editor__help">
      Assign exactly one primary author. Ordering is preserved within each role on the immutable revision.
    </p>

    <div v-if="modelValue.length" class="contributors-editor__rows">
      <div v-for="(contributor, index) in modelValue" :key="`${contributor.authorId}-${contributor.role}-${index}`" class="contributors-editor__row">
        <label>
          <span>Person</span>
          <select :value="contributor.authorId" required @change="updateContributor(index, { authorId: selectValue($event) })">
            <option value="" disabled>Select an author</option>
            <option
              v-for="author in authors"
              :key="author.id"
              :value="author.id"
              :disabled="author.status === 'inactive' && author.id !== contributor.authorId"
            >
              {{ author.displayName }}{{ author.status === 'inactive' ? ' · inactive' : '' }}
            </option>
          </select>
        </label>

        <label>
          <span>Role</span>
          <select :value="contributor.role" @change="updateContributor(index, { role: contributorRoleValue($event) })">
            <option v-for="role in contributorRoles" :key="role.value" :value="role.value">{{ role.label }}</option>
          </select>
        </label>

        <div class="contributors-editor__actions" aria-label="Contributor ordering controls">
          <button type="button" :disabled="!canMove(index, -1)" :aria-label="`Move ${authorName(contributor.authorId)} earlier`" @click="moveContributor(index, -1)">
            <ChevronUp :size="15" />
          </button>
          <button type="button" :disabled="!canMove(index, 1)" :aria-label="`Move ${authorName(contributor.authorId)} later`" @click="moveContributor(index, 1)">
            <ChevronDown :size="15" />
          </button>
          <button class="contributors-editor__remove" type="button" :aria-label="`Remove ${authorName(contributor.authorId)}`" @click="removeContributor(index)">
            <Trash2 :size="15" />
          </button>
        </div>
      </div>
    </div>

    <p v-else class="contributors-editor__empty">No contributor is assigned. Add a primary author before saving.</p>

    <div class="contributors-editor__footer">
      <button type="button" :disabled="authors.length === 0" @click="addContributor">
        <UserPlus :size="15" />Add contributor
      </button>
      <span :class="validationMessage ? 'contributors-editor__invalid' : 'contributors-editor__valid'">
        {{ validationMessage || 'Attribution is ready.' }}
      </span>
    </div>
  </fieldset>
</template>

<script setup lang="ts">
import { ChevronDown, ChevronUp, Trash2, UserPlus } from 'lucide-vue-next'
import type { AdminAuthor, RevisionContributorInput } from '~/composables/useAdminApi'
import { hasValidRevisionContributors } from '~/composables/useAdminApi'

const props = withDefaults(defineProps<{
  modelValue: RevisionContributorInput[]
  authors: AdminAuthor[]
  disabled?: boolean
}>(), {
  disabled: false
})

const emit = defineEmits<{
  'update:modelValue': [value: RevisionContributorInput[]]
}>()

const contributorRoles: Array<{ value: RevisionContributorInput['role'], label: string }> = [
  { value: 'primary_author', label: 'Primary author' },
  { value: 'co_author', label: 'Co-author' },
  { value: 'editor', label: 'Editor' },
  { value: 'expert_reviewer', label: 'Expert reviewer' },
  { value: 'photographer', label: 'Photographer' },
  { value: 'other', label: 'Other credit' }
]

const validationMessage = computed(() => {
  const primaryCount = props.modelValue.filter(item => item.role === 'primary_author').length
  if (primaryCount !== 1) return 'Choose exactly one primary author.'
  if (props.modelValue.some(item => !item.authorId)) return 'Choose a person for every contributor.'
  if (!hasValidRevisionContributors(props.modelValue)) return 'Remove duplicate role assignments.'
  return ''
})

function addContributor() {
  const activeAuthors = props.authors.filter(author => author.status !== 'inactive')
  const author = activeAuthors.find(candidate => !props.modelValue.some(item => item.authorId === candidate.id)) || activeAuthors[0]
  if (!author) return
  const role: RevisionContributorInput['role'] = props.modelValue.some(item => item.role === 'primary_author')
    ? 'co_author'
    : 'primary_author'
  emitNormalized([...props.modelValue, { authorId: author.id, role, position: 0 }])
}

function updateContributor(index: number, patch: Partial<RevisionContributorInput>) {
  const next = props.modelValue.map(item => ({ ...item }))
  const current = next[index]
  if (!current) return
  Object.assign(current, patch)
  if (patch.role === 'primary_author') {
    next.forEach((item, itemIndex) => {
      if (itemIndex !== index && item.role === 'primary_author') item.role = 'co_author'
    })
  }
  emitNormalized(next)
}

function removeContributor(index: number) {
  emitNormalized(props.modelValue.filter((_, itemIndex) => itemIndex !== index))
}

function canMove(index: number, direction: -1 | 1) {
  const contributor = props.modelValue[index]
  if (!contributor) return false
  const peerIndexes = props.modelValue
    .map((item, itemIndex) => ({ item, itemIndex }))
    .filter(candidate => candidate.item.role === contributor.role)
    .map(candidate => candidate.itemIndex)
  const peerPosition = peerIndexes.indexOf(index)
  return direction < 0 ? peerPosition > 0 : peerPosition >= 0 && peerPosition < peerIndexes.length - 1
}

function moveContributor(index: number, direction: -1 | 1) {
  const contributor = props.modelValue[index]
  if (!contributor) return
  const peerIndexes = props.modelValue
    .map((item, itemIndex) => ({ item, itemIndex }))
    .filter(candidate => candidate.item.role === contributor.role)
    .map(candidate => candidate.itemIndex)
  const peerPosition = peerIndexes.indexOf(index)
  const swapIndex = peerIndexes[peerPosition + direction]
  if (swapIndex === undefined) return
  const next = props.modelValue.map(item => ({ ...item }))
  const temporary = next[index]
  next[index] = next[swapIndex]!
  next[swapIndex] = temporary!
  emitNormalized(next)
}

function emitNormalized(value: RevisionContributorInput[]) {
  const positions = new Map<RevisionContributorInput['role'], number>()
  emit('update:modelValue', value.map((item) => {
    const position = positions.get(item.role) || 0
    positions.set(item.role, position + 1)
    return { ...item, position: item.role === 'primary_author' ? 0 : position }
  }))
}

function authorName(authorId: string) {
  return props.authors.find(author => author.id === authorId)?.displayName || 'contributor'
}

function selectValue(event: Event) {
  return (event.target as HTMLSelectElement).value
}

function contributorRoleValue(event: Event) {
  return (event.target as HTMLSelectElement).value as RevisionContributorInput['role']
}
</script>

<style scoped>
.contributors-editor { min-width: 0; margin: 0; padding: 14px; border: 1px solid var(--border, #d7ded8); border-radius: 7px; }
.contributors-editor legend { padding: 0 7px; font-size: 12px; font-weight: 650; }
.contributors-editor__help { margin: 0 0 12px; color: var(--text-faint, #667169); font-size: 11px; line-height: 1.5; }
.contributors-editor__rows { display: grid; gap: 8px; }
.contributors-editor__row { display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr) auto; align-items: end; gap: 8px; }
.contributors-editor__row label { display: grid; gap: 5px; min-width: 0; }
.contributors-editor__row label span { color: var(--text-soft, #4f5b54); font-size: 10px; font-weight: 650; }
.contributors-editor select { width: 100%; min-height: 40px; padding: 0 10px; border: 1px solid var(--border-strong, #bfcac3); border-radius: 6px; background: var(--surface, #fff); color: var(--text, #28342d); }
.contributors-editor__actions { display: flex; gap: 4px; }
.contributors-editor__actions button,
.contributors-editor__footer button { display: inline-flex; min-height: 40px; align-items: center; justify-content: center; gap: 6px; padding: 0 10px; border: 1px solid var(--border, #d7ded8); border-radius: 6px; background: var(--surface, #fff); color: var(--text-soft, #4f5b54); cursor: pointer; }
.contributors-editor__actions button { width: 40px; padding: 0; }
.contributors-editor__actions button:disabled,
.contributors-editor__footer button:disabled { cursor: not-allowed; opacity: .45; }
.contributors-editor__actions .contributors-editor__remove { color: #9b2d23; }
.contributors-editor__empty { margin: 0; padding: 12px; border-radius: 6px; background: var(--surface-subtle, #f5f7f5); color: var(--text-faint, #667169); font-size: 11px; }
.contributors-editor__footer { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; margin-top: 10px; }
.contributors-editor__footer button { min-height: 36px; }
.contributors-editor__valid,
.contributors-editor__invalid { font-size: 10px; font-weight: 650; }
.contributors-editor__valid { color: #165a4a; }
.contributors-editor__invalid { color: #9b2d23; }
@media (max-width: 680px) {
  .contributors-editor__row { grid-template-columns: 1fr; }
  .contributors-editor__actions { justify-content: flex-end; }
}
</style>

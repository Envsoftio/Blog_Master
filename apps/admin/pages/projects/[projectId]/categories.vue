<template>
  <div class="page-stack category-page">
    <div class="page-heading">
      <div>
        <p>Build a clear, project-scoped hierarchy for article discovery and primary-category assignment.</p>
      </div>
      <div class="page-heading__actions">
        <NuxtLink class="button button--compact" :to="`/projects/${projectID}/tags`">
          <Tags :size="15" />Tags
        </NuxtLink>
        <button class="button button--compact" type="button" :disabled="pending" @click="refresh">
          <RefreshCw :class="{ spin: pending }" :size="15" />Refresh
        </button>
        <button v-if="canManageTaxonomy" class="button button--primary button--compact" type="button" @click="beginCreate()">
          <Plus :size="15" />New category
        </button>
      </div>
    </div>

    <dl class="metric-grid category-metrics">
      <div class="metric-card surface">
        <div class="metric-card__top"><dt>Categories</dt><FolderTree :size="17" /></div>
        <dd class="metric-card__value">{{ categories.length }}</dd>
      </div>
      <div class="metric-card surface">
        <div class="metric-card__top"><dt>Root categories</dt><Folders :size="17" /></div>
        <dd class="metric-card__value">{{ categoryStats.roots }}</dd>
      </div>
      <div class="metric-card surface">
        <div class="metric-card__top"><dt>Subcategories</dt><Network :size="17" /></div>
        <dd class="metric-card__value">{{ categoryStats.children }}</dd>
      </div>
      <div class="metric-card surface">
        <div class="metric-card__top"><dt>Indexable</dt><Globe2 :size="17" /></div>
        <dd class="metric-card__value">{{ categoryStats.indexable }}</dd>
      </div>
    </dl>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success" role="status">{{ successMessage }}</p>

    <div class="category-toolbar surface surface--subtle">
      <label class="category-search">
        <Search :size="16" />
        <input v-model.trim="search" type="search" placeholder="Search names, paths, slugs, or descriptions" aria-label="Search categories">
      </label>
      <span><Layers3 :size="14" />Three levels maximum</span>
    </div>

    <div v-if="pending" class="loading-surface surface" aria-live="polite">
      <LoaderCircle class="spin" :size="18" />Loading categories
    </div>

    <div v-else class="category-layout" :class="{ 'category-layout--single': !canManageTaxonomy }">
      <section class="category-tree surface" aria-labelledby="category-tree-heading">
        <header class="category-tree__heading">
          <div>
            <p>Taxonomy</p>
            <h2 id="category-tree-heading">Category tree</h2>
          </div>
          <span>{{ visibleCategoryRows.length }} shown</span>
        </header>

        <div v-if="categories.length === 0" class="empty-state empty-state--embedded">
          <div>
            <span class="empty-state__icon"><FolderTree :size="20" /></span>
            <h3>No categories yet</h3>
            <p>Create a root category before drafting or publishing articles.</p>
            <button v-if="canManageTaxonomy" class="button button--primary" type="button" @click="beginCreate()">Create category</button>
          </div>
        </div>

        <div v-else-if="visibleCategoryRows.length === 0" class="empty-state empty-state--embedded">
          <div>
            <span class="empty-state__icon"><SearchX :size="20" /></span>
            <h3>No matching categories</h3>
            <p>Try another name, path, slug, or description.</p>
            <button class="button" type="button" @click="search = ''">Clear search</button>
          </div>
        </div>

        <div v-else class="tree-list" role="tree" aria-label="Project category hierarchy">
          <article
            v-for="row in visibleCategoryRows"
            :key="row.category.id"
            class="tree-item"
            :class="{ 'is-selected': editingCategoryID === row.category.id, 'is-match': search && row.matchesSearch }"
            :style="{ '--tree-indent': `${row.depth * 24}px`, '--tree-indent-mobile': `${row.depth * 14}px` }"
            role="treeitem"
            :aria-level="row.depth + 1"
            :aria-expanded="row.childCount ? !collapsedCategoryIDs.has(row.category.id) : undefined"
          >
            <div class="tree-item__lead">
              <button
                v-if="row.childCount"
                class="tree-toggle"
                type="button"
                :title="collapsedCategoryIDs.has(row.category.id) ? 'Expand category' : 'Collapse category'"
                :aria-label="`${collapsedCategoryIDs.has(row.category.id) ? 'Expand' : 'Collapse'} ${row.category.name}`"
                @click="toggleCategory(row.category.id)"
              >
                <ChevronRight :class="{ 'is-open': !collapsedCategoryIDs.has(row.category.id) }" :size="15" />
              </button>
              <span v-else class="tree-toggle tree-toggle--empty" aria-hidden="true" />
              <span class="tree-item__icon"><FolderOpen v-if="row.childCount && !collapsedCategoryIDs.has(row.category.id)" :size="17" /><Folder v-else :size="17" /></span>
              <div class="tree-item__copy">
                <div>
                  <h3>{{ row.category.name }}</h3>
                  <span class="level-label">Level {{ row.depth + 1 }}</span>
                </div>
                <p>{{ row.path }}</p>
                <small>/{{ row.category.slug }}<template v-if="row.childCount"> · {{ row.childCount }} {{ row.childCount === 1 ? 'child' : 'children' }}</template></small>
              </div>
            </div>

            <p class="tree-item__description">{{ row.category.description || 'No category description' }}</p>

            <div class="tree-item__status">
              <span class="status-pill" :class="{ 'status-pill--success': row.category.indexable }">
                {{ row.category.indexable ? 'Indexable' : 'No index' }}
              </span>
            </div>

            <div v-if="canManageTaxonomy" class="tree-item__actions">
              <button v-if="row.depth < 2" class="icon-button" type="button" title="Add child category" :aria-label="`Add a child to ${row.category.name}`" @click="beginCreate(row.category.id)">
                <FolderPlus :size="16" />
              </button>
              <button class="icon-button" type="button" title="Edit category" :aria-label="`Edit ${row.category.name}`" @click="startEdit(row.category)">
                <Pencil :size="16" />
              </button>
            </div>
          </article>
        </div>
      </section>

      <aside v-if="canManageTaxonomy" class="category-rail">
        <form v-if="formOpen" class="category-form surface" @submit.prevent="saveCategory">
          <header class="category-form__heading">
            <span class="category-form__icon"><FolderPen :size="18" /></span>
            <div>
              <p>{{ editingCategoryID ? 'Update taxonomy' : form.parentId ? 'New subcategory' : 'New root category' }}</p>
              <h2>{{ editingCategoryID ? 'Edit category' : 'Category details' }}</h2>
            </div>
            <button class="icon-button" type="button" title="Close form" aria-label="Close category form" @click="closeForm"><X :size="17" /></button>
          </header>

          <div class="category-form__body">
            <label class="field">
              <span>Name</span>
              <input v-model.trim="form.name" placeholder="Technical guides" required>
            </label>

            <label class="field">
              <span>Slug</span>
              <input v-model.trim="form.slug" class="mono-input" placeholder="technical-guides" required @input="slugTouched = true">
              <small v-if="duplicateSlug" class="field-error">This slug is already used by {{ duplicateSlug.name }}.</small>
              <small v-else-if="editingCategory && normalizedSlug !== editingCategory.slug" class="field-note">Changing this slug creates a permanent redirect from /categories/{{ editingCategory.slug }}.</small>
            </label>

            <label class="field">
              <span>Parent category</span>
              <select v-model="form.parentId">
                <option value="">No parent · root category</option>
                <option v-for="option in parentOptions" :key="option.id" :value="option.id">{{ option.label }}</option>
              </select>
              <small>Position: level {{ formDepth + 1 }} of 3<template v-if="selectedParentRow"> · {{ selectedParentRow.path }}</template></small>
            </label>

            <label class="field">
              <span>Description</span>
              <textarea v-model.trim="form.description" placeholder="Explain what readers will find in this archive." />
            </label>

            <label class="indexability-control">
              <input v-model="form.indexable" type="checkbox">
              <span>
                <strong>Indexable archive</strong>
                <small>Allow landing applications to include this category in discovery outputs.</small>
              </span>
            </label>

            <p v-if="moveExceedsDepth" class="form-warning" role="alert">
              <TriangleAlert :size="15" />This move would push part of the subtree beyond level 3.
            </p>
          </div>

          <footer class="category-form__footer">
            <button class="button" type="button" @click="closeForm">Cancel</button>
            <button class="button button--primary" type="submit" :disabled="saving || !canSave">
              <LoaderCircle v-if="saving" class="spin" :size="16" />
              <Check v-else :size="16" />
              {{ editingCategoryID ? 'Save changes' : 'Create category' }}
            </button>
          </footer>
        </form>

        <section v-else class="category-guide surface">
          <span class="category-guide__icon"><ShieldCheck :size="19" /></span>
          <h2>Hierarchy rules</h2>
          <p>The category model follows the publishing requirements in the PRD.</p>
          <ul>
            <li><span>1</span><p><strong>One project</strong><small>Parents and children always stay inside this project.</small></p></li>
            <li><span>2</span><p><strong>Three levels</strong><small>Root, child, and grandchild categories are supported.</small></p></li>
            <li><span>3</span><p><strong>Stable slugs</strong><small>Slugs are unique project-wide; changes preserve redirects.</small></p></li>
          </ul>
          <button class="button button--primary" type="button" @click="beginCreate()"><Plus :size="15" />Create category</button>
        </section>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  Check,
  ChevronRight,
  Folder,
  FolderOpen,
  FolderPen,
  FolderPlus,
  FolderTree,
  Folders,
  Globe2,
  Layers3,
  LoaderCircle,
  Network,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  SearchX,
  ShieldCheck,
  Tags,
  TriangleAlert,
  X
} from 'lucide-vue-next'
import type { AdminProject, TaxonomyTerm, TaxonomyCreatePayload } from '~/composables/useAdminApi'

type CategoryRow = {
  category: TaxonomyTerm
  depth: number
  path: string
  ancestorIds: string[]
  childCount: number
  matchesSearch: boolean
}

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => String(route.params.projectId || ''))
const project = ref<AdminProject | null>(null)
const categories = ref<TaxonomyTerm[]>([])
const pending = ref(true)
const saving = ref(false)
const formOpen = ref(false)
const editingCategoryID = ref('')
const slugTouched = ref(false)
const search = ref('')
const errorMessage = ref('')
const successMessage = ref('')
const collapsedCategoryIDs = ref(new Set<string>())
const form = reactive({ name: '', slug: '', parentId: '', description: '', indexable: true })

const canManageTaxonomy = computed(() => project.value?.status === 'active' && ['project_owner', 'project_admin', 'editor'].includes(project.value?.role || ''))
const categoriesByID = computed(() => new Map(categories.value.map(category => [category.id, category])))
const categoryRows = computed(() => buildCategoryRows(categories.value))
const rowsByID = computed(() => new Map(categoryRows.value.map(row => [row.category.id, row])))
const editingCategory = computed(() => categoriesByID.value.get(editingCategoryID.value) || null)
const selectedParentRow = computed(() => rowsByID.value.get(form.parentId) || null)
const formDepth = computed(() => selectedParentRow.value ? selectedParentRow.value.depth + 1 : 0)
const editingSubtreeHeight = computed(() => editingCategoryID.value ? subtreeHeight(editingCategoryID.value) : 0)
const normalizedSlug = computed(() => slugify(form.slug))
const duplicateSlug = computed(() => categories.value.find(category => category.slug === normalizedSlug.value && category.id !== editingCategoryID.value) || null)
const moveExceedsDepth = computed(() => formDepth.value + editingSubtreeHeight.value > 2)
const canSave = computed(() => Boolean(form.name.trim() && normalizedSlug.value && !duplicateSlug.value && !moveExceedsDepth.value))
const categoryStats = computed(() => ({
  roots: categories.value.filter(category => !category.parentId).length,
  children: categories.value.filter(category => category.parentId).length,
  indexable: categories.value.filter(category => category.indexable).length
}))
const parentOptions = computed(() => categoryRows.value
  .filter(row => {
    if (row.category.id === editingCategoryID.value) return false
    if (editingCategoryID.value && row.ancestorIds.includes(editingCategoryID.value)) return false
    return row.depth + 1 + editingSubtreeHeight.value <= 2
  })
  .map(row => ({ id: row.category.id, label: `${row.path} · level ${row.depth + 1}` })))
const visibleCategoryRows = computed(() => {
  const term = search.value.trim().toLowerCase()
  if (!term) {
    return categoryRows.value.filter(row => !row.ancestorIds.some(id => collapsedCategoryIDs.value.has(id)))
  }

  const visible = new Set<string>()
  for (const row of categoryRows.value) {
    const haystack = `${row.category.name} ${row.category.slug} ${row.category.description || ''} ${row.path}`.toLowerCase()
    if (!haystack.includes(term)) continue
    visible.add(row.category.id)
    for (const ancestorID of row.ancestorIds) visible.add(ancestorID)
  }
  return categoryRows.value
    .filter(row => visible.has(row.category.id))
    .map(row => ({
      ...row,
      matchesSearch: `${row.category.name} ${row.category.slug} ${row.category.description || ''} ${row.path}`.toLowerCase().includes(term)
    }))
})

watch(() => form.name, value => {
  if (!editingCategoryID.value && !slugTouched.value) form.slug = slugify(value)
})

onMounted(refresh)

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const [projectResponse, categoryResponse] = await Promise.all([
      api.getProject(projectID.value),
      api.listCategories(projectID.value)
    ])
    project.value = projectResponse.data
    categories.value = sortCategories(categoryResponse.data)
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load categories. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

function beginCreate(parentID = '') {
  editingCategoryID.value = ''
  form.name = ''
  form.slug = ''
  form.parentId = parentID
  form.description = ''
  form.indexable = true
  slugTouched.value = false
  formOpen.value = true
  clearMessages()
}

function startEdit(category: TaxonomyTerm) {
  editingCategoryID.value = category.id
  form.name = category.name
  form.slug = category.slug
  form.parentId = category.parentId || ''
  form.description = category.description || ''
  form.indexable = category.indexable
  slugTouched.value = true
  formOpen.value = true
  clearMessages()
}

function closeForm() {
  formOpen.value = false
  editingCategoryID.value = ''
  slugTouched.value = false
}

async function saveCategory() {
  if (!canSave.value) return
  saving.value = true
  clearMessages()
  const editing = Boolean(editingCategoryID.value)
  try {
    const payload: TaxonomyCreatePayload = {
      name: form.name.trim(),
      slug: normalizedSlug.value,
      parentId: form.parentId,
      description: form.description.trim(),
      indexable: form.indexable
    }
    if (editing) {
      await api.updateCategory(projectID.value, editingCategoryID.value, payload)
    } else {
      await api.createCategory(projectID.value, payload)
    }
    categories.value = sortCategories((await api.listCategories(projectID.value)).data)
    closeForm()
    successMessage.value = editing ? 'Category updated.' : 'Category created.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, editing ? 'Could not update category.' : 'Could not create category.')
  } finally {
    saving.value = false
  }
}

function toggleCategory(categoryID: string) {
  const next = new Set(collapsedCategoryIDs.value)
  if (next.has(categoryID)) next.delete(categoryID)
  else next.add(categoryID)
  collapsedCategoryIDs.value = next
}

function buildCategoryRows(values: TaxonomyTerm[]) {
  const byID = new Map(values.map(category => [category.id, category]))
  const byParent = new Map<string, TaxonomyTerm[]>()
  for (const category of values) {
    const parentKey = category.parentId && byID.has(category.parentId) ? category.parentId : ''
    byParent.set(parentKey, [...(byParent.get(parentKey) || []), category])
  }
  const rows: CategoryRow[] = []
  const visit = (category: TaxonomyTerm, depth: number, ancestorNames: string[], ancestorIds: string[]) => {
    const children = sortCategories(byParent.get(category.id) || [])
    rows.push({
      category,
      depth,
      path: [...ancestorNames, category.name].join(' / '),
      ancestorIds,
      childCount: children.length,
      matchesSearch: false
    })
    for (const child of children) visit(child, depth + 1, [...ancestorNames, category.name], [...ancestorIds, category.id])
  }
  for (const root of sortCategories(byParent.get('') || [])) visit(root, 0, [], [])
  return rows
}

function subtreeHeight(categoryID: string): number {
  const children = categories.value.filter(category => category.parentId === categoryID)
  if (!children.length) return 0
  return 1 + Math.max(...children.map(child => subtreeHeight(child.id)))
}

function sortCategories(values: TaxonomyTerm[]) {
  return [...values].sort((left, right) => left.name.localeCompare(right.name) || left.id.localeCompare(right.id))
}

function slugify(value: string) {
  return value.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
}

function clearMessages() {
  errorMessage.value = ''
  successMessage.value = ''
}
</script>

<style scoped>
.page-heading__actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 7px; }
.category-metrics { margin: 0; }
.category-metrics .metric-card:nth-child(1) svg { color: var(--blue); }
.category-metrics .metric-card:nth-child(2) svg { color: var(--primary); }
.category-metrics .metric-card:nth-child(3) svg { color: var(--amber); }
.category-metrics .metric-card:nth-child(4) svg { color: #7454c0; }
.category-metrics dd { margin-inline: 0; }
.category-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px; }
.category-search { display: flex; width: min(440px, 100%); align-items: center; gap: 8px; padding: 0 10px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--text-soft); }
.category-search input { width: 100%; min-height: 34px; padding: 0; border: 0 !important; box-shadow: none !important; background: transparent !important; font-size: 13px; }
.category-toolbar > span { display: inline-flex; align-items: center; gap: 6px; padding-right: 5px; color: var(--text-soft); font-size: 12px; white-space: nowrap; }
.category-layout { display: grid; grid-template-columns: minmax(0, 1fr) 360px; gap: 16px; align-items: start; }
.category-layout--single { grid-template-columns: 1fr; }
.category-tree { min-width: 0; overflow: hidden; }
.category-tree__heading { display: flex; min-height: 62px; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 16px; border-bottom: 1px solid var(--border); }
.category-tree__heading p, .category-tree__heading h2 { margin: 0; }
.category-tree__heading p { color: var(--text-soft); font-size: 12px; }
.category-tree__heading h2 { margin-top: 1px; font-size: 14px; }
.category-tree__heading > span { color: var(--text-soft); font-size: 12px; }
.tree-item { position: relative; display: grid; grid-template-columns: minmax(260px, 1.25fr) minmax(180px, .9fr) auto auto; gap: 14px; align-items: center; min-height: 72px; padding: 10px 14px 10px calc(14px + var(--tree-indent)); border-bottom: 1px solid var(--border); }
.tree-item:last-child { border-bottom: 0; }
.tree-item:hover, .tree-item.is-selected { background: var(--surface-subtle); }
.tree-item.is-selected { box-shadow: inset 3px 0 0 var(--primary); }
.tree-item.is-match .tree-item__copy h3 { color: var(--primary); }
.tree-item__lead { display: flex; min-width: 0; align-items: flex-start; gap: 8px; }
.tree-toggle { display: grid; width: 20px; height: 34px; flex: 0 0 20px; place-items: center; padding: 0; border: 0; background: transparent; color: var(--text-soft); cursor: pointer; }
.tree-toggle svg { transition: transform 140ms ease; }
.tree-toggle svg.is-open { transform: rotate(90deg); }
.tree-toggle--empty { cursor: default; }
.tree-item__icon { display: grid; width: 34px; height: 34px; flex: 0 0 34px; place-items: center; border-radius: 6px; background: var(--primary-soft); color: var(--primary); }
.tree-item__copy { min-width: 0; }
.tree-item__copy > div { display: flex; min-width: 0; align-items: center; gap: 7px; }
.tree-item__copy h3, .tree-item__copy p, .tree-item__copy small { overflow: hidden; margin: 0; text-overflow: ellipsis; white-space: nowrap; }
.tree-item__copy h3 { font-size: 13px; }
.tree-item__copy p { margin-top: 3px; color: var(--text-soft); font-size: 12px; }
.tree-item__copy small { display: block; margin-top: 3px; color: var(--text-faint); font-size: 12px; }
.level-label { flex: 0 0 auto; padding: 2px 5px; border-radius: 4px; background: var(--surface-subtle); color: var(--text-faint); font-size: 12px; font-weight: 650; }
.tree-item__description { overflow: hidden; margin: 0; color: var(--text-soft); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.tree-item__status { display: flex; justify-content: flex-end; }
.tree-item__actions { display: flex; justify-content: flex-end; gap: 3px; }
.tree-item__actions .icon-button { border-color: var(--border); background: var(--surface); }
.empty-state--embedded { min-height: 280px; border: 0; border-radius: 0; box-shadow: none; }
.empty-state .button { margin-top: 14px; }
.category-rail { position: sticky; top: 92px; }
.category-form, .category-guide { overflow: hidden; }
.category-form__heading { display: flex; min-height: 66px; align-items: center; gap: 10px; padding: 12px 14px; border-bottom: 1px solid var(--border); }
.category-form__icon { display: grid; width: 36px; height: 36px; flex: 0 0 36px; place-items: center; border-radius: 7px; background: var(--blue-soft); color: var(--blue); }
.category-form__heading > div { min-width: 0; }
.category-form__heading p, .category-form__heading h2 { margin: 0; }
.category-form__heading p { color: var(--text-soft); font-size: 12px; }
.category-form__heading h2 { margin-top: 1px; font-size: 14px; }
.category-form__heading .icon-button { margin-left: auto; }
.category-form__body { display: grid; gap: 14px; padding: 16px; }
.field small { color: var(--text-soft); font-size: 12px; }
.field .field-error { color: var(--danger); }
.field .field-note { color: var(--amber); line-height: 1.45; }
.mono-input { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 13px; }
.indexability-control { display: grid; grid-template-columns: 16px minmax(0, 1fr); align-items: start; gap: 9px; padding: 11px; border: 1px solid var(--border); border-radius: 7px; background: var(--surface-subtle); cursor: pointer; }
.indexability-control input { width: 15px; height: 15px; min-height: 0; margin: 2px 0 0; accent-color: var(--primary); }
.indexability-control span { display: flex; flex-direction: column; }
.indexability-control strong { font-size: 12px; }
.indexability-control small { margin-top: 2px; color: var(--text-soft); font-size: 12px; line-height: 1.45; }
.form-warning { display: flex; gap: 7px; margin: 0; padding: 9px 10px; border: 1px solid color-mix(in srgb, var(--danger) 35%, var(--border)); border-radius: 6px; background: var(--danger-soft); color: var(--danger); font-size: 12px; }
.category-form__footer { display: flex; justify-content: flex-end; gap: 7px; padding: 12px 16px; border-top: 1px solid var(--border); background: var(--surface-subtle); }
.category-guide { padding: 18px; }
.category-guide__icon { display: grid; width: 40px; height: 40px; place-items: center; border-radius: 8px; background: var(--primary-soft); color: var(--primary); }
.category-guide h2 { margin: 13px 0 0; font-size: 15px; }
.category-guide > p { margin: 5px 0 0; color: var(--text-soft); font-size: 12px; }
.category-guide ul { display: grid; gap: 11px; margin: 18px 0; padding: 0; list-style: none; }
.category-guide li { display: grid; grid-template-columns: 24px minmax(0, 1fr); gap: 9px; }
.category-guide li > span { display: grid; width: 24px; height: 24px; place-items: center; border-radius: 50%; background: var(--surface-subtle); color: var(--primary); font-size: 12px; font-weight: 700; }
.category-guide li p { display: flex; margin: 0; flex-direction: column; }
.category-guide li strong { font-size: 12px; }
.category-guide li small { margin-top: 2px; color: var(--text-soft); font-size: 12px; }
.loading-surface { display: flex; min-height: 150px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1180px) {
  .category-layout { grid-template-columns: 1fr 330px; }
  .tree-item { grid-template-columns: minmax(230px, 1fr) auto auto; }
  .tree-item__description { display: none; }
}
@media (max-width: 980px) {
  .category-layout { grid-template-columns: 1fr; }
  .category-rail { position: static; grid-row: 1; }
}
@media (max-width: 680px) {
  .page-heading { align-items: stretch; flex-direction: column; }
  .page-heading__actions { justify-content: flex-start; }
  .category-toolbar { align-items: stretch; flex-direction: column; }
  .category-search { width: 100%; }
  .category-toolbar > span { padding: 2px 4px; }
  .tree-item { grid-template-columns: minmax(0, 1fr) auto; gap: 8px; padding-left: calc(10px + var(--tree-indent-mobile)); }
  .tree-item__status { grid-column: 1; justify-content: flex-start; padding-left: 62px; }
  .tree-item__actions { grid-column: 2; grid-row: 1 / span 2; }
}
@media (max-width: 520px) {
  .page-heading__actions .button { flex: 1; }
  .tree-item__copy p { display: none; }
  .category-form__footer .button { flex: 1; }
}
</style>

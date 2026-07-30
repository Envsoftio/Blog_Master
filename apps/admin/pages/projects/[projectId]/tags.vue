<template>
  <div class="page-stack">
    <div class="page-heading">
      <div>
        <h2>Tags</h2>
        <p>Flat project labels used for discovery, relationships, and editorial filtering.</p>
      </div>
      <div class="taxonomy-actions">
        <NuxtLink class="button button--compact" :to="`/projects/${projectID}/categories`"><FolderTree :size="15" />Categories</NuxtLink>
        <button class="button button--primary button--compact" type="button" @click="formOpen = !formOpen"><Plus :size="15" />New tag</button>
      </div>
    </div>

    <form v-if="formOpen" class="surface tag-form" @submit.prevent="createTag">
      <label class="field"><span>Name</span><input v-model.trim="form.name" required></label>
      <label class="field"><span>Slug</span><input v-model.trim="form.slug" required></label>
      <label class="field tag-form__description"><span>Description</span><input v-model.trim="form.description"></label>
      <label class="checkbox-field"><input v-model="form.indexable" type="checkbox"><span>Indexable archive</span></label>
      <div class="tag-form__actions">
        <button class="button button--compact" type="button" @click="formOpen = false">Cancel</button>
        <button class="button button--primary button--compact" type="submit" :disabled="creating">
          <LoaderCircle v-if="creating" class="spin" :size="15" /><Plus v-else :size="15" />Create tag
        </button>
      </div>
    </form>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success">{{ successMessage }}</p>

    <div class="tag-toolbar surface surface--subtle">
      <label><Search :size="15" /><input v-model.trim="search" type="search" placeholder="Search tags" aria-label="Search tags"></label>
      <span>{{ filteredTags.length }} tags</span>
    </div>

    <div v-if="pending" class="loading-surface surface"><LoaderCircle class="spin" :size="18" />Loading tags</div>
    <div v-else-if="filteredTags.length === 0" class="empty-state">
      <div><span class="empty-state__icon"><Tags :size="20" /></span><h3>No tags yet</h3><p>Create a tag to organize related content.</p></div>
    </div>
    <div v-else class="tag-list surface">
      <article v-for="tag in filteredTags" :key="tag.id" class="tag-item">
        <span class="tag-item__icon"><Tag :size="16" /></span>
        <div><h3>{{ tag.name }}</h3><p>/{{ tag.slug }}</p></div>
        <p>{{ tag.description || 'No description' }}</p>
        <span class="status-pill" :class="{ 'status-pill--success': tag.indexable }">{{ tag.indexable ? 'Indexable' : 'No index' }}</span>
      </article>
    </div>
  </div>
</template>

<script setup lang="ts">
import { FolderTree, LoaderCircle, Plus, Search, Tag, Tags } from 'lucide-vue-next'
import type { TaxonomyTerm } from '~/composables/useAdminApi'

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => String(route.params.projectId || ''))
const tags = ref<TaxonomyTerm[]>([])
const pending = ref(true)
const creating = ref(false)
const formOpen = ref(false)
const search = ref('')
const errorMessage = ref('')
const successMessage = ref('')
const form = reactive({ name: '', slug: '', description: '', indexable: true })
const filteredTags = computed(() => {
  const term = search.value.toLowerCase()
  return tags.value.filter(tag => !term || `${tag.name} ${tag.slug} ${tag.description || ''}`.toLowerCase().includes(term))
})

watch(() => form.name, value => {
  if (!form.slug) form.slug = slugify(value)
})

onMounted(loadTags)

async function loadTags() {
  pending.value = true
  try {
    tags.value = (await api.listTaxonomy(projectID.value, 'tags')).data
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load tags.')
  } finally {
    pending.value = false
  }
}

async function createTag() {
  creating.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const response = await api.createTaxonomy(projectID.value, 'tags', form)
    tags.value = [...tags.value, response.data].sort((a, b) => a.name.localeCompare(b.name))
    form.name = ''
    form.slug = ''
    form.description = ''
    form.indexable = true
    formOpen.value = false
    successMessage.value = 'Tag created.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create the tag.')
  } finally {
    creating.value = false
  }
}
</script>

<style scoped>
.taxonomy-actions { display: flex; gap: 7px; }
.tag-form { display: grid; grid-template-columns: 1fr 1fr 1.5fr auto; gap: 12px; align-items: end; padding: 16px; }
.checkbox-field { display: inline-flex; min-height: 40px; align-items: center; gap: 7px; font-size: 10px; }
.checkbox-field input { width: 15px; height: 15px; min-height: 0; }
.tag-form__actions { display: flex; grid-column: 1 / -1; justify-content: flex-end; gap: 7px; }
.tag-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px; }
.tag-toolbar label { display: flex; width: min(320px, 100%); align-items: center; gap: 8px; padding: 0 10px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--text-soft); }
.tag-toolbar input { width: 100%; min-height: 34px; padding: 0; border: 0 !important; box-shadow: none !important; background: transparent !important; font-size: 11px; }
.tag-toolbar > span { padding-right: 6px; color: var(--text-soft); font-size: 10px; }
.tag-list { overflow: hidden; }
.tag-item { display: grid; grid-template-columns: 34px minmax(150px, .7fr) minmax(200px, 1.3fr) auto; align-items: center; gap: 12px; padding: 12px 15px; border-bottom: 1px solid var(--border); }
.tag-item:last-child { border-bottom: 0; }
.tag-item__icon { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 6px; background: var(--primary-soft); color: var(--primary); }
.tag-item h3, .tag-item p { overflow: hidden; margin: 0; text-overflow: ellipsis; white-space: nowrap; }
.tag-item h3 { font-size: 11px; }
.tag-item div p, .tag-item > p { margin-top: 2px; color: var(--text-soft); font-size: 9px; }
.loading-surface { display: flex; min-height: 120px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 900px) { .tag-form { grid-template-columns: 1fr 1fr; } .tag-form__description { grid-column: 1 / -1; } .tag-item { grid-template-columns: 34px minmax(0, 1fr) auto; } .tag-item > p { display: none; } }
@media (max-width: 600px) { .tag-form { grid-template-columns: 1fr; } .tag-form__description { grid-column: auto; } .taxonomy-actions { width: 100%; } .taxonomy-actions > * { flex: 1; } }
</style>

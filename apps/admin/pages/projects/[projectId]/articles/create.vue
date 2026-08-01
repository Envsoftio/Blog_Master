<template>
  <div class="page-stack create-page">
    <div class="page-heading">
      <div>
        <p>Start with the article brief, choose an editorial template, and prepare the opening revision.</p>
      </div>
      <div class="create-heading-actions">
        <NuxtLink class="button button--compact" :to="`/projects/${projectID}/articles`">
          <ArrowLeft :size="16" />Back to content
        </NuxtLink>
        <button class="button button--compact" type="button" :disabled="pending" @click="refresh">
          <RefreshCw :class="{ spin: pending }" :size="16" />Refresh
        </button>
      </div>
    </div>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success" role="status">{{ successMessage }}</p>

    <div v-if="pending" class="loading-surface surface" aria-live="polite">
      <LoaderCircle class="spin" :size="18" />Loading article workspace
    </div>

    <div v-else class="create-layout">
      <form v-if="canWriteArticles" class="create-form" @submit.prevent="createArticle">
        <section class="create-card surface">
          <div class="create-card__header">
            <span class="create-card__icon"><FileText :size="18" /></span>
            <div>
              <span>Article brief</span>
              <h2>Setup</h2>
              <small v-if="draftSavedAt">{{ draftSaving ? 'Saving locally…' : `Recovered locally · ${formatSavedDate(draftSavedAt)}` }}</small>
            </div>
            <span class="status-pill create-card__status" :class="categoryReady ? 'status-pill--success' : 'status-pill--warning'">
              {{ categoryReady ? 'Category ready' : 'Category required' }}
            </span>
          </div>

          <div class="create-card__body setup-fields">
            <label class="field setup-fields__title">
              <span>Title</span>
              <input v-model.trim="articleForm.title" placeholder="A clear, useful working title" required>
            </label>
            <label class="field">
              <span>Slug</span>
              <input v-model.trim="articleForm.slug" class="mono-input" placeholder="article-slug" required @input="slugTouched = true">
            </label>
            <label class="field setup-fields__category">
              <span>Primary category</span>
              <select v-model="articleForm.primaryCategoryId" :disabled="categories.length === 0" required>
                <option value="" disabled>Select category</option>
                <option v-for="category in categories" :key="category.id" :value="category.id">{{ categoryPathLabel(category) }}</option>
              </select>
              <small v-if="categories.length === 0">Create a category from the side panel before saving the draft.</small>
            </label>
          </div>
        </section>

        <section class="create-card surface">
          <div class="create-card__header">
            <span class="create-card__icon create-card__icon--blue"><Users :size="18" /></span>
            <div><span>Public attribution</span><h2>Authors and credits</h2></div>
          </div>
          <div class="create-card__body">
            <RevisionContributorsEditor v-model="articleForm.contributors" :authors="authors" />
            <small v-if="authors.length === 0" class="attribution-help">
              Create an active profile on the <NuxtLink :to="`/projects/${projectID}/authors`">Authors page</NuxtLink> before saving the draft.
            </small>
          </div>
        </section>

        <section class="create-card surface">
          <div class="create-card__header">
            <span class="create-card__icon create-card__icon--blue"><Layers3 :size="18" /></span>
            <div><span>Content structure</span><h2>Editorial template</h2></div>
          </div>
          <div class="template-grid" role="listbox" aria-label="Article type">
            <button
              v-for="type in articleTypes"
              :key="type"
              class="template-option"
              :class="{ 'is-selected': articleForm.articleType === type }"
              type="button"
              role="option"
              :aria-selected="articleForm.articleType === type"
              @click="articleForm.articleType = type"
            >
              <span>{{ labelize(type) }}</span>
              <CheckCircle2 v-if="articleForm.articleType === type" :size="16" />
            </button>
          </div>
        </section>

        <section class="create-card surface">
          <div class="create-card__header">
            <span class="create-card__icon"><BookOpenCheck :size="18" /></span>
            <div><span>Opening revision</span><h2>Editorial content</h2></div>
          </div>

          <div class="create-card__body revision-fields">
            <div class="revision-summary-grid">
              <label class="field">
                <span>Deck</span>
                <textarea v-model.trim="articleForm.deck" class="textarea--compact" placeholder="Supporting line beneath the title" />
              </label>
              <label class="field">
                <span>Excerpt</span>
                <textarea v-model.trim="articleForm.excerpt" class="textarea--compact" placeholder="Short summary for listings and feeds" />
              </label>
              <label class="field">
                <span>Short answer</span>
                <textarea v-model.trim="articleForm.shortAnswer" class="textarea--compact" placeholder="Direct answer for quick readers" />
              </label>
            </div>

            <fieldset class="seo-fields">
              <legend>SEO and social preview</legend>
              <div class="seo-fields__grid">
                <label class="field">
                  <span>SEO title</span>
                  <input v-model.trim="articleForm.seoTitle" placeholder="Defaults to article title">
                </label>
                <label class="field">
                  <span>Robots</span>
                  <select v-model="articleForm.robots">
                    <option value="index,follow">Index, follow</option>
                    <option value="index,nofollow">Index, nofollow</option>
                    <option value="noindex,follow">Noindex, follow</option>
                    <option value="noindex,nofollow">Noindex, nofollow</option>
                  </select>
                </label>
                <label class="field seo-fields__wide">
                  <span>Meta description</span>
                  <textarea v-model.trim="articleForm.seoDescription" class="textarea--compact" placeholder="Defaults to excerpt" />
                </label>
                <label class="field">
                  <span>Open Graph title</span>
                  <input v-model.trim="articleForm.openGraphTitle" placeholder="Title used when shared">
                </label>
                <label class="field">
                  <span>Open Graph image URL</span>
                  <input v-model.trim="articleForm.openGraphImage" type="url" placeholder="https://…">
                </label>
                <label class="field seo-fields__wide">
                  <span>Open Graph description</span>
                  <textarea v-model.trim="articleForm.openGraphDescription" class="textarea--compact" placeholder="Description used when shared" />
                </label>
              </div>
            </fieldset>

            <ArticleStructuredEditor
              v-model:html="articleForm.html"
              v-model:body-document="createBodyDocument"
              label="Opening article body"
            />
          </div>
        </section>

        <div class="create-form__actions surface">
          <div>
            <strong>{{ canCreateArticle ? 'Ready to create' : 'Complete the required fields' }}</strong>
            <span>Title, slug, category, and a primary author are required.</span>
          </div>
          <NuxtLink class="button" :to="`/projects/${projectID}/articles`">Cancel</NuxtLink>
          <button class="button button--primary" type="submit" :disabled="creatingArticle || !canCreateArticle">
            <LoaderCircle v-if="creatingArticle" class="spin" :size="16" />
            <Save v-else :size="16" />Create draft
          </button>
        </div>
      </form>

      <section v-else class="empty-state create-read-only">
        <div>
          <span class="empty-state__icon"><FileText :size="20" /></span>
          <h3>Article creation is read-only</h3>
          <p>This project is inactive or your current role cannot create article drafts.</p>
          <NuxtLink class="button" :to="`/projects/${projectID}/articles`"><ArrowLeft :size="16" />Back to content</NuxtLink>
        </div>
      </section>

      <aside class="create-sidebar">
        <section class="side-panel surface">
          <div class="side-panel__header">
            <span class="side-panel__icon"><FileText :size="17" /></span>
            <div><span>Live summary</span><h2>Draft preview</h2></div>
          </div>
          <div class="draft-preview">
            <span class="status-pill status-pill--warning">Draft</span>
            <h3>{{ articleForm.title || 'Untitled draft' }}</h3>
            <p>{{ articleForm.excerpt || articleForm.deck || 'Your excerpt or deck will appear here as you write.' }}</p>
            <dl>
              <div><dt>Template</dt><dd>{{ labelize(articleForm.articleType) }}</dd></div>
              <div><dt>Words</dt><dd>{{ wordCount }}</dd></div>
              <div><dt>Category</dt><dd>{{ selectedCategory ? categoryPathLabel(selectedCategory) : 'Not set' }}</dd></div>
              <div><dt>Author</dt><dd>{{ selectedAuthor?.displayName || 'Not set' }}</dd></div>
              <div><dt>Credits</dt><dd>{{ articleForm.contributors.length }}</dd></div>
            </dl>
            <div class="draft-preview__path"><span>Canonical path</span><code>{{ canonicalPath }}</code></div>
          </div>
        </section>

        <form v-if="canManageTaxonomy" class="side-panel surface" @submit.prevent="createCategory">
          <div class="side-panel__header">
            <span class="side-panel__icon side-panel__icon--amber"><FolderTree :size="17" /></span>
            <div><span>Taxonomy</span><h2>Create category</h2></div>
          </div>
          <div class="side-panel__body">
            <label class="field"><span>Name</span><input v-model.trim="categoryForm.name" placeholder="Category name" required></label>
            <label class="field"><span>Slug</span><input v-model.trim="categoryForm.slug" class="mono-input" placeholder="category-slug" required @input="categorySlugTouched = true"></label>
            <label class="field">
              <span>Parent category</span>
              <select v-model="categoryForm.parentId">
                <option value="">No parent · root category</option>
                <option v-for="category in categoryParentOptions" :key="category.id" :value="category.id">{{ categoryPathLabel(category) }}</option>
              </select>
            </label>
            <label class="field"><span>Description</span><textarea v-model.trim="categoryForm.description" class="textarea--compact" /></label>
            <label class="checkbox-field"><input v-model="categoryForm.indexable" type="checkbox"><span>Indexable archive</span></label>
            <button class="button side-panel__button" type="submit" :disabled="creatingCategory || !canCreateCategory">
              <LoaderCircle v-if="creatingCategory" class="spin" :size="15" />
              <Plus v-else :size="15" />Create and select category
            </button>
            <NuxtLink class="side-panel__link" :to="`/projects/${projectID}/categories`">Manage the full category tree</NuxtLink>
          </div>
        </form>

        <section v-if="recentArticles.length" class="side-panel surface">
          <div class="side-panel__header">
            <span class="side-panel__icon side-panel__icon--blue"><BookOpenCheck :size="17" /></span>
            <div><span>Project activity</span><h2>Recent articles</h2></div>
          </div>
          <div class="recent-articles">
            <NuxtLink v-for="article in recentArticles.slice(0, 4)" :key="article.id" :to="`/projects/${projectID}/articles/${article.id}`">
              <span>{{ article.title }}</span><small>/{{ article.slug }}</small>
            </NuxtLink>
          </div>
        </section>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  ArrowLeft,
  BookOpenCheck,
  CheckCircle2,
  FileText,
  FolderTree,
  Layers3,
  LoaderCircle,
  Plus,
  RefreshCw,
  Save,
  Users
} from 'lucide-vue-next'
import type { AdminArticle, AdminAuthor, AdminProject, RevisionContributorInput, SEOInputPayload, TaxonomyTerm } from '~/composables/useAdminApi'
import {
  ARTICLE_TYPES,
  articleBodyDocumentFromHTML,
  hasValidRevisionContributors,
  htmlToPlainText,
  labelize,
  normalizeAPIError,
  slugify,
  useAdminApi
} from '~/composables/useAdminApi'

const api = useAdminApi()
const route = useRoute()
const projectID = computed(() => {
  const value = route.params.projectId
  return Array.isArray(value) ? String(value[0] || '') : String(value || '')
})

const project = ref<AdminProject | null>(null)
const categories = ref<TaxonomyTerm[]>([])
const authors = ref<AdminAuthor[]>([])
const recentArticles = ref<AdminArticle[]>([])
const pending = ref(true)
const creatingArticle = ref(false)
const creatingCategory = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const slugTouched = ref(false)
const categorySlugTouched = ref(false)
const draftSavedAt = ref('')
const draftSaving = ref(false)
const createBodyDocument = ref<unknown>({ type: 'doc', schemaVersion: 'tiptap-v1', content: [] })
let draftPersistenceEnabled = false
let draftDirty = false
let draftSaveTimer: ReturnType<typeof setTimeout> | undefined

const articleTypes = ARTICLE_TYPES
const articleForm = reactive({
  articleType: 'guide',
  title: '',
  slug: '',
  primaryCategoryId: '',
  contributors: [] as RevisionContributorInput[],
  deck: '',
  excerpt: '',
  shortAnswer: '',
  seoTitle: '',
  seoDescription: '',
  robots: 'index,follow',
  openGraphTitle: '',
  openGraphDescription: '',
  openGraphImage: '',
  html: ''
})

const categoryForm = reactive({
  name: '',
  slug: '',
  parentId: '',
  description: '',
  indexable: true
})

const categoryReady = computed(() => categories.value.length > 0 && Boolean(articleForm.primaryCategoryId))
const projectIsActive = computed(() => project.value?.status === 'active')
const canWriteArticles = computed(() => projectIsActive.value && ['project_owner', 'project_admin', 'editor', 'writer'].includes(project.value?.role || ''))
const canManageTaxonomy = computed(() => projectIsActive.value && ['project_owner', 'project_admin', 'editor'].includes(project.value?.role || ''))
const canCreateArticle = computed(() => Boolean(
  canWriteArticles.value
  && articleForm.title.trim()
  && articleForm.slug.trim()
  && articleForm.primaryCategoryId
  && hasValidRevisionContributors(articleForm.contributors)
))
const canCreateCategory = computed(() => canManageTaxonomy.value && Boolean(categoryForm.name.trim() && categoryForm.slug.trim()))
const htmlForSubmission = computed(() => hasMeaningfulStructuredHTML(articleForm.html)
  ? articleForm.html.trim()
  : `<p>${escapeHTML(articleForm.title || 'Untitled draft')}</p>`)
const bodyDocumentForSubmission = computed(() => hasMeaningfulStructuredHTML(articleForm.html) && isStructuredBodyDocument(createBodyDocument.value)
  ? createBodyDocument.value
  : articleBodyDocumentFromHTML(htmlForSubmission.value, articleForm.title))
const plainText = computed(() => htmlToPlainText(htmlForSubmission.value))
const wordCount = computed(() => plainText.value ? plainText.value.split(/\s+/).length : 0)
const selectedCategory = computed(() => categories.value.find(category => category.id === articleForm.primaryCategoryId) || null)
const selectedAuthor = computed(() => {
  const primaryAuthorID = articleForm.contributors.find(contributor => contributor.role === 'primary_author')?.authorId
  return authors.value.find(author => author.id === primaryAuthorID) || null
})
const categoryParentOptions = computed(() => categories.value.filter(category => (category.ancestors?.length || 0) < 2))
const canonicalPath = computed(() => {
  const basePath = project.value?.blogBasePath || '/blog'
  const normalizedBase = basePath.startsWith('/') ? basePath : `/${basePath}`
  return `${normalizedBase.replace(/\/$/, '')}/${articleForm.slug || 'slug'}`
})

watch(() => articleForm.title, (value) => {
  if (!slugTouched.value) {
    articleForm.slug = slugify(value)
  }
})

watch(() => categoryForm.name, (value) => {
  if (!categorySlugTouched.value) {
    categoryForm.slug = slugify(value)
  }
})

watch(
  () => ({ ...articleForm, bodyDocument: createBodyDocument.value, slugTouched: slugTouched.value }),
  () => {
    if (!draftPersistenceEnabled || !import.meta.client) return
    draftDirty = true
    draftSaving.value = true
    if (draftSaveTimer) clearTimeout(draftSaveTimer)
    draftSaveTimer = setTimeout(persistCreateDraft, 750)
  },
  { deep: true }
)

onMounted(async () => {
  await refresh()
  restoreCreateDraft()
})

onBeforeUnmount(() => {
  if (draftSaveTimer) clearTimeout(draftSaveTimer)
  if (draftDirty) persistCreateDraft()
})

async function refresh() {
  pending.value = true
  clearMessages()
  try {
    const [projectResponse, categoryResponse, authorResponse, articleResponse] = await Promise.all([
      api.getProject(projectID.value),
      api.listCategories(projectID.value),
      api.listAuthors(projectID.value),
      api.listArticles(projectID.value, 20)
    ])
    project.value = projectResponse.data
    categories.value = [...categoryResponse.data].sort((left, right) => categoryPathLabel(left).localeCompare(categoryPathLabel(right)))
    authors.value = authorResponse.data
      .filter(author => author.status === 'active')
      .sort((left, right) => left.displayName.localeCompare(right.displayName))
    recentArticles.value = articleResponse.data
    if (!articleForm.primaryCategoryId && categories.value[0]) {
      articleForm.primaryCategoryId = categories.value[0].id
    }
    if (articleForm.contributors.length === 0 && authors.value[0]) {
      articleForm.contributors = [{ authorId: authors.value[0].id, role: 'primary_author', position: 0 }]
    }
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load article creation data.')
  } finally {
    pending.value = false
  }
}

async function createCategory() {
  creatingCategory.value = true
  clearMessages()
  try {
    const response = await api.createCategory(projectID.value, {
      name: categoryForm.name,
      slug: categoryForm.slug,
      parentId: categoryForm.parentId,
      description: categoryForm.description,
      indexable: categoryForm.indexable
    })
    categories.value = [...categories.value, response.data].sort((left, right) => categoryPathLabel(left).localeCompare(categoryPathLabel(right)))
    articleForm.primaryCategoryId = response.data.id
    categoryForm.name = ''
    categoryForm.slug = ''
    categorySlugTouched.value = false
    categoryForm.parentId = ''
    categoryForm.description = ''
    categoryForm.indexable = true
    successMessage.value = 'Category created.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create category.')
  } finally {
    creatingCategory.value = false
  }
}

async function createArticle() {
  if (!canCreateArticle.value) return
  creatingArticle.value = true
  clearMessages()
  try {
    const response = await api.createArticle(projectID.value, {
      articleType: articleForm.articleType,
      title: articleForm.title,
      slug: articleForm.slug,
      primaryCategoryId: articleForm.primaryCategoryId,
      contributors: articleForm.contributors,
      deck: articleForm.deck,
      excerpt: articleForm.excerpt,
      shortAnswer: articleForm.shortAnswer,
      bodyDocument: bodyDocumentForSubmission.value,
      html: htmlForSubmission.value,
      seo: {
        title: articleForm.seoTitle,
        description: articleForm.seoDescription,
        robots: articleForm.robots as SEOInputPayload['robots'],
        openGraphTitle: articleForm.openGraphTitle,
        openGraphDescription: articleForm.openGraphDescription,
        openGraphImage: articleForm.openGraphImage
      }
    })
    removeCreateDraft()
    draftDirty = false
    successMessage.value = 'Draft created.'
    await navigateTo(`/projects/${projectID.value}/articles/${response.data.id}`)
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create article.')
  } finally {
    creatingArticle.value = false
  }
}

function clearMessages() {
  errorMessage.value = ''
  successMessage.value = ''
}

function categoryPathLabel(category: TaxonomyTerm) {
  return [...(category.ancestors || []).map(ancestor => ancestor.name), category.name].join(' / ')
}

function createDraftKey() {
  return `seoblog:article-create-draft:${projectID.value}`
}

function persistCreateDraft() {
  if (!import.meta.client || !draftPersistenceEnabled || !draftDirty) return
  if (draftSaveTimer) {
    clearTimeout(draftSaveTimer)
    draftSaveTimer = undefined
  }
  const savedAt = new Date().toISOString()
  try {
    localStorage.setItem(createDraftKey(), JSON.stringify({
      schemaVersion: 3,
      projectId: projectID.value,
      savedAt,
      slugTouched: slugTouched.value,
      fields: { ...articleForm, bodyDocument: createBodyDocument.value }
    }))
    draftDirty = false
    draftSaving.value = false
    draftSavedAt.value = savedAt
  } catch {
    draftSaving.value = false
    errorMessage.value = 'This browser could not save the article draft locally.'
  }
}

function restoreCreateDraft() {
  if (!import.meta.client) return
  try {
    const raw = localStorage.getItem(createDraftKey())
    if (raw) {
      const saved = JSON.parse(raw) as {
        schemaVersion?: unknown
        projectId?: unknown
        savedAt?: unknown
        slugTouched?: unknown
        fields?: Record<string, unknown>
      }
      const stringKeys = [
        'articleType', 'title', 'slug', 'primaryCategoryId', 'deck', 'excerpt', 'shortAnswer',
        'seoTitle', 'seoDescription', 'robots', 'openGraphTitle', 'openGraphDescription', 'openGraphImage', 'html'
      ]
      if (
        (saved.schemaVersion === 2 || saved.schemaVersion === 3)
        && saved.projectId === projectID.value
        && typeof saved.savedAt === 'string'
        && saved.fields
        && stringKeys.every(key => typeof saved.fields?.[key] === 'string')
        && isContributorDraftValue(saved.fields.contributors)
        && (saved.schemaVersion !== 3 || isStructuredBodyDocument(saved.fields.bodyDocument))
      ) {
        Object.assign(articleForm, Object.fromEntries(stringKeys.map(key => [key, saved.fields?.[key]])))
        articleForm.contributors = saved.fields.contributors.map(contributor => ({ ...contributor }))
        createBodyDocument.value = saved.schemaVersion === 3
          ? saved.fields.bodyDocument
          : articleBodyDocumentFromHTML(articleForm.html, articleForm.title)
        slugTouched.value = saved.slugTouched === true
        draftSavedAt.value = saved.savedAt
      } else if (
        saved.schemaVersion === 1
        && saved.projectId === projectID.value
        && typeof saved.savedAt === 'string'
        && saved.fields
        && stringKeys.every(key => typeof saved.fields?.[key] === 'string')
        && typeof saved.fields.primaryAuthorId === 'string'
      ) {
        Object.assign(articleForm, Object.fromEntries(stringKeys.map(key => [key, saved.fields?.[key]])))
        articleForm.contributors = saved.fields.primaryAuthorId
          ? [{ authorId: saved.fields.primaryAuthorId, role: 'primary_author', position: 0 }]
          : articleForm.contributors
        createBodyDocument.value = articleBodyDocumentFromHTML(articleForm.html, articleForm.title)
        slugTouched.value = saved.slugTouched === true
        draftSavedAt.value = saved.savedAt
      } else {
        removeCreateDraft()
      }
    }
  } catch {
    removeCreateDraft()
  }
  nextTick(() => {
    draftPersistenceEnabled = true
  })
}

function isContributorDraftValue(value: unknown): value is RevisionContributorInput[] {
  return Array.isArray(value) && value.every((contributor) => {
    if (!contributor || typeof contributor !== 'object') return false
    const candidate = contributor as Record<string, unknown>
    return typeof candidate.authorId === 'string'
      && typeof candidate.role === 'string'
      && typeof candidate.position === 'number'
  })
}

function isStructuredBodyDocument(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object'
    && (value as Record<string, unknown>).type === 'doc'
    && Array.isArray((value as Record<string, unknown>).content))
}

function hasMeaningfulStructuredHTML(value: string) {
  return Boolean(htmlToPlainText(value) || /<(?:img|hr|table)\b/i.test(value))
}

function removeCreateDraft() {
  if (import.meta.client) localStorage.removeItem(createDraftKey())
}

function formatSavedDate(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

function escapeHTML(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}
</script>

<style scoped>
.create-heading-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; }
.create-layout { display: grid; grid-template-columns: minmax(0, 1fr) 330px; align-items: start; gap: 16px; }
.create-form { display: grid; min-width: 0; gap: 16px; }
.create-card { overflow: hidden; }
.create-card__header,
.side-panel__header { display: flex; align-items: flex-start; gap: 11px; padding: 14px 16px; border-bottom: 1px solid var(--border); }
.create-card__icon,
.side-panel__icon { display: grid; width: 36px; height: 36px; flex: 0 0 36px; place-items: center; border-radius: 7px; background: var(--primary-soft); color: var(--primary); }
.create-card__icon--blue,
.side-panel__icon--blue { background: var(--blue-soft); color: var(--blue); }
.side-panel__icon--amber { background: var(--amber-soft); color: var(--amber); }
.create-card__header > div,
.side-panel__header > div { min-width: 0; }
.create-card__header div > span,
.side-panel__header div > span { color: var(--text-soft); font-size: 12px; font-weight: 700; text-transform: uppercase; }
.create-card__header h2,
.side-panel__header h2 { margin: 1px 0 0; font-size: 15px; }
.create-card__header small { display: block; margin-top: 3px; color: var(--text-faint); font-size: 12px; }
.create-card__status { align-self: center; margin-left: auto; white-space: nowrap; }
.create-card__body { display: grid; gap: 14px; padding: 16px; }
.setup-fields { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.setup-fields__title,
.setup-fields__category { grid-column: 1 / -1; }
.field small { color: var(--text-faint); font-size: 12px; line-height: 1.45; }
.mono-input { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.template-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 7px; padding: 14px 16px 16px; }
.template-option { display: flex; min-height: 42px; align-items: center; justify-content: space-between; gap: 8px; padding: 8px 10px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--text-soft); font-size: 13px; font-weight: 600; text-align: left; cursor: pointer; transition: border-color 140ms ease, background 140ms ease, color 140ms ease; }
.template-option:hover { border-color: var(--border-strong); background: var(--surface-subtle); color: var(--text); }
.template-option.is-selected { border-color: color-mix(in srgb, var(--primary) 55%, var(--border)); background: var(--primary-soft); color: var(--primary); }
.revision-fields { gap: 16px; }
.attribution-help { color: var(--text-faint); font-size: 12px; }
.attribution-help a { color: var(--primary); }
.revision-summary-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.textarea--compact { min-height: 82px !important; }
.seo-fields { min-width: 0; margin: 0; padding: 14px; border: 1px solid var(--border); border-radius: 7px; background: var(--surface-subtle); }
.seo-fields legend { padding: 0 7px; color: var(--text-soft); font-size: 12px; font-weight: 700; text-transform: uppercase; }
.seo-fields__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.seo-fields__wide { grid-column: 1 / -1; }
.create-form__actions { position: sticky; z-index: 20; bottom: 12px; display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 8px; padding: 11px 12px; background: color-mix(in srgb, var(--surface) 92%, transparent); backdrop-filter: blur(14px); box-shadow: var(--shadow-md); }
.create-form__actions > div { display: grid; min-width: 0; }
.create-form__actions strong { font-size: 13px; }
.create-form__actions span { overflow: hidden; color: var(--text-faint); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.create-read-only { min-height: 360px; }
.create-read-only .button { margin-top: 16px; }
.create-sidebar { position: sticky; top: 96px; display: grid; gap: 14px; }
.side-panel { overflow: hidden; }
.side-panel__header { padding: 13px 14px; }
.side-panel__icon { width: 34px; height: 34px; flex-basis: 34px; }
.side-panel__header h2 { font-size: 14px; }
.side-panel__body { display: grid; gap: 12px; padding: 14px; }
.side-panel__button { width: 100%; }
.draft-preview { padding: 15px; }
.draft-preview h3 { overflow: hidden; margin: 10px 0 0; font-size: 16px; text-overflow: ellipsis; white-space: nowrap; }
.draft-preview > p { display: -webkit-box; overflow: hidden; min-height: 34px; margin: 5px 0 0; color: var(--text-soft); font-size: 12px; line-height: 1.55; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.draft-preview dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1px; margin: 14px 0 0; overflow: hidden; border: 1px solid var(--border); border-radius: 6px; background: var(--border); }
.draft-preview dl div { min-width: 0; padding: 9px; background: var(--surface-subtle); }
.draft-preview dt { color: var(--text-faint); font-size: 12px; font-weight: 700; text-transform: uppercase; }
.draft-preview dd { overflow: hidden; margin: 2px 0 0; font-size: 12px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
.draft-preview__path { display: grid; gap: 3px; margin-top: 12px; }
.draft-preview__path span { color: var(--text-faint); font-size: 12px; font-weight: 700; text-transform: uppercase; }
.draft-preview__path code { overflow: hidden; padding: 7px 9px; border-radius: 5px; background: var(--surface-subtle); color: var(--text-soft); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.checkbox-field { display: inline-flex; align-items: center; gap: 7px; color: var(--text-soft); font-size: 12px; cursor: pointer; }
.checkbox-field input { width: 15px; height: 15px; min-height: 0; margin: 0; accent-color: var(--primary); }
.recent-articles { padding: 4px 14px 9px; }
.recent-articles a { display: grid; min-width: 0; padding: 10px 0; border-bottom: 1px solid var(--border); color: var(--text); text-decoration: none; }
.recent-articles a:last-child { border-bottom: 0; }
.recent-articles a:hover span { color: var(--primary); }
.recent-articles span,
.recent-articles small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.recent-articles span { font-size: 12px; font-weight: 650; }
.recent-articles small { margin-top: 2px; color: var(--text-faint); font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 12px; }
.side-panel__link { color: var(--primary); font-size: 12px; text-align: center; text-decoration: none; }
.side-panel__link:hover { text-decoration: underline; }
.loading-surface { display: flex; min-height: 180px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1180px) {
  .create-layout { grid-template-columns: minmax(0, 1fr) 300px; }
  .template-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .revision-summary-grid { grid-template-columns: 1fr; }
}
@media (max-width: 980px) {
  .create-layout { grid-template-columns: 1fr; }
  .create-sidebar { position: static; grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .create-sidebar > :last-child:nth-child(odd) { grid-column: 1 / -1; }
}
@media (max-width: 680px) {
  .create-heading-actions,
  .create-heading-actions .button { width: 100%; }
  .setup-fields,
  .seo-fields__grid { grid-template-columns: 1fr; }
  .setup-fields__title,
  .setup-fields__category,
  .seo-fields__wide { grid-column: auto; }
  .create-card__status { width: 100%; margin: 8px 0 0 47px; }
  .create-card__header { flex-wrap: wrap; }
  .create-form__actions { grid-template-columns: 1fr 1fr; }
  .create-form__actions > div { grid-column: 1 / -1; }
  .create-sidebar { grid-template-columns: 1fr; }
  .create-sidebar > :last-child:nth-child(odd) { grid-column: auto; }
}
@media (max-width: 480px) {
  .template-grid { grid-template-columns: 1fr; }
  .create-form__actions .button { width: 100%; }
  .draft-preview dl { grid-template-columns: 1fr; }
}
</style>

<template>
  <section class="min-h-screen">
    <header class="border-b border-[#d7ded8] bg-white px-6 py-4 dark:border-[#343a38] dark:bg-[#202422]">
      <div class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4">
        <div class="flex min-w-0 items-center gap-3">
          <NuxtLink
            class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-[#c9d4cc] bg-white text-[#28342d] hover:bg-[#eef5f1] dark:border-[#414a45] dark:bg-[#252b28] dark:text-[#eef4ef]"
            :to="`/projects/${projectID}/articles`"
            title="Back to articles"
            aria-label="Back to articles"
          >
            <ArrowLeft class="h-4 w-4" />
          </NuxtLink>
          <div class="min-w-0">
            <p class="truncate text-sm text-[#5d6a61] dark:text-[#aeb8b0]">{{ project?.name || 'Project' }}</p>
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
      <ProjectNav :project-id="projectID" :project="project" active="articles" />

      <main class="space-y-5">
        <p v-if="errorMessage" class="rounded-md border border-[#edc6c2] bg-[#fff4f2] px-4 py-3 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
          {{ errorMessage }}
        </p>
        <p v-if="successMessage" class="rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-4 py-3 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]">
          {{ successMessage }}
        </p>

        <div v-if="pending" class="flex items-center gap-3 rounded-lg border border-[#cfd8d1] bg-white p-5 text-sm text-[#58625c] dark:border-[#3f4843] dark:bg-[#202522] dark:text-[#bec7c1]">
          <LoaderCircle class="h-4 w-4 animate-spin" />
          Loading create page
        </div>

        <div v-else class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
          <form class="space-y-5" @submit.prevent="createArticle">
            <section class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Brief</p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">Article setup</h2>
                </div>
                <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="categoryReady ? 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]' : 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'">
                  {{ categoryReady ? 'Category ready' : 'Category required' }}
                </span>
              </div>

              <div class="mt-5 grid gap-4 md:grid-cols-2">
                <label class="block space-y-2 md:col-span-2">
                  <span class="text-sm font-medium">Title</span>
                  <input v-model.trim="articleForm.title" class="h-11 w-full rounded-md border border-[#bfcac3] px-3 text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]" required />
                </label>

                <label class="block space-y-2">
                  <span class="text-sm font-medium">Slug</span>
                  <input
                    v-model.trim="articleForm.slug"
                    class="h-11 w-full rounded-md border border-[#bfcac3] px-3 font-mono text-sm text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]"
                    required
                    @input="slugTouched = true"
                  />
                </label>

                <label class="block space-y-2">
                  <span class="text-sm font-medium">Locale</span>
                  <input v-model.trim="articleForm.locale" class="h-11 w-full rounded-md border border-[#bfcac3] px-3 text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]" required />
                </label>

                <label class="block space-y-2 md:col-span-2">
                  <span class="text-sm font-medium">Primary category</span>
                  <select
                    v-model="articleForm.primaryCategoryId"
                    class="h-11 w-full rounded-md border border-[#bfcac3] bg-white px-3 text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 disabled:opacity-60 dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]"
                    :disabled="categories.length === 0"
                    required
                  >
                    <option value="" disabled>Select category</option>
                    <option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }}</option>
                  </select>
                </label>
              </div>
            </section>

            <section class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex items-start gap-3">
                <Layers3 class="mt-1 h-4 w-4 text-[#3162a3]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Type</p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">Editorial template</h2>
                </div>
              </div>

              <div class="mt-5 grid gap-2 sm:grid-cols-2 xl:grid-cols-3" role="listbox" aria-label="Article type">
                <button
                  v-for="type in articleTypes"
                  :key="type"
                  class="flex min-h-14 items-center justify-between gap-3 rounded-md border px-3 py-2 text-left text-sm transition"
                  :class="articleForm.articleType === type
                    ? 'border-[#165a4a] bg-[#e9f5ef] text-[#165a4a] dark:border-[#4b9479] dark:bg-[#142c24] dark:text-[#aee4d0]'
                    : 'border-[#cfd8d1] hover:bg-[#f2f5f3] dark:border-[#3f4843] dark:hover:bg-[#171b18]'"
                  type="button"
                  role="option"
                  :aria-selected="articleForm.articleType === type"
                  @click="articleForm.articleType = type"
                >
                  <span class="capitalize">{{ labelize(type) }}</span>
                  <CheckCircle2 v-if="articleForm.articleType === type" class="h-4 w-4 shrink-0" />
                </button>
              </div>
            </section>

            <section class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex items-start gap-3">
                <FileText class="mt-1 h-4 w-4 text-[#165a4a]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Draft</p>
                  <h2 class="mt-1 text-xl font-semibold tracking-normal">Opening revision</h2>
                </div>
              </div>

              <div class="mt-5 grid gap-4">
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Deck</span>
                  <textarea v-model.trim="articleForm.deck" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]" />
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Excerpt</span>
                  <textarea v-model.trim="articleForm.excerpt" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]" />
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">Short answer</span>
                  <textarea v-model.trim="articleForm.shortAnswer" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]" />
                </label>
                <label class="block space-y-2">
                  <span class="text-sm font-medium">HTML</span>
                  <textarea v-model.trim="articleForm.html" class="min-h-52 w-full rounded-md border border-[#bfcac3] px-3 py-2 font-mono text-sm text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]" />
                </label>
              </div>
            </section>

            <div class="flex flex-wrap items-center justify-end gap-3">
              <NuxtLink class="inline-flex h-10 items-center gap-2 rounded-md border border-[#c9d4cc] px-4 text-sm font-medium hover:bg-[#eef5f1] dark:border-[#414a45] dark:hover:bg-[#2a302d]" :to="`/projects/${projectID}/articles`">
                <ArrowLeft class="h-4 w-4" />
                Cancel
              </NuxtLink>
              <button
                class="inline-flex h-10 items-center gap-2 rounded-md bg-[#165a4a] px-4 text-sm font-medium text-white hover:bg-[#10463a] disabled:opacity-60"
                type="submit"
                :disabled="creatingArticle || !canCreateArticle"
              >
                <LoaderCircle v-if="creatingArticle" class="h-4 w-4 animate-spin" />
                <Save v-else class="h-4 w-4" />
                Create draft
              </button>
            </div>
          </form>

          <aside class="space-y-5">
            <section class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Preview</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">{{ articleForm.title || 'Untitled draft' }}</h2>
                </div>
                <FileText class="mt-1 h-4 w-4 text-[#3162a3]" />
              </div>
              <dl class="mt-5 grid gap-3 text-sm">
                <div>
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Type</dt>
                  <dd class="mt-1 capitalize">{{ labelize(articleForm.articleType) }}</dd>
                </div>
                <div>
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Slug</dt>
                  <dd class="mt-1 break-all font-mono text-xs">{{ articleForm.slug || 'not-set' }}</dd>
                </div>
                <div>
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Words</dt>
                  <dd class="mt-1">{{ wordCount }}</dd>
                </div>
                <div>
                  <dt class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">Canonical path</dt>
                  <dd class="mt-1 break-all font-mono text-xs">{{ canonicalPath }}</dd>
                </div>
              </dl>
            </section>

            <form class="space-y-4 rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]" @submit.prevent="createCategory">
              <div class="flex items-start gap-3">
                <FolderTree class="mt-1 h-4 w-4 text-[#8a5b00]" />
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Taxonomy</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Category</h2>
                </div>
              </div>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Name</span>
                <input v-model.trim="categoryForm.name" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Slug</span>
                <input v-model.trim="categoryForm.slug" class="h-10 w-full rounded-md border border-[#bfcac3] px-3 font-mono text-sm text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]" required />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">Description</span>
                <textarea v-model.trim="categoryForm.description" class="min-h-20 w-full rounded-md border border-[#bfcac3] px-3 py-2 text-[#20231f] outline-none transition focus:border-[#165a4a] focus:ring-2 focus:ring-[#165a4a]/15 dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]" />
              </label>
              <label class="flex items-center gap-2 text-sm">
                <input v-model="categoryForm.indexable" class="h-4 w-4 rounded border-[#bfcac3]" type="checkbox" />
                Indexable
              </label>
              <button
                class="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-4 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
                type="submit"
                :disabled="creatingCategory || !canCreateCategory"
              >
                <LoaderCircle v-if="creatingCategory" class="h-4 w-4 animate-spin" />
                <Plus v-else class="h-4 w-4" />
                Create category
              </button>
            </form>

            <section v-if="recentArticles.length" class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Recent</p>
                  <h2 class="mt-1 text-lg font-semibold tracking-normal">Articles</h2>
                </div>
                <BookOpenCheck class="h-4 w-4 text-[#165a4a]" />
              </div>
              <div class="mt-4 space-y-3">
                <NuxtLink
                  v-for="article in recentArticles.slice(0, 4)"
                  :key="article.id"
                  class="block rounded-md border border-[#d7ded8] px-3 py-2 hover:bg-[#f2f5f3] dark:border-[#3f4843] dark:hover:bg-[#171b18]"
                  :to="`/projects/${projectID}/articles/${article.id}`"
                >
                  <span class="block truncate text-sm font-medium">{{ article.title }}</span>
                  <span class="mt-1 block truncate font-mono text-xs text-[#667169] dark:text-[#aeb8b0]">{{ article.slug }}</span>
                </NuxtLink>
              </div>
            </section>
          </aside>
        </div>
      </main>
    </div>
  </section>
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
  LogOut,
  Plus,
  RefreshCw,
  Save
} from 'lucide-vue-next'
import type { AdminArticle, AdminProject, TaxonomyTerm } from '~/composables/useAdminApi'
import {
  ARTICLE_TYPES,
  articleBodyDocumentFromHTML,
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
const recentArticles = ref<AdminArticle[]>([])
const pending = ref(true)
const creatingArticle = ref(false)
const creatingCategory = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const slugTouched = ref(false)

const articleTypes = ARTICLE_TYPES
const articleForm = reactive({
  articleType: 'guide',
  title: '',
  slug: '',
  locale: '',
  primaryCategoryId: '',
  deck: '',
  excerpt: '',
  shortAnswer: '',
  html: ''
})

const categoryForm = reactive({
  name: '',
  slug: '',
  description: '',
  indexable: true
})

const categoryReady = computed(() => categories.value.length > 0 && Boolean(articleForm.primaryCategoryId))
const canCreateArticle = computed(() => Boolean(
  articleForm.title.trim()
  && articleForm.slug.trim()
  && articleForm.locale.trim()
  && articleForm.primaryCategoryId
))
const canCreateCategory = computed(() => Boolean(categoryForm.name.trim() && categoryForm.slug.trim()))
const htmlForSubmission = computed(() => articleForm.html.trim() || `<p>${escapeHTML(articleForm.title || 'Untitled draft')}</p>`)
const plainText = computed(() => htmlToPlainText(htmlForSubmission.value))
const wordCount = computed(() => plainText.value ? plainText.value.split(/\s+/).length : 0)
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
  if (!categoryForm.slug) {
    categoryForm.slug = slugify(value)
  }
})

onMounted(refresh)

async function refresh() {
  pending.value = true
  clearMessages()
  try {
    const [projectResponse, categoryResponse, articleResponse] = await Promise.all([
      api.getProject(projectID.value),
      api.listCategories(projectID.value),
      api.listArticles(projectID.value, 20)
    ])
    project.value = projectResponse.data
    categories.value = categoryResponse.data
    recentArticles.value = articleResponse.data
    if (!articleForm.locale) {
      articleForm.locale = project.value.defaultLocale || 'en'
    }
    if (!articleForm.primaryCategoryId && categories.value[0]) {
      articleForm.primaryCategoryId = categories.value[0].id
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
      description: categoryForm.description,
      indexable: categoryForm.indexable
    })
    categories.value = [...categories.value, response.data].sort((left, right) => left.name.localeCompare(right.name))
    articleForm.primaryCategoryId = response.data.id
    categoryForm.name = ''
    categoryForm.slug = ''
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
      locale: articleForm.locale,
      primaryCategoryId: articleForm.primaryCategoryId,
      deck: articleForm.deck,
      excerpt: articleForm.excerpt,
      shortAnswer: articleForm.shortAnswer,
      bodyDocument: articleBodyDocumentFromHTML(htmlForSubmission.value, articleForm.title),
      html: htmlForSubmission.value
    })
    successMessage.value = 'Draft created.'
    await navigateTo(`/projects/${projectID.value}/articles/${response.data.id}`)
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create article.')
  } finally {
    creatingArticle.value = false
  }
}

async function logout() {
  try {
    await api.logout()
  } finally {
    await navigateTo('/')
  }
}

function clearMessages() {
  errorMessage.value = ''
  successMessage.value = ''
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

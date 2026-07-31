<template>
  <section class="rounded-lg border border-[#cfd8d1] bg-white p-5 shadow-sm dark:border-[#3f4843] dark:bg-[#202522]">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-sm text-[#5d6a61] dark:text-[#aeb8b0]">Evidence and public trust</p>
        <h2 class="mt-1 text-lg font-semibold tracking-normal">Claims, notices, and preview</h2>
      </div>
      <button
        class="inline-flex h-9 items-center gap-2 rounded-md border border-[#c9d4cc] px-3 text-sm font-medium hover:bg-[#eef5f1] disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#2a302d]"
        type="button"
        :disabled="pending"
        @click="refresh"
      >
        <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': pending }" />
        Refresh
      </button>
    </div>

    <p v-if="errorMessage" class="mt-4 rounded-md border border-[#edc6c2] bg-[#fff4f2] px-3 py-2 text-sm text-[#9b2d23] dark:border-[#6d352f] dark:bg-[#2a1c1a] dark:text-[#ffc4bd]" role="alert">
      {{ errorMessage }}
    </p>
    <p v-if="successMessage" class="mt-4 rounded-md border border-[#b9dcc9] bg-[#edf9f1] px-3 py-2 text-sm text-[#165a4a] dark:border-[#2d644a] dark:bg-[#13261e] dark:text-[#aee4d0]">
      {{ successMessage }}
    </p>

    <div v-if="pending" class="mt-5 flex items-center gap-2 text-sm text-[#667169] dark:text-[#aeb8b0]">
      <LoaderCircle class="h-4 w-4 animate-spin" />
      Loading evidence
    </div>

    <div v-else class="mt-5 space-y-6">
      <div class="grid gap-5 lg:grid-cols-2">
        <div class="space-y-4">
          <div class="flex items-center justify-between gap-3">
            <h3 class="font-medium">Source library</h3>
            <span class="text-xs text-[#667169] dark:text-[#aeb8b0]">{{ sources.length }} project sources</span>
          </div>
          <form v-if="canWrite" class="grid gap-3 rounded-md bg-[#f4f7f5] p-4 dark:bg-[#171b18]" @submit.prevent="createSource">
            <input v-model.trim="sourceForm.title" class="rounded-md border border-[#bfcac3] px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#202522]" placeholder="Source title" required />
            <div class="grid gap-3 sm:grid-cols-2">
              <select v-model="sourceForm.sourceType" class="h-10 rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#202522]">
                <option v-for="type in sourceTypes" :key="type" :value="type">{{ labelize(type) }}</option>
              </select>
              <input v-model.trim="sourceForm.publisher" class="rounded-md border border-[#bfcac3] px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#202522]" placeholder="Publisher" />
            </div>
            <input v-model.trim="sourceForm.url" class="rounded-md border border-[#bfcac3] px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#202522]" type="url" placeholder="https://example.com/source" />
            <label class="flex items-center gap-2 text-sm"><input v-model="sourceForm.isPrimary" type="checkbox" /> Primary or first-party evidence</label>
            <button class="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-3 text-sm font-medium hover:bg-white disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#252b28]" type="submit" :disabled="creatingSource">
              <LoaderCircle v-if="creatingSource" class="h-4 w-4 animate-spin" />
              <Plus v-else class="h-4 w-4" />
              Add source
            </button>
          </form>
          <div v-if="sources.length" class="max-h-64 space-y-2 overflow-y-auto pr-1">
            <article v-for="source in sources" :key="source.id" class="rounded-md border border-[#d7ded8] p-3 text-sm dark:border-[#3f4843]">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <a v-if="source.url" class="font-medium text-[#245b99] underline decoration-[#9bb6d6] underline-offset-2 dark:text-[#b8d5ff]" :href="source.url" target="_blank" rel="noopener noreferrer">{{ source.title }}</a>
                  <p v-else class="font-medium">{{ source.title }}</p>
                  <p class="mt-1 text-xs text-[#667169] dark:text-[#aeb8b0]">{{ source.publisher || labelize(source.sourceType) }}</p>
                </div>
                <span v-if="source.isPrimary" class="rounded-full bg-[#e0f3e9] px-2 py-1 text-xs text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]">Primary</span>
              </div>
            </article>
          </div>
        </div>

        <div class="space-y-4">
          <div class="flex items-center justify-between gap-3">
            <h3 class="font-medium">Revision claims</h3>
            <span class="text-xs text-[#667169] dark:text-[#aeb8b0]">{{ claims.length }} on latest revision</span>
          </div>
          <form v-if="canWrite" class="grid gap-3 rounded-md bg-[#f4f7f5] p-4 dark:bg-[#171b18]" @submit.prevent="createClaim">
            <textarea v-model.trim="claimForm.claimText" class="min-h-20 rounded-md border border-[#bfcac3] px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#202522]" placeholder="Factual claim to verify" required />
            <div class="grid gap-3 sm:grid-cols-2">
              <select v-model="claimForm.importance" class="h-10 rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#202522]">
                <option v-for="importance in claimImportances" :key="importance" :value="importance">{{ labelize(importance) }}</option>
              </select>
              <input v-model.trim="claimForm.blockId" class="rounded-md border border-[#bfcac3] px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#202522]" placeholder="Block ID (optional)" />
            </div>
            <label class="space-y-1 text-xs text-[#667169] dark:text-[#aeb8b0]">
              <span>Supporting sources</span>
              <select v-model="claimForm.sourceIds" class="min-h-24 w-full rounded-md border border-[#bfcac3] px-3 py-2 text-sm text-[#20231f] dark:border-[#4b5650] dark:bg-[#202522] dark:text-[#f2f3ef]" multiple>
                <option v-for="source in sources" :key="source.id" :value="source.id">{{ source.title }}</option>
              </select>
            </label>
            <button class="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-[#c9d4cc] px-3 text-sm font-medium hover:bg-white disabled:opacity-60 dark:border-[#414a45] dark:hover:bg-[#252b28]" type="submit" :disabled="creatingClaim">
              <LoaderCircle v-if="creatingClaim" class="h-4 w-4 animate-spin" />
              <Plus v-else class="h-4 w-4" />
              Add claim
            </button>
          </form>
          <div v-if="claims.length" class="space-y-3">
            <article v-for="claim in claims" :key="claim.id" class="rounded-md border border-[#d7ded8] p-3 text-sm dark:border-[#3f4843]">
              <div class="flex flex-wrap items-start justify-between gap-2">
                <p class="min-w-0 flex-1">{{ claim.claimText }}</p>
                <span class="rounded-full px-2 py-1 text-xs" :class="claimStateClass(claim.verificationState)">{{ labelize(claim.verificationState) }}</span>
              </div>
              <p class="mt-2 text-xs text-[#667169] dark:text-[#aeb8b0]">{{ labelize(claim.importance) }} · {{ claim.sourceIds.length }} source{{ claim.sourceIds.length === 1 ? '' : 's' }}</p>
              <div v-if="canReview" class="mt-3 flex gap-2">
                <select v-model="claimStates[claim.id]" class="h-9 min-w-0 flex-1 rounded-md border border-[#bfcac3] px-2 text-xs dark:border-[#4b5650] dark:bg-[#171b18]">
                  <option v-for="state in claimVerificationStates" :key="state" :value="state">{{ labelize(state) }}</option>
                </select>
                <button class="rounded-md border border-[#c9d4cc] px-3 text-xs font-medium disabled:opacity-60 dark:border-[#414a45]" type="button" :disabled="claimPending === claim.id" @click="verifyClaim(claim)">Apply</button>
              </div>
            </article>
          </div>
        </div>
      </div>

      <div class="grid gap-5 border-t border-[#d7ded8] pt-6 dark:border-[#3f4843] lg:grid-cols-2">
        <div class="space-y-4">
          <h3 class="font-medium">Public disclosures</h3>
          <form v-if="canReview" class="grid gap-3" @submit.prevent="createDisclosure">
            <select v-model="disclosureForm.disclosureType" class="h-10 rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]">
              <option v-for="type in disclosureTypes" :key="type" :value="type">{{ labelize(type) }}</option>
            </select>
            <textarea v-model.trim="disclosureForm.publicText" class="min-h-20 rounded-md border border-[#bfcac3] px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" placeholder="Public disclosure text" required />
            <button class="h-10 rounded-md border border-[#c9d4cc] text-sm font-medium disabled:opacity-60 dark:border-[#414a45]" type="submit" :disabled="creatingDisclosure">Add disclosure</button>
          </form>
          <ul v-if="disclosures.length" class="space-y-2 text-sm">
            <li v-for="disclosure in disclosures" :key="disclosure.id" class="rounded-md bg-[#f4f7f5] p-3 dark:bg-[#171b18]">
              <p class="text-xs uppercase text-[#667169] dark:text-[#aeb8b0]">{{ labelize(disclosure.disclosureType) }}</p>
              <p class="mt-1">{{ disclosure.publicText }}</p>
            </li>
          </ul>
        </div>

        <div class="space-y-4">
          <h3 class="font-medium">Append-only corrections</h3>
          <form v-if="canReview" class="grid gap-3" @submit.prevent="createCorrection">
            <textarea v-model.trim="correctionForm.publicNote" class="min-h-20 rounded-md border border-[#bfcac3] px-3 py-2 text-sm dark:border-[#4b5650] dark:bg-[#171b18]" placeholder="What was corrected and why" required />
            <select v-model="correctionForm.supersedesNoticeId" class="h-10 rounded-md border border-[#bfcac3] px-3 text-sm dark:border-[#4b5650] dark:bg-[#171b18]">
              <option value="">Does not supersede a prior notice</option>
              <option v-for="notice in corrections" :key="notice.id" :value="notice.id">{{ notice.publicNote.slice(0, 80) }}</option>
            </select>
            <button class="h-10 rounded-md border border-[#c9d4cc] text-sm font-medium disabled:opacity-60 dark:border-[#414a45]" type="submit" :disabled="creatingCorrection">Add correction</button>
          </form>
          <ol v-if="corrections.length" class="space-y-2 text-sm">
            <li v-for="notice in corrections" :key="notice.id" class="rounded-md bg-[#f4f7f5] p-3 dark:bg-[#171b18]">
              <p>{{ notice.publicNote }}</p>
              <p class="mt-1 text-xs text-[#667169] dark:text-[#aeb8b0]">{{ formatDate(notice.correctedAt) }}<span v-if="notice.supersedesNoticeId"> · supersedes a prior notice</span></p>
            </li>
          </ol>
        </div>
      </div>

      <div class="border-t border-[#d7ded8] pt-6 dark:border-[#3f4843]">
        <div class="flex flex-wrap items-end justify-between gap-4">
          <div>
            <h3 class="font-medium">Non-indexable revision preview</h3>
            <p class="mt-1 text-sm text-[#667169] dark:text-[#aeb8b0]">Creates a single-purpose bearer credential and loads the exact latest revision from the preview API.</p>
          </div>
          <form v-if="canWrite" class="flex items-end gap-2" @submit.prevent="createPreview">
            <label class="space-y-1 text-xs text-[#667169] dark:text-[#aeb8b0]">
              <span>Lifetime</span>
              <select v-model.number="previewTTL" class="h-10 rounded-md border border-[#bfcac3] px-3 text-sm text-[#20231f] dark:border-[#4b5650] dark:bg-[#171b18] dark:text-[#f2f3ef]">
                <option :value="15">15 minutes</option>
                <option :value="30">30 minutes</option>
                <option :value="60">60 minutes</option>
              </select>
            </label>
            <button class="inline-flex h-10 items-center gap-2 rounded-md bg-[#165a4a] px-4 text-sm font-medium text-white disabled:opacity-60" type="submit" :disabled="creatingPreview">
              <LoaderCircle v-if="creatingPreview" class="h-4 w-4 animate-spin" />
              <Eye v-else class="h-4 w-4" />
              Generate preview
            </button>
          </form>
        </div>
        <div v-if="preview" class="mt-4 space-y-3 rounded-md border border-[#b9d5c8] bg-[#eef8f3] p-4 dark:border-[#315648] dark:bg-[#14251f]">
          <div class="flex flex-wrap items-center justify-between gap-2 text-sm">
            <span>Expires {{ formatDate(preview.token.expiresAt) }}</span>
            <button class="text-xs font-medium underline" type="button" @click="revokePreview">Revoke preview</button>
          </div>
          <p class="break-all font-mono text-xs"><span class="font-sans font-medium">One-time secret:</span> {{ preview.secret }}</p>
          <pre class="max-h-80 overflow-auto whitespace-pre-wrap rounded-md bg-white p-3 text-xs text-[#20231f] dark:bg-[#0e1210] dark:text-[#eef4ef]">{{ prettyJSON(previewData) }}</pre>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { Eye, LoaderCircle, Plus, RefreshCw } from 'lucide-vue-next'
import {
  apiListData,
  normalizeAPIError,
  type AdminClaim,
  type AdminCorrection,
  type AdminDisclosure,
  type AdminSource,
  type PreviewToken
} from '~/composables/useAdminApi'

const props = defineProps<{
  projectId: string
  articleId: string
  revisionId: string
  role: string
}>()

const api = useAdminApi()
const sources = ref<AdminSource[]>([])
const claims = ref<AdminClaim[]>([])
const disclosures = ref<AdminDisclosure[]>([])
const corrections = ref<AdminCorrection[]>([])
const claimStates = reactive<Record<string, string>>({})
const pending = ref(true)
const creatingSource = ref(false)
const creatingClaim = ref(false)
const creatingDisclosure = ref(false)
const creatingCorrection = ref(false)
const creatingPreview = ref(false)
const claimPending = ref('')
const errorMessage = ref('')
const successMessage = ref('')
const previewTTL = ref(30)
const preview = ref<{ token: PreviewToken, secret: string } | null>(null)
const previewData = ref<unknown>(null)
const canWrite = computed(() => ['project_owner', 'project_admin', 'editor', 'writer'].includes(props.role))
const canReview = computed(() => ['project_owner', 'project_admin', 'editor', 'reviewer'].includes(props.role))

const sourceTypes = ['web', 'book', 'report', 'dataset', 'interview', 'first_party', 'primary', 'other']
const claimImportances = ['low', 'normal', 'material', 'critical']
const claimVerificationStates = ['unverified', 'supported', 'partially_supported', 'unsupported', 'outdated', 'subject_expert_required', 'not_applicable']
const disclosureTypes = ['sponsorship', 'affiliate', 'ai_assistance', 'methodology', 'limitations', 'other']

const sourceForm = reactive({ title: '', publisher: '', url: '', sourceType: 'web', isPrimary: false })
const claimForm = reactive({ claimText: '', blockId: '', importance: 'normal', sourceIds: [] as string[] })
const disclosureForm = reactive({ disclosureType: 'methodology', publicText: '' })
const correctionForm = reactive({ publicNote: '', supersedesNoticeId: '' })

watch(() => props.revisionId, (next, previous) => {
  if (next && next !== previous) refresh()
})

onMounted(refresh)

async function refresh() {
  if (!props.revisionId) return
  pending.value = true
  errorMessage.value = ''
  try {
    const [sourceResponse, claimResponse, disclosureResponse, correctionResponse] = await Promise.all([
      api.listSources(props.projectId),
      api.listClaims(props.projectId, props.revisionId),
      api.listDisclosures(props.projectId, props.articleId),
      api.listCorrections(props.projectId, props.articleId)
    ])
    sources.value = apiListData(sourceResponse)
    claims.value = apiListData(claimResponse)
    disclosures.value = apiListData(disclosureResponse)
    corrections.value = apiListData(correctionResponse)
    for (const claim of claims.value) claimStates[claim.id] = claim.verificationState
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load evidence and trust data.')
  } finally {
    pending.value = false
  }
}

async function createSource() {
  creatingSource.value = true
  clearMessages()
  try {
    const response = await api.createSource(props.projectId, sourceForm)
    sources.value = [...sources.value, response.data]
    Object.assign(sourceForm, { title: '', publisher: '', url: '', sourceType: 'web', isPrimary: false })
    successMessage.value = 'Source added to the project library.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create source.')
  } finally {
    creatingSource.value = false
  }
}

async function createClaim() {
  creatingClaim.value = true
  clearMessages()
  try {
    const response = await api.createClaim(props.projectId, props.revisionId, claimForm)
    claims.value = [...claims.value, response.data]
    claimStates[response.data.id] = response.data.verificationState
    Object.assign(claimForm, { claimText: '', blockId: '', importance: 'normal', sourceIds: [] })
    successMessage.value = 'Claim added to the revision.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create claim.')
  } finally {
    creatingClaim.value = false
  }
}

async function verifyClaim(claim: AdminClaim) {
  claimPending.value = claim.id
  clearMessages()
  try {
    const response = await api.verifyClaim(props.projectId, claim.id, claimStates[claim.id] || claim.verificationState, claim.sourceIds)
    claims.value = claims.value.map(item => item.id === response.data.id ? response.data : item)
    successMessage.value = 'Claim verification updated.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not update claim verification.')
  } finally {
    claimPending.value = ''
  }
}

async function createDisclosure() {
  creatingDisclosure.value = true
  clearMessages()
  try {
    const response = await api.createDisclosure(props.projectId, props.articleId, {
      revisionId: props.revisionId,
      ...disclosureForm
    })
    disclosures.value = [...disclosures.value, response.data]
    disclosureForm.publicText = ''
    successMessage.value = 'Public disclosure added.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create disclosure.')
  } finally {
    creatingDisclosure.value = false
  }
}

async function createCorrection() {
  creatingCorrection.value = true
  clearMessages()
  try {
    const response = await api.createCorrection(props.projectId, props.articleId, {
      affectedRevisionId: props.revisionId,
      ...correctionForm
    })
    corrections.value = [...corrections.value, response.data]
    Object.assign(correctionForm, { publicNote: '', supersedesNoticeId: '' })
    successMessage.value = 'Append-only correction notice added.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create correction.')
  } finally {
    creatingCorrection.value = false
  }
}

async function createPreview() {
  creatingPreview.value = true
  clearMessages()
  try {
    const response = await api.createPreviewToken(props.projectId, props.articleId, props.revisionId, previewTTL.value)
    preview.value = response.data
    previewData.value = await api.request<unknown>(`/content/v1/preview/revisions/${props.revisionId}`, {
      headers: { Authorization: `Bearer ${response.data.secret}` }
    })
    successMessage.value = 'Short-lived preview generated. Copy the secret now; it is not stored in plaintext.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create preview.')
  } finally {
    creatingPreview.value = false
  }
}

async function revokePreview() {
  if (!preview.value) return
  clearMessages()
  try {
    await api.revokePreviewToken(props.projectId, preview.value.token.id)
    preview.value = null
    previewData.value = null
    successMessage.value = 'Preview token revoked.'
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not revoke preview.')
  }
}

function claimStateClass(state: string) {
  if (state === 'supported' || state === 'not_applicable') return 'bg-[#e0f3e9] text-[#165a4a] dark:bg-[#12382f] dark:text-[#aee4d0]'
  if (state === 'unverified' || state === 'partially_supported') return 'bg-[#fff0ce] text-[#7a4f00] dark:bg-[#3a2d12] dark:text-[#ffd98a]'
  return 'bg-[#ffe7e3] text-[#9b2d23] dark:bg-[#3a201d] dark:text-[#ffc4bd]'
}

function labelize(value: string) {
  return value.replaceAll('_', ' ')
}

function formatDate(value?: string) {
  if (!value) return 'Not set'
  const date = new Date(value.includes('T') ? value : `${value.replace(' ', 'T')}Z`)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

function prettyJSON(value: unknown) {
  return JSON.stringify(value, null, 2)
}

function clearMessages() {
  errorMessage.value = ''
  successMessage.value = ''
}
</script>

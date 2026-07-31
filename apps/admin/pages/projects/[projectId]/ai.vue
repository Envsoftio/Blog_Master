<template>
  <div class="page-stack">
    <div class="page-heading">
      <div>
        <p>Human-led briefs, evidence packets, generation jobs, and quality checks.</p>
      </div>
      <span class="status-pill" :class="serviceAvailable ? 'status-pill--success' : 'status-pill--warning'">
        {{ serviceAvailable ? 'Available' : 'Not configured' }}
      </span>
    </div>

    <div class="ai-tabs surface surface--subtle" role="tablist" aria-label="AI workspace">
      <button
        v-for="tab in tabs"
        :key="tab.value"
        type="button"
        :class="{ 'is-active': activeTab === tab.value }"
        @click="activeTab = tab.value"
      >
        <component :is="tab.icon" :size="16" />
        {{ tab.label }}
      </button>
    </div>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success">{{ successMessage }}</p>

    <div v-if="activeTab === 'brief'" class="ai-layout">
      <form class="surface ai-form" @submit.prevent="createJob">
        <div class="panel-heading">
          <span class="panel-heading__icon"><Sparkles :size="18" /></span>
          <div>
            <p>New generation</p>
            <h3>Editorial brief</h3>
          </div>
        </div>

        <div class="form-grid">
          <label class="field field--wide">
            <span>Working title</span>
            <input v-model.trim="brief.title" required placeholder="A clear, useful article title">
          </label>
          <label class="field">
            <span>Article type</span>
            <select v-model="brief.articleType">
              <option v-for="type in ARTICLE_TYPES" :key="type" :value="type">{{ labelize(type) }}</option>
            </select>
          </label>
          <label class="field">
            <span>Job</span>
            <select v-model="brief.jobType">
              <option value="outline">Evidence-aware outline</option>
              <option value="draft">Section draft</option>
              <option value="quality_check">Quality review</option>
            </select>
          </label>
          <label class="field field--wide">
            <span>Article</span>
            <select v-model="brief.contentId" required>
              <option value="" disabled>Select an article</option>
              <option v-for="article in articles" :key="article.id" :value="article.id">{{ article.title }}</option>
            </select>
          </label>
          <label class="field field--wide">
            <span>Approved evidence</span>
            <select v-model="brief.evidencePacketId" required :disabled="!brief.contentId">
              <option value="" disabled>Select an evidence version</option>
              <option v-for="packet in approvedBriefEvidence" :key="packet.id" :value="packet.id">
                Version {{ packet.version }} - {{ packet.packet.thesis }}
              </option>
            </select>
          </label>
          <label class="field field--wide">
            <span>Purpose and outcome</span>
            <textarea v-model.trim="brief.purpose" required placeholder="What should the reader understand or be able to do?"></textarea>
          </label>
          <label class="field">
            <span>Audience</span>
            <textarea v-model.trim="brief.audience" required placeholder="Who is this for?"></textarea>
          </label>
          <label class="field">
            <span>Unique angle</span>
            <textarea v-model.trim="brief.angle" required placeholder="What original value does this add?"></textarea>
          </label>
          <label class="field field--wide">
            <span>Evidence and constraints</span>
            <textarea v-model.trim="brief.evidence" placeholder="Sources, product facts, firsthand input, prohibited claims"></textarea>
          </label>
          <label class="field field--wide">
            <span>Call to action</span>
            <input v-model.trim="brief.cta" required placeholder="Relevant next step for the reader">
          </label>
        </div>

        <div class="form-footer">
          <span><ShieldCheck :size="15" />AI output remains a proposal until reviewed.</span>
          <button class="button button--primary" type="submit" :disabled="creatingJob || !canSubmit || !canWriteEvidence">
            <LoaderCircle v-if="creatingJob" class="spin" :size="16" />
            <WandSparkles v-else :size="16" />
            Create job
          </button>
        </div>
      </form>

      <aside class="ai-rail">
        <section class="surface rail-panel">
          <div class="rail-panel__heading">
            <div>
              <span>Brief quality</span>
              <h3>Input readiness</h3>
            </div>
            <span class="readiness-score">{{ readinessScore }}%</span>
          </div>
          <div class="readiness-track"><span :style="{ width: `${readinessScore}%` }" /></div>
          <ul class="readiness-list">
            <li v-for="check in readinessChecks" :key="check.label" :class="{ 'is-ready': check.ready }">
              <CheckCircle2 v-if="check.ready" :size="15" />
              <Circle v-else :size="15" />
              {{ check.label }}
            </li>
          </ul>
        </section>
        <section class="surface rail-panel">
          <div class="rail-panel__heading">
            <div>
              <span>Recent activity</span>
              <h3>Generation jobs</h3>
            </div>
            <span class="status-pill">{{ jobs.length }}</span>
          </div>
          <div v-if="jobs.length" class="job-mini-list">
            <button v-for="job in jobs.slice(0, 5)" :key="job.id" type="button" @click="activeTab = 'jobs'">
              <span class="job-status" :class="`job-status--${job.status}`" />
              <span><strong>{{ labelize(job.type) }}</strong><small>{{ relativeDate(job.updatedAt || job.createdAt) }}</small></span>
              <ChevronRight :size="14" />
            </button>
          </div>
          <p v-else class="rail-empty">{{ serviceAvailable ? 'No generation jobs yet' : 'AI service is not configured' }}</p>
        </section>
      </aside>
    </div>

    <div v-else-if="activeTab === 'evidence'" class="evidence-grid">
      <section class="surface evidence-panel">
        <div class="panel-heading">
          <span class="panel-heading__icon panel-heading__icon--blue"><Library :size="18" /></span>
          <div>
            <p>Research</p>
            <h3>Evidence packets</h3>
          </div>
          <span class="status-pill">{{ evidencePackets.length }}</span>
        </div>
        <div v-if="evidencePackets.length" class="evidence-list">
          <div v-for="packet in evidencePackets" :key="packet.id" class="evidence-row">
            <div>
              <strong>{{ articleTitle(packet.contentId) }}</strong>
              <span>Version {{ packet.version }} - {{ packetSourceCount(packet) }} sources</span>
              <small>{{ packet.packet.thesis }}</small>
            </div>
            <span class="status-pill" :class="packet.approvedAt ? 'status-pill--success' : 'status-pill--warning'">
              {{ evidenceStatus(packet) }}
            </span>
            <button
              v-if="!packet.approvedAt && packet.packet.publicationRecommendation === 'ready' && canApproveEvidence"
              class="button button--icon"
              type="button"
              title="Approve evidence packet"
              :disabled="approvingPacket === packet.id"
              @click="approvePacket(packet.id)"
            >
              <LoaderCircle v-if="approvingPacket === packet.id" class="spin" :size="15" />
              <Check v-else :size="15" />
            </button>
          </div>
        </div>
        <div v-else class="empty-state empty-state--embedded">
          <div>
            <span class="empty-state__icon"><Library :size="20" /></span>
            <h3>{{ serviceAvailable ? 'No evidence packets' : 'Evidence service is not available' }}</h3>
            <p>Evidence versions will appear here.</p>
          </div>
        </div>
      </section>
      <form class="surface evidence-panel" @submit.prevent="createPacket">
        <div class="panel-heading">
          <span class="panel-heading__icon panel-heading__icon--amber"><BookOpenCheck :size="18" /></span>
          <div>
            <p>New version</p>
            <h3>Evidence packet</h3>
          </div>
        </div>
        <div class="form-grid form-grid--single">
          <label class="field">
            <span>Article</span>
            <select v-model="evidenceForm.contentId" required>
              <option value="" disabled>Select an article</option>
              <option v-for="article in articles" :key="article.id" :value="article.id">{{ article.title }}</option>
            </select>
          </label>
          <label class="field">
            <span>Human brief</span>
            <textarea v-model.trim="evidenceForm.humanBrief" required></textarea>
          </label>
          <label class="field">
            <span>Search intent</span>
            <input v-model.trim="evidenceForm.searchIntent" required>
          </label>
          <label class="field">
            <span>Original thesis</span>
            <textarea v-model.trim="evidenceForm.thesis" required></textarea>
          </label>
          <fieldset class="source-picker">
            <legend>Sources</legend>
            <label v-for="source in sources" :key="source.id">
              <input v-model="evidenceForm.sourceIds" type="checkbox" :value="source.id">
              <span>{{ source.title }}</span>
            </label>
            <p v-if="!sources.length">No project sources</p>
          </fieldset>
          <label class="field">
            <span>Product facts</span>
            <textarea v-model="evidenceForm.productFacts" placeholder="One fact per line"></textarea>
          </label>
          <label class="field">
            <span>Subject notes</span>
            <textarea v-model="evidenceForm.subjectMatterNotes" placeholder="One note per line"></textarea>
          </label>
          <label class="field">
            <span>Firsthand observations</span>
            <textarea v-model="evidenceForm.firsthandObservations" placeholder="One observation per line"></textarea>
          </label>
          <label class="field">
            <span>Customer evidence</span>
            <textarea v-model="evidenceForm.customerEvidence" placeholder="One approved item per line"></textarea>
          </label>
          <label class="field">
            <span>Measurements</span>
            <textarea v-model="evidenceForm.measurements" placeholder="One measurement per line"></textarea>
          </label>
          <label class="field">
            <span>Allowed claims</span>
            <textarea v-model="evidenceForm.allowedClaims" placeholder="One claim per line"></textarea>
          </label>
          <label class="field">
            <span>Prohibited claims</span>
            <textarea v-model="evidenceForm.prohibitedClaims" placeholder="One claim per line"></textarea>
          </label>
          <label class="field">
            <span>Limitations</span>
            <textarea v-model="evidenceForm.limitations" placeholder="One limitation per line"></textarea>
          </label>
          <label class="field">
            <span>Required internal links</span>
            <textarea v-model="evidenceForm.requiredInternalLinks" placeholder="One path or content ID per line"></textarea>
          </label>
          <label class="field">
            <span>Call to action</span>
            <textarea v-model.trim="evidenceForm.callToAction" required></textarea>
          </label>
          <label class="field">
            <span>Publishing recommendation</span>
            <select v-model="evidenceForm.publicationRecommendation" required>
              <option value="" disabled>Select a recommendation</option>
              <option value="ready">Ready for evidence review</option>
              <option value="request_unique_evidence">Request unique evidence</option>
              <option value="do_not_publish">Do not publish</option>
            </select>
          </label>
        </div>
        <div class="form-footer">
          <span>Creates a new immutable evidence version.</span>
          <button class="button button--primary" type="submit" :disabled="savingEvidence || !canCreateEvidence || !canWriteEvidence">
            <LoaderCircle v-if="savingEvidence" class="spin" :size="16" />
            <Plus v-else :size="16" />
            Create packet
          </button>
        </div>
      </form>
    </div>

    <form v-else-if="activeTab === 'voice'" class="surface ai-form" @submit.prevent="saveVoiceProfile">
      <div class="panel-heading">
        <span class="panel-heading__icon"><Mic2 :size="18" /></span>
        <div>
          <p>{{ voiceProfile ? `Current version ${voiceProfile.version}` : 'Project context' }}</p>
          <h3>Voice profile</h3>
        </div>
      </div>
      <div class="form-grid">
        <label class="field">
          <span>Audience</span>
          <textarea v-model.trim="voiceForm.audience" required></textarea>
        </label>
        <label class="field">
          <span>Assumed knowledge</span>
          <textarea v-model.trim="voiceForm.assumedKnowledge" required></textarea>
        </label>
        <label class="field">
          <span>Brand purpose</span>
          <textarea v-model.trim="voiceForm.brandPurpose" required></textarea>
        </label>
        <label class="field">
          <span>Point of view</span>
          <textarea v-model.trim="voiceForm.pointOfView" required></textarea>
        </label>
        <label class="field">
          <span>Tone</span>
          <input v-model.trim="voiceForm.tone" required>
        </label>
        <label class="field">
          <span>Formality</span>
          <input v-model.trim="voiceForm.formality" required>
        </label>
        <label class="field">
          <span>Humour</span>
          <input v-model.trim="voiceForm.humor">
        </label>
        <label class="field">
          <span>Locale</span>
          <input v-model.trim="voiceForm.locale" required>
        </label>
        <label class="field">
          <span>Sentence preferences</span>
          <textarea v-model.trim="voiceForm.sentencePreferences" required></textarea>
        </label>
        <label class="field">
          <span>Paragraph preferences</span>
          <textarea v-model.trim="voiceForm.paragraphPreferences" required></textarea>
        </label>
        <label class="field">
          <span>Preferred vocabulary</span>
          <textarea v-model="voiceForm.preferredVocabulary" placeholder="One term per line"></textarea>
        </label>
        <label class="field">
          <span>Product terminology</span>
          <textarea v-model="voiceForm.productTerminology" placeholder="CMS: content platform"></textarea>
        </label>
        <label class="field">
          <span>Approved product facts</span>
          <textarea v-model="voiceForm.approvedProductFacts" placeholder="One fact per line"></textarea>
        </label>
        <label class="field">
          <span>Phrases to avoid</span>
          <textarea v-model="voiceForm.avoidPhrases" placeholder="One phrase per line"></textarea>
        </label>
        <label class="field">
          <span>Prohibited claims</span>
          <textarea v-model="voiceForm.prohibitedClaims" placeholder="One claim per line"></textarea>
        </label>
        <label class="field">
          <span>Content-type styles</span>
          <textarea v-model="voiceForm.contentTypeStyles" placeholder="guide: Use ordered decisions and practical examples"></textarea>
        </label>
        <label class="field">
          <span>Introduction rules</span>
          <textarea v-model.trim="voiceForm.introductionRules" required></textarea>
        </label>
        <label class="field">
          <span>Conclusion rules</span>
          <textarea v-model.trim="voiceForm.conclusionRules" required></textarea>
        </label>
        <label class="field">
          <span>Call-to-action rules</span>
          <textarea v-model.trim="voiceForm.callToActionRules" required></textarea>
        </label>
        <label class="field">
          <span>Regional spelling</span>
          <textarea v-model.trim="voiceForm.regionalSpelling" required></textarea>
        </label>
        <fieldset class="voice-examples field--wide">
          <legend>Approved writing examples</legend>
          <div v-for="(example, index) in voiceForm.writingExamples" :key="index" class="voice-example">
            <label class="field">
              <span>Example {{ index + 1 }} title</span>
              <input v-model.trim="example.title" required>
            </label>
            <label class="field">
              <span>Excerpt</span>
              <textarea v-model.trim="example.excerpt" required></textarea>
            </label>
            <button
              v-if="voiceForm.writingExamples.length > 3"
              class="button button--icon voice-example__remove"
              type="button"
              title="Remove writing example"
              :disabled="!canManageVoice"
              @click="voiceForm.writingExamples.splice(index, 1)"
            >
              <Trash2 :size="15" />
            </button>
          </div>
          <button
            class="button button--compact voice-examples__add"
            type="button"
            :disabled="!canManageVoice || voiceForm.writingExamples.length >= 20"
            @click="voiceForm.writingExamples.push({ title: '', excerpt: '' })"
          >
            <Plus :size="15" />
            Add example
          </button>
        </fieldset>
      </div>
      <div class="form-footer">
        <span>Saving creates a new immutable version.</span>
        <button class="button button--primary" type="submit" :disabled="savingVoice || !canSaveVoice || !canManageVoice">
          <LoaderCircle v-if="savingVoice" class="spin" :size="16" />
          <Save v-else :size="16" />
          Save new version
        </button>
      </div>
    </form>

    <section v-else-if="activeTab === 'jobs'" class="surface jobs-panel">
      <div class="jobs-header">
        <div>
          <span>Run history</span>
          <h3>AI jobs</h3>
        </div>
        <button class="button button--compact" type="button" :disabled="jobsPending" @click="loadWorkspace">
          <RefreshCw :class="{ spin: jobsPending }" :size="15" />
          Refresh
        </button>
      </div>
      <div v-if="jobs.length" class="jobs-table">
        <div class="jobs-row jobs-row--header"><span>Job</span><span>Status</span><span>Model</span><span>Updated</span><span>ID</span></div>
        <template v-for="job in jobs" :key="job.id">
          <div class="jobs-row">
            <span>
              <strong>{{ labelize(job.type) }}</strong>
              <small v-if="job.voiceProfileVersion && job.evidencePacketVersion">
                Voice v{{ job.voiceProfileVersion }} / Evidence v{{ job.evidencePacketVersion }}
              </small>
            </span>
            <span><i class="job-status" :class="`job-status--${job.status}`" />{{ labelize(job.status) }}</span>
            <span :title="modelForJob(job.id)">{{ modelForJob(job.id) }}</span>
            <span>{{ relativeDate(job.updatedAt || job.createdAt) }}</span>
            <span class="mono">{{ job.id }}</span>
          </div>
          <details v-if="job.result && job.status === 'succeeded'" class="job-result">
            <summary>Review generated proposal</summary>
            <pre>{{ prettyJSON(job.result) }}</pre>
          </details>
          <p v-else-if="job.error" class="job-error">{{ labelize(job.error) }}</p>
        </template>
      </div>
      <div v-else class="empty-state empty-state--embedded">
        <div><span class="empty-state__icon"><Bot :size="20" /></span><h3>No AI jobs</h3><p>Generation and quality jobs will appear here.</p></div>
      </div>
    </section>

    <section v-else class="quality-section">
      <div class="quality-heading">
        <div>
          <span>Latest by check</span>
          <h3>Quality results</h3>
        </div>
        <button class="button button--compact" type="button" :disabled="jobsPending" @click="loadWorkspace">
          <RefreshCw :class="{ spin: jobsPending }" :size="15" />
          Refresh
        </button>
      </div>
      <div v-if="latestQualityChecks.length" class="quality-grid">
        <article v-for="check in latestQualityChecks" :key="check.id" class="surface quality-card">
          <span class="quality-card__icon"><component :is="qualityIcon(check.checkType)" :size="18" /></span>
          <div>
            <h3>{{ labelize(check.checkType) }}</h3>
            <p>{{ check.message }}</p>
            <small>{{ labelize(check.severity) }} - {{ relativeDate(check.createdAt) }}</small>
          </div>
          <span class="status-pill" :class="qualityStatusClass(check.status)">{{ labelize(check.status) }}</span>
        </article>
      </div>
      <div v-else class="empty-state empty-state--embedded">
        <div><span class="empty-state__icon"><ListChecks :size="20" /></span><h3>No quality results</h3><p>Completed checks will appear here.</p></div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import {
  BookOpenCheck,
  Bot,
  Check,
  CheckCircle2,
  ChevronRight,
  Circle,
  FileSearch,
  Library,
  Link2,
  ListChecks,
  LoaderCircle,
  Mic2,
  Plus,
  RefreshCw,
  Save,
  ScanText,
  ShieldCheck,
  Sparkles,
  Trash2,
  WandSparkles
} from 'lucide-vue-next'
import {
  ARTICLE_TYPES,
  type AdminArticle,
  type AdminProject,
  type AdminSource,
  type AIJob,
  type AIRun,
  type EvidencePacket,
  type EvidencePacketDocument,
  type QualityCheckResult,
  type VoiceProfile,
  type VoiceProfileDocument
} from '~/composables/useAdminApi'

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => String(route.params.projectId || ''))
const activeTab = ref('brief')
const jobs = ref<AIJob[]>([])
const runs = ref<AIRun[]>([])
const qualityResults = ref<QualityCheckResult[]>([])
const project = ref<AdminProject | null>(null)
const articles = ref<AdminArticle[]>([])
const sources = ref<AdminSource[]>([])
const evidencePackets = ref<EvidencePacket[]>([])
const voiceProfile = ref<VoiceProfile | null>(null)
const jobsPending = ref(true)
const creatingJob = ref(false)
const savingVoice = ref(false)
const savingEvidence = ref(false)
const approvingPacket = ref('')
const serviceAvailable = ref(true)
const errorMessage = ref('')
const successMessage = ref('')
const tabs = [
  { value: 'brief', label: 'Brief', icon: Sparkles },
  { value: 'evidence', label: 'Evidence', icon: Library },
  { value: 'voice', label: 'Voice', icon: Mic2 },
  { value: 'jobs', label: 'Jobs', icon: Bot },
  { value: 'quality', label: 'Quality', icon: ListChecks }
]
const brief = reactive({
  title: '',
  articleType: 'standard',
  jobType: 'outline',
  contentId: '',
  evidencePacketId: '',
  purpose: '',
  audience: '',
  angle: '',
  evidence: '',
  cta: ''
})
const voiceForm = reactive({
  audience: '',
  assumedKnowledge: '',
  brandPurpose: '',
  pointOfView: '',
  tone: '',
  formality: '',
  humor: '',
  preferredVocabulary: '',
  productTerminology: '',
  approvedProductFacts: '',
  sentencePreferences: '',
  paragraphPreferences: '',
  avoidPhrases: '',
  prohibitedClaims: '',
  contentTypeStyles: '',
  introductionRules: '',
  conclusionRules: '',
  callToActionRules: '',
  regionalSpelling: '',
  locale: '',
  writingExamples: Array.from({ length: 3 }, () => ({ title: '', excerpt: '' }))
})
const evidenceForm = reactive({
  contentId: '',
  humanBrief: '',
  searchIntent: '',
  thesis: '',
  sourceIds: [] as string[],
  productFacts: '',
  subjectMatterNotes: '',
  firsthandObservations: '',
  customerEvidence: '',
  measurements: '',
  allowedClaims: '',
  prohibitedClaims: '',
  limitations: '',
  requiredInternalLinks: '',
  callToAction: '',
  publicationRecommendation: '' as EvidencePacketDocument['publicationRecommendation'] | ''
})
const approvedBriefEvidence = computed(() => evidencePackets.value.filter(packet =>
  packet.contentId === brief.contentId &&
  Boolean(packet.approvedAt) &&
  packet.packet.publicationRecommendation === 'ready'
))
const readinessChecks = computed(() => [
  { label: 'Article selected', ready: brief.contentId.length > 0 },
  { label: 'Clear purpose', ready: brief.purpose.trim().length >= 20 },
  { label: 'Defined audience', ready: brief.audience.trim().length >= 10 },
  { label: 'Unique angle', ready: brief.angle.trim().length >= 15 },
  { label: 'Approved evidence', ready: brief.evidencePacketId.length > 0 },
  { label: 'Voice profile', ready: Boolean(voiceProfile.value) },
  { label: 'Working title', ready: brief.title.trim().length >= 8 },
  { label: 'Call to action', ready: brief.cta.trim().length >= 5 }
])
const readinessScore = computed(() => Math.round(readinessChecks.value.filter(item => item.ready).length / readinessChecks.value.length * 100))
const canSubmit = computed(() => readinessChecks.value.every(item => item.ready))
const canManageVoice = computed(() => project.value?.role === 'project_owner' || project.value?.role === 'project_admin')
const canWriteEvidence = computed(() => ['project_owner', 'project_admin', 'editor', 'writer'].includes(project.value?.role || ''))
const canApproveEvidence = computed(() => ['project_owner', 'project_admin', 'editor', 'reviewer'].includes(project.value?.role || ''))
const hasUniqueEvidence = computed(() =>
  lineItems(evidenceForm.productFacts).length > 0 ||
  lineItems(evidenceForm.subjectMatterNotes).length > 0 ||
  lineItems(evidenceForm.firsthandObservations).length > 0 ||
  lineItems(evidenceForm.customerEvidence).length > 0 ||
  lineItems(evidenceForm.measurements).length > 0
)
const canSaveVoice = computed(() =>
  voiceForm.audience.length > 0 &&
  voiceForm.assumedKnowledge.length > 0 &&
  voiceForm.brandPurpose.length > 0 &&
  voiceForm.pointOfView.length > 0 &&
  voiceForm.tone.length > 0 &&
  voiceForm.formality.length > 0 &&
  voiceForm.sentencePreferences.length > 0 &&
  voiceForm.paragraphPreferences.length > 0 &&
  voiceForm.introductionRules.length > 0 &&
  voiceForm.conclusionRules.length > 0 &&
  voiceForm.callToActionRules.length > 0 &&
  voiceForm.regionalSpelling.length > 0 &&
  voiceForm.locale.length > 0 &&
  voiceForm.writingExamples.every(example => example.title.length > 0 && example.excerpt.length >= 40)
)
const canCreateEvidence = computed(() =>
  evidenceForm.contentId.length > 0 &&
  evidenceForm.humanBrief.length >= 20 &&
  evidenceForm.searchIntent.length >= 10 &&
  evidenceForm.thesis.length >= 20 &&
  evidenceForm.callToAction.length >= 5 &&
  evidenceForm.publicationRecommendation.length > 0 &&
  (evidenceForm.publicationRecommendation !== 'ready' || hasUniqueEvidence.value) &&
  (
    evidenceForm.sourceIds.length > 0 ||
    hasUniqueEvidence.value
  )
)
const runsByJob = computed(() => {
  const mapped = new Map<string, AIRun>()
  for (const run of runs.value) {
    if (run.jobId && !mapped.has(run.jobId)) mapped.set(run.jobId, run)
  }
  return mapped
})
const latestQualityChecks = computed(() => {
  const seen = new Set<string>()
  return qualityResults.value.filter((check) => {
    if (seen.has(check.checkType)) return false
    seen.add(check.checkType)
    return true
  })
})

onMounted(loadWorkspace)

watch(() => brief.contentId, (contentID) => {
  const article = articles.value.find(item => item.id === contentID)
  if (article) brief.articleType = article.articleType
  brief.evidencePacketId = approvedBriefEvidence.value[0]?.id || ''
})

async function loadWorkspace() {
  jobsPending.value = true
  errorMessage.value = ''
  try {
    const voiceRequest = api.getVoiceProfile(projectID.value).catch((error) => {
      if (apiStatus(error) === 404) return null
      throw error
    })
    const [
      projectResponse,
      articleResponse,
      sourceResponse,
      evidenceResponse,
      jobResponse,
      runResponse,
      qualityResponse,
      voiceResponse
    ] = await Promise.all([
      api.getProject(projectID.value),
      api.listArticles(projectID.value),
      api.listSources(projectID.value),
      api.listEvidencePackets(projectID.value),
      api.listAIJobs(projectID.value),
      api.listAIRuns(projectID.value),
      api.listQualityChecks(projectID.value),
      voiceRequest
    ])
    project.value = projectResponse.data
    articles.value = articleResponse.data
    sources.value = sourceResponse.data
    evidencePackets.value = evidenceResponse.data
    jobs.value = jobResponse.data
    runs.value = runResponse.data
    qualityResults.value = qualityResponse.data
    voiceProfile.value = voiceResponse?.data || null
    if (voiceProfile.value) {
      applyVoiceProfile(voiceProfile.value.profile)
    } else if (!voiceForm.locale) {
      voiceForm.locale = project.value.defaultLocale
    }
    serviceAvailable.value = true
  } catch (error) {
    if (apiStatus(error) === 501) {
      jobs.value = []
      runs.value = []
      qualityResults.value = []
      evidencePackets.value = []
      serviceAvailable.value = false
    } else {
      errorMessage.value = normalizeAPIError(error, 'Could not load the AI workspace.')
    }
  } finally {
    jobsPending.value = false
  }
}

async function createJob() {
  if (!canSubmit.value) return
  creatingJob.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const response = await api.createAIJob(projectID.value, {
      type: brief.jobType,
      contentId: brief.contentId,
      articleType: brief.articleType,
      evidencePacketId: brief.evidencePacketId,
      voiceProfileVersion: voiceProfile.value?.version || 0,
      brief: {
        title: brief.title,
        purpose: brief.purpose,
        audience: brief.audience,
        uniqueAngle: brief.angle,
        evidence: brief.evidence,
        cta: brief.cta
      }
    })
    jobs.value = [response.data, ...jobs.value.filter(job => job.id !== response.data.id)]
    successMessage.value = response.data.reused ? 'Matching AI job reused.' : 'AI job created.'
    activeTab.value = 'jobs'
  } catch (error) {
    errorMessage.value = apiStatus(error) === 501
      ? 'AI jobs are not enabled on this backend.'
      : normalizeAPIError(error, 'Could not create the AI job.')
  } finally {
    creatingJob.value = false
  }
}

async function saveVoiceProfile() {
  if (!canSaveVoice.value) return
  savingVoice.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const profile: VoiceProfileDocument = {
      audience: voiceForm.audience,
      assumedKnowledge: voiceForm.assumedKnowledge,
      brandPurpose: voiceForm.brandPurpose,
      pointOfView: voiceForm.pointOfView,
      tone: voiceForm.tone,
      formality: voiceForm.formality,
      humor: voiceForm.humor,
      preferredVocabulary: lineItems(voiceForm.preferredVocabulary),
      productTerminology: keyValueItems(voiceForm.productTerminology),
      approvedProductFacts: lineItems(voiceForm.approvedProductFacts),
      sentencePreferences: voiceForm.sentencePreferences,
      paragraphPreferences: voiceForm.paragraphPreferences,
      avoidPhrases: lineItems(voiceForm.avoidPhrases),
      prohibitedClaims: lineItems(voiceForm.prohibitedClaims),
      contentTypeStyles: keyValueItems(voiceForm.contentTypeStyles),
      writingExamples: voiceForm.writingExamples.map(example => ({ ...example })),
      introductionRules: voiceForm.introductionRules,
      conclusionRules: voiceForm.conclusionRules,
      callToActionRules: voiceForm.callToActionRules,
      regionalSpelling: voiceForm.regionalSpelling,
      locale: voiceForm.locale
    }
    const response = await api.createVoiceProfile(projectID.value, profile)
    voiceProfile.value = response.data
    successMessage.value = `Voice profile version ${response.data.version} created.`
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not save the voice profile.')
  } finally {
    savingVoice.value = false
  }
}

async function createPacket() {
  if (!canCreateEvidence.value) return
  savingEvidence.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const packet: EvidencePacketDocument = {
      humanBrief: evidenceForm.humanBrief,
      searchIntent: evidenceForm.searchIntent,
      thesis: evidenceForm.thesis,
      productFacts: lineItems(evidenceForm.productFacts).map(statement => ({
        statement,
        sourceIds: [...evidenceForm.sourceIds]
      })),
      subjectMatterNotes: lineItems(evidenceForm.subjectMatterNotes),
      firsthandObservations: lineItems(evidenceForm.firsthandObservations),
      sourceIds: [...evidenceForm.sourceIds],
      customerEvidence: lineItems(evidenceForm.customerEvidence),
      measurements: lineItems(evidenceForm.measurements),
      allowedClaims: lineItems(evidenceForm.allowedClaims),
      prohibitedClaims: lineItems(evidenceForm.prohibitedClaims),
      limitations: lineItems(evidenceForm.limitations),
      requiredInternalLinks: lineItems(evidenceForm.requiredInternalLinks),
      callToAction: evidenceForm.callToAction,
      publicationRecommendation: evidenceForm.publicationRecommendation as EvidencePacketDocument['publicationRecommendation']
    }
    const response = await api.createEvidencePacket(projectID.value, evidenceForm.contentId, packet)
    evidencePackets.value = [response.data, ...evidencePackets.value]
    successMessage.value = `Evidence packet version ${response.data.version} created.`
    resetEvidenceForm()
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not create the evidence packet.')
  } finally {
    savingEvidence.value = false
  }
}

async function approvePacket(packetID: string) {
  approvingPacket.value = packetID
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const response = await api.approveEvidencePacket(projectID.value, packetID)
    const index = evidencePackets.value.findIndex(packet => packet.id === packetID)
    if (index >= 0) evidencePackets.value[index] = response.data
    successMessage.value = `Evidence packet version ${response.data.version} approved.`
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not approve the evidence packet.')
  } finally {
    approvingPacket.value = ''
  }
}

function applyVoiceProfile(profile: VoiceProfileDocument) {
  voiceForm.audience = profile.audience
  voiceForm.assumedKnowledge = profile.assumedKnowledge
  voiceForm.brandPurpose = profile.brandPurpose
  voiceForm.pointOfView = profile.pointOfView
  voiceForm.tone = profile.tone
  voiceForm.formality = profile.formality
  voiceForm.humor = profile.humor
  voiceForm.preferredVocabulary = profile.preferredVocabulary.join('\n')
  voiceForm.productTerminology = mapItems(profile.productTerminology)
  voiceForm.approvedProductFacts = profile.approvedProductFacts.join('\n')
  voiceForm.sentencePreferences = profile.sentencePreferences
  voiceForm.paragraphPreferences = profile.paragraphPreferences
  voiceForm.avoidPhrases = profile.avoidPhrases.join('\n')
  voiceForm.prohibitedClaims = profile.prohibitedClaims.join('\n')
  voiceForm.contentTypeStyles = mapItems(profile.contentTypeStyles)
  voiceForm.introductionRules = profile.introductionRules
  voiceForm.conclusionRules = profile.conclusionRules
  voiceForm.callToActionRules = profile.callToActionRules
  voiceForm.regionalSpelling = profile.regionalSpelling
  voiceForm.locale = profile.locale
  voiceForm.writingExamples = profile.writingExamples.map(example => ({ ...example }))
  while (voiceForm.writingExamples.length < 3) {
    voiceForm.writingExamples.push({ title: '', excerpt: '' })
  }
}

function resetEvidenceForm() {
  evidenceForm.humanBrief = ''
  evidenceForm.searchIntent = ''
  evidenceForm.thesis = ''
  evidenceForm.sourceIds = []
  evidenceForm.productFacts = ''
  evidenceForm.subjectMatterNotes = ''
  evidenceForm.firsthandObservations = ''
  evidenceForm.customerEvidence = ''
  evidenceForm.measurements = ''
  evidenceForm.allowedClaims = ''
  evidenceForm.prohibitedClaims = ''
  evidenceForm.limitations = ''
  evidenceForm.requiredInternalLinks = ''
  evidenceForm.callToAction = ''
  evidenceForm.publicationRecommendation = ''
}

function lineItems(value: string) {
  return [...new Set(value.split('\n').map(item => item.trim()).filter(Boolean))]
}

function keyValueItems(value: string) {
  return Object.fromEntries(lineItems(value).flatMap((item) => {
    const separator = item.indexOf(':')
    if (separator < 1) return []
    const key = item.slice(0, separator).trim()
    const mappedValue = item.slice(separator + 1).trim()
    return key && mappedValue ? [[key, mappedValue]] : []
  }))
}

function mapItems(value: Record<string, string>) {
  return Object.entries(value).map(([key, mappedValue]) => `${key}: ${mappedValue}`).join('\n')
}

function articleTitle(contentID?: string) {
  if (!contentID) return 'Project evidence'
  return articles.value.find(article => article.id === contentID)?.title || contentID
}

function packetSourceCount(packet: EvidencePacket) {
  const sourceIDs = new Set(packet.packet.sourceIds)
  for (const fact of packet.packet.productFacts) {
    for (const sourceID of fact.sourceIds) sourceIDs.add(sourceID)
  }
  return sourceIDs.size
}

function evidenceStatus(packet: EvidencePacket) {
  if (packet.approvedAt) return 'Approved'
  if (packet.packet.publicationRecommendation === 'request_unique_evidence') return 'Needs evidence'
  if (packet.packet.publicationRecommendation === 'do_not_publish') return 'Do not publish'
  return 'Draft'
}

function modelForJob(jobID: string) {
  const run = runsByJob.value.get(jobID)
  return run ? `${run.provider} / ${run.modelIdentifier}` : 'Pending'
}

function prettyJSON(value: unknown) {
  return JSON.stringify(value, null, 2)
}

function qualityIcon(checkType: string) {
  if (checkType.includes('source') || checkType.includes('claim')) return Link2
  if (checkType.includes('duplicate') || checkType.includes('similar')) return FileSearch
  if (checkType.includes('clarity') || checkType.includes('readability')) return ScanText
  return ShieldCheck
}

function qualityStatusClass(status: QualityCheckResult['status']) {
  if (status === 'passed') return 'status-pill--success'
  if (status === 'overridden') return 'status-pill--warning'
  return 'status-pill--danger'
}

function relativeDate(value?: string) {
  if (!value) return 'Unknown'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Unknown'
  const minutes = Math.floor((Date.now() - date.getTime()) / 60000)
  if (minutes < 1) return 'Just now'
  if (minutes < 60) return `${minutes}m ago`
  if (minutes < 1440) return `${Math.floor(minutes / 60)}h ago`
  return `${Math.floor(minutes / 1440)}d ago`
}
</script>

<style scoped>
.ai-tabs { display: flex; gap: 4px; padding: 5px; overflow-x: auto; }
.ai-tabs button { display: inline-flex; min-height: 36px; align-items: center; gap: 7px; padding: 7px 12px; border: 0; border-radius: 5px; background: transparent; color: var(--text-soft); font-size: 13px; font-weight: 600; cursor: pointer; }
.ai-tabs button.is-active { background: var(--surface); color: var(--text); box-shadow: var(--shadow-sm); }
.ai-layout { display: grid; grid-template-columns: minmax(0, 1fr) 300px; gap: 16px; align-items: start; }
.ai-form { overflow: hidden; }
.panel-heading { display: flex; align-items: center; gap: 11px; min-height: 66px; padding: 13px 16px; border-bottom: 1px solid var(--border); }
.panel-heading > div { min-width: 0; }
.panel-heading > .button, .panel-heading > .status-pill { margin-left: auto; }
.panel-heading__icon { display: grid; width: 36px; height: 36px; flex: 0 0 36px; place-items: center; border-radius: 7px; background: var(--primary-soft); color: var(--primary); }
.panel-heading__icon--blue { background: var(--blue-soft); color: var(--blue); }
.panel-heading__icon--amber { background: var(--amber-soft); color: var(--amber); }
.panel-heading p, .panel-heading h3 { margin: 0; }
.panel-heading p { color: var(--text-soft); font-size: 12px; }
.panel-heading h3 { margin-top: 1px; font-size: 14px; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; padding: 18px; }
.form-grid--single { grid-template-columns: 1fr; }
.field--wide { grid-column: 1 / -1; }
.form-footer { display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 12px 18px; border-top: 1px solid var(--border); background: var(--surface-subtle); }
.form-footer > span { display: inline-flex; align-items: center; gap: 6px; color: var(--text-soft); font-size: 12px; }
.ai-rail { display: grid; gap: 14px; }
.rail-panel { overflow: hidden; }
.rail-panel__heading { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 13px 14px; border-bottom: 1px solid var(--border); }
.rail-panel__heading span { color: var(--text-soft); font-size: 12px; }
.rail-panel__heading h3 { margin: 1px 0 0; font-size: 13px; }
.readiness-score { color: var(--primary) !important; font-size: 17px !important; font-weight: 700; }
.readiness-track { height: 4px; background: var(--border); }
.readiness-track span { display: block; height: 100%; background: var(--primary); transition: width .2s ease; }
.readiness-list { display: grid; gap: 9px; margin: 0; padding: 14px; list-style: none; }
.readiness-list li { display: flex; align-items: center; gap: 8px; color: var(--text-faint); font-size: 12px; }
.readiness-list li.is-ready { color: var(--primary); }
.job-mini-list button { display: grid; width: 100%; grid-template-columns: 8px minmax(0, 1fr) 15px; align-items: center; gap: 8px; padding: 10px 14px; border: 0; border-bottom: 1px solid var(--border); background: transparent; color: var(--text); text-align: left; cursor: pointer; }
.job-mini-list button:hover { background: var(--surface-subtle); }
.job-mini-list button span:nth-child(2) { display: flex; min-width: 0; flex-direction: column; }
.job-mini-list strong, .job-mini-list small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.job-mini-list strong { font-size: 12px; text-transform: capitalize; }
.job-mini-list small { margin-top: 2px; color: var(--text-soft); font-size: 12px; }
.job-status { display: inline-block; width: 7px; height: 7px; border-radius: 50%; background: var(--text-faint); }
.job-status--completed, .job-status--succeeded { background: var(--primary); }
.job-status--running { background: var(--blue); }
.job-status--failed { background: var(--danger); }
.rail-empty { margin: 0; padding: 16px 14px; color: var(--text-soft); font-size: 12px; }
.evidence-grid { display: grid; grid-template-columns: minmax(0, 1.35fr) minmax(280px, .65fr); gap: 16px; }
.evidence-panel { overflow: hidden; }
.evidence-list { display: grid; }
.evidence-row { display: grid; grid-template-columns: minmax(0, 1fr) auto 32px; align-items: center; gap: 10px; min-height: 76px; padding: 12px 14px; border-bottom: 1px solid var(--border); }
.evidence-row > div { display: grid; min-width: 0; gap: 3px; }
.evidence-row strong, .evidence-row span, .evidence-row small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.evidence-row strong { font-size: 13px; }
.evidence-row div > span, .evidence-row small { color: var(--text-soft); font-size: 12px; }
.source-picker, .voice-examples { min-width: 0; margin: 0; padding: 0; border: 0; }
.source-picker legend, .voice-examples legend { margin-bottom: 8px; color: var(--text); font-size: 12px; font-weight: 650; }
.source-picker { display: grid; gap: 7px; }
.source-picker label { display: flex; min-width: 0; align-items: center; gap: 8px; font-size: 12px; }
.source-picker label span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.source-picker p { margin: 0; color: var(--text-soft); font-size: 12px; }
.voice-examples { display: grid; gap: 14px; }
.voice-examples > div { display: grid; grid-template-columns: minmax(180px, .4fr) minmax(0, 1fr) 32px; gap: 14px; padding-top: 14px; border-top: 1px solid var(--border); }
.voice-example__remove { margin-top: 18px; }
.voice-examples__add { justify-self: start; }
.empty-state--embedded { min-height: 280px; border: 0; border-radius: 0; box-shadow: none; }
.jobs-panel { overflow: hidden; }
.jobs-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 14px 16px; border-bottom: 1px solid var(--border); }
.jobs-header span { color: var(--text-soft); font-size: 12px; }
.jobs-header h3 { margin: 1px 0 0; font-size: 14px; }
.jobs-table { overflow-x: auto; }
.jobs-row { display: grid; min-width: 760px; grid-template-columns: 1fr .65fr 1fr .7fr 1fr; gap: 16px; align-items: center; padding: 11px 16px; border-bottom: 1px solid var(--border); font-size: 12px; }
.jobs-row > span { display: flex; min-width: 0; align-items: center; gap: 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.jobs-row:not(.jobs-row--header) > span:first-child { align-items: flex-start; flex-direction: column; gap: 2px; }
.jobs-row:not(.jobs-row--header) > span:first-child small { color: var(--text-soft); font-size: 12px; }
.jobs-row--header { background: var(--surface-subtle); color: var(--text-soft); font-size: 12px; font-weight: 650; text-transform: uppercase; }
.job-result { padding: 10px 16px 14px; border-bottom: 1px solid var(--border); background: var(--surface-subtle); }
.job-result summary { cursor: pointer; font-size: 12px; font-weight: 650; }
.job-result pre { max-height: 360px; margin: 10px 0 0; padding: 12px; overflow: auto; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); font-size: 12px; line-height: 1.55; white-space: pre-wrap; }
.job-error { margin: 0; padding: 8px 16px 12px; border-bottom: 1px solid var(--border); color: var(--danger); font-size: 12px; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; color: var(--text-soft); }
.quality-section { display: grid; gap: 12px; }
.quality-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.quality-heading span { color: var(--text-soft); font-size: 12px; }
.quality-heading h3 { margin: 1px 0 0; font-size: 14px; }
.quality-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.quality-card { display: grid; grid-template-columns: 38px minmax(0, 1fr) auto; align-items: center; gap: 12px; padding: 16px; }
.quality-card__icon { display: grid; width: 38px; height: 38px; place-items: center; border-radius: 7px; background: var(--surface-subtle); color: var(--text-soft); }
.quality-card h3 { margin: 0; font-size: 12px; }
.quality-card p { margin: 3px 0 0; color: var(--text-soft); font-size: 12px; }
.quality-card small { display: block; margin-top: 5px; color: var(--text-faint); font-size: 12px; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1000px) { .ai-layout, .evidence-grid { grid-template-columns: 1fr; } .ai-rail { grid-template-columns: 1fr 1fr; } }
@media (max-width: 680px) { .form-grid, .quality-grid, .ai-rail, .voice-examples > div { grid-template-columns: 1fr; } .field--wide { grid-column: auto; } .form-footer { align-items: stretch; flex-direction: column; } .quality-card { grid-template-columns: 38px minmax(0, 1fr); } .quality-card .status-pill { grid-column: 2; justify-self: start; } .evidence-row { grid-template-columns: minmax(0, 1fr) 32px; } .evidence-row > .status-pill { grid-column: 1; grid-row: 2; justify-self: start; } .voice-example__remove { justify-self: end; margin-top: 0; } }
</style>

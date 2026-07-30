<template>
  <div class="page-stack">
    <div class="page-heading">
      <div>
        <h2>AI workspace</h2>
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
            <input v-model.trim="brief.cta" placeholder="Optional next step for the reader">
          </label>
        </div>

        <div class="form-footer">
          <span><ShieldCheck :size="15" />AI output remains a proposal until reviewed.</span>
          <button class="button button--primary" type="submit" :disabled="creatingJob || !canSubmit">
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
          <button class="button button--compact" type="button" disabled><Plus :size="15" />New packet</button>
        </div>
        <div class="empty-state empty-state--embedded">
          <div>
            <span class="empty-state__icon"><Library :size="20" /></span>
            <h3>{{ serviceAvailable ? 'No evidence packets' : 'Evidence service is not available' }}</h3>
            <p>Approved sources and claim mappings will appear here.</p>
          </div>
        </div>
      </section>
      <section class="surface evidence-panel">
        <div class="panel-heading">
          <span class="panel-heading__icon panel-heading__icon--amber"><Quote :size="18" /></span>
          <div>
            <p>Coverage</p>
            <h3>Claim status</h3>
          </div>
        </div>
        <div class="claim-stats">
          <div><strong>0</strong><span>Verified</span></div>
          <div><strong>0</strong><span>Needs evidence</span></div>
          <div><strong>0</strong><span>Conflicting</span></div>
        </div>
      </section>
    </div>

    <section v-else-if="activeTab === 'jobs'" class="surface jobs-panel">
      <div class="jobs-header">
        <div>
          <span>Run history</span>
          <h3>AI jobs</h3>
        </div>
        <button class="button button--compact" type="button" :disabled="jobsPending" @click="loadJobs">
          <RefreshCw :class="{ spin: jobsPending }" :size="15" />
          Refresh
        </button>
      </div>
      <div v-if="jobs.length" class="jobs-table">
        <div class="jobs-row jobs-row--header"><span>Job</span><span>Status</span><span>Updated</span><span>ID</span></div>
        <div v-for="job in jobs" :key="job.id" class="jobs-row">
          <span><strong>{{ labelize(job.type) }}</strong></span>
          <span><i class="job-status" :class="`job-status--${job.status}`" />{{ labelize(job.status) }}</span>
          <span>{{ relativeDate(job.updatedAt || job.createdAt) }}</span>
          <span class="mono">{{ job.id }}</span>
        </div>
      </div>
      <div v-else class="empty-state empty-state--embedded">
        <div><span class="empty-state__icon"><Bot :size="20" /></span><h3>No AI jobs</h3><p>Generation and quality jobs will appear here.</p></div>
      </div>
    </section>

    <section v-else class="quality-grid">
      <article v-for="check in qualityChecks" :key="check.title" class="surface quality-card">
        <span class="quality-card__icon"><component :is="check.icon" :size="18" /></span>
        <div>
          <h3>{{ check.title }}</h3>
          <p>{{ check.description }}</p>
        </div>
        <span class="status-pill">No runs</span>
      </article>
    </section>
  </div>
</template>

<script setup lang="ts">
import {
  Bot,
  CheckCircle2,
  ChevronRight,
  Circle,
  FileSearch,
  Library,
  Link2,
  ListChecks,
  LoaderCircle,
  Plus,
  Quote,
  RefreshCw,
  ScanText,
  ShieldCheck,
  Sparkles,
  WandSparkles
} from 'lucide-vue-next'
import { ARTICLE_TYPES, type AIJob } from '~/composables/useAdminApi'

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => String(route.params.projectId || ''))
const activeTab = ref('brief')
const jobs = ref<AIJob[]>([])
const jobsPending = ref(true)
const creatingJob = ref(false)
const serviceAvailable = ref(true)
const errorMessage = ref('')
const successMessage = ref('')
const tabs = [
  { value: 'brief', label: 'Brief', icon: Sparkles },
  { value: 'evidence', label: 'Evidence', icon: Library },
  { value: 'jobs', label: 'Jobs', icon: Bot },
  { value: 'quality', label: 'Quality', icon: ListChecks }
]
const brief = reactive({
  title: '',
  articleType: 'standard',
  jobType: 'outline',
  purpose: '',
  audience: '',
  angle: '',
  evidence: '',
  cta: ''
})
const readinessChecks = computed(() => [
  { label: 'Clear purpose', ready: brief.purpose.trim().length >= 20 },
  { label: 'Defined audience', ready: brief.audience.trim().length >= 10 },
  { label: 'Unique angle', ready: brief.angle.trim().length >= 15 },
  { label: 'Evidence supplied', ready: brief.evidence.trim().length >= 20 },
  { label: 'Working title', ready: brief.title.trim().length >= 8 }
])
const readinessScore = computed(() => Math.round(readinessChecks.value.filter(item => item.ready).length / readinessChecks.value.length * 100))
const canSubmit = computed(() => readinessChecks.value.slice(0, 3).every(item => item.ready))
const qualityChecks = [
  { title: 'Source coverage', description: 'Claims mapped to accessible evidence.', icon: Link2 },
  { title: 'Content clarity', description: 'Readability, filler, and structural checks.', icon: ScanText },
  { title: 'Duplication', description: 'Project content similarity and topic overlap.', icon: FileSearch },
  { title: 'Editorial policy', description: 'Voice, prohibited claims, and disclosure checks.', icon: ShieldCheck }
]

onMounted(loadJobs)

async function loadJobs() {
  jobsPending.value = true
  try {
    jobs.value = (await api.listAIJobs(projectID.value)).data
    serviceAvailable.value = true
  } catch (error) {
    if (apiStatus(error) === 501) {
      jobs.value = []
      serviceAvailable.value = false
    } else {
      errorMessage.value = normalizeAPIError(error, 'Could not load AI jobs.')
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
      articleType: brief.articleType,
      brief: {
        title: brief.title,
        purpose: brief.purpose,
        audience: brief.audience,
        uniqueAngle: brief.angle,
        evidence: brief.evidence,
        cta: brief.cta
      }
    })
    jobs.value = [response.data, ...jobs.value]
    successMessage.value = 'AI job created.'
    activeTab.value = 'jobs'
  } catch (error) {
    errorMessage.value = apiStatus(error) === 501
      ? 'AI jobs are not enabled on this backend.'
      : normalizeAPIError(error, 'Could not create the AI job.')
  } finally {
    creatingJob.value = false
  }
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
.ai-tabs button { display: inline-flex; min-height: 36px; align-items: center; gap: 7px; padding: 7px 12px; border: 0; border-radius: 5px; background: transparent; color: var(--text-soft); font-size: 11px; font-weight: 600; cursor: pointer; }
.ai-tabs button.is-active { background: var(--surface); color: var(--text); box-shadow: var(--shadow-sm); }
.ai-layout { display: grid; grid-template-columns: minmax(0, 1fr) 300px; gap: 16px; align-items: start; }
.ai-form { overflow: hidden; }
.panel-heading { display: flex; align-items: center; gap: 11px; min-height: 66px; padding: 13px 16px; border-bottom: 1px solid var(--border); }
.panel-heading > div { min-width: 0; }
.panel-heading > .button { margin-left: auto; }
.panel-heading__icon { display: grid; width: 36px; height: 36px; flex: 0 0 36px; place-items: center; border-radius: 7px; background: var(--primary-soft); color: var(--primary); }
.panel-heading__icon--blue { background: var(--blue-soft); color: var(--blue); }
.panel-heading__icon--amber { background: var(--amber-soft); color: var(--amber); }
.panel-heading p, .panel-heading h3 { margin: 0; }
.panel-heading p { color: var(--text-soft); font-size: 9px; }
.panel-heading h3 { margin-top: 1px; font-size: 14px; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; padding: 18px; }
.field--wide { grid-column: 1 / -1; }
.form-footer { display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 12px 18px; border-top: 1px solid var(--border); background: var(--surface-subtle); }
.form-footer > span { display: inline-flex; align-items: center; gap: 6px; color: var(--text-soft); font-size: 9px; }
.ai-rail { display: grid; gap: 14px; }
.rail-panel { overflow: hidden; }
.rail-panel__heading { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 13px 14px; border-bottom: 1px solid var(--border); }
.rail-panel__heading span { color: var(--text-soft); font-size: 9px; }
.rail-panel__heading h3 { margin: 1px 0 0; font-size: 13px; }
.readiness-score { color: var(--primary) !important; font-size: 17px !important; font-weight: 700; }
.readiness-track { height: 4px; background: var(--border); }
.readiness-track span { display: block; height: 100%; background: var(--primary); transition: width .2s ease; }
.readiness-list { display: grid; gap: 9px; margin: 0; padding: 14px; list-style: none; }
.readiness-list li { display: flex; align-items: center; gap: 8px; color: var(--text-faint); font-size: 10px; }
.readiness-list li.is-ready { color: var(--primary); }
.job-mini-list button { display: grid; width: 100%; grid-template-columns: 8px minmax(0, 1fr) 15px; align-items: center; gap: 8px; padding: 10px 14px; border: 0; border-bottom: 1px solid var(--border); background: transparent; color: var(--text); text-align: left; cursor: pointer; }
.job-mini-list button:hover { background: var(--surface-subtle); }
.job-mini-list button span:nth-child(2) { display: flex; min-width: 0; flex-direction: column; }
.job-mini-list strong, .job-mini-list small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.job-mini-list strong { font-size: 10px; text-transform: capitalize; }
.job-mini-list small { margin-top: 2px; color: var(--text-soft); font-size: 8px; }
.job-status { display: inline-block; width: 7px; height: 7px; border-radius: 50%; background: var(--text-faint); }
.job-status--completed, .job-status--succeeded { background: var(--primary); }
.job-status--running { background: var(--blue); }
.job-status--failed { background: var(--danger); }
.rail-empty { margin: 0; padding: 16px 14px; color: var(--text-soft); font-size: 10px; }
.evidence-grid { display: grid; grid-template-columns: minmax(0, 1.35fr) minmax(280px, .65fr); gap: 16px; }
.evidence-panel { overflow: hidden; }
.empty-state--embedded { min-height: 280px; border: 0; border-radius: 0; box-shadow: none; }
.claim-stats { display: grid; gap: 1px; background: var(--border); }
.claim-stats > div { display: flex; align-items: center; justify-content: space-between; padding: 16px; background: var(--surface); }
.claim-stats strong { font-size: 18px; }
.claim-stats span { color: var(--text-soft); font-size: 10px; }
.jobs-panel { overflow: hidden; }
.jobs-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 14px 16px; border-bottom: 1px solid var(--border); }
.jobs-header span { color: var(--text-soft); font-size: 9px; }
.jobs-header h3 { margin: 1px 0 0; font-size: 14px; }
.jobs-table { overflow-x: auto; }
.jobs-row { display: grid; min-width: 680px; grid-template-columns: 1.2fr .7fr .7fr 1fr; gap: 16px; align-items: center; padding: 11px 16px; border-bottom: 1px solid var(--border); font-size: 10px; }
.jobs-row > span { display: flex; min-width: 0; align-items: center; gap: 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.jobs-row--header { background: var(--surface-subtle); color: var(--text-soft); font-size: 9px; font-weight: 650; text-transform: uppercase; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; color: var(--text-soft); }
.quality-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.quality-card { display: grid; grid-template-columns: 38px minmax(0, 1fr) auto; align-items: center; gap: 12px; padding: 16px; }
.quality-card__icon { display: grid; width: 38px; height: 38px; place-items: center; border-radius: 7px; background: var(--surface-subtle); color: var(--text-soft); }
.quality-card h3 { margin: 0; font-size: 12px; }
.quality-card p { margin: 3px 0 0; color: var(--text-soft); font-size: 9px; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1000px) { .ai-layout, .evidence-grid { grid-template-columns: 1fr; } .ai-rail { grid-template-columns: 1fr 1fr; } }
@media (max-width: 680px) { .form-grid, .quality-grid, .ai-rail { grid-template-columns: 1fr; } .field--wide { grid-column: auto; } .form-footer { align-items: stretch; flex-direction: column; } .quality-card { grid-template-columns: 38px minmax(0, 1fr); } .quality-card .status-pill { grid-column: 2; justify-self: start; } }
</style>

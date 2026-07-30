<template>
  <div class="page-stack">
    <div class="page-heading">
      <div>
        <p>API readiness, project dependencies, retained records, and recent operational events.</p>
      </div>
      <button class="button button--compact" type="button" :disabled="pending" @click="loadOperations">
        <RefreshCw :class="{ spin: pending }" :size="16" />
        Refresh
      </button>
    </div>

    <div class="health-grid">
      <article class="surface health-card">
        <span class="health-card__icon" :class="{ 'health-card__icon--ok': health.live === 'ok' }"><HeartPulse :size="19" /></span>
        <div><span>API process</span><h3>{{ health.live === 'ok' ? 'Operational' : 'Unavailable' }}</h3></div>
        <span class="status-pill" :class="health.live === 'ok' ? 'status-pill--success' : 'status-pill--danger'">{{ health.live }}</span>
      </article>
      <article class="surface health-card">
        <span class="health-card__icon" :class="{ 'health-card__icon--ok': health.ready === 'ok' }"><Database :size="19" /></span>
        <div><span>SQLite readiness</span><h3>{{ health.ready === 'ok' ? 'Connected' : 'Unavailable' }}</h3></div>
        <span class="status-pill" :class="health.ready === 'ok' ? 'status-pill--success' : 'status-pill--danger'">{{ health.ready }}</span>
      </article>
      <article class="surface health-card">
        <span class="health-card__icon"><PackageCheck :size="19" /></span>
        <div><span>API version</span><h3>{{ health.version || 'Unknown' }}</h3></div>
        <span class="status-pill">{{ health.service || 'service' }}</span>
      </article>
    </div>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>

    <div class="operations-layout">
      <section class="surface impact-panel">
        <div class="panel-heading">
          <span class="panel-heading__icon"><Network :size="18" /></span>
          <div><p>Dependency report</p><h3>Project footprint</h3></div>
          <span v-if="impact" class="status-pill" :class="impact.canDelete ? 'status-pill--success' : 'status-pill--warning'">
            {{ impact.canDelete ? 'Clear' : 'Retained data' }}
          </span>
        </div>
        <div v-if="impact" class="impact-grid">
          <div v-for="item in impactItems" :key="item.label">
            <component :is="item.icon" :size="16" />
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}</strong>
          </div>
        </div>
        <div v-else class="empty-state empty-state--embedded">
          <div><span class="empty-state__icon"><Network :size="20" /></span><h3>No dependency report</h3></div>
        </div>
      </section>

      <section class="surface events-panel">
        <div class="panel-heading">
          <span class="panel-heading__icon panel-heading__icon--blue"><Activity :size="18" /></span>
          <div><p>Latest activity</p><h3>Operational events</h3></div>
          <NuxtLink class="button button--compact" :to="`/projects/${projectID}/audit-events`">Audit log<ArrowUpRight :size="14" /></NuxtLink>
        </div>
        <div v-if="events.length" class="events-list">
          <div v-for="event in events.slice(0, 8)" :key="event.id" class="event-item">
            <span class="event-dot" :class="{ 'event-dot--failed': event.outcome !== 'success' }" />
            <div><strong>{{ labelize(event.action) }}</strong><small>{{ event.targetType ? `${labelize(event.targetType)} · ` : '' }}{{ relativeDate(event.createdAt) }}</small></div>
            <span class="status-pill" :class="event.outcome === 'success' ? 'status-pill--success' : 'status-pill--danger'">{{ event.outcome }}</span>
          </div>
        </div>
        <div v-else class="empty-state empty-state--embedded">
          <div><span class="empty-state__icon"><Activity :size="20" /></span><h3>No operational events</h3></div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  Activity,
  ArrowUpRight,
  CalendarClock,
  Database,
  FileText,
  HeartPulse,
  KeyRound,
  Link2,
  Network,
  PackageCheck,
  RefreshCw,
  UsersRound,
  Webhook
} from 'lucide-vue-next'
import type { AuditEvent, ProjectDeletionImpact } from '~/composables/useAdminApi'

type HealthState = { live: string, ready: string, service: string, version: string }

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => String(route.params.projectId || ''))
const impact = ref<ProjectDeletionImpact | null>(null)
const events = ref<AuditEvent[]>([])
const health = reactive<HealthState>({ live: 'unknown', ready: 'unknown', service: '', version: '' })
const pending = ref(true)
const errorMessage = ref('')
const impactItems = computed(() => impact.value ? [
  { label: 'Content items', value: impact.value.contentItems, icon: FileText },
  { label: 'Published', value: impact.value.publishedPublications, icon: Link2 },
  { label: 'Schedules', value: impact.value.scheduledPublications, icon: CalendarClock },
  { label: 'Active keys', value: impact.value.activeApiKeys, icon: KeyRound },
  { label: 'Members', value: impact.value.activeMembers, icon: UsersRound },
  { label: 'Webhooks', value: impact.value.webhooks, icon: Webhook }
] : [])

onMounted(loadOperations)

async function loadOperations() {
  pending.value = true
  errorMessage.value = ''
  const [liveResult, readyResult, impactResult, eventResult] = await Promise.allSettled([
    $fetch<{ status: string, service: string, version: string }>('/healthz'),
    $fetch<{ status: string, service: string, version: string }>('/readyz'),
    api.projectDeletionImpact(projectID.value),
    api.listAuditEvents(projectID.value, 20)
  ])
  if (liveResult.status === 'fulfilled') {
    health.live = liveResult.value.status
    health.service = liveResult.value.service
    health.version = liveResult.value.version
  } else {
    health.live = 'unavailable'
  }
  health.ready = readyResult.status === 'fulfilled' ? readyResult.value.status : 'unavailable'
  if (impactResult.status === 'fulfilled') impact.value = impactResult.value.data
  if (eventResult.status === 'fulfilled') events.value = eventResult.value.data
  if (impactResult.status === 'rejected' && eventResult.status === 'rejected') {
    errorMessage.value = normalizeAPIError(impactResult.reason, 'Could not load project operations.')
  }
  pending.value = false
}

function relativeDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Unknown time'
  const minutes = Math.floor((Date.now() - date.getTime()) / 60000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  if (minutes < 1440) return `${Math.floor(minutes / 60)}h ago`
  return `${Math.floor(minutes / 1440)}d ago`
}
</script>

<style scoped>
.health-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.health-card { display: grid; grid-template-columns: 40px minmax(0, 1fr) auto; align-items: center; gap: 11px; padding: 15px; }
.health-card__icon { display: grid; width: 40px; height: 40px; place-items: center; border-radius: 7px; background: var(--danger-soft); color: var(--danger); }
.health-card__icon--ok { background: var(--primary-soft); color: var(--primary); }
.health-card > div { min-width: 0; }
.health-card span, .health-card h3 { overflow: hidden; margin: 0; text-overflow: ellipsis; white-space: nowrap; }
.health-card > div span { color: var(--text-soft); font-size: 9px; }
.health-card h3 { margin-top: 2px; font-size: 13px; }
.operations-layout { display: grid; grid-template-columns: minmax(0, .85fr) minmax(0, 1.15fr); gap: 16px; align-items: start; }
.impact-panel, .events-panel { overflow: hidden; }
.panel-heading { display: flex; align-items: center; gap: 11px; min-height: 66px; padding: 13px 16px; border-bottom: 1px solid var(--border); }
.panel-heading__icon { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 7px; background: var(--primary-soft); color: var(--primary); }
.panel-heading__icon--blue { background: var(--blue-soft); color: var(--blue); }
.panel-heading p, .panel-heading h3 { margin: 0; }
.panel-heading p { color: var(--text-soft); font-size: 9px; }
.panel-heading h3 { margin-top: 1px; font-size: 14px; }
.panel-heading > :last-child { margin-left: auto; }
.impact-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1px; background: var(--border); }
.impact-grid > div { display: grid; grid-template-columns: 24px minmax(0, 1fr) auto; align-items: center; gap: 8px; min-height: 56px; padding: 10px 14px; background: var(--surface); color: var(--text-soft); }
.impact-grid span { font-size: 10px; }
.impact-grid strong { color: var(--text); font-size: 15px; }
.events-list { display: grid; }
.event-item { display: grid; grid-template-columns: 8px minmax(0, 1fr) auto; align-items: center; gap: 10px; padding: 11px 15px; border-bottom: 1px solid var(--border); }
.event-item:last-child { border-bottom: 0; }
.event-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--primary); }
.event-dot--failed { background: var(--danger); }
.event-item > div { display: flex; min-width: 0; flex-direction: column; }
.event-item strong, .event-item small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.event-item strong { font-size: 10px; text-transform: capitalize; }
.event-item small { margin-top: 2px; color: var(--text-soft); font-size: 8px; text-transform: capitalize; }
.empty-state--embedded { min-height: 220px; border: 0; border-radius: 0; box-shadow: none; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 960px) { .health-grid, .operations-layout { grid-template-columns: 1fr; } }
@media (max-width: 600px) { .impact-grid { grid-template-columns: 1fr; } .health-card { grid-template-columns: 40px minmax(0, 1fr); } .health-card > .status-pill { grid-column: 2; justify-self: start; } }
</style>

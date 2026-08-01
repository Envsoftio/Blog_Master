<template>
  <div class="page-stack audit-page">
    <div class="page-heading">
      <div>
        <p>Security, access, publishing, and configuration changes for this project.</p>
      </div>
      <button class="button button--compact" type="button" :disabled="pending" @click="refresh">
        <RefreshCw :class="{ spin: pending }" :size="16" />
        Refresh
      </button>
    </div>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>

    <div class="audit-summary">
      <article class="surface audit-summary__card">
        <span class="audit-summary__icon"><ShieldCheck :size="18" /></span>
        <div>
          <span>Loaded events</span>
          <strong>{{ events.length }}</strong>
        </div>
      </article>
      <article class="surface audit-summary__card">
        <span class="audit-summary__icon audit-summary__icon--success"><CheckCircle2 :size="18" /></span>
        <div>
          <span>Successful</span>
          <strong>{{ successCount }}</strong>
        </div>
      </article>
      <article class="surface audit-summary__card">
        <span class="audit-summary__icon audit-summary__icon--danger"><AlertTriangle :size="18" /></span>
        <div>
          <span>Exceptions</span>
          <strong>{{ failureCount }}</strong>
        </div>
      </article>
      <article class="surface audit-summary__card">
        <span class="audit-summary__icon audit-summary__icon--blue"><Clock3 :size="18" /></span>
        <div>
          <span>Latest event</span>
          <strong>{{ latestEventLabel }}</strong>
        </div>
      </article>
    </div>

    <section class="surface audit-panel" aria-labelledby="audit-events-heading">
      <div class="audit-panel__heading">
        <span class="audit-panel__icon"><ScrollText :size="18" /></span>
        <div>
          <p>Project events</p>
          <h2 id="audit-events-heading">Audit trail</h2>
        </div>
        <span class="status-pill">{{ events.length }} loaded</span>
      </div>

      <div v-if="pending" class="audit-loading">
        <LoaderCircle class="spin" :size="18" />
        Loading audit events
      </div>

      <div v-else-if="events.length === 0" class="empty-state empty-state--embedded">
        <div>
          <span class="empty-state__icon"><ScrollText :size="20" /></span>
          <h3>No audit events yet</h3>
        </div>
      </div>

      <ol v-else class="audit-list">
        <li v-for="event in events" :key="event.id" class="audit-event">
          <span class="audit-event__rail" aria-hidden="true">
            <span class="audit-event__dot" :class="event.outcome === 'success' ? 'audit-event__dot--success' : 'audit-event__dot--danger'">
              <component :is="eventIcon(event)" :size="14" />
            </span>
          </span>

          <article class="audit-event__body">
            <div class="audit-event__topline">
              <div class="audit-event__title">
                <span class="audit-event__family" :class="`audit-event__family--${eventFamilyKey(event.action)}`">
                  {{ eventFamily(event.action) }}
                </span>
                <h3>{{ labelize(event.action) }}</h3>
              </div>
              <div class="audit-event__meta">
                <span class="status-pill" :class="event.outcome === 'success' ? 'status-pill--success' : 'status-pill--danger'">
                  {{ event.outcome }}
                </span>
                <time :datetime="event.createdAt">{{ relativeDate(event.createdAt) }}</time>
              </div>
            </div>

            <dl class="audit-fields">
              <div>
                <dt><UserRound :size="14" />Actor</dt>
                <dd>
                  <span>{{ event.actorId || event.actorType || 'Not set' }}</span>
                  <button v-if="event.actorId" class="audit-copy" type="button" title="Copy actor ID" aria-label="Copy actor ID" @click="copyValue(event.actorId, `actor-${event.id}`)">
                    <Clipboard :size="13" />
                  </button>
                </dd>
              </div>
              <div>
                <dt><Crosshair :size="14" />Target</dt>
                <dd>
                  <span>{{ targetLabel(event) }}</span>
                  <button v-if="event.targetId" class="audit-copy" type="button" title="Copy target ID" aria-label="Copy target ID" @click="copyValue(event.targetId, `target-${event.id}`)">
                    <Clipboard :size="13" />
                  </button>
                </dd>
              </div>
              <div>
                <dt><History :size="14" />Time</dt>
                <dd><span>{{ formatDate(event.createdAt) }}</span></dd>
              </div>
              <div>
                <dt><ScrollText :size="14" />Request</dt>
                <dd>
                  <span>{{ event.requestId || 'Not set' }}</span>
                  <button v-if="event.requestId" class="audit-copy" type="button" title="Copy request ID" aria-label="Copy request ID" @click="copyValue(event.requestId, `request-${event.id}`)">
                    <Clipboard :size="13" />
                  </button>
                </dd>
              </div>
            </dl>

            <div class="audit-id">
              <span>{{ event.id }}</span>
              <button class="audit-copy" type="button" title="Copy event ID" aria-label="Copy event ID" @click="copyValue(event.id, `event-${event.id}`)">
                <Clipboard :size="13" />
              </button>
              <small v-if="copiedID === `event-${event.id}`">Copied</small>
            </div>

            <details v-if="hasMetadata(event)" class="audit-metadata">
              <summary>Metadata</summary>
              <pre>{{ metadataText(event) }}</pre>
            </details>
          </article>
        </li>
      </ol>

      <div v-if="nextCursor && !pending" class="audit-panel__footer">
        <button class="button" type="button" :disabled="loadingMore" @click="loadMore">
          <LoaderCircle v-if="loadingMore" class="spin" :size="16" />
          <RefreshCw v-else :size="16" />
          Load more events
        </button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import {
  AlertTriangle,
  CheckCircle2,
  Clipboard,
  Clock3,
  Crosshair,
  FileText,
  History,
  KeyRound,
  LoaderCircle,
  RefreshCw,
  ScrollText,
  Settings2,
  ShieldCheck,
  Tags,
  UserRound,
  UsersRound,
  Webhook
} from 'lucide-vue-next'
import type { APIListEnvelope, AuditEvent } from '~/composables/useAdminApi'

const route = useRoute()
const projectID = computed(() => String(route.params.projectId || ''))

const events = ref<AuditEvent[]>([])
const pending = ref(true)
const loadingMore = ref(false)
const nextCursor = ref('')
const errorMessage = ref('')
const copiedID = ref('')

const successCount = computed(() => events.value.filter(event => event.outcome === 'success').length)
const failureCount = computed(() => events.value.filter(event => event.outcome !== 'success').length)
const latestEventLabel = computed(() => events.value[0]?.createdAt ? relativeDate(events.value[0].createdAt) : 'None')

onMounted(refresh)

async function refresh() {
  pending.value = true
  errorMessage.value = ''
  try {
    const auditResponse = await $fetch<APIListEnvelope<AuditEvent>>(`/api/v1/projects/${projectID.value}/audit-events`, { credentials: 'include' })
    events.value = apiListData(auditResponse)
    nextCursor.value = auditResponse.meta?.nextCursor || ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load audit events. Sign in again if your session has expired.')
  } finally {
    pending.value = false
  }
}

async function loadMore() {
  if (!nextCursor.value || loadingMore.value) return
  loadingMore.value = true
  errorMessage.value = ''
  try {
    const response = await $fetch<APIListEnvelope<AuditEvent>>(`/api/v1/projects/${projectID.value}/audit-events`, {
      credentials: 'include',
      query: {
        cursor: nextCursor.value,
        limit: 50
      }
    })
    events.value = [...events.value, ...apiListData(response)]
    nextCursor.value = response.meta?.nextCursor || ''
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not load more audit events.')
  } finally {
    loadingMore.value = false
  }
}

async function copyValue(value: string | undefined, id: string) {
  if (!value || typeof navigator === 'undefined' || !navigator.clipboard) return
  await navigator.clipboard.writeText(value)
  copiedID.value = id
  window.setTimeout(() => {
    if (copiedID.value === id) copiedID.value = ''
  }, 1600)
}

function targetLabel(event: AuditEvent) {
  if (!event.targetType && !event.targetId) return 'Not set'
  return [labelize(event.targetType || ''), event.targetId].filter(Boolean).join(' - ')
}

function hasMetadata(event: AuditEvent) {
  return Boolean(event.metadata && Object.keys(event.metadata).length > 0)
}

function metadataText(event: AuditEvent) {
  return JSON.stringify(event.metadata, null, 2)
}

function parseBackendUTC(value: string) {
  return new Date(value.includes('T') ? value : `${value.replace(' ', 'T')}Z`)
}

function formatDate(value?: string) {
  if (!value) return 'Not set'
  const date = parseBackendUTC(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(date)
}

function relativeDate(value: string) {
  const date = parseBackendUTC(value)
  if (Number.isNaN(date.getTime())) return 'Unknown time'
  const seconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000))
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

function labelize(value: string) {
  return value
    .replace(/[_-]/g, ' ')
    .replace(/\./g, ' / ')
    .replace(/\b\w/g, character => character.toUpperCase())
}

function eventFamily(action: string) {
  return action.split('.')[0]?.replace(/[_-]/g, ' ') || 'event'
}

function eventFamilyKey(action: string) {
  return eventFamily(action).replace(/\s+/g, '-')
}

function eventIcon(event: AuditEvent) {
  const family = eventFamily(event.action)
  if (event.outcome !== 'success') return AlertTriangle
  if (family === 'api key') return KeyRound
  if (family === 'taxonomy') return Tags
  if (family === 'content' || family === 'article') return FileText
  if (family === 'project' || family === 'member') return UsersRound
  if (family === 'webhook' || family === 'integration') return Webhook
  if (family === 'setting' || family === 'settings') return Settings2
  return ShieldCheck
}

function normalizeAPIError(error: unknown, fallback: string) {
  if (typeof error === 'object' && error !== null && 'data' in error) {
    const data = (error as { data?: { title?: string, detail?: string } }).data
    return data?.detail || data?.title || fallback
  }
  return fallback
}
</script>

<style scoped>
.audit-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.audit-summary__card {
  display: grid;
  min-width: 0;
  grid-template-columns: 38px minmax(0, 1fr);
  align-items: center;
  gap: 11px;
  padding: 14px;
}

.audit-summary__icon,
.audit-panel__icon,
.audit-event__dot {
  display: grid;
  place-items: center;
}

.audit-summary__icon {
  width: 38px;
  height: 38px;
  border-radius: 7px;
  background: var(--surface-subtle);
  color: var(--text-soft);
}

.audit-summary__icon--success {
  background: var(--primary-soft);
  color: var(--primary);
}

.audit-summary__icon--danger {
  background: var(--danger-soft);
  color: var(--danger);
}

.audit-summary__icon--blue {
  background: var(--blue-soft);
  color: var(--blue);
}

.audit-summary__card div {
  min-width: 0;
}

.audit-summary__card span,
.audit-summary__card strong {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.audit-summary__card div span {
  color: var(--text-soft);
  font-size: 12px;
}

.audit-summary__card strong {
  margin-top: 2px;
  font-size: 18px;
  line-height: 1.2;
}

.audit-panel {
  overflow: hidden;
}

.audit-panel__heading {
  display: flex;
  min-height: 66px;
  align-items: center;
  gap: 11px;
  padding: 13px 16px;
  border-bottom: 1px solid var(--border);
}

.audit-panel__icon {
  width: 36px;
  height: 36px;
  border-radius: 7px;
  background: var(--primary-soft);
  color: var(--primary);
}

.audit-panel__heading div {
  min-width: 0;
}

.audit-panel__heading p,
.audit-panel__heading h2 {
  overflow: hidden;
  margin: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.audit-panel__heading p {
  color: var(--text-soft);
  font-size: 12px;
}

.audit-panel__heading h2 {
  margin-top: 1px;
  font-size: 15px;
}

.audit-panel__heading > :last-child {
  margin-left: auto;
}

.audit-loading {
  display: flex;
  min-height: 180px;
  align-items: center;
  justify-content: center;
  gap: 9px;
  color: var(--text-soft);
  font-size: 14px;
}

.audit-list {
  display: grid;
  margin: 0;
  padding: 0;
  list-style: none;
}

.audit-event {
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  gap: 10px;
  padding: 0 16px;
}

.audit-event + .audit-event {
  border-top: 1px solid var(--border);
}

.audit-event__rail {
  position: relative;
  display: flex;
  justify-content: center;
  padding-top: 18px;
}

.audit-event__rail::after {
  position: absolute;
  top: 48px;
  bottom: 0;
  width: 1px;
  background: var(--border);
  content: "";
}

.audit-event:last-child .audit-event__rail::after {
  display: none;
}

.audit-event__dot {
  z-index: 1;
  width: 30px;
  height: 30px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--surface);
  color: var(--text-soft);
  box-shadow: var(--shadow-sm);
}

.audit-event__dot--success {
  border-color: color-mix(in srgb, var(--primary) 34%, var(--border));
  color: var(--primary);
}

.audit-event__dot--danger {
  border-color: color-mix(in srgb, var(--danger) 34%, var(--border));
  color: var(--danger);
}

.audit-event__body {
  min-width: 0;
  padding: 15px 0;
}

.audit-event__topline,
.audit-event__title,
.audit-event__meta {
  display: flex;
  align-items: center;
  gap: 9px;
}

.audit-event__topline {
  justify-content: space-between;
}

.audit-event__title {
  min-width: 0;
}

.audit-event__title h3 {
  overflow: hidden;
  margin: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 15px;
}

.audit-event__family {
  flex: 0 0 auto;
  padding: 3px 7px;
  border-radius: 5px;
  background: var(--surface-subtle);
  color: var(--text-soft);
  font-size: 12px;
  font-weight: 700;
  text-transform: capitalize;
}

.audit-event__family--content,
.audit-event__family--article {
  background: var(--blue-soft);
  color: var(--blue);
}

.audit-event__family--project,
.audit-event__family--member {
  background: var(--primary-soft);
  color: var(--primary);
}

.audit-event__family--api-key {
  background: var(--amber-soft);
  color: var(--amber);
}

.audit-event__meta {
  flex: 0 0 auto;
  color: var(--text-soft);
  font-size: 13px;
}

.audit-fields {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin: 13px 0 0;
}

.audit-fields div {
  min-width: 0;
  padding: 9px 10px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--surface-subtle);
}

.audit-fields dt,
.audit-fields dd,
.audit-id {
  display: flex;
  align-items: center;
}

.audit-fields dt {
  gap: 6px;
  color: var(--text-soft);
  font-size: 12px;
  font-weight: 650;
}

.audit-fields dd {
  min-width: 0;
  gap: 7px;
  margin: 4px 0 0;
  font-size: 13px;
}

.audit-fields dd span,
.audit-id span {
  overflow: hidden;
  min-width: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
}

.audit-copy {
  display: inline-grid;
  width: 24px;
  height: 24px;
  flex: 0 0 24px;
  place-items: center;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--surface);
  color: var(--text-soft);
  cursor: pointer;
}

.audit-copy:hover {
  color: var(--text);
  border-color: var(--border-strong);
}

.audit-id {
  gap: 7px;
  max-width: 680px;
  margin-top: 10px;
  color: var(--text-faint);
  font-size: 12px;
}

.audit-id small {
  color: var(--primary);
  font-family: inherit;
  font-weight: 650;
}

.audit-metadata {
  margin-top: 11px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--surface-subtle);
}

.audit-metadata summary {
  cursor: pointer;
  padding: 8px 10px;
  color: var(--text-soft);
  font-size: 13px;
  font-weight: 650;
}

.audit-metadata pre {
  overflow-x: auto;
  margin: 0;
  padding: 10px;
  border-top: 1px solid var(--border);
  color: var(--text);
  font-size: 12px;
  line-height: 1.55;
}

.audit-panel__footer {
  padding: 15px 16px;
  border-top: 1px solid var(--border);
}

.audit-panel__footer .button {
  width: 100%;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 1120px) {
  .audit-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .audit-fields {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 680px) {
  .audit-summary,
  .audit-fields {
    grid-template-columns: 1fr;
  }

  .audit-panel__heading {
    align-items: flex-start;
  }

  .audit-panel__heading .status-pill {
    display: none;
  }

  .audit-event {
    grid-template-columns: 28px minmax(0, 1fr);
    gap: 8px;
    padding: 0 12px;
  }

  .audit-event__topline {
    align-items: flex-start;
    flex-direction: column;
    gap: 8px;
  }

  .audit-event__meta {
    flex-wrap: wrap;
  }
}
</style>

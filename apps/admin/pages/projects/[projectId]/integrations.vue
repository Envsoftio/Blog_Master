<template>
  <div class="page-stack">
    <div class="page-heading">
      <div>
        <h2>Integrations</h2>
        <p>Signed delivery webhooks, endpoint health, retries, and landing application status.</p>
      </div>
      <button class="button button--primary button--compact" type="button" @click="formOpen = !formOpen">
        <Plus :size="16" />
        Add endpoint
      </button>
    </div>

    <div class="metric-grid">
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Endpoints</span><Webhook :size="17" /></div>
        <p class="metric-card__value">{{ endpoints.length }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Active</span><CircleCheck :size="17" /></div>
        <p class="metric-card__value">{{ endpoints.filter(endpoint => endpoint.status === 'active').length }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Delivery failures</span><CircleAlert :size="17" /></div>
        <p class="metric-card__value">{{ numericStatus('failures') }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Pending deliveries</span><Clock3 :size="17" /></div>
        <p class="metric-card__value">{{ numericStatus('pending') }}</p>
      </article>
    </div>

    <form v-if="formOpen" class="surface endpoint-form" @submit.prevent="createEndpoint">
      <div class="panel-heading">
        <span class="panel-heading__icon"><Webhook :size="18" /></span>
        <div><p>New integration</p><h3>Webhook endpoint</h3></div>
        <button class="icon-button" type="button" title="Close" aria-label="Close" @click="formOpen = false"><X :size="17" /></button>
      </div>
      <div class="endpoint-form__body">
        <label class="field">
          <span>Name</span>
          <input v-model.trim="form.name" required placeholder="Production landing app">
        </label>
        <label class="field">
          <span>Endpoint URL</span>
          <input v-model.trim="form.url" required type="url" placeholder="https://example.com/api/revalidate">
        </label>
        <fieldset>
          <legend>Events</legend>
          <label v-for="event in eventOptions" :key="event"><input v-model="form.events" type="checkbox" :value="event"><span>{{ labelize(event) }}</span></label>
        </fieldset>
      </div>
      <div class="form-footer">
        <span><ShieldCheck :size="15" />Deliveries are signed by the provider.</span>
        <button class="button button--primary" type="submit" :disabled="creating || !canCreate">
          <LoaderCircle v-if="creating" class="spin" :size="16" />
          <Plus v-else :size="16" />
          Create endpoint
        </button>
      </div>
    </form>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success">{{ successMessage }}</p>

    <section v-if="createdSecret" class="secret-panel surface" aria-live="polite">
      <span class="secret-panel__icon"><KeyRound :size="18" /></span>
      <div>
        <span>Signing secret · shown once</span>
        <code>{{ createdSecret }}</code>
      </div>
      <button class="button button--compact" type="button" @click="copySecret">
        <Check v-if="secretCopied" :size="15" />
        <Copy v-else :size="15" />
        {{ secretCopied ? 'Copied' : 'Copy' }}
      </button>
      <button class="icon-button" type="button" title="Dismiss secret" aria-label="Dismiss secret" @click="createdSecret = ''"><X :size="16" /></button>
    </section>

    <div v-if="pending" class="loading-surface surface"><LoaderCircle class="spin" :size="18" />Loading integrations</div>

    <div v-else-if="endpoints.length === 0" class="empty-state">
      <div>
        <span class="empty-state__icon"><Webhook :size="20" /></span>
        <h3>{{ serviceAvailable ? 'No webhook endpoints' : 'Integration service is not available' }}</h3>
        <p>{{ serviceAvailable ? 'Delivery endpoints will appear here.' : 'Webhook and delivery APIs have not been enabled on this backend.' }}</p>
      </div>
    </div>

    <div v-else class="endpoint-list surface">
      <article v-for="endpoint in endpoints" :key="endpoint.id" class="endpoint-item">
        <span class="endpoint-item__icon"><Webhook :size="17" /></span>
        <div class="endpoint-item__copy">
          <div><h3>{{ endpoint.name }}</h3><span class="status-pill" :class="{ 'status-pill--success': endpoint.status === 'active' }">{{ endpoint.status }}</span></div>
          <p>{{ endpoint.url }}</p>
          <small>{{ endpoint.events.map(labelize).join(', ') }}</small>
        </div>
        <div class="endpoint-item__time"><span>Last delivered</span><strong>{{ formatDate(endpoint.lastDeliveredAt) }}</strong></div>
        <button class="icon-button surface" type="button" title="Endpoint options" aria-label="Endpoint options"><Ellipsis :size="17" /></button>
      </article>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  CircleAlert,
  CircleCheck,
  Clock3,
  Check,
  Copy,
  Ellipsis,
  KeyRound,
  LoaderCircle,
  Plus,
  ShieldCheck,
  Webhook,
  X
} from 'lucide-vue-next'
import type { WebhookEndpoint } from '~/composables/useAdminApi'

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => String(route.params.projectId || ''))
const endpoints = ref<WebhookEndpoint[]>([])
const delivery = ref<Record<string, unknown>>({})
const pending = ref(true)
const creating = ref(false)
const formOpen = ref(false)
const serviceAvailable = ref(true)
const errorMessage = ref('')
const successMessage = ref('')
const createdSecret = ref('')
const secretCopied = ref(false)
const eventOptions = ['content.published', 'content.updated', 'content.unpublished', 'content.restored', 'content.slug_changed']
const form = reactive({ name: '', url: '', events: ['content.published', 'content.updated'] })
const canCreate = computed(() => form.name.length >= 2 && form.url.startsWith('http') && form.events.length > 0)

onMounted(loadIntegrations)

async function loadIntegrations() {
  pending.value = true
  errorMessage.value = ''
  const [webhooks, status] = await Promise.allSettled([
    api.listWebhooks(projectID.value),
    api.deliveryStatus(projectID.value)
  ])
  if (webhooks.status === 'fulfilled') {
    endpoints.value = webhooks.value.data
  } else if (apiStatus(webhooks.reason) !== 501) {
    errorMessage.value = normalizeAPIError(webhooks.reason, 'Could not load webhook endpoints.')
  }
  if (status.status === 'fulfilled') delivery.value = status.value.data
  serviceAvailable.value = webhooks.status === 'fulfilled' || status.status === 'fulfilled'
  pending.value = false
}

async function createEndpoint() {
  creating.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const response = await api.createWebhook(projectID.value, { ...form })
    endpoints.value = [response.data, ...endpoints.value]
    createdSecret.value = response.data.secret
    secretCopied.value = false
    form.name = ''
    form.url = ''
    form.events = ['content.published', 'content.updated']
    formOpen.value = false
    successMessage.value = 'Webhook endpoint created.'
  } catch (error) {
    errorMessage.value = apiStatus(error) === 501
      ? 'Webhook management is not enabled on this backend.'
      : normalizeAPIError(error, 'Could not create the endpoint.')
  } finally {
    creating.value = false
  }
}

async function copySecret() {
  if (!createdSecret.value || !import.meta.client) return
  await navigator.clipboard.writeText(createdSecret.value)
  secretCopied.value = true
}

function numericStatus(key: string) {
  const value = delivery.value[key]
  return typeof value === 'number' ? value : 0
}

function formatDate(value?: string) {
  if (!value) return 'Never'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? 'Unknown' : date.toLocaleString()
}
</script>

<style scoped>
.endpoint-form { overflow: hidden; }
.panel-heading { display: flex; align-items: center; gap: 11px; min-height: 66px; padding: 13px 16px; border-bottom: 1px solid var(--border); }
.panel-heading__icon { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 7px; background: var(--primary-soft); color: var(--primary); }
.panel-heading p, .panel-heading h3 { margin: 0; }
.panel-heading p { color: var(--text-soft); font-size: 9px; }
.panel-heading h3 { margin-top: 1px; font-size: 14px; }
.panel-heading .icon-button { margin-left: auto; }
.endpoint-form__body { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; padding: 18px; }
.endpoint-form fieldset { display: flex; grid-column: 1 / -1; flex-wrap: wrap; gap: 8px; margin: 0; padding: 0; border: 0; }
.endpoint-form legend { width: 100%; margin-bottom: 6px; font-size: 11px; font-weight: 600; }
.endpoint-form fieldset label { display: inline-flex; align-items: center; gap: 6px; padding: 7px 9px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface-subtle); font-size: 9px; text-transform: capitalize; cursor: pointer; }
.endpoint-form fieldset input { width: 14px; height: 14px; min-height: 0; }
.form-footer { display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 12px 18px; border-top: 1px solid var(--border); background: var(--surface-subtle); }
.form-footer > span { display: inline-flex; align-items: center; gap: 6px; color: var(--text-soft); font-size: 9px; }
.endpoint-list { overflow: hidden; }
.secret-panel { display: grid; grid-template-columns: 38px minmax(0, 1fr) auto 34px; align-items: center; gap: 11px; padding: 13px 14px; border-color: color-mix(in srgb, var(--amber) 35%, var(--border)); background: var(--amber-soft); }
.secret-panel__icon { display: grid; width: 38px; height: 38px; place-items: center; border-radius: 7px; background: var(--surface); color: var(--amber); }
.secret-panel > div { display: flex; min-width: 0; flex-direction: column; }
.secret-panel > div > span { color: var(--amber); font-size: 9px; font-weight: 650; }
.secret-panel code { overflow: hidden; margin-top: 3px; color: var(--text); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.endpoint-item { display: grid; grid-template-columns: 38px minmax(0, 1fr) 150px 34px; align-items: center; gap: 12px; padding: 14px 16px; border-bottom: 1px solid var(--border); }
.endpoint-item:last-child { border-bottom: 0; }
.endpoint-item__icon { display: grid; width: 38px; height: 38px; place-items: center; border-radius: 7px; background: var(--blue-soft); color: var(--blue); }
.endpoint-item__copy { min-width: 0; }
.endpoint-item__copy > div { display: flex; align-items: center; gap: 8px; }
.endpoint-item h3, .endpoint-item p, .endpoint-item small { overflow: hidden; margin: 0; text-overflow: ellipsis; white-space: nowrap; }
.endpoint-item h3 { font-size: 12px; }
.endpoint-item p { margin-top: 4px; color: var(--text-soft); font-size: 10px; }
.endpoint-item small { display: block; margin-top: 5px; color: var(--text-faint); font-size: 8px; text-transform: capitalize; }
.endpoint-item__time { display: flex; flex-direction: column; }
.endpoint-item__time span { color: var(--text-soft); font-size: 8px; }
.endpoint-item__time strong { margin-top: 3px; font-size: 10px; }
.loading-surface { display: flex; min-height: 130px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 760px) { .endpoint-form__body { grid-template-columns: 1fr; } .endpoint-form fieldset { grid-column: auto; } .endpoint-item { grid-template-columns: 38px minmax(0, 1fr) 34px; } .endpoint-item__time { display: none; } }
@media (max-width: 560px) { .form-footer { align-items: stretch; flex-direction: column; } .secret-panel { grid-template-columns: 38px minmax(0, 1fr) 34px; } .secret-panel > .button { grid-column: 2; justify-self: start; } }
</style>

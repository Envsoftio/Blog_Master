<template>
  <div class="page-stack">
    <div class="page-heading">
      <div>
        <p>Project images, documents, processing state, and accessibility metadata.</p>
      </div>
      <button class="button button--primary button--compact" type="button" @click="fileInput?.click()">
        <Upload :size="16" />
        Upload
      </button>
      <input ref="fileInput" class="sr-only" type="file" accept="image/*,.pdf" multiple @change="selectFiles">
    </div>

    <div class="metric-grid">
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Assets</span><Images :size="17" /></div>
        <p class="metric-card__value">{{ assets.length }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Ready</span><CircleCheck :size="17" /></div>
        <p class="metric-card__value">{{ assets.filter(asset => asset.status === 'ready').length }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Processing</span><RefreshCw :size="17" /></div>
        <p class="metric-card__value">{{ assets.filter(asset => asset.status === 'processing').length }}</p>
      </article>
      <article class="metric-card surface">
        <div class="metric-card__top"><span>Missing alt text</span><ScanText :size="17" /></div>
        <p class="metric-card__value">{{ assets.filter(asset => !asset.altText).length }}</p>
      </article>
    </div>

    <div v-if="selectedFiles.length" class="upload-queue surface">
      <div class="upload-queue__heading">
        <div>
          <span>Upload queue</span>
          <h3>{{ selectedFiles.length }} {{ selectedFiles.length === 1 ? 'file' : 'files' }}</h3>
        </div>
        <div class="upload-queue__actions">
          <button class="button button--compact" type="button" @click="selectedFiles = []">Clear</button>
          <button class="button button--primary button--compact" type="button" :disabled="uploading" @click="uploadFiles">
            <LoaderCircle v-if="uploading" class="spin" :size="15" />
            <Upload v-else :size="15" />
            Register files
          </button>
        </div>
      </div>
      <div class="upload-files">
        <div v-for="file in selectedFiles" :key="`${file.name}-${file.size}`" class="upload-file">
          <FileImage :size="17" />
          <span><strong>{{ file.name }}</strong><small>{{ formatBytes(file.size) }} · {{ file.type || 'Unknown type' }}</small></span>
        </div>
      </div>
    </div>

    <p v-if="errorMessage" class="ui-alert ui-alert--danger" role="alert">{{ errorMessage }}</p>
    <p v-if="successMessage" class="ui-alert ui-alert--success">{{ successMessage }}</p>

    <div class="library-toolbar surface surface--subtle">
      <label class="library-search">
        <Search :size="16" />
        <input v-model.trim="search" type="search" placeholder="Search media" aria-label="Search media">
      </label>
      <div class="library-actions">
        <select v-model="typeFilter" class="input" aria-label="Media type">
          <option value="all">All types</option>
          <option value="image">Images</option>
          <option value="document">Documents</option>
        </select>
        <div class="view-switcher" role="group" aria-label="Media view">
          <button type="button" title="Grid view" aria-label="Grid view" :class="{ 'is-active': view === 'grid' }" @click="view = 'grid'"><Grid2X2 :size="15" /></button>
          <button type="button" title="List view" aria-label="List view" :class="{ 'is-active': view === 'list' }" @click="view = 'list'"><List :size="15" /></button>
        </div>
      </div>
    </div>

    <div v-if="pending" class="loading-surface surface">
      <LoaderCircle class="spin" :size="18" />
      Loading media
    </div>

    <div v-else-if="filteredAssets.length === 0" class="empty-state">
      <div>
        <span class="empty-state__icon"><Images :size="20" /></span>
        <h3>{{ serviceAvailable ? 'No media yet' : 'Media service is not available' }}</h3>
        <p>{{ serviceAvailable ? 'Uploaded project assets will appear here.' : 'The media API has not been enabled on this backend.' }}</p>
        <button v-if="serviceAvailable" class="button button--primary" type="button" @click="fileInput?.click()">
          <Upload :size="16" />
          Upload media
        </button>
      </div>
    </div>

    <div v-else class="media-grid" :class="{ 'media-grid--list': view === 'list' }">
      <article v-for="asset in filteredAssets" :key="asset.id" class="media-card surface">
        <div class="media-card__preview">
          <img v-if="asset.url && asset.contentType.startsWith('image/')" :src="asset.url" :alt="asset.altText || ''">
          <FileImage v-else :size="28" />
        </div>
        <div class="media-card__body">
          <div>
            <h3>{{ asset.filename }}</h3>
            <p>{{ asset.width && asset.height ? `${asset.width} × ${asset.height}` : labelize(asset.contentType) }} · {{ formatBytes(asset.bytes || 0) }}</p>
          </div>
          <span class="status-pill" :class="{ 'status-pill--success': asset.status === 'ready' }">{{ asset.status }}</span>
        </div>
      </article>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  CircleCheck,
  FileImage,
  Grid2X2,
  Images,
  List,
  LoaderCircle,
  RefreshCw,
  ScanText,
  Search,
  Upload
} from 'lucide-vue-next'
import type { AdminMediaAsset } from '~/composables/useAdminApi'

const route = useRoute()
const api = useAdminApi()
const projectID = computed(() => String(route.params.projectId || ''))
const fileInput = ref<HTMLInputElement | null>(null)
const assets = ref<AdminMediaAsset[]>([])
const selectedFiles = ref<File[]>([])
const pending = ref(true)
const uploading = ref(false)
const serviceAvailable = ref(true)
const search = ref('')
const typeFilter = ref('all')
const view = ref<'grid' | 'list'>('grid')
const errorMessage = ref('')
const successMessage = ref('')

const filteredAssets = computed(() => {
  const term = search.value.toLowerCase()
  return assets.value.filter(asset => {
    const typeMatches = typeFilter.value === 'all'
      || (typeFilter.value === 'image' ? asset.contentType.startsWith('image/') : !asset.contentType.startsWith('image/'))
    return typeMatches && (!term || `${asset.filename} ${asset.altText || ''}`.toLowerCase().includes(term))
  })
})

onMounted(loadMedia)

async function loadMedia() {
  pending.value = true
  errorMessage.value = ''
  try {
    assets.value = (await api.listMedia(projectID.value)).data
    serviceAvailable.value = true
  } catch (error) {
    if (apiStatus(error) === 501) {
      serviceAvailable.value = false
      assets.value = []
    } else {
      errorMessage.value = normalizeAPIError(error, 'Could not load media.')
    }
  } finally {
    pending.value = false
  }
}

function selectFiles(event: Event) {
  const input = event.target as HTMLInputElement
  selectedFiles.value = Array.from(input.files || [])
  input.value = ''
}

async function uploadFiles() {
  uploading.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    for (const file of selectedFiles.value) {
      await api.initiateMediaUpload(projectID.value, {
        filename: file.name,
        contentType: file.type || 'application/octet-stream',
        bytes: file.size
      })
    }
    successMessage.value = `${selectedFiles.value.length} media record${selectedFiles.value.length === 1 ? '' : 's'} registered for object-storage upload.`
    selectedFiles.value = []
    await loadMedia()
  } catch (error) {
    errorMessage.value = apiStatus(error) === 501
      ? 'Media registration is not enabled on this backend.'
      : normalizeAPIError(error, 'Could not register the media.')
  } finally {
    uploading.value = false
  }
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / (1024 ** index)).toFixed(index ? 1 : 0)} ${units[index]}`
}
</script>

<style scoped>
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0, 0, 0, 0); }
.upload-queue { overflow: hidden; }
.upload-queue__heading { display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 14px 16px; border-bottom: 1px solid var(--border); }
.upload-queue__heading span { color: var(--text-soft); font-size: 10px; }
.upload-queue__heading h3 { margin: 1px 0 0; font-size: 14px; }
.upload-queue__actions { display: flex; gap: 7px; }
.upload-files { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 1px; background: var(--border); }
.upload-file { display: flex; min-width: 0; align-items: center; gap: 9px; padding: 11px 14px; background: var(--surface); color: var(--text-soft); }
.upload-file span { display: flex; min-width: 0; flex-direction: column; }
.upload-file strong, .upload-file small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.upload-file strong { color: var(--text); font-size: 10px; }
.upload-file small { margin-top: 2px; font-size: 8px; }
.library-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px; }
.library-search { display: flex; width: min(320px, 100%); align-items: center; gap: 8px; padding: 0 10px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--text-soft); }
.library-search input { width: 100%; min-height: 34px; padding: 0; border: 0 !important; box-shadow: none !important; background: transparent !important; font-size: 11px; }
.library-actions { display: flex; align-items: center; gap: 7px; }
.library-actions .input { width: 130px; min-height: 34px; padding-block: 5px; font-size: 11px; }
.view-switcher { display: inline-flex; gap: 2px; padding: 3px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); }
.view-switcher button { display: grid; width: 28px; height: 27px; place-items: center; border: 0; border-radius: 4px; background: transparent; color: var(--text-soft); cursor: pointer; }
.view-switcher button.is-active { background: var(--surface-subtle); color: var(--text); }
.media-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.media-card { overflow: hidden; }
.media-card__preview { display: grid; aspect-ratio: 4 / 3; place-items: center; overflow: hidden; border-bottom: 1px solid var(--border); background: var(--surface-subtle); color: var(--text-faint); }
.media-card__preview img { width: 100%; height: 100%; object-fit: cover; }
.media-card__body { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; padding: 11px; }
.media-card__body div { min-width: 0; }
.media-card__body h3, .media-card__body p { overflow: hidden; margin: 0; text-overflow: ellipsis; white-space: nowrap; }
.media-card__body h3 { font-size: 11px; }
.media-card__body p { margin-top: 3px; color: var(--text-soft); font-size: 8px; }
.media-grid--list { grid-template-columns: 1fr; }
.media-grid--list .media-card { display: grid; grid-template-columns: 84px minmax(0, 1fr); }
.media-grid--list .media-card__preview { aspect-ratio: 4 / 3; border-right: 1px solid var(--border); border-bottom: 0; }
.media-grid--list .media-card__body { align-items: center; }
.loading-surface { display: flex; min-height: 130px; align-items: center; justify-content: center; gap: 9px; color: var(--text-soft); }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1050px) { .media-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 760px) { .upload-files { grid-template-columns: 1fr; } .media-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 580px) { .library-toolbar { align-items: stretch; flex-direction: column; } .library-search { width: 100%; } .library-actions { justify-content: space-between; } .media-grid { grid-template-columns: 1fr; } }
</style>

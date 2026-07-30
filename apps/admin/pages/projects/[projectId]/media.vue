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
      <input ref="fileInput" class="sr-only" type="file" :accept="acceptedUploadTypes" multiple @change="selectFiles">
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
            Upload files
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
          <img v-if="mediaPreviewURL(asset)" :src="mediaPreviewURL(asset)" :alt="asset.altText || ''">
          <FileImage v-else :size="28" />
        </div>
        <div class="media-card__body">
          <div>
            <h3>{{ asset.filename }}</h3>
            <p>{{ asset.width && asset.height ? `${asset.width} × ${asset.height}` : labelize(asset.contentType) }} · {{ formatBytes(asset.bytes || 0) }}</p>
          </div>
          <div class="media-card__state">
            <span class="status-pill" :class="mediaStatusClass(asset.status)">{{ labelize(asset.status) }}</span>
            <button
              v-if="canDeleteMediaAsset(asset)"
              class="icon-button media-card__delete"
              type="button"
              :title="mediaDeleteLabel(asset)"
              :aria-label="mediaDeleteLabel(asset)"
              :disabled="deletingAssetIDs.includes(asset.id)"
              @click="deleteMediaAsset(asset)"
            >
              <LoaderCircle v-if="deletingAssetIDs.includes(asset.id)" class="spin" :size="14" />
              <Trash2 v-else :size="14" />
            </button>
          </div>
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
  Trash2,
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
const deletingAssetIDs = ref<string[]>([])
const localPreviewURLs = ref<Record<string, string>>({})
const acceptedUploadTypes = '.jpg,.jpeg,.png,.webp,.gif,.pdf,image/jpeg,image/png,image/webp,image/gif,application/pdf'
const mediaTypeByExtension: Record<string, string> = {
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  png: 'image/png',
  webp: 'image/webp',
  gif: 'image/gif',
  pdf: 'application/pdf'
}

const filteredAssets = computed(() => {
  const term = search.value.toLowerCase()
  return assets.value.filter(asset => {
    const typeMatches = typeFilter.value === 'all'
      || (typeFilter.value === 'image' ? asset.contentType.startsWith('image/') : !asset.contentType.startsWith('image/'))
    return typeMatches && (!term || `${asset.filename} ${asset.altText || ''}`.toLowerCase().includes(term))
  })
})

onMounted(loadMedia)
onBeforeUnmount(revokeLocalPreviews)

async function loadMedia() {
  pending.value = true
  errorMessage.value = ''
  try {
    const loadedAssets = (await api.listMedia(projectID.value)).data
    assets.value = loadedAssets
    for (const asset of loadedAssets) {
      if (asset.url) {
        revokeLocalPreview(asset.id)
      }
    }
    for (const assetID of Object.keys(localPreviewURLs.value)) {
      if (!loadedAssets.some(asset => asset.id === assetID)) {
        revokeLocalPreview(assetID)
      }
    }
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
  const files = Array.from(input.files || [])
  selectedFiles.value = files.filter(file => Boolean(mediaContentType(file)))
  if (selectedFiles.value.length !== files.length) {
    errorMessage.value = 'Some selected files are not supported for media registration.'
  } else {
    errorMessage.value = ''
  }
  input.value = ''
}

async function uploadFiles() {
  uploading.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    let uploadedCount = 0
    let registeredCount = 0
    for (const file of selectedFiles.value) {
      let initiatedAsset: AdminMediaAsset | null = null
      try {
        const initiated = await api.initiateMediaUpload(projectID.value, {
          filename: file.name,
          contentType: mediaContentType(file) || 'application/octet-stream',
          bytes: file.size
        })
        initiatedAsset = initiated.data
        rememberLocalPreview(initiated.data, file)
        upsertMediaAsset(initiated.data)
        if (initiated.data.upload) {
          const sha256 = await fileSHA256(file)
          await uploadToSignedTarget(file, initiated.data.upload)
          const completed = await api.completeMediaUpload(projectID.value, initiated.data.id, { sha256 })
          upsertMediaAsset(completed.data)
          uploadedCount++
        } else {
          registeredCount++
        }
      } catch (error) {
        if (initiatedAsset) {
          await cleanupFailedUpload(initiatedAsset)
        }
        throw error
      }
    }
    successMessage.value = uploadedCount
      ? `${uploadedCount} media file${uploadedCount === 1 ? '' : 's'} uploaded and queued for scanning.`
      : `${registeredCount} media record${registeredCount === 1 ? '' : 's'} registered for object-storage upload.`
    selectedFiles.value = []
    await loadMedia()
  } catch (error) {
    errorMessage.value = apiStatus(error) === 501
      ? 'Media registration is not enabled on this backend.'
      : normalizeAPIError(error, 'Could not upload the media.')
  } finally {
    uploading.value = false
  }
}

async function uploadToSignedTarget(file: File, upload: NonNullable<AdminMediaAsset['upload']>) {
  if (upload.fields && Object.keys(upload.fields).length > 0) {
    const form = new FormData()
    for (const [key, value] of Object.entries(upload.fields)) {
      form.append(key, value)
    }
    form.append('file', file)
    const response = await fetch(upload.url, {
      method: upload.method || 'POST',
      body: form
    })
    if (!response.ok) {
      throw new Error(`Object storage upload failed with status ${response.status}.`)
    }
    return
  }
  const response = await fetch(upload.url, {
    method: upload.method || 'PUT',
    headers: upload.headers,
    body: file
  })
  if (!response.ok) {
    throw new Error(`Object storage upload failed with status ${response.status}.`)
  }
}

async function cleanupFailedUpload(asset: AdminMediaAsset) {
  try {
    await api.deleteMedia(projectID.value, asset.id)
    assets.value = assets.value.filter(item => item.id !== asset.id)
    revokeLocalPreview(asset.id)
  } catch {
    // Keep the original upload error visible; the stale row can still be deleted manually.
  }
}

function mediaPreviewURL(asset: AdminMediaAsset) {
  if (!asset.contentType.startsWith('image/')) return ''
  return asset.url || localPreviewURLs.value[asset.id] || ''
}

function rememberLocalPreview(asset: AdminMediaAsset, file: File) {
  if (!asset.contentType.startsWith('image/')) return
  revokeLocalPreview(asset.id)
  localPreviewURLs.value = {
    ...localPreviewURLs.value,
    [asset.id]: URL.createObjectURL(file)
  }
}

function revokeLocalPreview(assetID: string) {
  const previewURL = localPreviewURLs.value[assetID]
  if (!previewURL) return
  URL.revokeObjectURL(previewURL)
  const { [assetID]: _removed, ...remaining } = localPreviewURLs.value
  localPreviewURLs.value = remaining
}

function revokeLocalPreviews() {
  for (const previewURL of Object.values(localPreviewURLs.value)) {
    URL.revokeObjectURL(previewURL)
  }
  localPreviewURLs.value = {}
}

function upsertMediaAsset(asset: AdminMediaAsset) {
  const index = assets.value.findIndex(item => item.id === asset.id)
  if (index === -1) {
    assets.value = [asset, ...assets.value]
    return
  }
  assets.value = assets.value.map(item => item.id === asset.id ? asset : item)
}

function canDeleteMediaAsset(asset: AdminMediaAsset) {
  return ['registered', 'uploading', 'processing', 'failed', 'rejected'].includes(asset.status)
}

function mediaDeleteLabel(asset: AdminMediaAsset) {
  if (asset.status === 'uploading' || asset.status === 'registered') {
    return `Delete unfinished upload for ${asset.filename}`
  }
  if (asset.status === 'processing') {
    return `Delete processing media ${asset.filename}`
  }
  return `Delete failed media ${asset.filename}`
}

async function deleteMediaAsset(asset: AdminMediaAsset) {
  if (!canDeleteMediaAsset(asset) || deletingAssetIDs.value.includes(asset.id)) return
  if (!window.confirm(`Delete ${asset.filename}?`)) return
  deletingAssetIDs.value = [...deletingAssetIDs.value, asset.id]
  errorMessage.value = ''
  successMessage.value = ''
  try {
    await api.deleteMedia(projectID.value, asset.id)
    assets.value = assets.value.filter(item => item.id !== asset.id)
    revokeLocalPreview(asset.id)
    successMessage.value = `${asset.filename} deleted.`
  } catch (error) {
    errorMessage.value = normalizeAPIError(error, 'Could not delete the media.')
  } finally {
    deletingAssetIDs.value = deletingAssetIDs.value.filter(id => id !== asset.id)
  }
}

async function fileSHA256(file: File) {
  if (!globalThis.crypto?.subtle) return ''
  const digest = await globalThis.crypto.subtle.digest('SHA-256', await file.arrayBuffer())
  return Array.from(new Uint8Array(digest)).map(byte => byte.toString(16).padStart(2, '0')).join('')
}

function mediaContentType(file: File) {
  const extension = file.name.split('.').pop()?.toLowerCase() || ''
  const expectedType = mediaTypeByExtension[extension]
  if (!expectedType) return ''
  const browserType = normalizeMediaType(file.type)
  if (!browserType) return expectedType
  return browserType === expectedType ? browserType : ''
}

function normalizeMediaType(value: string) {
  const mediaType = value.split(';')[0]?.trim().toLowerCase() || ''
  return mediaType === 'image/jpg' ? 'image/jpeg' : mediaType
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / (1024 ** index)).toFixed(index ? 1 : 0)} ${units[index]}`
}

function mediaStatusClass(status: string) {
  return {
    'status-pill--success': status === 'ready',
    'status-pill--warning': status === 'processing' || status === 'uploading',
    'status-pill--danger': status === 'failed' || status === 'rejected'
  }
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
.media-card__state { display: flex; flex: 0 0 auto; align-items: center; gap: 6px; }
.media-card__delete { width: 28px; height: 28px; flex-basis: 28px; color: var(--danger); }
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

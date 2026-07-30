export type APIEnvelope<T> = {
  data: T
}

export type APIListEnvelope<T> = {
  data: T[]
  meta?: {
    projectId?: string
    nextCursor?: string
    limit: number
    openCount?: number
  }
}

type NullableAPIListEnvelope<T> = Omit<APIListEnvelope<T>, 'data'> & {
  data?: T[] | null
}

export type AdminProject = {
  id: string
  slug: string
  name: string
  status: string
  publicProjectKey?: string
  primaryDomain?: string
  blogBasePath: string
  defaultLocale: string
  supportedLocales?: string[]
  timezone?: string
  role: string
  publisherName?: string
  publisherUrl?: string
  defaultRobotsPolicy?: string
  createdAt?: string
  updatedAt?: string
}

export type AdminUser = {
  id: string
  email: string
  status: string
  createdAt?: string
  lastSeenAt?: string
}

export type AdminRevision = {
  id: string
  articleId: string
  revisionNumber: number
  title: string
  deck?: string
  excerpt?: string
  shortAnswer?: string
  locale: string
  editorialState: string
  contentHash?: string
  createdAt?: string
}

export type AdminArticle = {
  id: string
  projectId: string
  articleType: string
  slug: string
  locale: string
  title: string
  editorialState: string
  publicationState: string
  canonicalPolicy?: string
  scheduledForUtc?: string
  publishedAt?: string
  canonicalUrl?: string
  latestRevision?: AdminRevision
  createdAt: string
}

export type TaxonomyTerm = {
  id: string
  type: string
  slug: string
  name: string
  description?: string
  parentId?: string
  indexable: boolean
}

export type AdminProjectMember = {
  projectId: string
  userId: string
  email: string
  role: string
  status: string
  invitedBy?: string
  invitedAt?: string
  joinedAt?: string
  updatedAt?: string
  removedAt?: string
}

export type AdminAuthor = {
  id: string
  slug: string
  displayName: string
  shortBio?: string
  fullBio?: string
  photoAssetId?: string
  jobTitle?: string
  organization?: string
  credentials?: string[]
  expertise?: string[]
  profileUrl?: string
  externalProfiles?: string[]
  sameAs?: string[]
  loginUserId?: string
  loginEmail?: string
  loginRole?: string
  loginStatus?: string
  status?: string
  createdAt?: string
  updatedAt?: string
}

export type AdminSeries = {
  id: string
  slug: string
  name: string
  description?: string
  indexable: boolean
}

export type AuthorPayload = {
  slug?: string
  displayName?: string
  shortBio?: string
  fullBio?: string
  photoAssetId?: string
  jobTitle?: string
  organization?: string
  credentials?: string[]
  expertise?: string[]
  profileUrl?: string
  externalProfiles?: string[]
  sameAs?: string[]
  loginUserId?: string
  status?: string
}

export type AdminAPIKey = {
  id: string
  projectId: string
  environment: string
  name: string
  tokenPrefix: string
  scopes: string[]
  expiresAt?: string
  lastUsedAt?: string
  createdBy: string
  createdAt: string
  revokedAt?: string
}

export type ReviewComment = {
  id: string
  projectId: string
  articleId: string
  revisionId: string
  blockId?: string
  body: string
  status: string
  createdBy: string
  createdAt: string
  resolvedBy?: string
  resolvedAt?: string
}

export type ReviewAssignment = {
  id: string
  projectId: string
  articleId: string
  revisionId?: string
  assignedTo: string
  assigneeEmail?: string
  assigneeRole?: string
  assignmentType: string
  dueAt?: string
  status: string
  createdBy: string
  createdAt: string
  closedBy?: string
  closedAt?: string
}

export type AuditEvent = {
  id: string
  projectId?: string
  actorType: string
  actorId?: string
  action: string
  targetType?: string
  targetId?: string
  outcome: string
  requestId?: string
  metadata?: Record<string, unknown>
  createdAt: string
}

export type ProjectDeletionImpact = {
  projectId: string
  canDelete: boolean
  activeApiKeys: number
  activeMembers: number
  contentItems: number
  publishedPublications: number
  scheduledPublications: number
  redirects: number
  assets: number
  webhooks: number
  pendingJobs: number
}

export type MediaUploadTarget = {
  url: string
  method: string
  headers: Record<string, string>
  fields?: Record<string, string>
  expiresAt: string
  maxBytes: number
}

export type AdminMediaVariant = {
  id: string
  name: string
  objectKey: string
  contentType: string
  width?: number
  height?: number
  bytes: number
  url?: string
  createdAt?: string
}

export type AdminMediaAsset = {
  id: string
  projectId: string
  filename: string
  contentType: string
  status: string
  width?: number
  height?: number
  bytes?: number
  altText?: string
  decorative: boolean
  caption?: string
  credit?: string
  license?: string
  objectKey: string
  bucket?: string
  sha256?: string
  scanStatus?: string
  scanReason?: string
  metadata?: Record<string, unknown>
  variants?: AdminMediaVariant[]
  upload?: MediaUploadTarget
  createdAt?: string
  updatedAt?: string
  url?: string
}

export type WebhookEndpoint = {
  id: string
  projectId: string
  name: string
  url: string
  events: string[]
  status: string
  createdAt?: string
  lastDeliveredAt?: string
}

export type WebhookWithSecret = WebhookEndpoint & {
  secret: string
}

export type WebhookAttempt = {
  id: string
  projectId: string
  endpointId: string
  endpointName: string
  outboxEventId: string
  eventType: string
  aggregateType: string
  aggregateId: string
  status: string
  statusCode?: number
  errorCategory?: string
  attemptCount: number
  maxAttempts: number
  nextAttemptAt?: string
  responseDurationMillis?: number
  lastErrorSafeMessage?: string
  completedAt?: string
  replayOfAttemptId?: string
  attemptedAt: string
}

export type AIJob = {
  id: string
  projectId: string
  contentId?: string
  revisionId?: string
  type: string
  articleType?: string
  status: string
  promptTemplateVersion?: string
  voiceProfileId?: string
  voiceProfileVersion?: number
  evidencePacketId?: string
  evidencePacketVersion?: number
  inputHash?: string
  sourceRevisionHash?: string
  createdAt?: string
  updatedAt?: string
  result?: unknown
  error?: string
  reused?: boolean
}

export type AIJobEvent = {
  id: string
  projectId: string
  jobId: string
  sequence: number
  type: string
  status: string
  progress: number
  message?: string
  metadata: Record<string, unknown>
  createdAt: string
}

export type AIRun = {
  id: string
  projectId: string
  contentId?: string
  revisionId?: string
  jobId?: string
  type: string
  provider: string
  modelIdentifier: string
  promptTemplateVersion: string
  sourceIds: string[]
  startedAt: string
  completedAt?: string
  status: string
  inputTokens?: number
  outputTokens?: number
  estimatedCostCents?: number
  errorCategory?: string
}

export type QualityCheckResult = {
  id: string
  projectId: string
  contentId?: string
  revisionId?: string
  checkType: string
  severity: 'info' | 'warning' | 'blocking' | 'critical'
  status: 'passed' | 'failed' | 'overridden'
  message: string
  evidence: Record<string, unknown>
  overrideReason?: string
  createdAt: string
}

export type VoiceWritingExample = {
  title: string
  excerpt: string
}

export type VoiceProfileDocument = {
  audience: string
  assumedKnowledge: string
  brandPurpose: string
  pointOfView: string
  tone: string
  formality: string
  humor: string
  preferredVocabulary: string[]
  productTerminology: Record<string, string>
  approvedProductFacts: string[]
  sentencePreferences: string
  paragraphPreferences: string
  avoidPhrases: string[]
  prohibitedClaims: string[]
  contentTypeStyles: Record<string, string>
  writingExamples: VoiceWritingExample[]
  introductionRules: string
  conclusionRules: string
  callToActionRules: string
  regionalSpelling: string
  locale: string
}

export type VoiceProfile = {
  id: string
  projectId: string
  version: number
  profile: VoiceProfileDocument
  createdBy: string
  createdAt: string
}

export type EvidenceFact = {
  statement: string
  sourceIds: string[]
}

export type EvidencePacketDocument = {
  humanBrief: string
  searchIntent: string
  thesis: string
  productFacts: EvidenceFact[]
  subjectMatterNotes: string[]
  firsthandObservations: string[]
  sourceIds: string[]
  customerEvidence: string[]
  measurements: string[]
  allowedClaims: string[]
  prohibitedClaims: string[]
  limitations: string[]
  requiredInternalLinks: string[]
  callToAction: string
  publicationRecommendation: 'ready' | 'request_unique_evidence' | 'do_not_publish'
}

export type EvidencePacket = {
  id: string
  projectId: string
  contentId?: string
  version: number
  packet: EvidencePacketDocument
  approvedBy?: string
  approvedAt?: string
  createdBy: string
  createdAt: string
}

export type AdminSource = {
  id: string
  projectId: string
  title: string
  publisher?: string
  author?: string
  url?: string
  sourceType: string
  isPrimary: boolean
  createdAt: string
}

export type ProjectCreatePayload = {
  name: string
  slug: string
  primaryDomain?: string
  blogBasePath: string
  defaultLocale: string
  supportedLocales: string[]
  timezone: string
}

export type CategoryCreatePayload = {
  name: string
  slug: string
  description?: string
  indexable: boolean
}

export type TaxonomyCreatePayload = CategoryCreatePayload & {
  parentId?: string
}

export type ProjectUpdatePayload = Partial<{
  name: string
  primaryDomain: string
  verifiedDomains: string[]
  blogBasePath: string
  defaultLocale: string
  supportedLocales: string[]
  timezone: string
  publisherName: string
  publisherUrl: string
  defaultRobotsPolicy: string
}>

export type ArticleCreatePayload = {
  articleType: string
  title: string
  slug: string
  locale?: string
  primaryCategoryId: string
  deck?: string
  excerpt?: string
  shortAnswer?: string
  bodyDocument?: unknown
  html?: string
}

export const ARTICLE_TYPES = [
  'standard',
  'guide',
  'tutorial',
  'comparison',
  'case_study',
  'research',
  'listicle',
  'news_update',
  'opinion',
  'reference',
  'glossary',
  'release_note'
] as const

type AdminFetchOptions = Parameters<typeof $fetch>[1]

export function apiListData<T>(response: NullableAPIListEnvelope<T> | null | undefined): T[] {
  return Array.isArray(response?.data) ? response.data : []
}

function normalizeAPIListEnvelope<T>(response: NullableAPIListEnvelope<T> | null | undefined): APIListEnvelope<T> {
  return {
    ...(response || {}),
    data: apiListData(response)
  }
}

export function useAdminProjectsState() {
  const state = useState<AdminProject[] | null>('admin-projects', () => [])
  return computed<AdminProject[]>({
    get: () => apiListData({ data: state.value }),
    set: value => {
      state.value = apiListData({ data: value })
    }
  })
}

export function useAdminApi() {
  async function request<T>(path: string, options?: AdminFetchOptions) {
    return await $fetch<T>(path, {
      credentials: 'include',
      ...options
    })
  }

  async function getCSRFToken() {
    const response = await request<APIEnvelope<{ csrfToken: string }>>('/api/v1/auth/csrf')
    return response.data.csrfToken
  }

  async function withCSRF(options: AdminFetchOptions = {}) {
    const csrfToken = await getCSRFToken()
    return {
      ...options,
      headers: {
        ...(options.headers || {}),
        'X-CSRF-Token': csrfToken
      }
    } satisfies AdminFetchOptions
  }

  async function login(email: string, password: string) {
    return await request<APIEnvelope<{ user: AdminUser, csrfToken: string }>>('/api/v1/auth/login', {
      method: 'POST',
      body: { email, password }
    })
  }

  async function currentUser() {
    return await request<APIEnvelope<AdminUser>>('/api/v1/auth/me')
  }

  async function reauthenticate(password: string) {
    return await request<APIEnvelope<{ validUntil: string }>>('/api/v1/auth/reauthenticate', await withCSRF({
      method: 'POST',
      body: { password }
    }))
  }

  async function forgotPassword(email: string) {
    return await request<APIEnvelope<Record<string, never>>>('/api/v1/auth/forgot-password', {
      method: 'POST',
      body: { email }
    })
  }

  async function resetPassword(token: string, password: string) {
    return await request<APIEnvelope<Record<string, never>>>('/api/v1/auth/reset-password', {
      method: 'POST',
      body: { token, password }
    })
  }

  async function logout() {
    await request('/api/v1/auth/logout', await withCSRF({ method: 'POST' }))
  }

  async function listProjects(limit = 100) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminProject>>('/api/v1/projects', {
      query: { limit }
    }))
  }

  async function getProject(projectID: string) {
    return await request<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID}`)
  }

  async function createProject(payload: ProjectCreatePayload) {
    return await request<APIEnvelope<AdminProject>>('/api/v1/projects', await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function updateProject(projectID: string, payload: ProjectUpdatePayload) {
    return await request<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID}`, await withCSRF({
      method: 'PATCH',
      body: payload
    }))
  }

  async function projectAction(projectID: string, action: 'suspend' | 'archive') {
    return await request<APIEnvelope<AdminProject>>(`/api/v1/projects/${projectID}/${action}`, await withCSRF({
      method: 'POST'
    }))
  }

  async function projectDeletionImpact(projectID: string) {
    return await request<APIEnvelope<ProjectDeletionImpact>>(`/api/v1/projects/${projectID}/deletion-impact`)
  }

  async function listMembers(projectID: string) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminProjectMember>>(`/api/v1/projects/${projectID}/members`))
  }

  async function inviteMember(projectID: string, payload: { email: string, role: string, expiresAt?: string }) {
    return await request<APIEnvelope<{ member: AdminProjectMember, token: string, expiresAt: string }>>(
      `/api/v1/projects/${projectID}/invitations`,
      await withCSRF({ method: 'POST', body: payload })
    )
  }

  async function updateMember(projectID: string, userID: string, role: string) {
    return await request<APIEnvelope<AdminProjectMember>>(`/api/v1/projects/${projectID}/members/${userID}`, await withCSRF({
      method: 'PATCH',
      body: { role }
    }))
  }

  async function removeMember(projectID: string, userID: string) {
    return await request(`/api/v1/projects/${projectID}/members/${userID}`, await withCSRF({ method: 'DELETE' }))
  }

  async function listAPIKeys(projectID: string) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminAPIKey>>(`/api/v1/projects/${projectID}/api-keys`))
  }

  async function createAPIKey(projectID: string, payload: { environment: string, name: string, scopes: string[], expiresAt?: string }) {
    return await request<APIEnvelope<{ key: AdminAPIKey, secret: string }>>(`/api/v1/projects/${projectID}/api-keys`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function mutateAPIKey(projectID: string, keyID: string, action: 'rotate' | 'revoke') {
    return await request<APIEnvelope<AdminAPIKey | { key: AdminAPIKey, secret: string }>>(
      `/api/v1/projects/${projectID}/api-keys/${keyID}/${action}`,
      await withCSRF({ method: 'POST' })
    )
  }

  async function listTaxonomy(projectID: string, type: 'categories' | 'tags') {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID}/${type}`))
  }

  async function createTaxonomy(projectID: string, type: 'categories' | 'tags', payload: TaxonomyCreatePayload) {
    return await request<APIEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID}/${type}`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function listCategories(projectID: string) {
    return await listTaxonomy(projectID, 'categories')
  }

  async function createCategory(projectID: string, payload: CategoryCreatePayload) {
    return await createTaxonomy(projectID, 'categories', payload)
  }

  async function listAuthors(projectID: string) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminAuthor>>(`/api/v1/projects/${projectID}/authors`))
  }

  async function getAuthor(projectID: string, authorID: string) {
    return await request<APIEnvelope<AdminAuthor>>(`/api/v1/projects/${projectID}/authors/${authorID}`)
  }

  async function createAuthor(projectID: string, payload: AuthorPayload) {
    return await request<APIEnvelope<AdminAuthor>>(`/api/v1/projects/${projectID}/authors`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function updateAuthor(projectID: string, authorID: string, payload: AuthorPayload) {
    return await request<APIEnvelope<AdminAuthor>>(`/api/v1/projects/${projectID}/authors/${authorID}`, await withCSRF({
      method: 'PATCH',
      body: payload
    }))
  }

  async function deleteAuthor(projectID: string, authorID: string) {
    return await request<APIEnvelope<AdminAuthor>>(`/api/v1/projects/${projectID}/authors/${authorID}`, await withCSRF({
      method: 'DELETE'
    }))
  }

  async function listSeries(projectID: string) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminSeries>>(`/api/v1/projects/${projectID}/series`))
  }

  async function listArticles(projectID: string, limit = 100) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminArticle>>(`/api/v1/projects/${projectID}/articles`, {
      query: { limit }
    }))
  }

  async function createArticle(projectID: string, payload: ArticleCreatePayload) {
    return await request<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID}/articles`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function getArticle(projectID: string, articleID: string) {
    return await request<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID}/articles/${articleID}`)
  }

  async function listRevisions(projectID: string, articleID: string, limit = 100) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminRevision>>(`/api/v1/projects/${projectID}/articles/${articleID}/revisions`, {
      query: { limit }
    }))
  }

  async function articleAction(
    projectID: string,
    articleID: string,
    action: 'publish' | 'schedule' | 'unpublish' | 'rollback',
    body: Record<string, unknown> = {}
  ) {
    return await request<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID}/articles/${articleID}/${action}`, await withCSRF({
      method: 'POST',
      body
    }))
  }

  async function revisionAction(projectID: string, revisionID: string, action: 'submit' | 'request-changes' | 'approve', body: Record<string, unknown> = {}) {
    return await request<APIEnvelope<AdminRevision>>(`/api/v1/projects/${projectID}/revisions/${revisionID}/${action}`, await withCSRF({
      method: 'POST',
      body
    }))
  }

  async function listComments(projectID: string, articleID: string) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<ReviewComment>>(`/api/v1/projects/${projectID}/articles/${articleID}/comments`))
  }

  async function createComment(projectID: string, articleID: string, payload: { revisionId: string, blockId?: string, body: string }) {
    return await request<APIEnvelope<ReviewComment>>(`/api/v1/projects/${projectID}/articles/${articleID}/comments`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function mutateComment(projectID: string, commentID: string, action: 'resolve' | 'reopen') {
    return await request<APIEnvelope<ReviewComment>>(`/api/v1/projects/${projectID}/comments/${commentID}/${action}`, await withCSRF({
      method: 'POST'
    }))
  }

  async function listReviewAssignees(projectID: string, cursor = '', limit = 100) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminProjectMember>>(`/api/v1/projects/${projectID}/review-assignees`, {
      query: { limit, ...(cursor ? { cursor } : {}) }
    }))
  }

  async function listReviewAssignments(projectID: string, articleID: string, cursor = '', limit = 50) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<ReviewAssignment>>(`/api/v1/projects/${projectID}/articles/${articleID}/assignments`, {
      query: { limit, ...(cursor ? { cursor } : {}) }
    }))
  }

  async function createReviewAssignment(projectID: string, articleID: string, payload: { revisionId?: string, assignedTo: string, assignmentType: string, dueAt?: string }) {
    return await request<APIEnvelope<ReviewAssignment>>(`/api/v1/projects/${projectID}/articles/${articleID}/assignments`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function mutateReviewAssignment(projectID: string, assignmentID: string, action: 'complete' | 'cancel') {
    return await request<APIEnvelope<ReviewAssignment>>(`/api/v1/projects/${projectID}/assignments/${assignmentID}/${action}`, await withCSRF({
      method: 'POST',
      body: {}
    }))
  }

  async function listAuditEvents(projectID: string, limit = 100) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AuditEvent>>(`/api/v1/projects/${projectID}/audit-events`, {
      query: { limit }
    }))
  }

  async function listMedia(projectID: string) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminMediaAsset>>(`/api/v1/projects/${projectID}/media`))
  }

  async function initiateMediaUpload(projectID: string, payload: { filename: string, contentType: string, bytes: number }) {
    return await request<APIEnvelope<AdminMediaAsset>>(`/api/v1/projects/${projectID}/media/uploads`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function completeMediaUpload(projectID: string, assetID: string, payload: { sha256?: string } = {}) {
    return await request<APIEnvelope<AdminMediaAsset>>(`/api/v1/projects/${projectID}/media/${assetID}/complete`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function deleteMedia(projectID: string, assetID: string) {
    await request(`/api/v1/projects/${projectID}/media/${assetID}`, await withCSRF({
      method: 'DELETE',
      body: {}
    }))
  }

  async function listWebhooks(projectID: string) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<WebhookEndpoint>>(`/api/v1/projects/${projectID}/webhooks`))
  }

  async function createWebhook(projectID: string, payload: { name: string, url: string, events: string[] }) {
    return await request<APIEnvelope<WebhookWithSecret>>(`/api/v1/projects/${projectID}/webhooks`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function revokeWebhook(projectID: string, endpointID: string) {
    return await request<APIEnvelope<WebhookEndpoint>>(`/api/v1/projects/${projectID}/webhooks/${endpointID}/revoke`, await withCSRF({
      method: 'POST',
      body: {}
    }))
  }

  async function listWebhookAttempts(projectID: string, cursor = '', limit = 25) {
    const query = new URLSearchParams({ limit: String(limit) })
    if (cursor) query.set('cursor', cursor)
    return normalizeAPIListEnvelope(await request<APIListEnvelope<WebhookAttempt>>(`/api/v1/projects/${projectID}/webhook-attempts?${query}`))
  }

  async function replayWebhookAttempt(projectID: string, attemptID: string) {
    return await request<APIEnvelope<WebhookAttempt>>(`/api/v1/projects/${projectID}/webhook-attempts/${attemptID}/replay`, await withCSRF({
      method: 'POST',
      body: {}
    }))
  }

  async function deliveryStatus(projectID: string) {
    return await request<APIEnvelope<Record<string, unknown>>>(`/api/v1/projects/${projectID}/delivery/status`)
  }

  async function listAIJobs(projectID: string) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AIJob>>(`/api/v1/projects/${projectID}/ai/jobs`))
  }

  async function createAIJob(projectID: string, payload: Record<string, unknown>) {
    return await request<APIEnvelope<AIJob>>(`/api/v1/projects/${projectID}/ai/jobs`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function listAIJobEvents(projectID: string, jobID: string, after = 0) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AIJobEvent>>(
      `/api/v1/projects/${projectID}/ai/jobs/${jobID}/events?after=${after}`
    ))
  }

  async function listAIRuns(projectID: string, filters: { contentId?: string, revisionId?: string, jobId?: string, status?: string } = {}) {
    const query = new URLSearchParams()
    for (const [key, value] of Object.entries(filters)) {
      if (value) query.set(key, value)
    }
    const suffix = query.size ? `?${query.toString()}` : ''
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AIRun>>(`/api/v1/projects/${projectID}/ai/runs${suffix}`))
  }

  async function listQualityChecks(projectID: string, filters: { contentId?: string, revisionId?: string, severity?: string, status?: string } = {}) {
    const query = new URLSearchParams()
    for (const [key, value] of Object.entries(filters)) {
      if (value) query.set(key, value)
    }
    const suffix = query.size ? `?${query.toString()}` : ''
    return normalizeAPIListEnvelope(await request<APIListEnvelope<QualityCheckResult>>(`/api/v1/projects/${projectID}/quality-checks${suffix}`))
  }

  async function getVoiceProfile(projectID: string) {
    return await request<APIEnvelope<VoiceProfile>>(`/api/v1/projects/${projectID}/voice-profile`)
  }

  async function createVoiceProfile(projectID: string, profile: VoiceProfileDocument) {
    return await request<APIEnvelope<VoiceProfile>>(`/api/v1/projects/${projectID}/voice-profile`, await withCSRF({
      method: 'POST',
      body: { profile }
    }))
  }

  async function listEvidencePackets(projectID: string, filters: { contentId?: string, approvalState?: string } = {}) {
    const query = new URLSearchParams()
    for (const [key, value] of Object.entries(filters)) {
      if (value) query.set(key, value)
    }
    const suffix = query.size ? `?${query.toString()}` : ''
    return normalizeAPIListEnvelope(await request<APIListEnvelope<EvidencePacket>>(`/api/v1/projects/${projectID}/evidence-packets${suffix}`))
  }

  async function createEvidencePacket(projectID: string, contentId: string, packet: EvidencePacketDocument) {
    return await request<APIEnvelope<EvidencePacket>>(`/api/v1/projects/${projectID}/evidence-packets`, await withCSRF({
      method: 'POST',
      body: { contentId, packet }
    }))
  }

  async function approveEvidencePacket(projectID: string, packetID: string) {
    return await request<APIEnvelope<EvidencePacket>>(`/api/v1/projects/${projectID}/evidence-packets/${packetID}/approve`, await withCSRF({
      method: 'POST',
      body: {}
    }))
  }

  async function listSources(projectID: string) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminSource>>(`/api/v1/projects/${projectID}/sources?limit=100`))
  }

  return {
    request,
    getCSRFToken,
    login,
    currentUser,
    reauthenticate,
    forgotPassword,
    resetPassword,
    logout,
    listProjects,
    getProject,
    createProject,
    updateProject,
    projectAction,
    projectDeletionImpact,
    listMembers,
    inviteMember,
    updateMember,
    removeMember,
    listAPIKeys,
    createAPIKey,
    mutateAPIKey,
    listTaxonomy,
    createTaxonomy,
    listCategories,
    createCategory,
    listAuthors,
    getAuthor,
    createAuthor,
    updateAuthor,
    deleteAuthor,
    listSeries,
    listArticles,
    createArticle,
    getArticle,
    listRevisions,
    articleAction,
    revisionAction,
    listComments,
    createComment,
    mutateComment,
    listReviewAssignees,
    listReviewAssignments,
    createReviewAssignment,
    mutateReviewAssignment,
    listAuditEvents,
    listMedia,
    initiateMediaUpload,
    completeMediaUpload,
    deleteMedia,
    listWebhooks,
    createWebhook,
    revokeWebhook,
    listWebhookAttempts,
    replayWebhookAttempt,
    deliveryStatus,
    listAIJobs,
    createAIJob,
    listAIJobEvents,
    listAIRuns,
    listQualityChecks,
    getVoiceProfile,
    createVoiceProfile,
    listEvidencePackets,
    createEvidencePacket,
    approveEvidencePacket,
    listSources
  }
}

export function normalizeAPIError(error: unknown, fallback: string) {
  if (typeof error === 'object' && error !== null && 'data' in error) {
    const data = (error as { data?: { title?: string, detail?: string, status?: number, statusCode?: number, message?: string } }).data
    if (data?.statusCode === 502 || data?.status === 502) {
      return 'The admin API is unavailable. Start the Go API on the configured proxy port or set NUXT_API_BASE_URL to the running API.'
    }
    return data?.detail || data?.title || data?.message || fallback
  }
  if (error instanceof Error && error.message) {
    return error.message
  }
  return fallback
}

export function apiStatus(error: unknown) {
  if (typeof error !== 'object' || error === null) return 0
  const value = error as {
    status?: number
    statusCode?: number
    data?: { status?: number, statusCode?: number }
  }
  return value.status || value.statusCode || value.data?.status || value.data?.statusCode || 0
}

export function slugify(value: string) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}

export function labelize(value: string) {
  return value.replaceAll('_', ' ')
}

export function htmlToPlainText(value: string) {
  return value
    .replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, ' ')
    .replace(/<style\b[^<]*(?:(?!<\/style>)<[^<]*)*<\/style>/gi, ' ')
    .replace(/<[^>]+>/g, ' ')
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#039;/g, "'")
    .replace(/\s+/g, ' ')
    .trim()
}

export function articleBodyDocumentFromHTML(html: string, fallbackText: string) {
  const text = htmlToPlainText(html) || fallbackText.trim()
  return {
    type: 'doc',
    content: text
      ? [
          {
            type: 'paragraph',
            content: [{ type: 'text', text }]
          }
        ]
      : []
  }
}

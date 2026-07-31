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
  soloOwnerApprovalEnabled: boolean
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
  projectId?: string
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
  archivedAt?: string
  latestRevision?: AdminRevision
  createdAt: string
}

export type AdminRevisionSummary = AdminRevision & {
  baseRevisionId?: string
  publishedLocales: string[]
}

export type AdminRevisionDetail = AdminRevisionSummary & {
  alternateTitle?: string
  bodyDocument: unknown
  tableOfContents: unknown
  authorSnapshot: unknown
  contributorSnapshot: unknown
  taxonomySnapshot: unknown
  sourceSnapshot: unknown
  claimSnapshot: unknown
  seoSnapshot: unknown
  socialSnapshot: unknown
  mediaSnapshot: unknown
  disclosureSnapshot: unknown
  correctionSummary: unknown
  sanitizedHtml: string
  plainText: string
  markdownExport: string
  wordCount: number
  readingTimeSeconds: number
  changeSummary?: string
}

export type TaxonomyTerm = {
  id: string
  type: string
  slug: string
  name: string
  description?: string
  parentId?: string
  ancestors?: TaxonomyTerm[]
  children?: TaxonomyTerm[]
  indexable: boolean
}

export type AdminProjectMember = {
  projectId: string
  userId: string
  email: string
  role: string
  status: string
  userStatus: string
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
  publicationDate?: string
  accessedAt?: string
  sourceType: string
  isPrimary: boolean
  archivedCopyReference?: string
  notes?: string
  createdAt: string
}

export type AdminClaim = {
  id: string
  projectId: string
  articleId: string
  revisionId: string
  claimText: string
  blockId?: string
  importance: string
  verificationState: string
  verifiedBy?: string
  verifiedAt?: string
  sourceIds: string[]
}

export type AdminDisclosure = {
  id: string
  projectId: string
  articleId: string
  revisionId?: string
  disclosureType: string
  publicText: string
  createdBy: string
  createdAt: string
}

export type AdminCorrection = {
  id: string
  projectId: string
  articleId: string
  affectedRevisionId?: string
  publicNote: string
  correctedBy: string
  correctedAt: string
  supersedesNoticeId?: string
}

export type PreviewToken = {
  id: string
  projectId: string
  articleId: string
  revisionId: string
  expiresAt: string
  lastUsedAt?: string
  createdBy: string
  createdAt: string
  revokedAt?: string
}

export type ProjectCreatePayload = {
  name: string
  slug: string
  primaryDomain?: string
  blogBasePath: string
  defaultLocale: string
  supportedLocales: string[]
  timezone: string
  soloOwnerApprovalEnabled?: boolean
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
  soloOwnerApprovalEnabled: boolean
}>

export type ArticleCreatePayload = {
  articleType: string
  title: string
  slug: string
  locale?: string
  primaryCategoryId: string
  contributors?: RevisionContributorInput[]
  deck?: string
  excerpt?: string
  shortAnswer?: string
  bodyDocument?: unknown
  html?: string
  seo?: SEOInputPayload
}

export type ArticleRevisionPayload = {
  baseRevisionId: string
  title: string
  primaryCategoryId?: string
  contributors?: RevisionContributorInput[]
  deck?: string
  excerpt?: string
  shortAnswer?: string
  bodyDocument?: unknown
  html?: string
  seo?: SEOInputPayload
}

export type RevisionContributorInput = {
  authorId: string
  role: 'primary_author' | 'co_author' | 'editor' | 'expert_reviewer' | 'photographer' | 'other'
  position: number
}

export function hasValidRevisionContributors(input: RevisionContributorInput[]) {
  if (input.filter(contributor => contributor.role === 'primary_author').length !== 1) return false
  const assignments = new Set<string>()
  const positions = new Set<string>()
  for (const contributor of input) {
    if (!contributor.authorId || contributor.position < 0) return false
    const assignment = `${contributor.authorId}\u0000${contributor.role}`
    const position = `${contributor.role}\u0000${contributor.position}`
    if (assignments.has(assignment) || positions.has(position)) return false
    assignments.add(assignment)
    positions.add(position)
  }
  return true
}

export type ArticleCopyPayload = {
  destinationProjectId: string
  sourceRevisionId: string
  primaryCategoryId: string
  slug: string
  locale?: string
  canonicalDecision: 'canonical_original' | 'material_adaptation'
  canonicalOriginalUrl?: string
}

export type ArticleListOptions = {
  cursor?: string
  limit?: number
  search?: string
  editorialState?: '' | 'draft' | 'in_review' | 'changes_requested' | 'approved'
  publicationState?: '' | 'unpublished' | 'scheduled' | 'published' | 'archived'
  includeArchived?: boolean
}

export type SEOInputPayload = {
  title?: string
  description?: string
  robots?: 'index,follow' | 'index,nofollow' | 'noindex,follow' | 'noindex,nofollow'
  openGraphTitle?: string
  openGraphDescription?: string
  openGraphImage?: string
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

  async function listMembers(projectID: string, cursor = '', limit = 50) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminProjectMember>>(`/api/v1/projects/${projectID}/members`, {
      query: { limit, ...(cursor ? { cursor } : {}) }
    }))
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

  async function memberLoginAction(projectID: string, userID: string, action: 'disable' | 'enable') {
    const pathAction = action === 'disable' ? 'disable-login' : 'enable-login'
    return await request<APIEnvelope<AdminProjectMember>>(
      `/api/v1/projects/${projectID}/members/${userID}/${pathAction}`,
      await withCSRF({ method: 'POST' })
    )
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

  async function listTaxonomy(projectID: string, type: 'categories' | 'tags', cursor = '', limit = 100) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID}/${type}`, {
      query: { limit, ...(cursor ? { cursor } : {}) }
    }))
  }

  async function createTaxonomy(projectID: string, type: 'categories' | 'tags', payload: TaxonomyCreatePayload) {
    return await request<APIEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID}/${type}`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function listCategories(projectID: string) {
    const categories = new Map<string, TaxonomyTerm>()
    const seenCursors = new Set<string>()
    let cursor = ''

    do {
      const response = await listTaxonomy(projectID, 'categories', cursor, 100)
      for (const category of response.data) categories.set(category.id, category)
      const nextCursor = response.meta?.nextCursor || ''
      if (nextCursor && seenCursors.has(nextCursor)) throw new Error('Category pagination returned a repeated cursor')
      if (nextCursor) seenCursors.add(nextCursor)
      cursor = nextCursor
    } while (cursor)

    return {
      data: [...categories.values()],
      meta: { projectId: projectID, limit: categories.size }
    } satisfies APIListEnvelope<TaxonomyTerm>
  }

  async function createCategory(projectID: string, payload: TaxonomyCreatePayload) {
    return await createTaxonomy(projectID, 'categories', payload)
  }

  async function updateCategory(projectID: string, categoryID: string, payload: Partial<TaxonomyCreatePayload>) {
    return await request<APIEnvelope<TaxonomyTerm>>(`/api/v1/projects/${projectID}/categories/${categoryID}`, await withCSRF({
      method: 'PATCH',
      body: payload
    }))
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

  async function listArticles(projectID: string, options: number | ArticleListOptions = 100) {
    const normalized = typeof options === 'number' ? { limit: options } : options
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminArticle>>(`/api/v1/projects/${projectID}/articles`, {
      query: {
        limit: normalized.limit || 50,
        ...(normalized.cursor ? { cursor: normalized.cursor } : {}),
        ...(normalized.search ? { q: normalized.search } : {}),
        ...(normalized.editorialState ? { editorialState: normalized.editorialState } : {}),
        ...(normalized.publicationState ? { publicationState: normalized.publicationState } : {}),
        ...(normalized.includeArchived ? { includeArchived: true } : {})
      }
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

  async function deleteArticle(projectID: string, articleID: string) {
    return await request<void>(`/api/v1/projects/${projectID}/articles/${articleID}`, await withCSRF({
      method: 'DELETE'
    }))
  }

  async function listRevisions(projectID: string, articleID: string, options: number | { cursor?: string, limit?: number } = 100) {
    const normalized = typeof options === 'number' ? { limit: options } : options
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminRevisionSummary>>(`/api/v1/projects/${projectID}/articles/${articleID}/revisions`, {
      query: { limit: normalized.limit || 25, ...(normalized.cursor ? { cursor: normalized.cursor } : {}) }
    }))
  }

  async function getRevision(projectID: string, articleID: string, revisionID: string) {
    return await request<APIEnvelope<AdminRevisionDetail>>(`/api/v1/projects/${projectID}/articles/${articleID}/revisions/${revisionID}`)
  }

  async function createRevision(projectID: string, articleID: string, payload: ArticleRevisionPayload) {
    return await request<APIEnvelope<AdminRevision>>(`/api/v1/projects/${projectID}/articles/${articleID}/revisions`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function copyArticle(projectID: string, articleID: string, payload: ArticleCopyPayload) {
    return await request<APIEnvelope<AdminArticle>>(`/api/v1/projects/${projectID}/articles/${articleID}/copy-to-project`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function articleAction(
    projectID: string,
    articleID: string,
    action: 'publish' | 'schedule' | 'unpublish' | 'rollback' | 'restore',
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

  async function createSource(projectID: string, payload: {
    title: string
    publisher?: string
    author?: string
    url?: string
    publicationDate?: string
    accessedAt?: string
    sourceType: string
    isPrimary: boolean
    archivedCopyReference?: string
    notes?: string
  }) {
    return await request<APIEnvelope<AdminSource>>(`/api/v1/projects/${projectID}/sources`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function updateSource(projectID: string, sourceID: string, payload: Partial<AdminSource>) {
    return await request<APIEnvelope<AdminSource>>(`/api/v1/projects/${projectID}/sources/${sourceID}`, await withCSRF({
      method: 'PATCH',
      body: payload
    }))
  }

  async function listClaims(projectID: string, revisionID: string) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminClaim>>(`/api/v1/projects/${projectID}/revisions/${revisionID}/claims`))
  }

  async function createClaim(projectID: string, revisionID: string, payload: {
    claimText: string
    blockId?: string
    importance: string
    sourceIds: string[]
  }) {
    return await request<APIEnvelope<AdminClaim>>(`/api/v1/projects/${projectID}/revisions/${revisionID}/claims`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function verifyClaim(projectID: string, claimID: string, verificationState: string, sourceIds?: string[]) {
    return await request<APIEnvelope<AdminClaim>>(`/api/v1/projects/${projectID}/claims/${claimID}/verify`, await withCSRF({
      method: 'POST',
      body: { verificationState, ...(sourceIds ? { sourceIds } : {}) }
    }))
  }

  async function listDisclosures(projectID: string, articleID: string) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminDisclosure>>(`/api/v1/projects/${projectID}/articles/${articleID}/disclosures`))
  }

  async function createDisclosure(projectID: string, articleID: string, payload: {
    revisionId?: string
    disclosureType: string
    publicText: string
  }) {
    return await request<APIEnvelope<AdminDisclosure>>(`/api/v1/projects/${projectID}/articles/${articleID}/disclosures`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function listCorrections(projectID: string, articleID: string) {
    return normalizeAPIListEnvelope(await request<APIListEnvelope<AdminCorrection>>(`/api/v1/projects/${projectID}/articles/${articleID}/corrections`))
  }

  async function createCorrection(projectID: string, articleID: string, payload: {
    affectedRevisionId?: string
    publicNote: string
    supersedesNoticeId?: string
  }) {
    return await request<APIEnvelope<AdminCorrection>>(`/api/v1/projects/${projectID}/articles/${articleID}/corrections`, await withCSRF({
      method: 'POST',
      body: payload
    }))
  }

  async function createPreviewToken(projectID: string, articleID: string, revisionID: string, ttlMinutes = 30) {
    return await request<APIEnvelope<{ token: PreviewToken, secret: string }>>(`/api/v1/projects/${projectID}/preview-tokens`, await withCSRF({
      method: 'POST',
      body: { articleId: articleID, revisionId: revisionID, ttlMinutes }
    }))
  }

  async function revokePreviewToken(projectID: string, tokenID: string) {
    return await request<APIEnvelope<PreviewToken>>(`/api/v1/projects/${projectID}/preview-tokens/${tokenID}/revoke`, await withCSRF({
      method: 'POST',
      body: {}
    }))
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
    memberLoginAction,
    listAPIKeys,
    createAPIKey,
    mutateAPIKey,
    listTaxonomy,
    createTaxonomy,
    listCategories,
    createCategory,
    updateCategory,
    listAuthors,
    getAuthor,
    createAuthor,
    updateAuthor,
    deleteAuthor,
    listSeries,
    listArticles,
    createArticle,
    getArticle,
    deleteArticle,
    listRevisions,
    getRevision,
    createRevision,
    copyArticle,
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
    listSources,
    createSource,
    updateSource,
    listClaims,
    createClaim,
    verifyClaim,
    listDisclosures,
    createDisclosure,
    listCorrections,
    createCorrection,
    createPreviewToken,
    revokePreviewToken
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
  if (typeof DOMParser !== 'undefined') {
    const parsed = new DOMParser().parseFromString(`<body>${html}</body>`, 'text/html')
    const content = structuredBlockNodes(parsed.body)
    if (content.length > 0) {
      return { type: 'doc', schemaVersion: 'tiptap-v1', content }
    }
  }
  const text = htmlToPlainText(html) || fallbackText.trim()
  return {
    type: 'doc',
    schemaVersion: 'tiptap-v1',
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

type StructuredNode = {
  type: string
  text?: string
  attrs?: Record<string, unknown>
  marks?: Array<{ type: string, attrs?: Record<string, unknown> }>
  content?: StructuredNode[]
}

function structuredBlockNodes(parent: ParentNode): StructuredNode[] {
  const nodes: StructuredNode[] = []
  for (const child of parent.childNodes) {
    if (child.nodeType === Node.TEXT_NODE) {
      const text = child.textContent?.trim()
      if (text) nodes.push({ type: 'paragraph', content: [{ type: 'text', text }] })
      continue
    }
    if (!(child instanceof HTMLElement)) continue
    const tag = child.tagName.toLowerCase()
    if (tag === 'script' || tag === 'style' || tag === 'template') continue
    if (tag === 'p') {
      nodes.push({ type: 'paragraph', content: structuredInlineNodes(child) })
    } else if (/^h[2-4]$/.test(tag)) {
      const text = child.textContent?.trim() || ''
      nodes.push({
        type: 'heading',
        attrs: { level: Number(tag.slice(1)), id: child.id || structuredHeadingID(text) },
        content: structuredInlineNodes(child)
      })
    } else if (tag === 'ul' || tag === 'ol') {
      nodes.push(structuredListNode(child))
    } else if (tag === 'blockquote') {
      nodes.push({ type: 'blockquote', content: structuredBlockNodes(child) })
    } else if (tag === 'pre') {
      const code = child.querySelector('code')
      const languageClass = [...(code?.classList || [])].find(value => value.startsWith('language-'))
      nodes.push({
        type: 'codeBlock',
        attrs: languageClass ? { language: languageClass.slice('language-'.length) } : {},
        content: [{ type: 'text', text: code?.textContent || child.textContent || '' }]
      })
    } else if (tag === 'hr') {
      nodes.push({ type: 'horizontalRule' })
    } else if (tag === 'img') {
      nodes.push(structuredImageNode(child as HTMLImageElement))
    } else if (tag === 'figure') {
      nodes.push({ type: 'figure', content: structuredFigureNodes(child) })
    } else if (tag === 'table') {
      nodes.push({ type: 'table', content: structuredTableRows(child) })
    } else {
      nodes.push(...structuredBlockNodes(child))
    }
  }
  return nodes.filter(node => node.type !== 'paragraph' || Boolean(node.content?.length))
}

function structuredInlineNodes(parent: ParentNode, marks: StructuredNode['marks'] = []): StructuredNode[] {
  const nodes: StructuredNode[] = []
  for (const child of parent.childNodes) {
    if (child.nodeType === Node.TEXT_NODE) {
      if (child.textContent) nodes.push({ type: 'text', text: child.textContent, ...(marks.length ? { marks } : {}) })
      continue
    }
    if (!(child instanceof HTMLElement)) continue
    const tag = child.tagName.toLowerCase()
    if (tag === 'br') {
      nodes.push({ type: 'hardBreak' })
      continue
    }
    if (tag === 'img') {
      nodes.push(structuredImageNode(child as HTMLImageElement))
      continue
    }
    const nextMarks = [...marks]
    if (tag === 'strong' || tag === 'b') nextMarks.push({ type: 'bold' })
    if (tag === 'em' || tag === 'i') nextMarks.push({ type: 'italic' })
    if (tag === 'u') nextMarks.push({ type: 'underline' })
    if (tag === 's' || tag === 'del') nextMarks.push({ type: 'strike' })
    if (tag === 'code') nextMarks.push({ type: 'code' })
    if (tag === 'sup') nextMarks.push({ type: 'superscript' })
    if (tag === 'sub') nextMarks.push({ type: 'subscript' })
    if (tag === 'a') nextMarks.push({ type: 'link', attrs: { href: child.getAttribute('href') || '', title: child.getAttribute('title') || '' } })
    nodes.push(...structuredInlineNodes(child, nextMarks))
  }
  return nodes
}

function structuredListItem(item: HTMLElement): StructuredNode[] {
  const nestedLists = [...item.children].filter(child => ['ul', 'ol'].includes(child.tagName.toLowerCase()))
  const inlineContainer = item.cloneNode(true) as HTMLElement
  for (const nested of [...inlineContainer.querySelectorAll(':scope > ul, :scope > ol')]) nested.remove()
  const content: StructuredNode[] = [{ type: 'paragraph', content: structuredInlineNodes(inlineContainer) }]
  for (const list of nestedLists) content.push(structuredListNode(list as HTMLElement))
  return content
}

function structuredListNode(list: HTMLElement): StructuredNode {
  const tag = list.tagName.toLowerCase()
  return {
    type: tag === 'ul' ? 'bulletList' : 'orderedList',
    content: [...list.children]
      .filter(item => item.tagName.toLowerCase() === 'li')
      .map(item => ({ type: 'listItem', content: structuredListItem(item as HTMLElement) }))
  }
}

function structuredImageNode(image: HTMLImageElement): StructuredNode {
  return {
    type: 'image',
    attrs: {
      src: image.getAttribute('src') || '',
      alt: image.getAttribute('alt') || '',
      decorative: image.dataset.decorative === 'true',
      ...(image.width ? { width: image.width } : {}),
      ...(image.height ? { height: image.height } : {})
    }
  }
}

function structuredFigureNodes(figure: HTMLElement): StructuredNode[] {
  const content: StructuredNode[] = []
  const image = figure.querySelector(':scope > img')
  if (image) content.push(structuredImageNode(image as HTMLImageElement))
  const caption = figure.querySelector(':scope > figcaption')
  if (caption) content.push({ type: 'figcaption', content: structuredInlineNodes(caption) })
  return content
}

function structuredTableRows(table: HTMLElement): StructuredNode[] {
  return [...table.querySelectorAll(':scope > thead > tr, :scope > tbody > tr, :scope > tfoot > tr, :scope > tr')].map(row => ({
    type: 'tableRow',
    content: [...row.children].filter(cell => ['th', 'td'].includes(cell.tagName.toLowerCase())).map(cell => ({
      type: cell.tagName.toLowerCase() === 'th' ? 'tableHeader' : 'tableCell',
      attrs: {
        colspan: Number(cell.getAttribute('colspan') || 1),
        rowspan: Number(cell.getAttribute('rowspan') || 1),
        ...(cell.getAttribute('scope') ? { scope: cell.getAttribute('scope') } : {})
      },
      content: [{ type: 'paragraph', content: structuredInlineNodes(cell) }]
    }))
  }))
}

function structuredHeadingID(value: string) {
  return value.toLowerCase().trim().replace(/[^\p{L}\p{N}]+/gu, '-').replace(/^-|-$/g, '').slice(0, 96) || 'section'
}

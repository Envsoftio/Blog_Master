export type ContentClientOptions = {
  baseUrl: string
  apiKey: string
  fetch?: typeof globalThis.fetch
}

export type APIEnvelope<T> = {
  data: T
  meta?: {
    projectId?: string
    contentGeneration?: number
    etag?: string
  }
}

export type APIListEnvelope<T> = {
  data: T[]
  meta: {
    projectId?: string
    contentGeneration?: number
    limit: number
    nextCursor?: string
  }
}

export type ListPostsParams = {
  category?: string
  categoryMode?: 'descendants' | 'exact'
  tag?: string
  author?: string
  articleType?: string
  series?: string
  publishedFrom?: string
  publishedTo?: string
  cursor?: string
  limit?: number
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

export type Author = {
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
  status?: string
  createdAt?: string
  updatedAt?: string
}

export type Contributor = {
  author: Author
  role: string
  position: number
}

export type Series = {
  id: string
  slug: string
  name: string
  description?: string
  indexable: boolean
  position?: number
  previous?: PublishedArticleSummary
  next?: PublishedArticleSummary
}

export type PublishedTaxonomy = {
  primaryCategory: TaxonomyTerm | null
  categories: TaxonomyTerm[]
  tags: TaxonomyTerm[]
  series?: Series
  topics: TaxonomyTerm[]
}

export type JSONLDPrimitive = string | number | boolean | null
export type JSONLDValue = JSONLDPrimitive | JSONLDObject | JSONLDValue[]
export type JSONLDObject = { [key: string]: JSONLDValue }

export type PublishedOpenGraph = {
  title?: string
  description?: string
  image?: string
}

export type PublishedSEO = {
  title: string
  description?: string
  canonicalUrl: string
  robots: string
  index: boolean
  openGraph: PublishedOpenGraph
  structuredData: JSONLDObject[]
}

export type PublishedMediaVariant = {
  name: string
  url: string
  mimeType: string
  width: number
  height: number
}

export type PublishedMediaAsset = {
  id: string
  url: string
  mimeType: string
  width: number
  height: number
  altText?: string
  decorative: boolean
  caption?: string
  credit?: string
  license?: string
  variants: PublishedMediaVariant[]
}

export type PublishedMedia = {
  hero: PublishedMediaAsset | null
}

export type PublishedArticleSummary = {
  id: string
  slug: string
  title: string
  excerpt?: string
  canonicalUrl: string
}

export type PublishedArticleLink = {
  article: PublishedArticleSummary
  relationshipType: 'related' | 'pillar' | 'cluster'
  origin: 'manual' | 'deterministic' | 'imported'
  position: number
}

export type PublishedPost = {
  id: string
  articleType: string
  slug: string
  revision: number
  title: string
  deck?: string
  excerpt?: string
  shortAnswer?: string
  content: {
    format: string
    document: unknown
    html: string
    tableOfContents: unknown
  }
  taxonomy: PublishedTaxonomy
  authors: Author[]
  contributors: Contributor[]
  media: PublishedMedia
  sources: unknown
  claims: unknown
  disclosures: unknown
  corrections: unknown
  seo: PublishedSEO
  relatedArticles: PublishedArticleLink[]
  topicRelationships: PublishedArticleLink[]
  publishedAt?: string
  modifiedAt?: string
}

export type RelatedPost = {
  post: PublishedPost
  origin: 'manual' | 'deterministic' | 'imported'
}

export type DiscoveryEntry = {
  id: string
  canonicalUrl: string
  lastModified: string
}

export type RedirectRecord = {
  sourcePath: string
  targetPath: string
  statusCode: number
}

export type ChangeRecord = {
  id: string
  type: string
  aggregateId: string
  createdAt: string
}

export type HeadPostOptions = {
  ifNoneMatch?: string
  ifModifiedSince?: string
}

export type HeadPostResult = {
  status: 200 | 304
  etag?: string
  lastModified?: string
  cacheControl?: string
}

export class ContentClient {
  private readonly baseUrl: string
  private readonly apiKey: string
  private readonly fetcher: typeof globalThis.fetch

  constructor(options: ContentClientOptions) {
    if (typeof window !== 'undefined') {
      throw new Error('ContentClient contains a secret API key and may only be created in a trusted server or build environment')
    }
    this.baseUrl = options.baseUrl.replace(/\/$/, '')
    this.apiKey = options.apiKey
    this.fetcher = options.fetch ?? globalThis.fetch
  }

  getPost(slug: string) {
    return this.request<APIEnvelope<PublishedPost>>(`/content/v1/posts/${encodeURIComponent(slug)}`)
  }

  async headPost(slug: string, options: HeadPostOptions = {}): Promise<HeadPostResult> {
    const headers = new Headers({
      Authorization: `Bearer ${this.apiKey}`,
      Accept: 'application/json'
    })
    if (options.ifNoneMatch) headers.set('If-None-Match', options.ifNoneMatch)
    if (options.ifModifiedSince) headers.set('If-Modified-Since', options.ifModifiedSince)
    const response = await this.fetcher(
      `${this.baseUrl}/content/v1/posts/${encodeURIComponent(slug)}`,
      { method: 'HEAD', headers }
    )
    if (!response.ok && response.status !== 304) {
      throw new Error(`Content API request failed: ${response.status}`)
    }
    return {
      status: response.status as 200 | 304,
      etag: response.headers.get('ETag') ?? undefined,
      lastModified: response.headers.get('Last-Modified') ?? undefined,
      cacheControl: response.headers.get('Cache-Control') ?? undefined
    }
  }

  getPostByID(contentID: string) {
    return this.request<APIEnvelope<PublishedPost>>(`/content/v1/posts/by-id/${encodeURIComponent(contentID)}`)
  }

  getPreviewRevision(revisionID: string, previewToken: string) {
    return this.requestWithBearer<APIEnvelope<PublishedPost>>(
      `/content/v1/preview/revisions/${encodeURIComponent(revisionID)}`,
      previewToken
    )
  }

  listPosts(params: ListPostsParams = {}) {
    return this.request<APIListEnvelope<PublishedPost>>(`/content/v1/posts${query(params)}`)
  }

  relatedPosts(slug: string, limit = 6) {
    return this.request<APIListEnvelope<RelatedPost>>(
      `/content/v1/posts/${encodeURIComponent(slug)}/related${query({ limit })}`
    )
  }

  categories(params: { cursor?: string, limit?: number } = {}) {
    return this.request<APIListEnvelope<TaxonomyTerm>>(`/content/v1/categories${query(params)}`)
  }

  getCategory(slug: string) {
    return this.request<APIEnvelope<TaxonomyTerm>>(`/content/v1/categories/${encodeURIComponent(slug)}`)
  }

  tags(params: { cursor?: string, limit?: number } = {}) {
    return this.request<APIListEnvelope<TaxonomyTerm>>(`/content/v1/tags${query(params)}`)
  }

  getTag(slug: string) {
    return this.request<APIEnvelope<TaxonomyTerm>>(`/content/v1/tags/${encodeURIComponent(slug)}`)
  }

  authors(params: { cursor?: string, limit?: number } = {}) {
    return this.request<APIListEnvelope<Author>>(`/content/v1/authors${query(params)}`)
  }

  getAuthor(slug: string) {
    return this.request<APIEnvelope<Author>>(`/content/v1/authors/${encodeURIComponent(slug)}`)
  }

  series(params: { cursor?: string, limit?: number } = {}) {
    return this.request<APIListEnvelope<Series>>(`/content/v1/series${query(params)}`)
  }

  getSeries(slug: string) {
    return this.request<APIEnvelope<Series>>(`/content/v1/series/${encodeURIComponent(slug)}`)
  }

  feedData(params: ListPostsParams = {}) {
    return this.request<APIListEnvelope<PublishedPost>>(`/content/v1/feed-data${query(params)}`)
  }

  discoveryManifest() {
    return this.request<APIEnvelope<{ urls: DiscoveryEntry[] }>>('/content/v1/discovery-manifest')
  }

  redirects(params: { cursor?: string, limit?: number } = {}) {
    return this.request<APIListEnvelope<RedirectRecord>>(`/content/v1/redirects${query(params)}`)
  }

  changes(after?: string, limit = 100) {
    return this.request<APIListEnvelope<ChangeRecord>>(`/content/v1/changes${query({ after, limit })}`)
  }

  private async request<T>(path: string): Promise<T> {
    return this.requestWithBearer<T>(path, this.apiKey)
  }

  private async requestWithBearer<T>(path: string, bearerToken: string): Promise<T> {
    const response = await this.fetcher(`${this.baseUrl}${path}`, {
      headers: {
        Authorization: `Bearer ${bearerToken}`,
        Accept: 'application/json'
      }
    })
    if (!response.ok) {
      throw new Error(`Content API request failed: ${response.status}`)
    }
    return response.json() as Promise<T>
  }
}

function query(params: object): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') search.set(key, String(value))
  }
  const value = search.toString()
  return value ? `?${value}` : ''
}

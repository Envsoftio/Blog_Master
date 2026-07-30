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
    limit: number
    nextCursor?: string
  }
}

export type ListPostsParams = {
  locale?: string
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

export type PublishedPost = {
  id: string
  articleType: string
  slug: string
  locale: string
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
  taxonomy: unknown
  authors: unknown[]
  contributors: unknown[]
  media: unknown
  sources: unknown
  claims: unknown
  disclosures: unknown
  corrections: unknown
  seo: {
    title: string
    description?: string
    canonicalUrl: string
    robots: string
    index: boolean
    openGraph: unknown
    structuredData: unknown
    hreflang: unknown
  }
  publishedAt?: string
  modifiedAt?: string
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

  getPost(slug: string, locale = 'en') {
    return this.request<APIEnvelope<PublishedPost>>(`/content/v1/posts/${encodeURIComponent(slug)}?locale=${encodeURIComponent(locale)}`)
  }

  getPostByID(contentID: string, locale = 'en') {
    return this.request<APIEnvelope<PublishedPost>>(`/content/v1/posts/by-id/${encodeURIComponent(contentID)}?locale=${encodeURIComponent(locale)}`)
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

  relatedPosts(slug: string, locale = 'en', limit = 6) {
    return this.request<APIListEnvelope<{ post: PublishedPost, origin: 'manual' | 'deterministic' | 'imported' }>>(
      `/content/v1/posts/${encodeURIComponent(slug)}/related${query({ locale, limit })}`
    )
  }

  categories(params: { cursor?: string, limit?: number } = {}) {
    return this.request<APIListEnvelope<unknown>>(`/content/v1/categories${query(params)}`)
  }

  tags(params: { cursor?: string, limit?: number } = {}) {
    return this.request<APIListEnvelope<unknown>>(`/content/v1/tags${query(params)}`)
  }

  authors(params: { cursor?: string, limit?: number } = {}) {
    return this.request<APIListEnvelope<unknown>>(`/content/v1/authors${query(params)}`)
  }

  series(params: { cursor?: string, limit?: number } = {}) {
    return this.request<APIListEnvelope<unknown>>(`/content/v1/series${query(params)}`)
  }

  discoveryManifest(locale?: string) {
    return this.request<APIEnvelope<{ urls: unknown[] }>>(`/content/v1/discovery-manifest${query({ locale })}`)
  }

  redirects(params: { cursor?: string, limit?: number } = {}) {
    return this.request<APIListEnvelope<unknown>>(`/content/v1/redirects${query(params)}`)
  }

  changes(after?: string, limit = 100) {
    return this.request<APIListEnvelope<unknown>>(`/content/v1/changes${query({ after, limit })}`)
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

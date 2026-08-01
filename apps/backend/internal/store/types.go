package store

type PublishedPost struct {
	ID                 string                 `json:"id"`
	ArticleType        string                 `json:"articleType"`
	Slug               string                 `json:"slug"`
	Revision           int64                  `json:"revision"`
	Title              string                 `json:"title"`
	Deck               string                 `json:"deck,omitempty"`
	Excerpt            string                 `json:"excerpt,omitempty"`
	ShortAnswer        string                 `json:"shortAnswer,omitempty"`
	Content            PublishedContent       `json:"content"`
	Taxonomy           PublishedTaxonomy      `json:"taxonomy"`
	Authors            []Author               `json:"authors"`
	Contributors       []Contributor          `json:"contributors"`
	Media              PublishedMedia         `json:"media"`
	Sources            any                    `json:"sources"`
	Claims             any                    `json:"claims"`
	Disclosures        any                    `json:"disclosures"`
	Corrections        any                    `json:"corrections"`
	SEO                PublishedSEO           `json:"seo"`
	RelatedArticles    []PublishedArticleLink `json:"relatedArticles"`
	TopicRelationships []PublishedArticleLink `json:"topicRelationships"`
	PublishedAt        string                 `json:"publishedAt,omitempty"`
	ModifiedAt         string                 `json:"modifiedAt,omitempty"`
	ContentHash        string                 `json:"-"`
	PaginationKey      string                 `json:"-"`
	PublisherName      string                 `json:"-"`
	PublisherURL       string                 `json:"-"`
}

type PublishedContent struct {
	Format          string `json:"format"`
	Document        any    `json:"document"`
	HTML            string `json:"html"`
	TableOfContents any    `json:"tableOfContents"`
}

type PublishedTaxonomy struct {
	PrimaryCategory *TaxonomyTerm  `json:"primaryCategory"`
	Categories      []TaxonomyTerm `json:"categories"`
	Tags            []TaxonomyTerm `json:"tags"`
	Series          *Series        `json:"series,omitempty"`
	Topics          []TaxonomyTerm `json:"topics"`
}

type PublishedMedia struct {
	Hero *PublishedMediaAsset `json:"hero"`
}

type PublishedMediaAsset struct {
	ID         string                  `json:"id"`
	URL        string                  `json:"url"`
	MIMEType   string                  `json:"mimeType"`
	Width      int64                   `json:"width"`
	Height     int64                   `json:"height"`
	AltText    string                  `json:"altText,omitempty"`
	Decorative bool                    `json:"decorative"`
	Caption    string                  `json:"caption,omitempty"`
	Credit     string                  `json:"credit,omitempty"`
	License    string                  `json:"license,omitempty"`
	Variants   []PublishedMediaVariant `json:"variants"`
}

type PublishedMediaVariant struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	MIMEType string `json:"mimeType"`
	Width    int64  `json:"width"`
	Height   int64  `json:"height"`
}

type PublishedSEO struct {
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	CanonicalURL   string `json:"canonicalUrl"`
	Robots         string `json:"robots"`
	Index          bool   `json:"index"`
	OpenGraph      any    `json:"openGraph"`
	StructuredData any    `json:"structuredData"`
}

type TaxonomyTerm struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	ParentID    string         `json:"parentId,omitempty"`
	Ancestors   []TaxonomyTerm `json:"ancestors,omitempty"`
	Children    []TaxonomyTerm `json:"children,omitempty"`
	Indexable   bool           `json:"indexable"`
}

type Author struct {
	ID               string   `json:"id"`
	Slug             string   `json:"slug"`
	DisplayName      string   `json:"displayName"`
	ShortBio         string   `json:"shortBio,omitempty"`
	FullBio          string   `json:"fullBio,omitempty"`
	PhotoAssetID     string   `json:"photoAssetId,omitempty"`
	JobTitle         string   `json:"jobTitle,omitempty"`
	Organization     string   `json:"organization,omitempty"`
	Credentials      []string `json:"credentials,omitempty"`
	Expertise        []string `json:"expertise,omitempty"`
	ProfileURL       string   `json:"profileUrl,omitempty"`
	ExternalProfiles []string `json:"externalProfiles,omitempty"`
	SameAs           []string `json:"sameAs,omitempty"`
	LoginUserID      string   `json:"loginUserId,omitempty"`
	LoginEmail       string   `json:"loginEmail,omitempty"`
	LoginRole        string   `json:"loginRole,omitempty"`
	LoginStatus      string   `json:"loginStatus,omitempty"`
	Status           string   `json:"status,omitempty"`
	CreatedAt        string   `json:"createdAt,omitempty"`
	UpdatedAt        string   `json:"updatedAt,omitempty"`
}

type Contributor struct {
	Author   Author `json:"author"`
	Role     string `json:"role"`
	Position int    `json:"position"`
}

type Series struct {
	ID          string                   `json:"id"`
	Slug        string                   `json:"slug"`
	Name        string                   `json:"name"`
	Description string                   `json:"description,omitempty"`
	Indexable   bool                     `json:"indexable"`
	Position    int                      `json:"position"`
	Previous    *PublishedArticleSummary `json:"previous,omitempty"`
	Next        *PublishedArticleSummary `json:"next,omitempty"`
}

type PublishedArticleSummary struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Excerpt      string `json:"excerpt,omitempty"`
	CanonicalURL string `json:"canonicalUrl"`
}

type PublishedArticleLink struct {
	Article          PublishedArticleSummary `json:"article"`
	RelationshipType string                  `json:"relationshipType"`
	Origin           string                  `json:"origin"`
	Position         int                     `json:"position"`
}

type RelatedPost struct {
	Post   PublishedPost `json:"post"`
	Origin string        `json:"origin"`
}

type RedirectRecord struct {
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
	StatusCode int    `json:"statusCode"`
}

type ChangeRecord struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	AggregateID string `json:"aggregateId"`
	CreatedAt   string `json:"createdAt"`
}

type DiscoveryEntry struct {
	ID           string `json:"id"`
	CanonicalURL string `json:"canonicalUrl"`
	LastModified string `json:"lastModified"`
}

package httpapi

type Envelope[T any] struct {
	Data T        `json:"data"`
	Meta MetaData `json:"meta,omitempty"`
}

type ListEnvelope[T any] struct {
	Data []T      `json:"data"`
	Meta PageMeta `json:"meta"`
}

type MetaData struct {
	ProjectID         string `json:"projectId,omitempty"`
	ContentGeneration int64  `json:"contentGeneration,omitempty"`
	ETag              string `json:"etag,omitempty"`
}

type PageMeta struct {
	ProjectID  string `json:"projectId,omitempty"`
	NextCursor string `json:"nextCursor,omitempty"`
	Limit      int    `json:"limit"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

type ProjectContext struct {
	ProjectID string
	KeyID     string
	Scopes    []string
}

type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

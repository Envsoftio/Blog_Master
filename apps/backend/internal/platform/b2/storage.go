package b2

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	awsAlgorithm    = "AWS4-HMAC-SHA256"
	awsService      = "s3"
	unsignedPayload = "UNSIGNED-PAYLOAD"
)

type Config struct {
	Endpoint             string
	Region               string
	Bucket               string
	KeyID                string
	ApplicationKey       string
	PublicBaseURL        string
	PresignTTL           time.Duration
	ServerSideEncryption string
}

type Client struct {
	endpoint             *url.URL
	region               string
	bucket               string
	keyID                string
	applicationKey       string
	publicBaseURL        string
	presignTTL           time.Duration
	serverSideEncryption string
	httpClient           *http.Client
}

type SignedUpload struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	Fields    map[string]string `json:"fields,omitempty"`
	ExpiresAt string            `json:"expiresAt"`
	MaxBytes  int64             `json:"maxBytes"`
}

type objectVersion struct {
	Key       string `xml:"Key"`
	VersionID string `xml:"VersionId"`
}

type listObjectVersionsResponse struct {
	IsTruncated         bool            `xml:"IsTruncated"`
	NextKeyMarker       string          `xml:"NextKeyMarker"`
	NextVersionIDMarker string          `xml:"NextVersionIdMarker"`
	Versions            []objectVersion `xml:"Version"`
	DeleteMarkers       []objectVersion `xml:"DeleteMarker"`
}

func New(config Config) (*Client, error) {
	endpoint, err := url.Parse(strings.TrimRight(strings.TrimSpace(config.Endpoint), "/"))
	if err != nil {
		return nil, fmt.Errorf("parse B2 media endpoint: %w", err)
	}
	if endpoint.Scheme != "https" || endpoint.Host == "" {
		return nil, fmt.Errorf("B2 media endpoint must be an HTTPS URL")
	}
	ttl := config.PresignTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if ttl > 7*24*time.Hour {
		ttl = 7 * 24 * time.Hour
	}
	bucket := strings.TrimSpace(config.Bucket)
	region := normalizeRegion(strings.TrimSpace(config.Region), endpoint.Host)
	publicBaseURL := strings.TrimRight(strings.TrimSpace(config.PublicBaseURL), "/")
	if publicBaseURL == "" && bucket != "" {
		bucketURL := *endpoint
		bucketURL.Path = "/" + strings.Trim(bucket, "/")
		bucketURL.RawPath = "/" + escapeKey(bucket)
		bucketURL.RawQuery = ""
		publicBaseURL = bucketURL.String()
	}
	return &Client{
		endpoint:             endpoint,
		region:               region,
		bucket:               bucket,
		keyID:                strings.TrimSpace(config.KeyID),
		applicationKey:       config.ApplicationKey,
		publicBaseURL:        publicBaseURL,
		presignTTL:           ttl,
		serverSideEncryption: strings.TrimSpace(config.ServerSideEncryption),
		httpClient:           &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func normalizeRegion(region, endpointHost string) string {
	if normalized := regionFromEndpointHost(region); normalized != "" {
		return normalized
	}
	if region != "" {
		return region
	}
	if normalized := regionFromEndpointHost(endpointHost); normalized != "" {
		return normalized
	}
	return "us-west-004"
}

func regionFromEndpointHost(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "s3.")
	value = strings.TrimSuffix(value, "/")
	const suffix = ".backblazeb2.com"
	if !strings.HasSuffix(value, suffix) {
		return ""
	}
	region := strings.TrimSuffix(value, suffix)
	if region == "" || strings.Contains(region, "/") {
		return ""
	}
	return region
}

func (c *Client) Bucket() string {
	if c == nil {
		return ""
	}
	return c.bucket
}

func (c *Client) PublicURL(key string) string {
	if c == nil || c.publicBaseURL == "" || key == "" {
		return ""
	}
	return c.publicBaseURL + "/" + escapeKey(key)
}

func (c *Client) PresignPut(key, contentType string, maxBytes int64, now time.Time) (SignedUpload, error) {
	if c == nil {
		return SignedUpload{}, fmt.Errorf("B2 media storage is not configured")
	}
	if maxBytes <= 0 {
		return SignedUpload{}, fmt.Errorf("B2 media upload size must be positive")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	expiresSeconds := int64(c.presignTTL / time.Second)
	if expiresSeconds <= 0 {
		expiresSeconds = 1
	}
	expiresAt := now.Add(time.Duration(expiresSeconds) * time.Second).UTC()
	amzDate := now.UTC().Format("20060102T150405Z")
	shortDate := now.UTC().Format("20060102")
	credentialScope := shortDate + "/" + c.region + "/" + awsService + "/aws4_request"
	credential := c.keyID + "/" + credentialScope
	headers := map[string]string{}
	if contentType != "" {
		headers["content-type"] = contentType
	}
	if c.serverSideEncryption != "" {
		headers["x-amz-server-side-encryption"] = c.serverSideEncryption
	}

	signedHeaders := signedHeaderNames(headers, c.endpoint.Host)
	query := url.Values{}
	query.Set("X-Amz-Algorithm", awsAlgorithm)
	query.Set("X-Amz-Credential", credential)
	query.Set("X-Amz-Date", amzDate)
	query.Set("X-Amz-Expires", strconv.FormatInt(expiresSeconds, 10))
	query.Set("X-Amz-SignedHeaders", strings.Join(signedHeaders, ";"))
	canonicalRequest := strings.Join([]string{
		http.MethodPut,
		canonicalPath(c.bucket, key),
		query.Encode(),
		canonicalHeaders(headers, c.endpoint.Host),
		strings.Join(signedHeaders, ";"),
		unsignedPayload,
	}, "\n")
	query.Set("X-Amz-Signature", c.signature(shortDate, credentialScope, amzDate, canonicalRequest))
	uploadURL := c.objectURL(key)
	uploadURL.RawQuery = query.Encode()

	return SignedUpload{
		URL:       uploadURL.String(),
		Method:    http.MethodPut,
		Headers:   publicHeaders(headers),
		ExpiresAt: expiresAt.Format(time.RFC3339),
		MaxBytes:  maxBytes,
	}, nil
}

func (c *Client) PutObject(ctx context.Context, key string, body []byte, contentType string) error {
	request, err := c.signedRequest(ctx, http.MethodPut, key, body, contentType)
	if err != nil {
		return err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("put B2 object: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return b2StatusError("put B2 object", response)
	}
	return nil
}

// DeleteObject permanently removes all versions of key. A versionless S3
// DELETE only creates a delete marker in B2 and leaves the stored bytes behind.
func (c *Client) DeleteObject(ctx context.Context, key string) error {
	versions, err := c.listObjectVersions(ctx, key)
	if err != nil {
		return err
	}
	var errs []error
	for _, version := range versions {
		query := url.Values{"versionId": {version.VersionID}}
		request, err := c.signedObjectRequest(ctx, http.MethodDelete, key, query, nil, "")
		if err != nil {
			errs = append(errs, err)
			continue
		}
		response, err := c.httpClient.Do(request)
		if err != nil {
			errs = append(errs, fmt.Errorf("delete B2 object version %q: %w", version.VersionID, err))
			continue
		}
		if response.StatusCode != http.StatusNotFound && (response.StatusCode < 200 || response.StatusCode >= 300) {
			errs = append(errs, b2StatusError("delete B2 object version", response))
		}
		response.Body.Close()
	}
	return errors.Join(errs...)
}

func (c *Client) listObjectVersions(ctx context.Context, key string) ([]objectVersion, error) {
	query := url.Values{
		"max-keys": {"1000"},
		"prefix":   {key},
		"versions": {""},
	}
	var result []objectVersion
	previousCursor := ""
	for {
		request, err := c.signedBucketRequest(ctx, http.MethodGet, query)
		if err != nil {
			return nil, err
		}
		response, err := c.httpClient.Do(request)
		if err != nil {
			return nil, fmt.Errorf("list B2 object versions: %w", err)
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			err := b2StatusError("list B2 object versions", response)
			response.Body.Close()
			return nil, err
		}
		var page listObjectVersionsResponse
		err = xml.NewDecoder(io.LimitReader(response.Body, 4*1024*1024)).Decode(&page)
		response.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decode B2 object versions: %w", err)
		}
		for _, version := range append(page.Versions, page.DeleteMarkers...) {
			if version.Key == key && strings.TrimSpace(version.VersionID) != "" {
				result = append(result, version)
			}
		}
		if !page.IsTruncated {
			return result, nil
		}
		cursor := page.NextKeyMarker + "\x00" + page.NextVersionIDMarker
		if strings.TrimSpace(page.NextKeyMarker) == "" || cursor == previousCursor {
			return nil, fmt.Errorf("list B2 object versions returned an invalid pagination cursor")
		}
		previousCursor = cursor
		query.Set("key-marker", page.NextKeyMarker)
		if page.NextVersionIDMarker == "" {
			query.Del("version-id-marker")
		} else {
			query.Set("version-id-marker", page.NextVersionIDMarker)
		}
	}
}

func (c *Client) GetObject(ctx context.Context, key string, maxBytes int64) ([]byte, string, error) {
	request, err := c.signedRequest(ctx, http.MethodGet, key, nil, "")
	if err != nil {
		return nil, "", err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("get B2 object: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", b2StatusError("get B2 object", response)
	}
	reader := response.Body
	if maxBytes > 0 {
		reader = io.NopCloser(io.LimitReader(response.Body, maxBytes+1))
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("read B2 object: %w", err)
	}
	if maxBytes > 0 && int64(len(body)) > maxBytes {
		return nil, "", fmt.Errorf("B2 object exceeds the %d byte processing limit", maxBytes)
	}
	return body, response.Header.Get("Content-Type"), nil
}

func (c *Client) signedRequest(ctx context.Context, method, key string, body []byte, contentType string) (*http.Request, error) {
	return c.signedObjectRequest(ctx, method, key, nil, body, contentType)
}

func (c *Client) signedObjectRequest(ctx context.Context, method, key string, query url.Values, body []byte, contentType string) (*http.Request, error) {
	if c == nil {
		return nil, fmt.Errorf("B2 media storage is not configured")
	}
	target := c.objectURL(key)
	return c.signedURLRequest(ctx, method, target, canonicalPath(c.bucket, key), query, body, contentType)
}

func (c *Client) signedBucketRequest(ctx context.Context, method string, query url.Values) (*http.Request, error) {
	if c == nil {
		return nil, fmt.Errorf("B2 media storage is not configured")
	}
	target := c.bucketURL()
	return c.signedURLRequest(ctx, method, target, "/"+escapeKey(c.bucket), query, nil, "")
}

func (c *Client) signedURLRequest(ctx context.Context, method string, target *url.URL, canonicalURI string, query url.Values, body []byte, contentType string) (*http.Request, error) {
	if c == nil {
		return nil, fmt.Errorf("B2 media storage is not configured")
	}
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")
	credentialScope := shortDate + "/" + c.region + "/" + awsService + "/aws4_request"
	payloadHash := sha256Hex(body)
	var reader io.Reader = http.NoBody
	if body != nil {
		reader = bytes.NewReader(body)
	}
	canonicalQuery := canonicalQueryString(query)
	target.RawQuery = canonicalQuery
	request, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	if contentType != "" {
		headers["content-type"] = contentType
	}
	if method == http.MethodPut && c.serverSideEncryption != "" {
		headers["x-amz-server-side-encryption"] = c.serverSideEncryption
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	signedHeaders := signedHeaderNames(headers, target.Host)
	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders(headers, target.Host),
		strings.Join(signedHeaders, ";"),
		payloadHash,
	}, "\n")
	stringToSign := strings.Join([]string{
		awsAlgorithm,
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	signature := hex.EncodeToString(hmacSHA256(signingKey(c.applicationKey, shortDate, c.region), []byte(stringToSign)))
	request.Header.Set("Authorization", fmt.Sprintf(
		"%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		awsAlgorithm,
		c.keyID,
		credentialScope,
		strings.Join(signedHeaders, ";"),
		signature,
	))
	return request, nil
}

func canonicalQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		items := append([]string(nil), values[key]...)
		if len(items) == 0 {
			items = []string{""}
		}
		sort.Strings(items)
		for _, value := range items {
			parts = append(parts, awsQueryEscape(key)+"="+awsQueryEscape(value))
		}
	}
	return strings.Join(parts, "&")
}

func awsQueryEscape(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func (c *Client) signature(shortDate, credentialScope, amzDate, canonicalRequest string) string {
	stringToSign := strings.Join([]string{
		awsAlgorithm,
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	return hex.EncodeToString(hmacSHA256(signingKey(c.applicationKey, shortDate, c.region), []byte(stringToSign)))
}

func (c *Client) objectURL(key string) *url.URL {
	result := *c.endpoint
	result.Path = "/" + strings.Trim(c.bucket, "/") + "/" + strings.TrimLeft(key, "/")
	result.RawPath = canonicalPath(c.bucket, key)
	result.RawQuery = ""
	return &result
}

func (c *Client) bucketURL() *url.URL {
	result := *c.endpoint
	result.Path = "/" + strings.Trim(c.bucket, "/")
	result.RawPath = "/" + escapeKey(c.bucket)
	result.RawQuery = ""
	return &result
}

func canonicalPath(bucket, key string) string {
	return "/" + escapeKey(bucket) + "/" + escapeKey(key)
}

func escapeKey(value string) string {
	parts := strings.Split(value, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func signedHeaderNames(headers map[string]string, host string) []string {
	names := make([]string, 0, len(headers)+1)
	names = append(names, "host")
	for name := range headers {
		names = append(names, strings.ToLower(name))
	}
	sort.Strings(names)
	return names
}

func canonicalHeaders(headers map[string]string, host string) string {
	normalized := map[string]string{"host": host}
	for name, value := range headers {
		normalized[strings.ToLower(name)] = strings.Join(strings.Fields(value), " ")
	}
	names := make([]string, 0, len(normalized))
	for name := range normalized {
		names = append(names, name)
	}
	sort.Strings(names)
	var builder strings.Builder
	for _, name := range names {
		builder.WriteString(name)
		builder.WriteByte(':')
		builder.WriteString(normalized[name])
		builder.WriteByte('\n')
	}
	return builder.String()
}

func publicHeaders(headers map[string]string) map[string]string {
	result := make(map[string]string, len(headers))
	for name, value := range headers {
		result[canonicalHeaderName(name)] = value
	}
	return result
}

func canonicalHeaderName(name string) string {
	parts := strings.Split(strings.ToLower(name), "-")
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "-")
}

func signingKey(secret, shortDate, region string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+secret), []byte(shortDate))
	regionKey := hmacSHA256(dateKey, []byte(region))
	serviceKey := hmacSHA256(regionKey, []byte(awsService))
	return hmacSHA256(serviceKey, []byte("aws4_request"))
}

func hmacSHA256(key, value []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(value)
	return mac.Sum(nil)
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func b2StatusError(action string, response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Errorf("%s failed with status %d", action, response.StatusCode)
	}
	return fmt.Errorf("%s failed with status %d: %s", action, response.StatusCode, detail)
}

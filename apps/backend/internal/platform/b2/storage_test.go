package b2

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPresignPutBuildsB2S3CompatibleUploadURL(t *testing.T) {
	client, err := New(Config{
		Endpoint:             "https://s3.us-west-004.backblazeb2.com",
		Region:               "us-west-004",
		Bucket:               "media-bucket",
		KeyID:                "key-id",
		ApplicationKey:       "application-key",
		ServerSideEncryption: "AES256",
	})
	if err != nil {
		t.Fatal(err)
	}
	signed, err := client.PresignPut("blogSEO/projects/project-a/media/originals/hero image.png", "image/png", 2048, time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(signed.URL)
	if err != nil {
		t.Fatal(err)
	}
	if signed.Method != "PUT" || signed.MaxBytes != 2048 {
		t.Fatalf("unexpected signed target %#v", signed)
	}
	if parsed.EscapedPath() != "/media-bucket/blogSEO/projects/project-a/media/originals/hero%20image.png" {
		t.Fatalf("unexpected bucket path %q", parsed.EscapedPath())
	}
	if len(signed.Fields) != 0 {
		t.Fatalf("expected raw PUT upload without form fields, got %#v", signed.Fields)
	}
	if signed.Headers["Content-Type"] != "image/png" ||
		signed.Headers["X-Amz-Server-Side-Encryption"] != "AES256" {
		t.Fatalf("unexpected signed headers %#v", signed.Headers)
	}
	query := parsed.Query()
	if query.Get("X-Amz-Algorithm") != awsAlgorithm ||
		!strings.Contains(query.Get("X-Amz-Credential"), "/us-west-004/s3/aws4_request") ||
		query.Get("X-Amz-Date") != "20260731T100000Z" ||
		query.Get("X-Amz-Expires") != "900" ||
		query.Get("X-Amz-SignedHeaders") != "content-type;host;x-amz-server-side-encryption" ||
		query.Get("X-Amz-Signature") == "" {
		t.Fatalf("unexpected signed query %s", parsed.RawQuery)
	}
	if signed.ExpiresAt != "2026-07-31T10:15:00Z" {
		t.Fatalf("unexpected expiration %q", signed.ExpiresAt)
	}
}

func TestNewDerivesRegionAndPublicBaseURLFromEndpoint(t *testing.T) {
	client, err := New(Config{
		Endpoint:       "https://s3.eu-central-003.backblazeb2.com",
		Region:         "s3.eu-central-003.backblazeb2.com",
		Bucket:         "seoBlog",
		KeyID:          "key-id",
		ApplicationKey: "application-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.region != "eu-central-003" {
		t.Fatalf("expected endpoint-derived region, got %q", client.region)
	}
	if got := client.PublicURL("blogSEO/projects/project-a/media/variants/asset-a/square_1x1.jpg"); got != "https://s3.eu-central-003.backblazeb2.com/seoBlog/blogSEO/projects/project-a/media/variants/asset-a/square_1x1.jpg" {
		t.Fatalf("unexpected derived public URL %q", got)
	}
}

func TestDeleteObjectPermanentlyDeletesEveryB2Version(t *testing.T) {
	client, err := New(Config{
		Endpoint:       "https://s3.us-west-004.backblazeb2.com",
		Region:         "us-west-004",
		Bucket:         "media-bucket",
		KeyID:          "key-id",
		ApplicationKey: "application-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	objectKey := "blogSEO/projects/project-a/media/variants/asset-a/square_1x1.jpg"
	var listRequests []*http.Request
	deletedVersions := map[string]int{}
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if !strings.HasPrefix(request.Header.Get("Authorization"), awsAlgorithm+" Credential=key-id/") {
			t.Fatalf("expected signed B2 request, got %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("X-Amz-Content-Sha256") == "" || request.Header.Get("X-Amz-Date") == "" {
			t.Fatalf("expected signed B2 headers, got %#v", request.Header)
		}
		switch request.Method {
		case http.MethodGet:
			listRequests = append(listRequests, request.Clone(request.Context()))
			if request.URL.EscapedPath() != "/media-bucket" {
				t.Fatalf("unexpected version-list path %q", request.URL.EscapedPath())
			}
			if request.URL.Query().Get("prefix") != objectKey {
				t.Fatalf("unexpected version-list prefix %q", request.URL.Query().Get("prefix"))
			}
			if _, exists := request.URL.Query()["versions"]; !exists {
				t.Fatalf("expected versions query, got %q", request.URL.RawQuery)
			}
			body := `<?xml version="1.0" encoding="UTF-8"?>
<ListVersionsResult>
  <IsTruncated>true</IsTruncated>
  <NextKeyMarker>` + objectKey + `</NextKeyMarker>
  <NextVersionIdMarker>upload-version</NextVersionIdMarker>
  <Version><Key>` + objectKey + `</Key><VersionId>upload-version</VersionId></Version>
  <Version><Key>` + objectKey + `.other</Key><VersionId>different-key-version</VersionId></Version>
</ListVersionsResult>`
			if len(listRequests) == 2 {
				if request.URL.Query().Get("key-marker") != objectKey || request.URL.Query().Get("version-id-marker") != "upload-version" {
					t.Fatalf("unexpected pagination query %q", request.URL.RawQuery)
				}
				body = `<?xml version="1.0" encoding="UTF-8"?>
<ListVersionsResult>
  <IsTruncated>false</IsTruncated>
  <DeleteMarker><Key>` + objectKey + `</Key><VersionId>delete-marker-version</VersionId></DeleteMarker>
</ListVersionsResult>`
			}
			return testHTTPResponse(request, http.StatusOK, body), nil
		case http.MethodDelete:
			if request.URL.EscapedPath() != "/media-bucket/"+objectKey {
				t.Fatalf("unexpected B2 delete path %q", request.URL.EscapedPath())
			}
			versionID := request.URL.Query().Get("versionId")
			if versionID == "" {
				t.Fatalf("expected permanent delete with versionId, got %q", request.URL.RawQuery)
			}
			deletedVersions[versionID]++
			status := http.StatusNoContent
			if versionID == "delete-marker-version" {
				status = http.StatusNotFound
			}
			return testHTTPResponse(request, status, ""), nil
		default:
			t.Fatalf("unexpected B2 method %s", request.Method)
			return nil, nil
		}
	})}

	if err := client.DeleteObject(context.Background(), objectKey); err != nil {
		t.Fatal(err)
	}
	if len(listRequests) != 2 {
		t.Fatalf("expected two paginated version-list requests, got %d", len(listRequests))
	}
	if deletedVersions["upload-version"] != 1 || deletedVersions["delete-marker-version"] != 1 {
		t.Fatalf("expected every exact-key version to be permanently deleted, got %#v", deletedVersions)
	}
	if deletedVersions["different-key-version"] != 0 {
		t.Fatalf("expected prefix sibling to remain untouched, got %#v", deletedVersions)
	}
}

func TestDeleteObjectAcceptsObjectWithoutVersions(t *testing.T) {
	client, err := New(Config{
		Endpoint:       "https://s3.us-west-004.backblazeb2.com",
		Region:         "us-west-004",
		Bucket:         "media-bucket",
		KeyID:          "key-id",
		ApplicationKey: "application-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet {
			t.Fatalf("expected only a version-list request, got %s", request.Method)
		}
		return testHTTPResponse(request, http.StatusOK, `<ListVersionsResult><IsTruncated>false</IsTruncated></ListVersionsResult>`), nil
	})}
	if err := client.DeleteObject(context.Background(), "blogSEO/pending/project-a/asset-a/file.png"); err != nil {
		t.Fatal(err)
	}
}

func testHTTPResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

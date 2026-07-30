package b2

import (
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

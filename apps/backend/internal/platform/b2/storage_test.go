package b2

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPresignPostBuildsB2S3CompatibleUploadPolicy(t *testing.T) {
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
	signed, err := client.PresignPost("blogSEO/projects/project-a/media/originals/hero image.png", "image/png", 2048, time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(signed.URL)
	if err != nil {
		t.Fatal(err)
	}
	if signed.Method != "POST" || signed.MaxBytes != 2048 {
		t.Fatalf("unexpected signed target %#v", signed)
	}
	if parsed.EscapedPath() != "/media-bucket" {
		t.Fatalf("unexpected bucket path %q", parsed.EscapedPath())
	}
	if signed.Fields["key"] != "blogSEO/projects/project-a/media/originals/hero image.png" ||
		signed.Fields["Content-Type"] != "image/png" ||
		signed.Fields["x-amz-algorithm"] != awsAlgorithm ||
		!strings.Contains(signed.Fields["x-amz-credential"], "/us-west-004/s3/aws4_request") ||
		signed.Fields["x-amz-signature"] == "" {
		t.Fatalf("unexpected signed fields %#v", signed.Fields)
	}
	rawPolicy, err := base64.StdEncoding.DecodeString(signed.Fields["policy"])
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		Conditions []any `json:"conditions"`
	}
	if err := json.Unmarshal(rawPolicy, &policy); err != nil {
		t.Fatal(err)
	}
	policyJSON := string(rawPolicy)
	if !strings.Contains(policyJSON, `"content-length-range",1,2048`) ||
		!strings.Contains(policyJSON, `"bucket":"media-bucket"`) ||
		!strings.Contains(policyJSON, `"x-amz-server-side-encryption":"AES256"`) {
		t.Fatalf("expected bounded upload policy, got %s", policyJSON)
	}
}

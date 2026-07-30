package media

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestProcessUploadExtractsImageMetadataAndGeneratesVariants(t *testing.T) {
	body := testPNG(t, 8, 6)
	result, err := ProcessUpload("hero.png", "image/png", body, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.ContentType != "image/png" || result.Width != 8 || result.Height != 6 || len(result.SHA256) != 64 {
		t.Fatalf("unexpected image result %#v", result)
	}
	if len(result.Variants) != 3 {
		t.Fatalf("expected 3 generated variants, got %d", len(result.Variants))
	}
	for _, variant := range result.Variants {
		if variant.ContentType != "image/jpeg" || len(variant.Bytes) == 0 || variant.Width <= 0 || variant.Height <= 0 {
			t.Fatalf("unexpected variant %#v", variant)
		}
	}
	var metadata map[string]any
	if err := json.Unmarshal(result.Metadata, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["variantGeneration"] != "generated" || metadata["metadataPolicy"] == "" {
		t.Fatalf("expected variant and metadata policy details, got %#v", metadata)
	}
}

func TestProcessUploadRejectsDisguisedSVGAndActivePDF(t *testing.T) {
	cases := []struct {
		name        string
		filename    string
		contentType string
		body        []byte
	}{
		{
			name:        "svg",
			filename:    "hero.png",
			contentType: "image/png",
			body:        []byte(`<svg><script>alert(1)</script></svg>`),
		},
		{
			name:        "pdf javascript",
			filename:    "brief.pdf",
			contentType: "application/pdf",
			body:        []byte("%PDF-1.7\n1 0 obj << /OpenAction << /S /JavaScript /JS (app.alert(1)) >> >> endobj\n"),
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := ProcessUpload(testCase.filename, testCase.contentType, testCase.body, Options{}); !errors.Is(err, ErrUnsafeUpload) {
				t.Fatalf("expected unsafe upload error, got %v", err)
			}
		})
	}
}

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	image := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			image.Set(x, y, color.RGBA{R: uint8(20 * x), G: uint8(20 * y), B: 120, A: 255})
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, image); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

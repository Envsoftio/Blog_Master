package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	defaultMaxBytes     = 50 * 1024 * 1024
	defaultMaxPixels    = 40_000_000
	defaultMaxGIFFrames = 180
)

var (
	ErrUnsafeUpload = errors.New("unsafe media upload")
	pdfPagePattern  = regexp.MustCompile(`/Type\s*/Page\b`)
)

type Options struct {
	MaxBytes     int64
	MaxPixels    int64
	MaxGIFFrames int
}

type Result struct {
	ContentType string
	SHA256      string
	Width       int
	Height      int
	Metadata    json.RawMessage
	Variants    []Variant
}

type Variant struct {
	Name        string
	ContentType string
	Width       int
	Height      int
	Bytes       []byte
}

type UnsafeError struct {
	Reason string
}

func (e UnsafeError) Error() string {
	return ErrUnsafeUpload.Error() + ": " + e.Reason
}

func (e UnsafeError) Is(target error) bool {
	return target == ErrUnsafeUpload
}

func ProcessUpload(filename, declaredContentType string, body []byte, options Options) (Result, error) {
	options = normalizeOptions(options)
	if len(body) == 0 {
		return Result{}, unsafe("empty upload")
	}
	if int64(len(body)) > options.MaxBytes {
		return Result{}, unsafe(fmt.Sprintf("upload exceeds %d bytes", options.MaxBytes))
	}
	declaredContentType = normalizeContentType(declaredContentType)
	actualContentType, err := sniffContentType(body)
	if err != nil {
		return Result{}, err
	}
	if declaredContentType != "" && actualContentType != declaredContentType {
		return Result{}, unsafe("detected content type does not match the registered content type")
	}
	if !extensionMatches(filename, actualContentType) {
		return Result{}, unsafe("filename extension does not match detected content type")
	}

	sum := sha256.Sum256(body)
	result := Result{
		ContentType: actualContentType,
		SHA256:      hex.EncodeToString(sum[:]),
	}
	metadata := map[string]any{
		"detectedContentType": actualContentType,
		"byteSize":            len(body),
		"scanner":             "builtin-deep-scan-v1",
	}
	switch {
	case strings.HasPrefix(actualContentType, "image/"):
		width, height, imageMetadata, err := inspectImage(actualContentType, body, options)
		if err != nil {
			return Result{}, err
		}
		result.Width = width
		result.Height = height
		for key, value := range imageMetadata {
			metadata[key] = value
		}
		if canGenerateVariants(actualContentType) {
			variants, err := GenerateVariants(body)
			if err != nil {
				return Result{}, fmt.Errorf("generate image variants: %w", err)
			}
			result.Variants = variants
			metadata["variantGeneration"] = "generated"
			metadata["metadataPolicy"] = "derivatives_reencoded_without_exif"
		} else {
			metadata["variantGeneration"] = "skipped_decoder_unavailable"
		}
	case actualContentType == "application/pdf":
		pdfMetadata, err := inspectPDF(body)
		if err != nil {
			return Result{}, err
		}
		for key, value := range pdfMetadata {
			metadata[key] = value
		}
	default:
		return Result{}, unsafe("unsupported content type")
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return Result{}, err
	}
	result.Metadata = encoded
	return result, nil
}

func GenerateVariants(body []byte) ([]Variant, error) {
	source, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	targets := []struct {
		name   string
		width  int
		height int
	}{
		{name: "square_1x1", width: 1200, height: 1200},
		{name: "landscape_4x3", width: 1200, height: 900},
		{name: "widescreen_16x9", width: 1600, height: 900},
	}
	var variants []Variant
	for _, target := range targets {
		image := coverResize(source, target.width, target.height)
		var output bytes.Buffer
		if err := jpeg.Encode(&output, image, &jpeg.Options{Quality: 84}); err != nil {
			return nil, err
		}
		variants = append(variants, Variant{
			Name:        target.name,
			ContentType: "image/jpeg",
			Width:       target.width,
			Height:      target.height,
			Bytes:       output.Bytes(),
		})
	}
	return variants, nil
}

func normalizeOptions(options Options) Options {
	if options.MaxBytes <= 0 {
		options.MaxBytes = defaultMaxBytes
	}
	if options.MaxPixels <= 0 {
		options.MaxPixels = defaultMaxPixels
	}
	if options.MaxGIFFrames <= 0 {
		options.MaxGIFFrames = defaultMaxGIFFrames
	}
	return options
}

func sniffContentType(body []byte) (string, error) {
	sample := body
	if len(sample) > 2048 {
		sample = sample[:2048]
	}
	trimmed := bytes.TrimSpace(sample)
	lowerSample := bytes.ToLower(sample)
	switch {
	case bytes.HasPrefix(body, []byte{0xff, 0xd8, 0xff}):
		return "image/jpeg", nil
	case bytes.HasPrefix(body, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return "image/png", nil
	case bytes.HasPrefix(body, []byte("GIF87a")) || bytes.HasPrefix(body, []byte("GIF89a")):
		return "image/gif", nil
	case len(body) >= 12 && bytes.Equal(body[:4], []byte("RIFF")) && bytes.Equal(body[8:12], []byte("WEBP")):
		return "image/webp", nil
	case bytes.HasPrefix(trimmed, []byte("%PDF-")):
		return "application/pdf", nil
	case bytes.Contains(lowerSample, []byte("<svg")) || bytes.Contains(lowerSample, []byte("<!doctype svg")):
		return "", unsafe("SVG uploads require a dedicated sanitizer")
	case bytes.HasPrefix(body, []byte("PK\x03\x04")) || bytes.HasPrefix(body, []byte("PK\x05\x06")) || bytes.HasPrefix(body, []byte("PK\x07\x08")):
		return "", unsafe("archive uploads are not enabled for the media library")
	}
	detected := normalizeContentType(http.DetectContentType(sample))
	if detected == "text/xml" || detected == "text/html" || strings.HasPrefix(detected, "text/") {
		return "", unsafe("text-like uploads are not enabled for the media library")
	}
	return "", unsafe("could not identify a supported media type")
}

func inspectImage(contentType string, body []byte, options Options) (int, int, map[string]any, error) {
	metadata := map[string]any{}
	var width, height int
	if contentType == "image/webp" {
		var err error
		width, height, metadata, err = inspectWebP(body)
		if err != nil {
			return 0, 0, nil, err
		}
	} else {
		config, format, err := image.DecodeConfig(bytes.NewReader(body))
		if err != nil {
			return 0, 0, nil, unsafe("image header could not be decoded")
		}
		width = config.Width
		height = config.Height
		metadata["imageFormat"] = format
	}
	if width <= 0 || height <= 0 {
		return 0, 0, nil, unsafe("image dimensions are missing")
	}
	pixels := int64(width) * int64(height)
	if pixels > options.MaxPixels {
		return 0, 0, nil, unsafe(fmt.Sprintf("image exceeds %d decoded pixels", options.MaxPixels))
	}
	metadata["width"] = width
	metadata["height"] = height
	metadata["pixelCount"] = pixels
	if contentType == "image/gif" {
		frames, err := countGIFFrames(body)
		if err != nil {
			return 0, 0, nil, err
		}
		if frames > options.MaxGIFFrames {
			return 0, 0, nil, unsafe(fmt.Sprintf("animated GIF exceeds %d frames", options.MaxGIFFrames))
		}
		metadata["frameCount"] = frames
	}
	for key, value := range passiveImageMetadata(contentType, body) {
		metadata[key] = value
	}
	return width, height, metadata, nil
}

func inspectPDF(body []byte) (map[string]any, error) {
	trimmed := bytes.TrimLeft(body, "\xef\xbb\xbf\r\n\t ")
	if !bytes.HasPrefix(trimmed, []byte("%PDF-")) {
		return nil, unsafe("PDF header is not at the beginning of the upload")
	}
	lower := bytes.ToLower(body)
	blockedTokens := []string{
		"/javascript",
		"/js",
		"/openaction",
		"/aa",
		"/launch",
		"/embeddedfile",
		"/richmedia",
		"/xfa",
	}
	for _, token := range blockedTokens {
		if bytes.Contains(lower, []byte(token)) {
			return nil, unsafe("PDF contains active or embedded content")
		}
	}
	return map[string]any{
		"documentFormat": "pdf",
		"pageCountHint":  len(pdfPagePattern.FindAll(body, -1)),
		"scanner":        "builtin-pdf-active-content-scan-v1",
	}, nil
}

func inspectWebP(body []byte) (int, int, map[string]any, error) {
	offset := 12
	metadata := map[string]any{"imageFormat": "webp"}
	for offset+8 <= len(body) {
		chunkType := string(body[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(body[offset+4 : offset+8]))
		dataStart := offset + 8
		dataEnd := dataStart + chunkSize
		if chunkSize < 0 || dataEnd > len(body) {
			return 0, 0, nil, unsafe("WEBP chunk exceeds upload bounds")
		}
		chunk := body[dataStart:dataEnd]
		switch chunkType {
		case "VP8X":
			if len(chunk) < 10 {
				return 0, 0, nil, unsafe("WEBP extended header is truncated")
			}
			width := 1 + int(chunk[4]) + int(chunk[5])<<8 + int(chunk[6])<<16
			height := 1 + int(chunk[7]) + int(chunk[8])<<8 + int(chunk[9])<<16
			metadata["webpProfile"] = "extended"
			return width, height, metadata, nil
		case "VP8L":
			if len(chunk) < 5 || chunk[0] != 0x2f {
				return 0, 0, nil, unsafe("WEBP lossless header is invalid")
			}
			width := 1 + int(chunk[1]) + int(chunk[2]&0x3f)<<8
			height := 1 + int(chunk[2]>>6) + int(chunk[3])<<2 + int(chunk[4]&0x0f)<<10
			metadata["webpProfile"] = "lossless"
			return width, height, metadata, nil
		case "VP8 ":
			if len(chunk) < 10 || chunk[3] != 0x9d || chunk[4] != 0x01 || chunk[5] != 0x2a {
				return 0, 0, nil, unsafe("WEBP lossy header is invalid")
			}
			width := int(binary.LittleEndian.Uint16(chunk[6:8]) & 0x3fff)
			height := int(binary.LittleEndian.Uint16(chunk[8:10]) & 0x3fff)
			metadata["webpProfile"] = "lossy"
			return width, height, metadata, nil
		case "EXIF":
			metadata["exifPresent"] = true
		case "XMP ":
			metadata["xmpPresent"] = true
		}
		offset = dataEnd
		if offset%2 == 1 {
			offset++
		}
	}
	return 0, 0, nil, unsafe("WEBP dimensions were not found")
}

func countGIFFrames(body []byte) (int, error) {
	if len(body) < 13 {
		return 0, unsafe("GIF header is truncated")
	}
	offset := 13
	if body[10]&0x80 != 0 {
		offset += 3 * (1 << ((body[10] & 0x07) + 1))
	}
	frames := 0
	for offset < len(body) {
		switch body[offset] {
		case 0x2c:
			frames++
			offset += 10
			if offset > len(body) {
				return 0, unsafe("GIF image descriptor is truncated")
			}
			if body[offset-1]&0x80 != 0 {
				offset += 3 * (1 << ((body[offset-1] & 0x07) + 1))
			}
			if offset >= len(body) {
				return 0, unsafe("GIF image data is truncated")
			}
			offset++
			next, err := skipSubBlocks(body, offset)
			if err != nil {
				return 0, err
			}
			offset = next
		case 0x21:
			offset += 2
			next, err := skipSubBlocks(body, offset)
			if err != nil {
				return 0, err
			}
			offset = next
		case 0x3b:
			return frames, nil
		default:
			return 0, unsafe("GIF contains an invalid block")
		}
	}
	return 0, unsafe("GIF trailer is missing")
}

func skipSubBlocks(body []byte, offset int) (int, error) {
	for offset < len(body) {
		size := int(body[offset])
		offset++
		if size == 0 {
			return offset, nil
		}
		offset += size
		if offset > len(body) {
			return 0, unsafe("GIF sub-block is truncated")
		}
	}
	return 0, unsafe("GIF sub-block terminator is missing")
}

func passiveImageMetadata(contentType string, body []byte) map[string]any {
	metadata := map[string]any{}
	switch contentType {
	case "image/jpeg":
		app1, gps := jpegMetadataHints(body)
		if app1 {
			metadata["exifPresent"] = true
		}
		if gps {
			metadata["gpsMetadataHint"] = true
		}
	case "image/png":
		textChunks := pngTextChunkCount(body)
		if textChunks > 0 {
			metadata["pngTextChunkCount"] = textChunks
		}
	}
	return metadata
}

func jpegMetadataHints(body []byte) (bool, bool) {
	if len(body) < 4 || body[0] != 0xff || body[1] != 0xd8 {
		return false, false
	}
	offset := 2
	app1 := false
	gps := false
	for offset+4 <= len(body) {
		if body[offset] != 0xff {
			break
		}
		marker := body[offset+1]
		offset += 2
		if marker == 0xda || marker == 0xd9 {
			break
		}
		if offset+2 > len(body) {
			break
		}
		length := int(binary.BigEndian.Uint16(body[offset : offset+2]))
		if length < 2 || offset+length > len(body) {
			break
		}
		segment := body[offset+2 : offset+length]
		if marker == 0xe1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			app1 = true
			if bytes.Contains(bytes.ToLower(segment), []byte("gps")) {
				gps = true
			}
		}
		offset += length
	}
	return app1, gps
}

func pngTextChunkCount(body []byte) int {
	if len(body) < 8 || !bytes.Equal(body[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return 0
	}
	count := 0
	offset := 8
	for offset+8 <= len(body) {
		length := int(binary.BigEndian.Uint32(body[offset : offset+4]))
		chunkType := string(body[offset+4 : offset+8])
		offset += 8
		if length < 0 || offset+length+4 > len(body) {
			return count
		}
		if chunkType == "tEXt" || chunkType == "iTXt" || chunkType == "zTXt" {
			count++
		}
		offset += length + 4
	}
	return count
}

func coverResize(source image.Image, width, height int) image.Image {
	bounds := source.Bounds()
	sourceWidth := bounds.Dx()
	sourceHeight := bounds.Dy()
	sourceRatio := float64(sourceWidth) / float64(sourceHeight)
	targetRatio := float64(width) / float64(height)
	crop := bounds
	if sourceRatio > targetRatio {
		cropWidth := int(math.Round(float64(sourceHeight) * targetRatio))
		x0 := bounds.Min.X + (sourceWidth-cropWidth)/2
		crop = image.Rect(x0, bounds.Min.Y, x0+cropWidth, bounds.Max.Y)
	} else if sourceRatio < targetRatio {
		cropHeight := int(math.Round(float64(sourceWidth) / targetRatio))
		y0 := bounds.Min.Y + (sourceHeight-cropHeight)/2
		crop = image.Rect(bounds.Min.X, y0, bounds.Max.X, y0+cropHeight)
	}
	destination := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(destination, destination.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	for y := 0; y < height; y++ {
		sourceY := crop.Min.Y + int(float64(y)*float64(crop.Dy())/float64(height))
		if sourceY >= crop.Max.Y {
			sourceY = crop.Max.Y - 1
		}
		for x := 0; x < width; x++ {
			sourceX := crop.Min.X + int(float64(x)*float64(crop.Dx())/float64(width))
			if sourceX >= crop.Max.X {
				sourceX = crop.Max.X - 1
			}
			destination.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	return destination
}

func canGenerateVariants(contentType string) bool {
	return contentType == "image/jpeg" || contentType == "image/png" || contentType == "image/gif"
}

func extensionMatches(filename, contentType string) bool {
	extension := strings.ToLower(filepath.Ext(filename))
	switch contentType {
	case "image/jpeg":
		return extension == ".jpg" || extension == ".jpeg"
	case "image/png":
		return extension == ".png"
	case "image/webp":
		return extension == ".webp"
	case "image/gif":
		return extension == ".gif"
	case "application/pdf":
		return extension == ".pdf"
	default:
		return false
	}
}

func normalizeContentType(value string) string {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	if value == "image/jpg" {
		return "image/jpeg"
	}
	return value
}

func unsafe(reason string) error {
	return UnsafeError{Reason: reason}
}

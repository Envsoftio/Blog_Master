package store

import (
	"encoding/json"
	"net/url"
	"slices"
	"strings"
)

type publishedHeroReference struct {
	AssetID    string
	AltText    string
	Decorative bool
}

func publishedMediaFromSnapshot(raw string) PublishedMedia {
	media := PublishedMedia{}
	if err := json.Unmarshal([]byte(raw), &media); err != nil || media.Hero == nil {
		return PublishedMedia{}
	}
	hero := media.Hero
	hero.ID = strings.TrimSpace(hero.ID)
	hero.URL = safePublishedMediaURL(hero.URL)
	hero.MIMEType = strings.TrimSpace(hero.MIMEType)
	if hero.ID == "" || hero.URL == "" || hero.Width <= 0 || hero.Height <= 0 || hero.MIMEType == "" {
		return PublishedMedia{}
	}
	variants := make([]PublishedMediaVariant, 0, len(hero.Variants))
	for _, variant := range hero.Variants {
		variant.Name = strings.TrimSpace(variant.Name)
		variant.URL = safePublishedMediaURL(variant.URL)
		variant.MIMEType = strings.TrimSpace(variant.MIMEType)
		if variant.Name == "" || variant.URL == "" || variant.MIMEType == "" || variant.Width <= 0 || variant.Height <= 0 {
			continue
		}
		variants = append(variants, variant)
	}
	hero.Variants = variants
	return media
}

// firstPublishedHeroReference treats the first project image in article order
// as the implicit hero when an older revision has no explicit media snapshot.
// This keeps already-published articles compatible with consumers that have
// always implemented the typed media.hero contract.
func firstPublishedHeroReference(document any, projectID string) publishedHeroReference {
	node, ok := document.(map[string]any)
	if !ok {
		return publishedHeroReference{}
	}
	if strings.EqualFold(stringFromMap(node, "type", ""), "image") {
		attrs, _ := node["attrs"].(map[string]any)
		assetID := strings.TrimSpace(stringFromMap(attrs, "assetId", ""))
		if assetID == "" {
			assetID = projectMediaAssetID(stringFromMap(attrs, "src", ""), projectID)
		}
		if assetID != "" {
			decorative, _ := attrs["decorative"].(bool)
			return publishedHeroReference{
				AssetID:    assetID,
				AltText:    strings.TrimSpace(stringFromMap(attrs, "alt", "")),
				Decorative: decorative,
			}
		}
	}
	children, _ := node["content"].([]any)
	for _, child := range children {
		if reference := firstPublishedHeroReference(child, projectID); reference.AssetID != "" {
			return reference
		}
	}
	return publishedHeroReference{}
}

func projectMediaAssetID(raw, projectID string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 7 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "projects" || parts[4] != "media" || parts[6] != "file" {
		return ""
	}
	resolvedProjectID, err := url.PathUnescape(parts[3])
	if err != nil || resolvedProjectID != projectID {
		return ""
	}
	assetID, err := url.PathUnescape(parts[5])
	if err != nil {
		return ""
	}
	return strings.TrimSpace(assetID)
}

func publishedHeroFromAsset(asset AdminMediaAsset, reference publishedHeroReference) *PublishedMediaAsset {
	if asset.Status != "ready" || !strings.HasPrefix(asset.ContentType, "image/") || asset.Width <= 0 || asset.Height <= 0 {
		return nil
	}
	hero := &PublishedMediaAsset{
		ID:         asset.ID,
		URL:        publishedMediaFileURL(asset.ProjectID, asset.ID, ""),
		MIMEType:   asset.ContentType,
		Width:      asset.Width,
		Height:     asset.Height,
		AltText:    asset.AltText,
		Decorative: asset.Decorative,
		Caption:    asset.Caption,
		Credit:     asset.Credit,
		License:    asset.License,
		Variants:   []PublishedMediaVariant{},
	}
	if reference.AltText != "" || reference.Decorative {
		hero.AltText = reference.AltText
		hero.Decorative = reference.Decorative
	}
	for _, variant := range asset.Variants {
		if variant.Name == "" || variant.ContentType == "" || variant.Width <= 0 || variant.Height <= 0 {
			continue
		}
		hero.Variants = append(hero.Variants, PublishedMediaVariant{
			Name:     variant.Name,
			URL:      publishedMediaFileURL(asset.ProjectID, asset.ID, variant.Name),
			MIMEType: variant.ContentType,
			Width:    variant.Width,
			Height:   variant.Height,
		})
	}
	for _, preferredName := range []string{"widescreen_16x9", "landscape_4x3", "square_1x1"} {
		index := slices.IndexFunc(hero.Variants, func(variant PublishedMediaVariant) bool {
			return variant.Name == preferredName
		})
		if index >= 0 {
			preferred := hero.Variants[index]
			hero.URL = preferred.URL
			hero.MIMEType = preferred.MIMEType
			hero.Width = preferred.Width
			hero.Height = preferred.Height
			break
		}
	}
	return hero
}

func publishedMediaFileURL(projectID, assetID, variantName string) string {
	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/media/" + url.PathEscape(assetID) + "/file"
	if strings.TrimSpace(variantName) == "" {
		return path
	}
	return path + "?variant=" + url.QueryEscape(variantName)
}

func safePublishedMediaURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n\\") {
		return ""
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	if value, ok := safePublishedAbsoluteURL(raw); ok {
		return value
	}
	return ""
}

func safePublishedAbsoluteURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n\\") {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || strings.ToLower(parsed.Scheme) != "https" {
		return "", false
	}
	parsed.Scheme = "https"
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), true
}

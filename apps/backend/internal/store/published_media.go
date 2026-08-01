package store

import (
	"encoding/json"
	"strings"
)

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

func safePublishedMediaURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n\\") {
		return ""
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	if value, ok := safeStructuredDataURL(raw, true); ok {
		return value
	}
	return ""
}

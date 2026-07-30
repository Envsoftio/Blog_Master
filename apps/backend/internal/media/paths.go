package media

import "strings"

const ObjectRootPrefix = "blogSEO"

func OriginalObjectKeyPrefix(projectID, assetID string) string {
	return ObjectRootPrefix + "/pending/" + projectID + "/" + assetID
}

func ProcessedOriginalObjectKey(projectID, assetID, filename string) string {
	return processedOriginalObjectKeyPrefix(projectID, assetID) + safeObjectFilename(filename)
}

func VariantObjectKey(projectID, assetID, variantName string) string {
	variantName = strings.Trim(strings.ToLower(variantName), ".-_/")
	if variantName == "" {
		variantName = "variant"
	}
	return ObjectRootPrefix + "/projects/" + projectID + "/media/variants/" + assetID + "/" + variantName + ".jpg"
}

func DeletableObjectKeyForAsset(projectID, assetID, key string) bool {
	projectID = strings.TrimSpace(projectID)
	assetID = strings.TrimSpace(assetID)
	key = strings.TrimSpace(key)
	if projectID == "" || assetID == "" || key == "" || strings.Contains(key, "\\") || hasUnsafeObjectKeySegment(key) {
		return false
	}
	for _, prefix := range []string{
		OriginalObjectKeyPrefix(projectID, assetID) + "/",
		processedOriginalObjectKeyPrefix(projectID, assetID),
		variantObjectKeyPrefix(projectID, assetID),
	} {
		if suffix := strings.TrimPrefix(key, prefix); suffix != key && suffix != "" && !strings.Contains(suffix, "/") {
			return true
		}
	}
	return false
}

func processedOriginalObjectKeyPrefix(projectID, assetID string) string {
	return ObjectRootPrefix + "/projects/" + projectID + "/media/originals/" + assetID + "/"
}

func variantObjectKeyPrefix(projectID, assetID string) string {
	return ObjectRootPrefix + "/projects/" + projectID + "/media/variants/" + assetID + "/"
}

func safeObjectFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(character rune) rune {
		switch {
		case character >= 'a' && character <= 'z':
			return character
		case character >= 'A' && character <= 'Z':
			return character
		case character >= '0' && character <= '9':
			return character
		case character == '.', character == '-', character == '_':
			return character
		default:
			return '-'
		}
	}, value)
	value = strings.Trim(value, ".-_")
	if value == "" {
		return "asset"
	}
	return value
}

func hasUnsafeObjectKeySegment(key string) bool {
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return true
		}
	}
	return false
}

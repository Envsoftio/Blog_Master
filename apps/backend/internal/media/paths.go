package media

import "strings"

const ObjectRootPrefix = "blogSEO"

func OriginalObjectKeyPrefix(projectID, assetID string) string {
	return ObjectRootPrefix + "/pending/" + projectID + "/" + assetID
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
		variantObjectKeyPrefix(projectID, assetID),
	} {
		if suffix := strings.TrimPrefix(key, prefix); suffix != key && suffix != "" && !strings.Contains(suffix, "/") {
			return true
		}
	}
	return false
}

func variantObjectKeyPrefix(projectID, assetID string) string {
	return ObjectRootPrefix + "/projects/" + projectID + "/media/variants/" + assetID + "/"
}

func hasUnsafeObjectKeySegment(key string) bool {
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return true
		}
	}
	return false
}

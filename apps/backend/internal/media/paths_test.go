package media

import "testing"

func TestDeletableObjectKeyForAsset(t *testing.T) {
	projectID := "project_123"
	assetID := "asset_456"

	cases := []struct {
		name string
		key  string
		want bool
	}{
		{
			name: "original file",
			key:  OriginalObjectKeyPrefix(projectID, assetID) + "/hero.png",
			want: true,
		},
		{
			name: "variant file",
			key:  VariantObjectKey(projectID, assetID, "square_1x1"),
			want: true,
		},
		{
			name: "root folder",
			key:  ObjectRootPrefix + "/",
			want: false,
		},
		{
			name: "project pending folder",
			key:  ObjectRootPrefix + "/pending/" + projectID + "/",
			want: false,
		},
		{
			name: "asset pending folder",
			key:  OriginalObjectKeyPrefix(projectID, assetID) + "/",
			want: false,
		},
		{
			name: "other asset original",
			key:  OriginalObjectKeyPrefix(projectID, "asset_other") + "/hero.png",
			want: false,
		},
		{
			name: "other project original",
			key:  OriginalObjectKeyPrefix("project_other", assetID) + "/hero.png",
			want: false,
		},
		{
			name: "other asset variant",
			key:  VariantObjectKey(projectID, "asset_other", "square_1x1"),
			want: false,
		},
		{
			name: "unsafe parent segment",
			key:  OriginalObjectKeyPrefix(projectID, assetID) + "/../hero.png",
			want: false,
		},
		{
			name: "unsafe empty segment",
			key:  OriginalObjectKeyPrefix(projectID, assetID) + "//hero.png",
			want: false,
		},
		{
			name: "nested file under asset prefix",
			key:  OriginalObjectKeyPrefix(projectID, assetID) + "/nested/hero.png",
			want: false,
		},
		{
			name: "unsafe backslash",
			key:  OriginalObjectKeyPrefix(projectID, assetID) + "/hero\\image.png",
			want: false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := DeletableObjectKeyForAsset(projectID, assetID, testCase.key); got != testCase.want {
				t.Fatalf("DeletableObjectKeyForAsset() = %v, want %v", got, testCase.want)
			}
		})
	}
}

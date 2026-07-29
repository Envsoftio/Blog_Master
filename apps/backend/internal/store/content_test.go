package store

import "testing"

func TestDecodeJSONObjectRejectsNonObjects(t *testing.T) {
	for _, input := range []string{`[]`, `"text"`, `42`, `{broken`} {
		value := decodeJSONObject(input)
		if len(value) != 0 {
			t.Fatalf("expected empty object for %q, got %#v", input, value)
		}
	}
}

package httpapi

import (
	"encoding/json"
	"testing"
)

func TestListEnvelopeMarshalNilDataAsEmptyArray(t *testing.T) {
	body, err := json.Marshal(ListEnvelope[string]{Meta: PageMeta{Limit: 50}})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"data":[],"meta":{"limit":50}}` {
		t.Fatalf("expected nil list data to marshal as an empty array, got %s", body)
	}
}

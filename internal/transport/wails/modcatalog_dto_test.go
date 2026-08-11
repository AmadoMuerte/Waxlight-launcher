package wails

import (
	"encoding/json"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/mods"
)

func TestUploadModsResultDTOSerializesEmptyArrays(t *testing.T) {
	dto := uploadModsResultDTO(mods.UploadModsResult{})
	data, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"linked", "notMatched", "skipped", "failed"} {
		value, ok := parsed[key]
		if !ok {
			t.Fatalf("missing %q in %s", key, data)
		}
		arr, ok := value.([]any)
		if !ok || len(arr) != 0 {
			t.Fatalf("%q must serialize as an empty array, got %s", key, data)
		}
	}
}

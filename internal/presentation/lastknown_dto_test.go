package presentation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/recovery"
)

func TestLastKnownGoodDTOAlwaysSerializesChangeLists(t *testing.T) {
	dto := lastKnownGoodDTO(recovery.LastKnownGoodStatus{
		RecordedAt:  time.Now().UTC(),
		GameVersion: "1.22.5",
		ModCount:    4,
		Changes: recovery.ConfigurationChanges{
			GameVersionFrom: "1.21.5",
			GameVersionTo:   "1.22.5",
		},
	})
	data, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	changes, ok := parsed["changes"].(map[string]any)
	if !ok {
		t.Fatalf("changes must be an object, got %s", data)
	}
	for _, key := range []string{"updated", "added", "removed"} {
		value, ok := changes[key]
		if !ok {
			t.Fatalf("missing %q in %s", key, data)
		}
		arr, ok := value.([]any)
		if !ok || len(arr) != 0 {
			t.Fatalf("%q must serialize as an empty array, got %s", key, data)
		}
	}
}

func TestLastKnownGoodDTOWithoutMarkerHasEmptyRecordedAt(t *testing.T) {
	// An instance that never had a successful launch returns the zero status.
	// The frontend hides the section when recordedAt is empty; it must never
	// receive the zero Go time as a truthy string.
	dto := lastKnownGoodDTO(recovery.LastKnownGoodStatus{})
	data, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if value, ok := parsed["recordedAt"]; !ok || value != "" {
		t.Fatalf("recordedAt must be empty, got %s", data)
	}
	changes, ok := parsed["changes"].(map[string]any)
	if !ok {
		t.Fatalf("changes must be an object, got %s", data)
	}
	for _, key := range []string{"updated", "added", "removed"} {
		if _, ok := changes[key].([]any); !ok {
			t.Fatalf("%q must serialize as an array, got %s", key, data)
		}
	}
}

func TestConfigurationChangesJSONAlwaysHasModLists(t *testing.T) {
	// The domain type is published verbatim in the game:recovery-suggestion
	// event; its mod lists must survive JSON round trips as arrays. Producers
	// initialize the lists; the tags must never drop them.
	data, err := json.Marshal(recovery.ConfigurationChanges{
		Updated: []recovery.ModChange{},
		Added:   []recovery.ModChange{},
		Removed: []recovery.ModChange{},
	})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"updated", "added", "removed"} {
		if _, ok := parsed[key].([]any); !ok {
			t.Fatalf("%q must serialize as an array, got %s", key, data)
		}
	}
}

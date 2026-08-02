package filesystem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

func testAccount() domain.Account {
	return domain.Account{
		Username:         "Ada",
		UID:              "player-uid",
		SessionKey:       "session-key",
		SessionSignature: "session-signature",
	}
}

func TestPatchClientSettingsPreservesValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clientsettings.json")
	original := `{"intsettings":{"windowWidth":1920},"stringsettings":{"language":"en"}}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (ClientSettingsService{}).Patch(path, testAccount()); err != nil {
		t.Fatal(err)
	}
	var result map[string]map[string]any
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(contents, &result); err != nil {
		t.Fatal(err)
	}
	if result["intsettings"]["windowWidth"] != float64(1920) || result["stringsettings"]["language"] != "en" {
		t.Fatalf("existing values were not preserved: %#v", result)
	}
	for key, want := range map[string]string{
		"sessionkey":       "session-key",
		"sessionsignature": "session-signature",
		"playeruid":        "player-uid",
		"playername":       "Ada",
	} {
		if result["stringsettings"][key] != want {
			t.Fatalf("unexpected %s: %#v", key, result["stringsettings"][key])
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("unexpected permissions %o", info.Mode().Perm())
		}
	}
}

func TestPatchClientSettingsCreatesMissingStructures(t *testing.T) {
	for _, original := range []string{"", `{}`} {
		t.Run(original, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "nested", "clientsettings.json")
			if original != "" {
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := (ClientSettingsService{}).Patch(path, testAccount()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPatchClientSettingsRejectsInvalidInput(t *testing.T) {
	for _, original := range []string{
		`not json`,
		`[]`,
		`{"stringsettings":[]}`,
		`{"stringsettings":null}`,
	} {
		t.Run(original, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "clientsettings.json")
			if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := (ClientSettingsService{}).Patch(path, testAccount()); err == nil {
				t.Fatal("expected invalid settings error")
			}
		})
	}
}

func TestClearClientSettingsRemovesOnlyAuthentication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clientsettings.json")
	if err := (ClientSettingsService{}).Patch(path, testAccount()); err != nil {
		t.Fatal(err)
	}
	if err := (ClientSettingsService{}).Clear(path); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]map[string]any
	if err := json.Unmarshal(contents, &root); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"sessionkey", "sessionsignature", "playeruid", "playername"} {
		if _, ok := root["stringsettings"][key]; ok {
			t.Fatalf("authentication key %s was not removed", key)
		}
	}
}

func TestPatchClientSettingsReportsWriteError(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("block"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (ClientSettingsService{}).Patch(filepath.Join(parentFile, "clientsettings.json"), testAccount()); err == nil {
		t.Fatal("expected write error")
	}
}

package filesystem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	if _, err := (ClientSettingsService{}).Inject(path, testAccount()); err != nil {
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
			if _, err := (ClientSettingsService{}).Inject(path, testAccount()); err != nil {
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
			if _, err := (ClientSettingsService{}).Inject(path, testAccount()); err == nil {
				t.Fatal("expected invalid settings error")
			}
		})
	}
}

func TestClearClientSettingsRemovesOnlyAuthentication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clientsettings.json")
	if _, err := (ClientSettingsService{}).Inject(path, testAccount()); err != nil {
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
	if _, err := (ClientSettingsService{}).Inject(filepath.Join(parentFile, "clientsettings.json"), testAccount()); err == nil {
		t.Fatal("expected write error")
	}
}

func TestInjectionCleanupAndCrashReconciliation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clientsettings.json")
	cleanup, err := (ClientSettingsService{}).Inject(path, testAccount())
	if err != nil {
		t.Fatal(err)
	}
	journal := path + injectionJournalSuffix
	if runtime.GOOS != "windows" {
		info, err := os.Stat(journal)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("unsafe journal permissions: %v, %v", info, err)
		}
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("cleanup is not idempotent: %v", err)
	}
	contents, _ := os.ReadFile(path)
	if string(contents) == "" || stringContainsAuth(contents) {
		t.Fatalf("credentials remain after cleanup: %s", contents)
	}

	if _, err := (ClientSettingsService{}).Inject(path, testAccount()); err != nil {
		t.Fatal(err)
	}
	if err := (ClientSettingsService{}).Reconcile(path); err != nil {
		t.Fatal(err)
	}
	contents, _ = os.ReadFile(path)
	if stringContainsAuth(contents) {
		t.Fatalf("credentials remain after crash reconciliation: %s", contents)
	}
}

func TestClientSettingsRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "clientsettings.json")
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := (ClientSettingsService{}).Inject(path, testAccount()); err == nil {
		t.Fatal("symlink target was accepted")
	}
}

func stringContainsAuth(contents []byte) bool {
	text := string(contents)
	for _, value := range []string{"session-key", "session-signature", `"sessionkey"`, `"sessionsignature"`, `"playeruid"`, `"playername"`} {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func TestSanitizeClientSettingsRemovesAuthAndModPaths(t *testing.T) {
	original := `{"stringsettings":{"sessionkey":"s","playername":"Ada","language":"en"},"stringListSettings":{"multiplayerservers":[],"disabledMods":["m1"],"modPaths":["Mods","/home/user/.config/waxlight/instances/original/Mods"]},"intsettings":{"viewDistance":256}}`
	sanitized, err := SanitizeClientSettings([]byte(original))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(sanitized))
	for _, forbidden := range []string{"sessionkey", "playername", "modpaths", "instances/original", "session-key"} {
		if strings.Contains(lower, strings.ToLower(forbidden)) {
			t.Fatalf("sanitized settings still contain %q: %s", forbidden, sanitized)
		}
	}
	var result struct {
		StringSettings     map[string]string `json:"stringsettings"`
		StringListSettings json.RawMessage   `json:"stringListSettings"`
		IntSettings        map[string]int    `json:"intsettings"`
	}
	if err := json.Unmarshal(sanitized, &result); err != nil {
		t.Fatal(err)
	}
	if result.StringSettings["language"] != "en" {
		t.Fatalf("language was not preserved: %s", sanitized)
	}
	if result.IntSettings["viewDistance"] != 256 {
		t.Fatalf("int settings were not preserved: %s", sanitized)
	}
	var lists map[string]json.RawMessage
	if err := json.Unmarshal(result.StringListSettings, &lists); err != nil {
		t.Fatal(err)
	}
	if _, ok := lists["modPaths"]; ok {
		t.Fatalf("modPaths was not removed: %s", sanitized)
	}
	var disabledMods []string
	if err := json.Unmarshal(lists["disabledMods"], &disabledMods); err != nil || len(disabledMods) != 1 {
		t.Fatalf("disabledMods were not preserved: %s", sanitized)
	}
}

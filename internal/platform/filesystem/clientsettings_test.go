package filesystem

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
)

func testAccount() accounts.Account {
	return accounts.Account{
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
	original := `{"stringsettings":{"sessionkey":"s","sessionsignature":"sig","playeruid":"uid","playername":"Ada","useremail":"user@example.com","mptoken":"token","entitlements":"premium","language":"en"},"stringListSettings":{"multiplayerservers":["Just cozy server (:,192.0.2.1:42420,"],"disabledMods":["m1"],"modPaths":["Mods","/home/user/.config/waxlight/instances/original/Mods"]},"intsettings":{"viewDistance":256}}`
	sanitized, err := SanitizeClientSettings([]byte(original))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(sanitized))
	for _, forbidden := range []string{
		"sessionkey", "sessionsignature", "playeruid", "playername",
		"useremail", "mptoken", "entitlements", "modpaths",
		"instances/original", "session-key", "192.0.2.1",
	} {
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
	if _, ok := lists["multiplayerservers"]; ok {
		t.Fatalf("multiplayerservers was not removed: %s", sanitized)
	}
	var disabledMods []string
	if err := json.Unmarshal(lists["disabledMods"], &disabledMods); err != nil || len(disabledMods) != 1 {
		t.Fatalf("disabledMods were not preserved: %s", sanitized)
	}
}

func TestSanitizeClientSettingsStripsCamelCaseSections(t *testing.T) {
	original := `{"boolSettings":{"bloom":true},"floatSettings":{"guiScale":1.25},"intSettings":{"viewDistance":448},"stringListSettings":{"multiplayerservers":["Just cozy server (:,192.0.2.1:42420,"],"disabledMods":[],"modPaths":["Mods","/home/user/.config/VintagestoryData/Mods"]},"stringSettings":{"settingsVersion":"1.16","language":"ru","sessionkey":"redacted-session-key","sessionsignature":"redacted-signature","useremail":"player@example.com","entitlements":null,"mptoken":null,"playeruid":"redacted-uid","playername":"ExamplePlayer"}}`
	sanitized, err := SanitizeClientSettings([]byte(original))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(sanitized))
	for _, forbidden := range []string{
		"sessionkey", "sessionsignature", "playeruid", "playername",
		"useremail", "mptoken", "entitlements", "modpaths",
		"192.0.2.1", "redacted-session-key", "player@example.com",
	} {
		if strings.Contains(lower, strings.ToLower(forbidden)) {
			t.Fatalf("sanitized settings still contain %q: %s", forbidden, sanitized)
		}
	}
	var result struct {
		BoolSettings     map[string]bool `json:"boolSettings"`
		FloatSettings    map[string]any  `json:"floatSettings"`
		IntSettings      map[string]int  `json:"intSettings"`
		StringSettings   map[string]any  `json:"stringSettings"`
		StringListOffset json.RawMessage `json:"stringListSettings"`
	}
	if err := json.Unmarshal(sanitized, &result); err != nil {
		t.Fatal(err)
	}
	if result.StringSettings["language"] != "ru" || result.StringSettings["settingsVersion"] != "1.16" {
		t.Fatalf("camelCase string settings lost non-sensitive values: %s", sanitized)
	}
	if result.IntSettings["viewDistance"] != 448 || !result.BoolSettings["bloom"] {
		t.Fatalf("settings lost non-sensitive values: %s", sanitized)
	}
	var lists map[string]json.RawMessage
	if err := json.Unmarshal(result.StringListOffset, &lists); err != nil {
		t.Fatal(err)
	}
	if _, ok := lists["modPaths"]; ok {
		t.Fatalf("camelCase modPaths was not removed: %s", sanitized)
	}
	if _, ok := lists["multiplayerservers"]; ok {
		t.Fatalf("camelCase multiplayerservers was not removed: %s", sanitized)
	}
}

func TestSanitizeClientSettingsStripsBothSectionSpellings(t *testing.T) {
	original := `{"stringSettings":{"sessionkey":"camel","language":"en"},"stringsettings":{"sessionkey":"lower","fov":90}}`
	sanitized, err := SanitizeClientSettings([]byte(original))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(sanitized))
	if strings.Contains(lower, "sessionkey") || strings.Contains(lower, "camel") || strings.Contains(lower, "lower") {
		t.Fatalf("sanitized settings still contain credentials: %s", sanitized)
	}
	var result map[string]map[string]any
	if err := json.Unmarshal(sanitized, &result); err != nil {
		t.Fatal(err)
	}
	if result["stringSettings"]["language"] != "en" || result["stringsettings"]["fov"] != float64(90) {
		t.Fatalf("non-sensitive values were not preserved: %s", sanitized)
	}
}

func TestSanitizeClientSettingsAllowsNullSections(t *testing.T) {
	sanitized, err := SanitizeClientSettings([]byte(`{"stringsettings":null,"intsettings":{"viewDistance":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(sanitized, &result); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(bytes.TrimSpace(result["intsettings"]), []byte("null")) {
		t.Fatalf("int settings were lost: %s", sanitized)
	}
}

func TestPatchClientSettingsWritesIntoExistingCamelCaseSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clientsettings.json")
	if err := os.WriteFile(path, []byte(`{"stringSettings":{"language":"en"}}`), 0o600); err != nil {
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
	if result["stringSettings"]["sessionkey"] != "session-key" || result["stringSettings"]["language"] != "en" {
		t.Fatalf("credentials were not written into the camelCase section: %s", contents)
	}
	if _, ok := result["stringsettings"]; ok {
		t.Fatalf("a duplicate lowercase section was created: %s", contents)
	}
}

func TestClearClientSettingsRemovesCamelCaseCredentials(t *testing.T) {
	// The game itself persists auth fields under "stringSettings". Clear must
	// remove them, not only the lowercase section Waxlight once used.
	path := filepath.Join(t.TempDir(), "clientsettings.json")
	if err := os.WriteFile(path, []byte(`{"stringSettings":{"sessionkey":"stale-key","sessionsignature":"stale-sig","playeruid":"uid","playername":"Ada","language":"en"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (ClientSettingsService{}).Clear(path); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if stringContainsAuth(contents) {
		t.Fatalf("camelCase credentials remain after clear: %s", contents)
	}
	var result map[string]map[string]any
	if err := json.Unmarshal(contents, &result); err != nil {
		t.Fatal(err)
	}
	if result["stringSettings"]["language"] != "en" {
		t.Fatalf("camelCase non-sensitive values were lost: %s", contents)
	}
}

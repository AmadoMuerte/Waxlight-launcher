package filesystem

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/atomicfile"
)

const (
	maxClientSettingsBytes = 8 << 20
	injectionJournalSuffix = ".waxlight-auth-injection"
)

var authKeys = []string{"sessionkey", "sessionsignature", "playeruid", "playername"}

// sensitiveStringKeys are authentication properties and account personal data
// that must never leave the machine inside an exported package.
var sensitiveStringKeys = []string{
	"sessionkey", "sessionsignature", "playeruid", "playername",
	"useremail", "mptoken", "entitlements",
}

// sensitiveStringListKeys are machine-specific settings that must never leave
// the machine inside an exported package.
var sensitiveStringListKeys = []string{"modPaths", "modpaths", "multiplayerservers"}

type rawSection struct {
	key string
	raw json.RawMessage
}

// matchingSections returns every root section whose key equals name ignoring
// case. The game writes CamelCase sections ("stringSettings",
// "stringListSettings") while older Waxlight versions wrote lowercase ones;
// both spellings must be handled.
func matchingSections(root map[string]json.RawMessage, name string) []rawSection {
	var sections []rawSection
	for key, raw := range root {
		if strings.EqualFold(key, name) {
			sections = append(sections, rawSection{key: key, raw: raw})
		}
	}
	return sections
}

func stripKeys(settings map[string]json.RawMessage, forbidden []string) {
	for key := range settings {
		for _, candidate := range forbidden {
			if strings.EqualFold(key, candidate) {
				delete(settings, key)
				break
			}
		}
	}
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

type ClientSettingsService struct{}

type injectionJournal struct {
	Version    int    `json:"version"`
	InjectedAt string `json:"injectedAt"`
}

// Inject records a non-secret crash-recovery marker before atomically writing
// the four fields required by Vintage Story. The returned cleanup is
// idempotent and removes those fields after exit or launch failure.
func (ClientSettingsService) Inject(path string, account accounts.Account) (func() error, error) {
	journalPath := path + injectionJournalSuffix
	journal, err := json.Marshal(injectionJournal{Version: 1, InjectedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		return nil, errors.New("encode authentication injection marker")
	}
	if err := atomicfile.Write(journalPath, append(journal, '\n'), 0o600); err != nil {
		return nil, fmt.Errorf("write authentication injection marker: %w", err)
	}
	if err := patchClientSettings(path, &account); err != nil {
		_ = patchClientSettings(path, nil)
		_ = os.Remove(journalPath)
		return nil, err
	}
	cleanup := func() error {
		if err := patchClientSettings(path, nil); err != nil {
			return err
		}
		if err := os.Remove(journalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return cleanup, nil
}

func (ClientSettingsService) Clear(path string) error {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		if removeErr := os.Remove(path + injectionJournalSuffix); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return removeErr
		}
		return nil
	}
	if err := patchClientSettings(path, nil); err != nil {
		return err
	}
	if err := os.Remove(path + injectionJournalSuffix); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Reconcile clears all authentication properties. This also removes values
// left by pre-journal Waxlight versions after a launcher crash.
func (service ClientSettingsService) Reconcile(path string) error { return service.Clear(path) }

// SanitizeClientSettings removes every authentication property, account
// personal data and machine specific mod path from a client settings document.
// The result can be safely embedded in exported instance packages. An empty
// output means the document held no settings.
func SanitizeClientSettings(contents []byte) ([]byte, error) {
	root := map[string]json.RawMessage{}
	if err := decodeJSONObject(contents, &root); err != nil {
		return nil, fmt.Errorf("invalid client settings: %w", err)
	}

	for _, section := range matchingSections(root, "stringsettings") {
		if isJSONNull(section.raw) {
			continue
		}
		stringSettings := map[string]json.RawMessage{}
		if err := decodeJSONObject(section.raw, &stringSettings); err != nil {
			return nil, errors.New("client settings string settings must be an object")
		}
		stripKeys(stringSettings, sensitiveStringKeys)
		encodedStringSettings, err := json.Marshal(stringSettings)
		if err != nil {
			return nil, errors.New("encode string settings")
		}
		root[section.key] = encodedStringSettings
	}

	for _, section := range matchingSections(root, "stringlistsettings") {
		if isJSONNull(section.raw) {
			continue
		}
		stringListSettings := map[string]json.RawMessage{}
		if err := decodeJSONObject(section.raw, &stringListSettings); err != nil {
			return nil, errors.New("client settings string list settings must be an object")
		}
		stripKeys(stringListSettings, sensitiveStringListKeys)
		encodedListSettings, err := json.Marshal(stringListSettings)
		if err != nil {
			return nil, errors.New("encode string list settings")
		}
		root[section.key] = encodedListSettings
	}

	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, errors.New("encode client settings")
	}
	return append(output, '\n'), nil
}

func patchClientSettings(path string, account *accounts.Account) error {
	root := map[string]json.RawMessage{}
	contents, err := readRegularFile(path, maxClientSettingsBytes)
	if err == nil {
		if err := decodeJSONObject(contents, &root); err != nil {
			return fmt.Errorf("invalid client settings: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read client settings: %w", err)
	}

	// The game persists auth fields under the CamelCase "stringSettings"
	// section; older Waxlight versions used lowercase "stringsettings". Both
	// must be cleared so credentials cannot survive after exit, and a write
	// targets whichever spelling already exists.
	sections := map[string]map[string]json.RawMessage{}
	for _, section := range matchingSections(root, "stringsettings") {
		stringSettings := map[string]json.RawMessage{}
		if isJSONNull(section.raw) || decodeJSONObject(section.raw, &stringSettings) != nil {
			return errors.New("client settings stringsettings must be an object")
		}
		sections[section.key] = stringSettings
	}
	for _, settings := range sections {
		stripKeys(settings, authKeys)
	}
	if account != nil {
		primary := "stringsettings"
		for _, candidate := range []string{"stringSettings", "stringsettings"} {
			if _, ok := sections[candidate]; ok {
				primary = candidate
				break
			}
		}
		settings := sections[primary]
		if settings == nil {
			settings = map[string]json.RawMessage{}
		}
		values := map[string]string{
			"sessionkey": account.SessionKey, "sessionsignature": account.SessionSignature,
			"playeruid": account.UID, "playername": account.Username,
		}
		for key, value := range values {
			encoded, err := json.Marshal(value)
			if err != nil {
				return errors.New("encode authentication settings")
			}
			settings[key] = encoded
		}
		sections[primary] = settings
	}
	for key, settings := range sections {
		encoded, err := json.Marshal(settings)
		if err != nil {
			return errors.New("encode authentication settings")
		}
		root[key] = encoded
	}
	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return errors.New("encode client settings")
	}
	return atomicfile.Write(path, append(output, '\n'), 0o600)
}

func readRegularFile(path string, max int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("path is not a regular file")
	}
	if info.Size() > max {
		return nil, errors.New("file is too large")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(contents)) > max {
		return nil, errors.New("file is too large")
	}
	return contents, nil
}

func decodeJSONObject(contents []byte, target *map[string]json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if *target == nil {
		return errors.New("value is not an object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return errors.New("multiple JSON values")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

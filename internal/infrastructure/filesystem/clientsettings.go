package filesystem

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/atomicfile"
)

const (
	maxClientSettingsBytes = 8 << 20
	injectionJournalSuffix = ".waxlight-auth-injection"
)

var authKeys = []string{"sessionkey", "sessionsignature", "playeruid", "playername"}

type ClientSettingsService struct{}

type injectionJournal struct {
	Version    int    `json:"version"`
	InjectedAt string `json:"injectedAt"`
}

// Inject records a non-secret crash-recovery marker before atomically writing
// the four fields required by Vintage Story. The returned cleanup is
// idempotent and removes those fields after exit or launch failure.
func (ClientSettingsService) Inject(path string, account domain.Account) (func() error, error) {
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

// SanitizeClientSettings removes every authentication property and machine
// specific mod path from a client settings document. The result can be safely
// embedded in exported instance packages. An empty output means the document
// held no settings.
func SanitizeClientSettings(contents []byte) ([]byte, error) {
	root := map[string]json.RawMessage{}
	if err := decodeJSONObject(contents, &root); err != nil {
		return nil, fmt.Errorf("invalid client settings: %w", err)
	}

	stringSettings := map[string]json.RawMessage{}
	if raw, ok := root["stringsettings"]; ok {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || decodeJSONObject(raw, &stringSettings) != nil {
			return nil, errors.New("client settings stringsettings must be an object")
		}
	}
	for _, key := range authKeys {
		delete(stringSettings, key)
	}
	encodedStringSettings, err := json.Marshal(stringSettings)
	if err != nil {
		return nil, errors.New("encode string settings")
	}
	root["stringsettings"] = encodedStringSettings

	// Vintage Story records absolute mod directory paths here. They point at
	// the exporting machine's instance and must never travel inside a package.
	for _, listKey := range []string{"stringListSettings", "stringlistsettings"} {
		raw, ok := root[listKey]
		if !ok {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			continue
		}
		stringListSettings := map[string]json.RawMessage{}
		if decodeJSONObject(raw, &stringListSettings) != nil {
			return nil, errors.New("client settings stringlistsettings must be an object")
		}
		for _, modPathKey := range []string{"modPaths", "modpaths"} {
			delete(stringListSettings, modPathKey)
		}
		encodedListSettings, err := json.Marshal(stringListSettings)
		if err != nil {
			return nil, errors.New("encode string list settings")
		}
		root[listKey] = encodedListSettings
	}

	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, errors.New("encode client settings")
	}
	return append(output, '\n'), nil
}

func patchClientSettings(path string, account *domain.Account) error {
	root := map[string]json.RawMessage{}
	contents, err := readRegularFile(path, maxClientSettingsBytes)
	if err == nil {
		if err := decodeJSONObject(contents, &root); err != nil {
			return fmt.Errorf("invalid client settings: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read client settings: %w", err)
	}

	stringSettings := map[string]json.RawMessage{}
	if raw, ok := root["stringsettings"]; ok {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || decodeJSONObject(raw, &stringSettings) != nil {
			return errors.New("client settings stringsettings must be an object")
		}
	}
	for _, key := range authKeys {
		delete(stringSettings, key)
	}
	if account != nil {
		values := map[string]string{
			"sessionkey": account.SessionKey, "sessionsignature": account.SessionSignature,
			"playeruid": account.UID, "playername": account.Username,
		}
		for key, value := range values {
			encoded, err := json.Marshal(value)
			if err != nil {
				return errors.New("encode authentication settings")
			}
			stringSettings[key] = encoded
		}
	}
	encodedStringSettings, err := json.Marshal(stringSettings)
	if err != nil {
		return errors.New("encode string settings")
	}
	root["stringsettings"] = encodedStringSettings
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

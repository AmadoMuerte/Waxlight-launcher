package filesystem

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/atomicfile"
)

type ClientSettingsService struct{}

func (ClientSettingsService) Patch(path string, account domain.Account) error {
	return patchClientSettings(path, &account)
}

func (ClientSettingsService) Clear(path string) error {
	return patchClientSettings(path, nil)
}

func patchClientSettings(path string, account *domain.Account) error {
	root := map[string]json.RawMessage{}
	contents, err := os.ReadFile(path)
	if err == nil {
		if err := decodeJSONObject(contents, &root); err != nil {
			return fmt.Errorf("invalid client settings: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read client settings: %w", err)
	}

	stringSettings := map[string]json.RawMessage{}
	if raw, ok := root["stringsettings"]; ok {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) ||
			decodeJSONObject(raw, &stringSettings) != nil {
			return errors.New("client settings stringsettings must be an object")
		}
	}

	authKeys := []string{
		"sessionkey",
		"sessionsignature",
		"playeruid",
		"playername",
	}
	if account == nil {
		for _, key := range authKeys {
			delete(stringSettings, key)
		}
	} else {
		values := map[string]string{
			"sessionkey":       account.SessionKey,
			"sessionsignature": account.SessionSignature,
			"playeruid":        account.UID,
			"playername":       account.Username,
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
	output = append(output, '\n')
	if err := atomicfile.Write(path, output, 0o600); err != nil {
		return fmt.Errorf("write client settings: %w", err)
	}
	return nil
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

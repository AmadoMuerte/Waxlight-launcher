package credentials

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
)

const secretVersion = 1

type encodedSecret struct {
	Version          int    `json:"version"`
	SessionKey       string `json:"sessionKey"`
	SessionSignature string `json:"sessionSignature"`
}

func encodeSecret(secret accounts.Credential) (string, error) {
	if secret.SessionKey == "" || secret.SessionSignature == "" {
		return "", accounts.ErrCorruptCredentials
	}
	contents, err := json.Marshal(encodedSecret{
		Version: secretVersion, SessionKey: secret.SessionKey,
		SessionSignature: secret.SessionSignature,
	})
	if err != nil {
		return "", accounts.ErrCorruptCredentials
	}
	return string(contents), nil
}

func decodeSecret(value string) (accounts.Credential, error) {
	if len(value) == 0 || len(value) > 64*1024 {
		return accounts.Credential{}, accounts.ErrCorruptCredentials
	}
	decoder := json.NewDecoder(bytes.NewBufferString(value))
	decoder.DisallowUnknownFields()
	var stored encodedSecret
	if err := decoder.Decode(&stored); err != nil {
		return accounts.Credential{}, accounts.ErrCorruptCredentials
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return accounts.Credential{}, accounts.ErrCorruptCredentials
	}
	if stored.Version != secretVersion || stored.SessionKey == "" || stored.SessionSignature == "" {
		return accounts.Credential{}, accounts.ErrCorruptCredentials
	}
	return accounts.Credential{SessionKey: stored.SessionKey, SessionSignature: stored.SessionSignature}, nil
}

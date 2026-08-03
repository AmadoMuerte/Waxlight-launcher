package credentials

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/waxlight/waxlight-launcher/internal/application"
)

const secretVersion = 1

type encodedSecret struct {
	Version          int    `json:"version"`
	SessionKey       string `json:"sessionKey"`
	SessionSignature string `json:"sessionSignature"`
}

func encodeSecret(secret application.Secret) (string, error) {
	if secret.SessionKey == "" || secret.SessionSignature == "" {
		return "", application.ErrCorruptSecret
	}
	contents, err := json.Marshal(encodedSecret{
		Version: secretVersion, SessionKey: secret.SessionKey,
		SessionSignature: secret.SessionSignature,
	})
	if err != nil {
		return "", application.ErrCorruptSecret
	}
	return string(contents), nil
}

func decodeSecret(value string) (application.Secret, error) {
	if len(value) == 0 || len(value) > 64*1024 {
		return application.Secret{}, application.ErrCorruptSecret
	}
	decoder := json.NewDecoder(bytes.NewBufferString(value))
	decoder.DisallowUnknownFields()
	var stored encodedSecret
	if err := decoder.Decode(&stored); err != nil {
		return application.Secret{}, application.ErrCorruptSecret
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return application.Secret{}, application.ErrCorruptSecret
	}
	if stored.Version != secretVersion || stored.SessionKey == "" || stored.SessionSignature == "" {
		return application.Secret{}, application.ErrCorruptSecret
	}
	return application.Secret{SessionKey: stored.SessionKey, SessionSignature: stored.SessionSignature}, nil
}

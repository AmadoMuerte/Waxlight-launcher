package auth

import (
	"encoding/json"
	"fmt"
	"strings"
)

type flexibleBool bool

func (value *flexibleBool) UnmarshalJSON(data []byte) error {
	normalized := strings.ToLower(strings.TrimSpace(string(data)))

	switch normalized {
	case "1", "true", `"1"`, `"true"`:
		*value = true
		return nil

	case "0", "false", `"0"`, `"false"`:
		*value = false
		return nil

	default:
		return fmt.Errorf("unsupported boolean representation: %q", normalized)
	}
}

type Session struct {
	SessionKey       string
	SessionSignature string
	UID              string
	PlayerName       string
}

type TOTPChallenge struct {
	PreLoginToken string
}

type loginResponse struct {
	SessionKey       *string         `json:"sessionkey"`
	SessionSignature *string         `json:"sessionsignature"`
	MPToken          json.RawMessage `json:"mptoken"`
	UID              *string         `json:"uid"`
	Entitlements     json.RawMessage `json:"entitlements"`
	PlayerName       *string         `json:"playername"`
	HasGameServer    *bool           `json:"hasgameserver"`
	Valid            flexibleBool    `json:"valid"`
	PreLoginToken    *string         `json:"prelogintoken"`
	Reason           *string         `json:"reason"`
}

type validateResponse struct {
	Valid flexibleBool `json:"valid"`
}

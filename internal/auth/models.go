package auth

import "encoding/json"

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
	Valid            int             `json:"valid"`
	PreLoginToken    *string         `json:"prelogintoken"`
	Reason           *string         `json:"reason"`
}

type validateResponse struct {
	Valid int `json:"valid"`
}

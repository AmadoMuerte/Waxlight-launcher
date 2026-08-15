// Package deeplink validates Waxlight custom-protocol targets from the OS.
package deeplink

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

const (
	Scheme        = "waxlight"
	ModKind       = "mod"
	maxLinkLength = len(Scheme) + len("://") + len(ModKind) + 1 + 64
)

var modIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

// Target is the normalized, safe navigation target exposed to the frontend.
type Target struct {
	Type  string `json:"type"`
	ModID string `json:"modId"`
}

// Parse accepts only the Waxlight mod URI contract.
func Parse(raw string) (Target, error) {
	if len(raw) > maxLinkLength {
		return Target{}, errors.New("Waxlight link is too long")
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return Target{}, errors.New("invalid Waxlight link")
	}
	if parsed.Scheme != Scheme || parsed.Host != ModKind || parsed.User != nil || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return Target{}, errors.New("unsupported Waxlight link")
	}

	modID := strings.TrimPrefix(parsed.Path, "/")
	if parsed.Path != "/"+modID || !modIDPattern.MatchString(modID) {
		return Target{}, errors.New("invalid Waxlight mod ID")
	}
	return Target{Type: ModKind, ModID: modID}, nil
}

// Extract returns valid targets and the number of rejected Waxlight URI arguments.
func Extract(args []string) (targets []Target, rejected int) {
	for _, arg := range args {
		if !strings.HasPrefix(arg, Scheme+":") {
			continue
		}
		target, err := Parse(arg)
		if err != nil {
			rejected++
			continue
		}
		targets = append(targets, target)
	}
	return targets, rejected
}

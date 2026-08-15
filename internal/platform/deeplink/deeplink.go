// Package deeplink validates Waxlight custom-protocol targets from the OS.
package deeplink

import (
	"errors"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	Scheme        = "waxlight"
	ModKind       = "mod"
	ServerKind    = "server"
	maxLinkLength = len(Scheme) + len("://") + len(ServerKind) + 1 + 255*3
)

var modIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

// Target is the normalized, safe navigation target exposed to the frontend.
type Target struct {
	Type    string `json:"type"`
	ModID   string `json:"modId,omitempty"`
	Address string `json:"address,omitempty"`
}

// Parse accepts only the Waxlight mod and server URI contracts.
func Parse(raw string) (Target, error) {
	if len(raw) > maxLinkLength {
		return Target{}, errors.New("Waxlight link is too long")
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return Target{}, errors.New("invalid Waxlight link")
	}
	if parsed.Scheme != Scheme || parsed.User != nil || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return Target{}, errors.New("unsupported Waxlight link")
	}

	switch parsed.Host {
	case ModKind:
		modID := strings.TrimPrefix(parsed.Path, "/")
		if parsed.Path != "/"+modID || !modIDPattern.MatchString(modID) {
			return Target{}, errors.New("invalid Waxlight mod ID")
		}
		return Target{Type: ModKind, ModID: modID}, nil
	case ServerKind:
		address, ok := normalizeServerAddress(strings.TrimPrefix(parsed.Path, "/"))
		if parsed.Path != "/"+strings.TrimPrefix(parsed.Path, "/") || !ok {
			return Target{}, errors.New("invalid Waxlight server address")
		}
		return Target{Type: ServerKind, Address: address}, nil
	default:
		return Target{}, errors.New("unsupported Waxlight link")
	}
}

func normalizeServerAddress(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 255 || strings.ContainsAny(value, "\r\n\t /\\?#@") {
		return "", false
	}

	if address, err := netip.ParseAddr(value); err == nil {
		return address.String(), true
	}

	host, port, hasPort := value, "", false
	if parsedHost, parsedPort, err := net.SplitHostPort(value); err == nil {
		host, port, hasPort = parsedHost, parsedPort, true
		portNumber, err := strconv.ParseUint(port, 10, 16)
		if err != nil || portNumber == 0 {
			return "", false
		}
	}

	if address, err := netip.ParseAddr(host); err == nil {
		if hasPort {
			return net.JoinHostPort(address.String(), port), true
		}
		return address.String(), true
	}
	if !validHostname(host) {
		return "", false
	}
	host = strings.ToLower(host)
	if hasPort {
		return net.JoinHostPort(host, port), true
	}
	return host, true
}

func validHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-') {
				return false
			}
		}
	}
	return true
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

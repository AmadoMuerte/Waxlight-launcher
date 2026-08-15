package deeplink

import (
	"strings"
	"testing"
)

func TestParseMod(t *testing.T) {
	for _, raw := range []string{"waxlight://mod/optimum", "waxlight://mod/betterruins"} {
		target, err := Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q): %v", raw, err)
		}
		if target.Type != ModKind || target.ModID == "" {
			t.Fatalf("Parse(%q) = %#v", raw, target)
		}
	}
}

func TestParseServer(t *testing.T) {
	for _, test := range []struct {
		raw, address string
	}{
		{"waxlight://server/play.example.com", "play.example.com"},
		{"waxlight://server/PLAY.example.com:42420", "play.example.com:42420"},
		{"waxlight://server/127.0.0.1:42420", "127.0.0.1:42420"},
		{"waxlight://server/%5B2001:DB8::1%5D:42420", "[2001:db8::1]:42420"},
	} {
		target, err := Parse(test.raw)
		if err != nil || target.Type != ServerKind || target.Address != test.address {
			t.Errorf("Parse(%q) = %#v, %v", test.raw, target, err)
		}
	}
}

func TestParseRejectsInvalidLinks(t *testing.T) {
	for _, raw := range []string{
		"waxlight://mod/",
		"waxlight://mod/UPPERCASE",
		"waxlight://mod/foo_bar",
		"waxlight://mod/../foo",
		"waxlight://mod/../../etc/passwd",
		"waxlight://mod/javascript:alert(1)",
		"http://mod/optimum",
		"file:///etc/passwd",
		"waxlight://server/",
		"waxlight://server/javascript:foo",
		"waxlight://server/http://evil.example",
		"waxlight://server/file:///etc/passwd",
		"waxlight://server/../../foo",
		"waxlight://unknown/foo",
		"https://waxlight.by/mod/optimum",
		"javascript://mod/foo",
		"waxlight://mod/https://evil.example",
	} {
		if _, err := Parse(raw); err == nil {
			t.Errorf("Parse(%q) succeeded", raw)
		}
	}
}

func TestParseModIDBounds(t *testing.T) {
	for _, modID := range []string{"a", strings.Repeat("a", 64)} {
		if _, err := Parse("waxlight://mod/" + modID); err != nil {
			t.Errorf("Parse valid ID length %d: %v", len(modID), err)
		}
	}
	if _, err := Parse("waxlight://mod/" + strings.Repeat("a", 65)); err == nil {
		t.Error("Parse accepted a 65-character ID")
	}
}

func TestExtractOnlyUsesWaxlightArguments(t *testing.T) {
	targets, rejected := Extract([]string{"--update-wait-pid", "42", "waxlight://mod/optimum", "https://waxlight.by/mod/optimum"})
	if rejected != 0 || len(targets) != 1 || targets[0].ModID != "optimum" {
		t.Fatalf("Extract() = %#v, %d", targets, rejected)
	}

	targets, rejected = Extract([]string{"waxlight://mod/UPPERCASE", "--update-wait-pid", "42"})
	if len(targets) != 0 || rejected != 1 {
		t.Fatalf("Extract() = %#v, %d", targets, rejected)
	}
}

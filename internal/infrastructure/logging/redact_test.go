package logging

import (
	"strings"
	"testing"
)

func TestRedactMasksSensitiveAssignments(t *testing.T) {
	for _, original := range []string{
		`sessionkey = 4xeyHpPItrH/hhm5LByxCTd6i5RBwN87UXGEDfN6p6M=`,
		`"sessionkey": "ZNCWFrAOPUmC3ZoMwVQt+2ZtfJox0jur0HclHv5cUfM"`,
		`password=supersecret`,
		`prelogintoken = abc-def`,
		`token: "tok-12345"`,
		`authorization: Bearer abcdef123456`,
		`email: player@example.com`,
	} {
		redacted := Redact(original)
		if strings.Contains(redacted, "4xeyHpPItrH") ||
			strings.Contains(redacted, "supersecret") ||
			strings.Contains(redacted, "tok-12345") ||
			strings.Contains(redacted, "abcdef123456") ||
			strings.Contains(redacted, "player@example.com") {
			t.Fatalf("redaction leaked value: %q -> %q", original, redacted)
		}
	}
}

func TestRedactPreservesOrdinaryText(t *testing.T) {
	original := "starting install of version 1.20 for instance \"My world\""
	if redacted := Redact(original); redacted != original {
		t.Fatalf("ordinary text was changed: %q -> %q", original, redacted)
	}
}

func TestRedactPreservesSensitiveKeyNamesWithInnocentValues(t *testing.T) {
	original := `"playername": "Ada"` // playername is intentionally not sensitive
	redacted := Redact(original)
	if redacted != original {
		t.Fatalf("non-sensitive field was altered: %q -> %q", original, redacted)
	}
}

func TestRedactMasksLongBase64Blobs(t *testing.T) {
	original := "wrote signature ZNCWFrAOPUmC3ZoMwVQt+2ZtfJox0jur0HclHv5cUfMH1KqRlOnWlCOX4SOaWUsKzSJPz9Vemu/Z06GQJIhR+4+jeWC4AJdctWdK/Zuwy7nulZazmwaM3tPnDLjBm/g2qjAbo4i28lHCF93hH0vnwHqZCxznBXuK0bVnUGwNLWJKtipmZ1+svjZ+kPYNZGh0vYXGe2D/nTu9Vn5eAx5NzW3xt+uThC1uGk8GnHNf1zk+CtfjdwKXLp4UxwxX46KQOUqQJOGFtrpvw7ZZnxdFlKd9Ktg5zP03QUYvoxLIQ0iXD0v8GSRUu7+ca84eS+RexjxwixnpKf03v+/qCd/cqA=="
	redacted := Redact(original)
	if strings.Contains(redacted, "ZNCWFrAOPUmC3") {
		t.Fatalf("long base64 blob leaked: %q", redacted)
	}
	if !strings.Contains(redacted, "***") {
		t.Fatalf("expected masking placeholder: %q", redacted)
	}
}

func TestContainsSensitiveKey(t *testing.T) {
	for _, text := range []string{"failed to validate sessionkey", "timeout on token fetch", "account email mismatch"} {
		if !ContainsSensitiveKey(text) {
			t.Fatalf("expected sensitive key detection for %q", text)
		}
	}
	if ContainsSensitiveKey("playername must be set") {
		t.Fatal("playername is not a sensitive key")
	}
}

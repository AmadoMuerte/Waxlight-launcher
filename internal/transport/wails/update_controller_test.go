package wails

import "testing"

func TestValidExternalURL(t *testing.T) {
	accepted := []string{
		"https://github.com/AmadoMuerte/Waxlight-launcher/releases/tag/v0.3.0",
		"http://example.com/page?q=1",
		"https://user:pass@example.com/",
	}
	for _, raw := range accepted {
		if !validExternalURL(raw) {
			t.Errorf("expected %q to be accepted", raw)
		}
	}

	rejected := []string{
		"",
		"not a url",
		"javascript:alert(1)",
		"file:///etc/passwd",
		"ftp://example.com/file",
		"wails://local",
		"https://",
		"/relative/path",
	}
	for _, raw := range rejected {
		if validExternalURL(raw) {
			t.Errorf("expected %q to be rejected", raw)
		}
	}
}

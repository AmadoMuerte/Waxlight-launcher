package waxlight

import "testing"

func TestVersionMatchesEmbeddedApplicationConfig(t *testing.T) {
	if Version() == "" || Version() == "0.0.0" {
		t.Fatalf("embedded application version was not loaded: %q", Version())
	}
}

func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"0.2.0-beta.9", "0.2.0-beta.9"},
		{"0.2.0", "0.2.0"},
		{"1.0.0", "1.0.0"},
		{"0.2.0.0", "0.2.0"},
		{"1.2.3.0", "1.2.3"},
		{"0.0.0.0", "0.0.0"},
		{"1.0.0-beta.1.0", "1.0.0-beta.1.0"},
	}
	for _, tc := range cases {
		actual := normalizeVersion(tc.input)
		if actual != tc.expected {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tc.input, actual, tc.expected)
		}
	}
}

package version

import "testing"

func TestVersionMatchesEmbeddedApplicationConfig(t *testing.T) {
	previousBuildVersion := buildVersion
	buildVersion = ""
	t.Cleanup(func() { buildVersion = previousBuildVersion })

	if Version() == "" || Version() == "0.0.0" {
		t.Fatalf("embedded application version was not loaded: %q", Version())
	}
}

func TestVersionPrefersInjectedSemanticVersion(t *testing.T) {
	previousBuildVersion := buildVersion
	previousConfig := applicationConfigJSON
	buildVersion = "v0.2.1-beta.3"
	applicationConfigJSON = []byte(`{"info":{"productVersion":"0.2.1.3"}}`)
	t.Cleanup(func() {
		buildVersion = previousBuildVersion
		applicationConfigJSON = previousConfig
	})

	if actual := Version(); actual != "0.2.1-beta.3" {
		t.Fatalf("Version() = %q, want %q", actual, "0.2.1-beta.3")
	}
}

func TestVersionRejectsNumericWindowsPrereleaseWithoutSemanticVersion(t *testing.T) {
	previousBuildVersion := buildVersion
	previousConfig := applicationConfigJSON
	buildVersion = ""
	applicationConfigJSON = []byte(`{"info":{"productVersion":"0.2.1.3"}}`)
	t.Cleanup(func() {
		buildVersion = previousBuildVersion
		applicationConfigJSON = previousConfig
	})

	if actual := Version(); actual != "0.0.0" {
		t.Fatalf("Version() = %q, want fail-safe version %q", actual, "0.0.0")
	}
}

func TestCanonicalSemanticVersion(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"0.2.1", "0.2.1"},
		{"v0.2.1-beta.3", "0.2.1-beta.3"},
		{" 0.2.1-rc.1 ", "0.2.1-rc.1"},
		{"0.2.1.3", ""},
		{"01.2.3", ""},
		{"1.2.3-beta.03", ""},
		{"beta.3", ""},
		{"", ""},
	}

	for _, tc := range cases {
		if actual := canonicalSemanticVersion(tc.input); actual != tc.expected {
			t.Errorf("canonicalSemanticVersion(%q) = %q, want %q", tc.input, actual, tc.expected)
		}
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

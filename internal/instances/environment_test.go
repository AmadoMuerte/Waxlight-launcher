package instances

import "testing"

func TestCleanEnvironmentVariables(t *testing.T) {
	t.Parallel()

	values, err := cleanEnvironmentVariables(map[string]string{
		"__NV_PRIME_RENDER_OFFLOAD":      "1",
		"__GLX_VENDOR_LIBRARY_NAME":      "nvidia",
		"mesa_glthread":                  "true",
		" VALUE_WITH_SURROUNDING_SPACE ": "kept",
	})
	if err != nil {
		t.Fatalf("clean environment variables: %v", err)
	}
	if values["__NV_PRIME_RENDER_OFFLOAD"] != "1" {
		t.Fatalf("unexpected PRIME value: %q", values["__NV_PRIME_RENDER_OFFLOAD"])
	}
	if values["VALUE_WITH_SURROUNDING_SPACE"] != "kept" {
		t.Fatalf("expected variable name to be trimmed")
	}
}

func TestCleanEnvironmentVariablesRejectsInvalidNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "1INVALID", "INVALID-NAME", "INVALID=NAME"} {
		if _, err := cleanEnvironmentVariables(map[string]string{name: "value"}); err == nil {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}

func TestCleanEnvironmentVariablesRejectsReservedVariable(t *testing.T) {
	t.Parallel()

	if _, err := cleanEnvironmentVariables(map[string]string{"WAXLIGHT_INSTANCE_DIR": "/tmp/nope"}); err == nil {
		t.Fatal("expected Waxlight-owned environment variable to be rejected")
	}
}

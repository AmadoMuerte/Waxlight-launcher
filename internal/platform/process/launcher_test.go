package process

import (
	"os"
	"testing"
)

func TestEnvironmentOverrideReplacesInheritedValue(t *testing.T) {
	t.Setenv("WAXLIGHT_PROCESS_ENV_TEST", "inherited")
	merged := mergeEnvironment(os.Environ(), map[string]string{"WAXLIGHT_PROCESS_ENV_TEST": "instance"})
	count := 0
	for _, entry := range merged {
		key, value, ok := splitEnvironment(entry)
		if ok && key == "WAXLIGHT_PROCESS_ENV_TEST" {
			count++
			if value != "instance" {
				t.Fatalf("value = %q, want instance", value)
			}
		}
	}
	if count != 1 {
		t.Fatalf("override count = %d, want 1", count)
	}
}

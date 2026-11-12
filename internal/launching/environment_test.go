package launching

import (
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/envvars"
)

func TestBuildEnvironmentAppliesInstanceOverrides(t *testing.T) {
	t.Parallel()

	global := map[string]string{"SHARED": "global", "GLOBAL_ONLY": "kept", "CLEARED": "global"}
	instance := map[string]string{"SHARED": "instance", "INSTANCE_ONLY": "value", "CLEARED": ""}
	environment := buildEnvironment(global, instance, "/data/instances/one")
	if environment["SHARED"] != "instance" {
		t.Fatalf("instance value did not override global: %q", environment["SHARED"])
	}
	if value, exists := environment["CLEARED"]; !exists || value != "" {
		t.Fatalf("instance value did not override with an empty string: %q, %v", value, exists)
	}
	if environment["GLOBAL_ONLY"] != "kept" || environment["INSTANCE_ONLY"] != "value" {
		t.Fatalf("unexpected merged environment: %#v", environment)
	}
	if environment[envvars.InstanceDirectory] != "/data/instances/one" {
		t.Fatalf("instance directory was not set: %#v", environment)
	}
	if global["SHARED"] != "global" || instance["SHARED"] != "instance" {
		t.Fatal("buildEnvironment mutated its inputs")
	}
}

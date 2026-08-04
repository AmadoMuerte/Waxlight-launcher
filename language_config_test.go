package waxlight

import "testing"

func TestLanguagesNormalizeConfiguredAndLegacyCodes(t *testing.T) {
	config, err := Languages()
	if err != nil {
		t.Fatal(err)
	}
	if config.Normalize("ES_mx") != "es" {
		t.Fatal("Spanish was not normalized")
	}
	if config.Normalize("by-BY") != "be" {
		t.Fatal("legacy Belarusian code was not migrated")
	}
	if config.Normalize("unknown") != config.DefaultLanguage {
		t.Fatal("unknown language did not use the default")
	}
}

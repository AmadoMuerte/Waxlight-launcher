package mutations

import "testing"

func TestGateAtomicallyExcludesMutationsAndRelocation(t *testing.T) {
	gate := &Gate{}
	if err := gate.Begin(); err != nil {
		t.Fatal(err)
	}
	if err := gate.BeginRelocation(); err == nil {
		t.Fatal("relocation acquired while a mutation was active")
	}
	gate.End()
	if err := gate.BeginRelocation(); err != nil {
		t.Fatal(err)
	}
	if err := gate.Begin(); err == nil {
		t.Fatal("mutation acquired while relocation was active")
	}
	gate.EndRelocation()
	if err := gate.Begin(); err != nil {
		t.Fatalf("gate was not released: %v", err)
	}
	gate.End()
}

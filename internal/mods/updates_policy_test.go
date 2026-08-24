package mods

import (
	"context"
	"testing"
)

func TestPendingModUpdatesRespectPolicies(t *testing.T) {
	installed := []InstalledMod{
		{Source: "moddb:auto:old", UpdatePolicy: UpdatePolicyAutomatic},
		{Source: "moddb:pinned:old", UpdatePolicy: UpdatePolicyPinned},
		{Source: "moddb:ignored:old", UpdatePolicy: UpdatePolicyPinned},
	}
	service := &CatalogService{catalog: policyTestCatalog{}}
	pending, skipped, err := service.pendingModUpdates(context.Background(), installed, []ModUpdateTarget{
		{ModID: "auto", VersionID: "new"},
		{ModID: "pinned", VersionID: "new"},
		{ModID: "ignored", VersionID: "new"},
	}, "1.20")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ModID != "auto" || skipped != 2 {
		t.Fatalf("pending = %+v, skipped = %d", pending, skipped)
	}
}

type policyTestCatalog struct{}

func (policyTestCatalog) List(context.Context) ([]ModSummary, error) { return nil, nil }
func (policyTestCatalog) Search(context.Context, ModSearchQuery) (ModSearchResult, error) {
	return ModSearchResult{}, nil
}
func (policyTestCatalog) Get(context.Context, string) (ModDetails, error) {
	return ModDetails{}, nil
}
func (policyTestCatalog) ListTags(context.Context) ([]ModTag, error) { return nil, nil }

func TestInstalledReplacementKeepsPolicy(t *testing.T) {
	if NormalizeUpdatePolicy(UpdatePolicyPinned) != UpdatePolicyPinned {
		t.Fatal("pinned policy was not preserved")
	}
}

func TestLegacyIgnorePolicyBecomesPinned(t *testing.T) {
	if NormalizeUpdatePolicy("ignore") != UpdatePolicyPinned {
		t.Fatal("legacy ignore policy was not preserved as pinned")
	}
}

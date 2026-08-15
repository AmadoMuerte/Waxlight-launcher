package optimum

import (
	"errors"
	"strings"
	"testing"
)

type testLocator struct {
	installation Installation
	detectErr    error
	inspectErr   error
	vanilla      string
	inUse        bool
}

func (locator testLocator) Detect() (Installation, error) {
	return locator.installation, locator.detectErr
}

func (locator testLocator) Inspect(string) (Installation, error) {
	return locator.installation, locator.inspectErr
}

func (locator testLocator) GameVersion(string) string { return locator.vanilla }

func (locator testLocator) InUse(Installation) (bool, error) { return locator.inUse, nil }

func TestResolveUsesDetectedInstallation(t *testing.T) {
	want := Installation{Path: "/optimum", Executable: "/optimum/run.sh", GameVersion: "1.22.5"}
	resolved, err := NewService(testLocator{installation: want, vanilla: "1.22.5"}).Resolve("", "/vanilla")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != want {
		t.Fatalf("resolved installation = %#v, want %#v", resolved, want)
	}
}

func TestResolveRejectsMissingInstallation(t *testing.T) {
	_, err := NewService(testLocator{detectErr: ErrNotFound}).Resolve("", "/vanilla")
	if err == nil || !strings.Contains(err.Error(), "installation could not be found") {
		t.Fatalf("missing installation error = %v", err)
	}
}

func TestResolveRejectsVersionMismatch(t *testing.T) {
	locator := testLocator{
		installation: Installation{GameVersion: "1.22.5"},
		vanilla:      "1.22.6",
	}
	_, err := NewService(locator).Resolve("", "/vanilla")
	if err == nil || !strings.Contains(err.Error(), "1.22.5") || !strings.Contains(err.Error(), "1.22.6") {
		t.Fatalf("version mismatch error = %v", err)
	}
}

func TestInspectReportsExecutableError(t *testing.T) {
	_, err := NewService(testLocator{inspectErr: errors.Join(ErrExecutableMissing, errors.New("missing"))}).Inspect("/optimum")
	if err == nil || !strings.Contains(err.Error(), "expected executable") {
		t.Fatalf("inspect error = %v", err)
	}
}

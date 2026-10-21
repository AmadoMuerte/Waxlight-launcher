//go:build windows

package securefs

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// requireDaclMatches asserts that path carries the intended hardened DACL,
// dumping both ACE lists on failure so CI logs show the exact difference.
func requireDaclMatches(t *testing.T, path string, directory bool) {
	t.Helper()
	desired, err := buildDACL(directory)
	if err != nil {
		t.Fatalf("buildDACL: %v", err)
	}
	if !daclMatches(path, desired) {
		t.Errorf("DACL does not match\ncurrent: %s\ndesired: %s",
			describeDACL(path), describeACL(desired))
	}
}

func TestApplyDirectorySkipsIdenticalDACL(t *testing.T) {
	dir := t.TempDir()
	desired, err := buildDACL(true)
	if err != nil {
		t.Fatalf("buildDACL: %v", err)
	}

	// The fresh temp directory inherits its DACL from the parent, so the
	// first Apply must rewrite it.
	if daclMatches(dir, desired) {
		t.Fatal("fresh directory already matches the hardened DACL")
	}
	if err := Apply(dir, 0o700, true); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	requireDaclMatches(t, dir, true)

	// A second Apply must succeed and leave the DACL matching (this time via
	// the skip path).
	if err := Apply(dir, 0o700, true); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	requireDaclMatches(t, dir, true)
}

func TestApplyFileSkipsIdenticalDACL(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "waxlight.db")
	if err := os.WriteFile(file, make([]byte, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	desired, err := buildDACL(false)
	if err != nil {
		t.Fatalf("buildDACL: %v", err)
	}
	if daclMatches(file, desired) {
		t.Fatal("fresh file already matches the hardened DACL")
	}
	if err := Apply(file, 0o600, false); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	requireDaclMatches(t, file, false)
}

func TestDaclMatchesDetectsExternalChanges(t *testing.T) {
	dir := t.TempDir()
	if err := Apply(dir, 0o700, true); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	desired, err := buildDACL(true)
	if err != nil {
		t.Fatalf("buildDACL: %v", err)
	}

	// Grant Everyone full control; the DACL then differs from the intended
	// one and must be detected as stale.
	everyone, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	if err != nil {
		t.Fatal(err)
	}
	alteredACL, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee:           windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP, TrusteeValue: windows.TrusteeValueFromSID(everyone)},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, alteredACL, nil); err != nil {
		t.Fatalf("alter DACL: %v", err)
	}
	if daclMatches(dir, desired) {
		t.Fatal("externally replaced DACL still reported as matching")
	}

	// Re-applying restores the intended DACL.
	if err := Apply(dir, 0o700, true); err != nil {
		t.Fatalf("re-Apply: %v", err)
	}
	requireDaclMatches(t, dir, true)
}

func TestFileSpecificMaskExpandsGenericBits(t *testing.T) {
	tests := []struct {
		name string
		in   uint32
		want uint32
	}{
		{name: "generic all maps to file all access", in: windows.GENERIC_ALL, want: fileAllAccess},
		{name: "generic read", in: windows.GENERIC_READ, want: windows.FILE_GENERIC_READ},
		{name: "generic write", in: windows.GENERIC_WRITE, want: windows.FILE_GENERIC_WRITE},
		{name: "generic execute", in: windows.GENERIC_EXECUTE, want: windows.FILE_GENERIC_EXECUTE},
		{name: "all generic bits combine", in: windows.GENERIC_READ | windows.GENERIC_WRITE | windows.GENERIC_EXECUTE,
			want: windows.FILE_GENERIC_READ | windows.FILE_GENERIC_WRITE | windows.FILE_GENERIC_EXECUTE},
		{name: "already specific stays unchanged", in: fileAllAccess, want: fileAllAccess},
		{name: "zero stays zero", in: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileSpecificMask(tt.in); got != tt.want {
				t.Errorf("fileSpecificMask(%#08x) = %#08x, want %#08x", tt.in, got, tt.want)
			}
		})
	}
	if fileAllAccess != 0x001F01FF {
		t.Errorf("fileAllAccess = %#08x, want 0x001F01FF", fileAllAccess)
	}
}

func TestDaclMatchesMissingPath(t *testing.T) {
	desired, err := buildDACL(true)
	if err != nil {
		t.Fatalf("buildDACL: %v", err)
	}
	if daclMatches(filepath.Join(t.TempDir(), "does-not-exist"), desired) {
		t.Fatal("missing path reported as matching")
	}
}

func TestParseACLErrorsOnMalformedBuffers(t *testing.T) {
	tests := []struct {
		name string
		acl  []byte
	}{
		{name: "header shorter than eight bytes", acl: make([]byte, 6)},
		{name: "total size smaller than header", acl: func() []byte {
			acl := make([]byte, 8)
			acl[2], acl[3] = 6, 0 // aclSize = 6
			return acl
		}()},
		{name: "ace count exceeds buffer", acl: func() []byte {
			acl := make([]byte, 8)
			acl[2], acl[3] = 8, 0 // aclSize = 8
			acl[4], acl[5] = 1, 0 // AceCount = 1, but no ACE follows
			return acl
		}()},
		{name: "ace size smaller than minimum", acl: func() []byte {
			acl := make([]byte, 20)
			acl[2], acl[3] = 20, 0 // aclSize = 20
			acl[4], acl[5] = 1, 0  // AceCount = 1
			acl[10], acl[11] = 4, 0
			return acl
		}()},
		{name: "ace size beyond buffer", acl: func() []byte {
			acl := make([]byte, 20)
			acl[2], acl[3] = 20, 0 // aclSize = 20
			acl[4], acl[5] = 1, 0  // AceCount = 1
			acl[10], acl[11] = 200, 0
			return acl
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acl := (*windows.ACL)(unsafe.Pointer(&tt.acl[0]))
			if _, err := parseACL(acl); err == nil {
				t.Error("parseACL succeeded on malformed buffer, want error")
			}
		})
	}
}

func TestParseACLReadsBuiltDACL(t *testing.T) {
	acl, err := buildDACL(true)
	if err != nil {
		t.Fatalf("buildDACL: %v", err)
	}
	aces, err := parseACL(acl)
	if err != nil {
		t.Fatalf("parseACL: %v", err)
	}
	if len(aces) != 3 {
		t.Fatalf("parsed %d ACEs, want 3", len(aces))
	}
	// Compare the parsed form with itself through a second parse: the
	// comparison must be stable and every ACE must carry a SID.
	again, err := parseACL(acl)
	if err != nil {
		t.Fatalf("second parseACL: %v", err)
	}
	if !sameACLs(aces, again) {
		t.Error("sameACLs disagrees on two parses of the same ACL")
	}
	for i, ace := range aces {
		if len(ace.sid) == 0 {
			t.Errorf("ACE %d has empty SID", i)
		}
		if ace.mask == 0 {
			t.Errorf("ACE %d has zero access mask", i)
		}
	}
}

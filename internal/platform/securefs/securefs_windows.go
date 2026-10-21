//go:build windows

package securefs

import (
	"encoding/binary"
	"errors"
	"log/slog"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Apply replaces inherited ACLs with full control for the current user,
// LocalSystem, and the local Administrators group.
//
// Rewriting an identical protected DACL is not free: SetNamedSecurityInfo
// propagates the inheritable ACEs and thereby walks every existing descendant
// of the directory, which costs seconds on real launcher data roots. The
// current DACL is read back and compared first, and rewritten only when it
// differs from the intended one.
func Apply(path string, _ os.FileMode, directory bool) error {
	acl, err := buildDACL(directory)
	if err != nil {
		return err
	}
	if daclMatches(path, acl) {
		slog.Debug("securefs: DACL already applied", "path", path)
		return nil
	}
	err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil)
	if errors.Is(err, windows.ERROR_INVALID_FUNCTION) || errors.Is(err, windows.ERROR_NOT_SUPPORTED) {
		// Filesystems without ACL support (exFAT, FAT32, some removable or
		// network drives) cannot be hardened; treat hardening as a no-op so
		// the data folder may live there.
		return nil
	}
	return err
}

// buildDACL returns the protected DACL Apply installs: full control for the
// current user, LocalSystem, and the local Administrators group, inheritable
// by children when the object is a directory.
func buildDACL(directory bool) (*windows.ACL, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	administrators, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return nil, err
	}
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return nil, err
	}
	inheritance := uint32(windows.NO_INHERITANCE)
	if directory {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	entry := func(sid *windows.SID, trusteeType windows.TRUSTEE_TYPE) windows.EXPLICIT_ACCESS {
		return windows.EXPLICIT_ACCESS{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       inheritance,
			Trustee:           windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: trusteeType, TrusteeValue: windows.TrusteeValueFromSID(sid)},
		}
	}
	return windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		entry(user.User.Sid, windows.TRUSTEE_IS_USER),
		entry(system, windows.TRUSTEE_IS_USER),
		entry(administrators, windows.TRUSTEE_IS_GROUP),
	}, nil)
}

// daclMatches reports whether path already carries exactly the desired
// protected DACL. Read or parse failures count as a mismatch: the caller then
// falls back to rewriting the DACL, which is the pre-existing behavior.
func daclMatches(path string, desired *windows.ACL) bool {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil || sd == nil {
		return false
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		// An unprotected DACL still inherits ACEs from the parent directory
		// and must be replaced.
		return false
	}
	current, _, err := sd.DACL()
	if err != nil || current == nil {
		return false
	}
	currentACEs, err := parseACL(current)
	if err != nil {
		return false
	}
	desiredACEs, err := parseACL(desired)
	if err != nil {
		return false
	}
	return sameACLs(currentACEs, desiredACEs)
}

var errMalformedACL = errors.New("securefs: malformed ACL")

// parseACL walks the raw ACL buffer: an 8-byte header (aclRevision, sbz1,
// aclSize, AceCount, sbz2) followed by AceCount ACEs, each a 4-byte ACE_HEADER
// followed, for allowed and denied ACEs, by the 4-byte access mask and the
// trustee SID.
func parseACL(acl *windows.ACL) ([]aclACE, error) {
	// The windows.ACL struct is exactly the 8-byte ACL header.
	header := (*[8]byte)(unsafe.Pointer(acl))
	total := int(binary.LittleEndian.Uint16(header[2:4]))
	count := int(binary.LittleEndian.Uint16(header[4:6]))
	if total < 8 {
		return nil, errMalformedACL
	}
	raw := unsafe.Slice((*byte)(unsafe.Pointer(acl)), total)
	aces := make([]aclACE, 0, count)
	offset := 8
	for i := 0; i < count; i++ {
		if offset+4 > total {
			return nil, errMalformedACL
		}
		aceSize := int(binary.LittleEndian.Uint16(raw[offset+2 : offset+4]))
		// 4 header bytes, 4 mask bytes, and at least an 8-byte SID.
		if aceSize < 12 || offset+aceSize > total {
			return nil, errMalformedACL
		}
		sid := make([]byte, aceSize-8)
		copy(sid, raw[offset+8:offset+aceSize])
		aces = append(aces, aclACE{
			aceType:  raw[offset],
			aceFlags: raw[offset+1],
			mask:     binary.LittleEndian.Uint32(raw[offset+4 : offset+8]),
			sid:      sid,
		})
		offset += aceSize
	}
	return aces, nil
}

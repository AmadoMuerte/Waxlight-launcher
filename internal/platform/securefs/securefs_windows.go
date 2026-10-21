//go:build windows

package securefs

import (
	"encoding/binary"
	"errors"
	"fmt"
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
	// Windows maps the GENERIC_* bits to file-specific rights when the DACL
	// is stored on an SE_FILE_OBJECT and GetNamedSecurityInfo returns the
	// mapped form, while the ACL built by ACLFromEntries keeps the generic
	// bits. Both sides must therefore be compared in specific form.
	normalizeGenericMasks(currentACEs)
	normalizeGenericMasks(desiredACEs)
	// Storing an inheritable ACE on a directory splits it into an explicit
	// ACE for the object and an inherit-only ACE for the children, so both
	// sides are canonicalized into one logical ACE per trustee before the
	// comparison.
	currentACEs, ok := canonicalizeACEs(currentACEs)
	if !ok {
		return false
	}
	desiredACEs, ok = canonicalizeACEs(desiredACEs)
	if !ok {
		return false
	}
	return sameACLs(currentACEs, desiredACEs)
}

// fileAllAccess is the file-specific expansion of GENERIC_ALL, equivalent to
// FILE_ALL_ACCESS from winnt.h: STANDARD_RIGHTS_REQUIRED | SYNCHRONIZE | 0x1FF.
const fileAllAccess uint32 = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0x1FF

// normalizeGenericMasks expands the generic access bits in every ACE mask to
// the file-specific rights they stand for on file-system objects.
func normalizeGenericMasks(aces []aclACE) {
	for i := range aces {
		aces[i].mask = fileSpecificMask(aces[i].mask)
	}
}

// fileSpecificMask expands the GENERIC_* access bits into the file-specific
// rights (the file object's generic mapping), mirroring MapGenericMask.
func fileSpecificMask(mask uint32) uint32 {
	specific := mask &^ (windows.GENERIC_READ | windows.GENERIC_WRITE | windows.GENERIC_EXECUTE | windows.GENERIC_ALL)
	if mask&windows.GENERIC_READ != 0 {
		specific |= windows.FILE_GENERIC_READ
	}
	if mask&windows.GENERIC_WRITE != 0 {
		specific |= windows.FILE_GENERIC_WRITE
	}
	if mask&windows.GENERIC_EXECUTE != 0 {
		specific |= windows.FILE_GENERIC_EXECUTE
	}
	if mask&windows.GENERIC_ALL != 0 {
		specific |= fileAllAccess
	}
	return specific
}

// describeDACL returns a human-readable dump of the DACL currently stored on
// path, for test diagnostics.
func describeDACL(path string) string {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Sprintf("read error: %v", err)
	}
	control, _, err := sd.Control()
	if err != nil {
		return fmt.Sprintf("control error: %v", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Sprintf("dacl error: %v", err)
	}
	if dacl == nil {
		return fmt.Sprintf("control=%#04x no DACL", control)
	}
	aces, err := parseACL(dacl)
	if err != nil {
		return fmt.Sprintf("control=%#04x parse error: %v", control, err)
	}
	return fmt.Sprintf("control=%#04x %s", control, describeACEs(aces))
}

// describeACL returns a human-readable dump of the parsed ACEs of acl.
func describeACL(acl *windows.ACL) string {
	aces, err := parseACL(acl)
	if err != nil {
		return fmt.Sprintf("parse error: %v", err)
	}
	return describeACEs(aces)
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

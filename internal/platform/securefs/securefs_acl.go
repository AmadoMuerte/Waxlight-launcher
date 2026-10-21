package securefs

import (
	"bytes"
	"fmt"
	"strings"
)

// aceInheritOnly is the INHERIT_ONLY_ACE flag: an ACE that applies
// exclusively to children, not to the object itself.
const aceInheritOnly uint8 = 0x08

// aclACE is the normalized form of one access-control entry, used to compare
// ACLs for equality. The SID is the raw binary SID stored in the ACE.
type aclACE struct {
	aceType  uint8
	aceFlags uint8
	mask     uint32
	sid      []byte
}

// aceGroup identifies the ACEs that belong to one trustee.
type aceGroup struct {
	aceType uint8
	sid     string
}

// canonicalizeACEs collapses the ACE pairs Windows splits an inheritable ACE
// into when a directory DACL is stored: one explicit ACE for the object
// itself (no inheritance flags, access mask mapped to file-specific rights)
// and one inherit-only ACE for the children (inheritance flags, generic
// mask). ACEs are grouped by (type, SID) in first-appearance order; the
// merged ACE keeps the union of the non-inherit-only flags and every member
// of a group must hold the same access mask. It reports false when a group's
// masks disagree, because the stored DACL then is not an exact split of one
// uniform grant and must be rewritten.
func canonicalizeACEs(aces []aclACE) ([]aclACE, bool) {
	merged := make([]aclACE, 0, len(aces))
	index := make(map[aceGroup]int, len(aces))
	for _, ace := range aces {
		key := aceGroup{aceType: ace.aceType, sid: string(ace.sid)}
		i, ok := index[key]
		if !ok {
			index[key] = len(merged)
			merged = append(merged, aclACE{
				aceType:  ace.aceType,
				aceFlags: ace.aceFlags &^ aceInheritOnly,
				mask:     ace.mask,
				sid:      ace.sid,
			})
			continue
		}
		merged[i].aceFlags |= ace.aceFlags &^ aceInheritOnly
		if merged[i].mask != ace.mask {
			return nil, false
		}
	}
	return merged, true
}

// sameACLs reports whether two ACLs hold the same ACEs in the same order.
// Order matters: the DACL only counts as matching when it is exactly the one
// this package would write, so any externally reordered variant is rewritten.
func sameACLs(current, desired []aclACE) bool {
	if len(current) != len(desired) {
		return false
	}
	for i := range current {
		if current[i].aceType != desired[i].aceType ||
			current[i].aceFlags != desired[i].aceFlags ||
			current[i].mask != desired[i].mask ||
			!bytes.Equal(current[i].sid, desired[i].sid) {
			return false
		}
	}
	return true
}

// describeACEs returns a human-readable dump of parsed ACEs, for diagnostics.
func describeACEs(aces []aclACE) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d ACEs", len(aces))
	for i, ace := range aces {
		fmt.Fprintf(&b, "; [%d] type=%#02x flags=%#02x mask=%#08x sid=%x", i, ace.aceType, ace.aceFlags, ace.mask, ace.sid)
	}
	return b.String()
}

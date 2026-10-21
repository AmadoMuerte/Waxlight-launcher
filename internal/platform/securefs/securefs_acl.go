package securefs

import "bytes"

// aclACE is the normalized form of one access-control entry, used to compare
// ACLs for equality. The SID is the raw binary SID stored in the ACE.
type aclACE struct {
	aceType  uint8
	aceFlags uint8
	mask     uint32
	sid      []byte
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

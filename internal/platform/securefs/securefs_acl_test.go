package securefs

import (
	"bytes"
	"testing"
)

func cloneACEs(aces []aclACE) []aclACE {
	cloned := make([]aclACE, len(aces))
	for i, ace := range aces {
		cloned[i] = ace
		cloned[i].sid = bytes.Clone(ace.sid)
	}
	return cloned
}

func TestSameACLs(t *testing.T) {
	base := []aclACE{
		{aceType: 0, aceFlags: 0x03, mask: 0x10000000, sid: []byte{1, 1, 0, 0, 0, 0, 0, 5}},
		{aceType: 0, aceFlags: 0x03, mask: 0x10000000, sid: []byte{1, 2, 0, 0, 0, 0, 0, 5, 32, 32, 0, 0, 0, 1}},
	}
	tests := []struct {
		name    string
		current []aclACE
		desired []aclACE
		want    bool
	}{
		{
			name:    "identical lists",
			current: base,
			desired: cloneACEs(base),
			want:    true,
		},
		{
			name:    "both empty",
			current: nil,
			desired: nil,
			want:    true,
		},
		{
			name:    "different lengths",
			current: base,
			desired: base[:1],
			want:    false,
		},
		{
			name:    "different order",
			current: []aclACE{base[1], base[0]},
			desired: base,
			want:    false,
		},
		{
			name:    "different mask",
			current: []aclACE{{aceType: 0, aceFlags: 3, mask: 0x1f01ff, sid: base[0].sid}},
			desired: []aclACE{{aceType: 0, aceFlags: 3, mask: 0x10000000, sid: base[0].sid}},
			want:    false,
		},
		{
			name:    "different ace type",
			current: []aclACE{{aceType: 1, aceFlags: 3, mask: 0x10000000, sid: base[0].sid}},
			desired: []aclACE{{aceType: 0, aceFlags: 3, mask: 0x10000000, sid: base[0].sid}},
			want:    false,
		},
		{
			name:    "different ace flags",
			current: []aclACE{{aceType: 0, aceFlags: 0x13, mask: 0x10000000, sid: base[0].sid}},
			desired: []aclACE{{aceType: 0, aceFlags: 0x03, mask: 0x10000000, sid: base[0].sid}},
			want:    false,
		},
		{
			name:    "different sid",
			current: []aclACE{{aceType: 0, aceFlags: 3, mask: 0x10000000, sid: base[1].sid}},
			desired: []aclACE{{aceType: 0, aceFlags: 3, mask: 0x10000000, sid: base[0].sid}},
			want:    false,
		},
		{
			name:    "extra ace appended",
			current: append(cloneACEs(base), aclACE{aceType: 0, aceFlags: 3, mask: 0x10000000, sid: base[0].sid}),
			desired: base,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameACLs(tt.current, tt.desired); got != tt.want {
				t.Errorf("sameACLs() = %v, want %v", got, tt.want)
			}
		})
	}
}

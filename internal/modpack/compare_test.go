package modpack

import "testing"

func TestVersionEquals(t *testing.T) {
	for _, test := range []struct {
		left, right string
		equal       bool
	}{
		{"1.2.3", "v1.2.3", true},
		{"1.2.3", "1.2.3", true},
		{"1.2.3", "1.2.4", false},
		{"1.2.3", "", false},
		{"", "", true},
		{"1.2.3-beta.1", "v1.2.3-beta.1", true},
	} {
		if actual := VersionEquals(test.left, test.right); actual != test.equal {
			t.Errorf("VersionEquals(%q, %q) = %v, want %v", test.left, test.right, actual, test.equal)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	for _, test := range []struct {
		left, right string
		want        int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.2", "1.2.3", -1},
		{"1.3.0", "1.2.3", 1},
		{"v1.2.3", "1.2.3", 0},
		{"1.2.3-beta", "1.2.3", -1},
		{"foo", "bar", 1},
	} {
		got := CompareVersions(test.left, test.right)
		if (got < 0 && test.want >= 0) || (got > 0 && test.want <= 0) || (got == 0 && test.want != 0) {
			t.Errorf("CompareVersions(%q, %q) = %d, want sign %d", test.left, test.right, got, test.want)
		}
	}
}

func TestSupportsGameVersion(t *testing.T) {
	for _, test := range []struct {
		supported []string
		requested string
		want      bool
	}{
		{[]string{"1.19"}, "1.19.8", true},
		{[]string{"1.19"}, "1.19", true},
		{[]string{"1.19.8"}, "1.19.8", true},
		{[]string{"1.20"}, "1.19.8", false},
		{nil, "1.19.8", true},
		{[]string{"1.19", "1.20"}, "1.20.3", true},
	} {
		if actual := supportsGameVersion(test.supported, test.requested); actual != test.want {
			t.Errorf("supportsGameVersion(%v, %q) = %v, want %v", test.supported, test.requested, actual, test.want)
		}
	}
}

func TestIsBuiltInDependency(t *testing.T) {
	for _, modID := range []string{"game", "survival", "creative", "Game", " SURVIVAL "} {
		if !isBuiltInDependency(modID) {
			t.Errorf("expected %q to be a built-in dependency", modID)
		}
	}
	for _, modID := range []string{"stonequarry", "xskills", ""} {
		if isBuiltInDependency(modID) {
			t.Errorf("expected %q not to be a built-in dependency", modID)
		}
	}
}

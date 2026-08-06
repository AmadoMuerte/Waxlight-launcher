package modpack

import (
	"strings"

	"golang.org/x/mod/semver"
)

// VersionEquals reports whether two version strings identify the same version.
// It compares them as semantic versions when possible and falls back to a
// case-insensitive string comparison otherwise, so "1.2.3" equals "v1.2.3".
func VersionEquals(left, right string) bool {
	if left == "" || right == "" {
		return strings.EqualFold(left, right)
	}
	leftSemver := normalizeSemver(left)
	rightSemver := normalizeSemver(right)
	if leftSemver != "" && rightSemver != "" {
		return leftSemver == rightSemver
	}
	return strings.EqualFold(left, right)
}

// CompareVersions compares two version strings. It returns a negative value
// when left is older than right, zero when they are equal, and a positive
// value when left is newer. Versions that cannot be parsed as semantic
// versions are compared as plain strings and sort before parseable versions.
func CompareVersions(left, right string) int {
	leftSemver := normalizeSemver(left)
	rightSemver := normalizeSemver(right)
	switch {
	case leftSemver != "" && rightSemver != "":
		return semver.Compare(leftSemver, rightSemver)
	case leftSemver != "":
		return 1
	case rightSemver != "":
		return -1
	default:
		return strings.Compare(strings.ToLower(left), strings.ToLower(right))
	}
}

// supportsGameVersion reports whether a mod version declares support for the
// requested Vintage Story version. A version entry such as "1.19" also covers
// every "1.19.x" release. An empty supported list means the version does not
// restrict compatibility.
func supportsGameVersion(supported []string, requested string) bool {
	for _, version := range supported {
		if version == requested {
			return true
		}
		majorMinor := majorMinor(version)
		if majorMinor != "" && strings.HasPrefix(requested, majorMinor+".") {
			return true
		}
	}
	return len(supported) == 0
}

func majorMinor(version string) string {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "." + parts[1]
}

func normalizeSemver(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(version), "v") {
		version = "v" + version
	}
	if !semver.IsValid(version) {
		return ""
	}
	return semver.Canonical(version)
}

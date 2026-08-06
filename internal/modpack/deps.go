package modpack

import "strings"

// dependencyDiff compares the dependencies declared by the installed version
// of a mod with the dependencies required by the target update version.
//
// AddedDeps contains target dependencies that are not installed in the build
// and are not built-in game modules. Dependencies that the installed version
// already declared are not reported, since they were presumably satisfied
// before the update.
//
// RemovedDeps contains installed dependencies that the target version no longer
// requires and that are installed in the build. Such mods may become unused
// after the update.
func dependencyDiff(
	installedDeps []string,
	targetDeps []Dependency,
	installed map[string]struct{},
) (added []Dependency, removed []Dependency) {
	declared := make(map[string]struct{}, len(installedDeps))
	for _, modID := range installedDeps {
		declared[modID] = struct{}{}
	}
	for _, dependency := range targetDeps {
		if isBuiltInDependency(dependency.ModID) {
			continue
		}
		if _, present := installed[dependency.ModID]; present {
			continue
		}
		if _, alreadyDeclared := declared[dependency.ModID]; alreadyDeclared {
			continue
		}
		added = append(added, dependency)
	}
	for _, modID := range installedDeps {
		if isBuiltInDependency(modID) {
			continue
		}
		if !stillRequired(modID, targetDeps) {
			if _, present := installed[modID]; present {
				removed = append(removed, Dependency{ModID: modID})
			}
		}
	}
	return added, removed
}

func stillRequired(modID string, targetDeps []Dependency) bool {
	for _, dependency := range targetDeps {
		if dependency.ModID == modID {
			return true
		}
	}
	return false
}

func isBuiltInDependency(modID string) bool {
	switch strings.ToLower(strings.TrimSpace(modID)) {
	case "game", "survival", "creative":
		return true
	default:
		return false
	}
}

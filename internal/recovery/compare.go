package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/snapshots"
)

// compareConfigurations reports the facts that differ between the Last Known
// Good state and the current instance state. It never attributes the
// differences to a cause.
func compareConfigurations(
	lkg LastKnownGood,
	currentMods []snapshots.Mod,
	currentNames map[string]string,
	currentGameVersion string,
) ConfigurationChanges {
	changes := ConfigurationChanges{
		Updated: []ModChange{},
		Added:   []ModChange{},
		Removed: []ModChange{},
	}
	if lkg.GameVersion != currentGameVersion {
		changes.GameVersionFrom = lkg.GameVersion
		changes.GameVersionTo = currentGameVersion
	}

	lkgByKey := make(map[string]snapshots.Mod, len(lkg.Mods))
	for _, mod := range lkg.Mods {
		lkgByKey[snapshots.ModKey(mod)] = mod
	}
	currentByKey := make(map[string]snapshots.Mod, len(currentMods))
	for _, mod := range currentMods {
		currentByKey[snapshots.ModKey(mod)] = mod
	}

	for key, mod := range currentByKey {
		previous, ok := lkgByKey[key]
		if !ok {
			changes.Added = append(changes.Added, ModChange{
				Name: configurationModName(key, currentNames, mod),
				To:   mod.Version,
			})
			continue
		}
		if previous.Version != mod.Version {
			changes.Updated = append(changes.Updated, ModChange{
				Name: configurationModName(key, currentNames, mod),
				From: previous.Version,
				To:   mod.Version,
			})
		}
	}
	for key, mod := range lkgByKey {
		if _, ok := currentByKey[key]; !ok {
			changes.Removed = append(changes.Removed, ModChange{
				Name: snapshots.ModDisplayName(mod),
				From: mod.Version,
			})
		}
	}

	sortModChanges(changes.Updated)
	sortModChanges(changes.Added)
	sortModChanges(changes.Removed)
	return changes
}

func configurationModName(key string, names map[string]string, mod snapshots.Mod) string {
	if name := names[key]; strings.TrimSpace(name) != "" {
		return name
	}
	return snapshots.ModDisplayName(mod)
}

func sortModChanges(changes []ModChange) {
	sort.Slice(changes, func(left, right int) bool {
		return strings.ToLower(changes[left].Name) < strings.ToLower(changes[right].Name)
	})
}

// installedModKey derives the stable identity of an installed mod record,
// matching snapshots.ModKey for the same mod.
func installedModKey(mod snapshots.InstalledMod) string {
	if modID, _, ok := snapshots.ParseModDBSource(mod.Source); ok {
		return "moddb:" + modID
	}
	name := strings.TrimSpace(mod.Name)
	if name == "" {
		name = strings.TrimSpace(mod.FileName)
	}
	return "local:" + name
}

// configurationSignature is a stable fingerprint of an instance configuration
// (game version and exact mod releases). The frontend uses it to suppress
// repeat recovery prompts for the same failed state.
func configurationSignature(mods []snapshots.Mod, gameVersion string) string {
	hash := sha256.New()
	hash.Write([]byte(gameVersion))
	entries := make([]string, 0, len(mods))
	for _, mod := range mods {
		entries = append(entries, snapshots.ModKey(mod)+"::"+mod.Version)
	}
	sort.Strings(entries)
	for _, entry := range entries {
		hash.Write([]byte{0})
		hash.Write([]byte(entry))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

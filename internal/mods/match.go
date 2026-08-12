package mods

import (
	"context"
	"path/filepath"
	"strings"
)

// modVersionMatch is a local mod file matched to a catalog record and release.
type modVersionMatch struct {
	details ModDetails
	version ModVersion
}

// matchLocalModForFile resolves the catalog identity of a local mod file.
func matchLocalModForFile(ctx context.Context, catalog Catalog, filePath string) (modVersionMatch, bool) {
	info, err := ReadModArchiveInfo(filePath)
	if err != nil {
		return modVersionMatch{}, false
	}
	match, _, found := matchLocalMod(ctx, catalog, info.ModID, info.Version, filepath.Base(filePath))
	return match, found
}

// matchLocalMod finds the catalog record and release of a local mod file.
// The reason string explains why no match was found.
func matchLocalMod(
	ctx context.Context,
	catalog Catalog,
	modID string,
	version string,
	fileName string,
) (modVersionMatch, string, bool) {
	if catalog == nil {
		return modVersionMatch{}, "catalog_unavailable", false
	}
	summaries, err := catalog.List(ctx)
	if err != nil {
		return modVersionMatch{}, "catalog_unavailable", false
	}
	candidates := matchCandidates(summaries, modID)
	if len(candidates) == 0 {
		return modVersionMatch{}, "not_in_catalog", false
	}
	matches := make([]modVersionMatch, 0, 2)
	seen := make(map[string]struct{})
	for _, summary := range candidates {
		details, getErr := catalog.Get(ctx, summary.ID)
		if getErr != nil {
			continue
		}
		selected, ok := pickLocalModVersion(details.Versions, version, fileName)
		if !ok {
			continue
		}
		key := details.ID + ":" + selected.ID
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		matches = append(matches, modVersionMatch{details: details, version: selected})
	}
	switch len(matches) {
	case 1:
		return matches[0], "", true
	case 0:
		return modVersionMatch{}, "version_not_found", false
	default:
		return modVersionMatch{}, "ambiguous", false
	}
}

func matchCandidates(summaries []ModSummary, modID string) []ModSummary {
	if strings.TrimSpace(modID) == "" {
		return nil
	}
	var byModID []ModSummary
	for _, summary := range summaries {
		for _, candidate := range summary.ModIDStrings {
			if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(modID)) {
				byModID = append(byModID, summary)
				break
			}
		}
	}
	return byModID
}

func pickLocalModVersion(versions []ModVersion, version, fileName string) (ModVersion, bool) {
	version = strings.TrimSpace(version)
	if version != "" {
		var matches []ModVersion
		for _, candidate := range versions {
			if strings.EqualFold(strings.TrimSpace(candidate.Version), version) {
				matches = append(matches, candidate)
			}
		}
		if len(matches) == 1 {
			return matches[0], true
		}
		if len(matches) > 1 {
			if selected, ok := matchByFileName(matches, fileName); ok {
				return selected, true
			}
			return matches[0], true
		}
	}
	if selected, ok := matchByFileName(versions, fileName); ok {
		return selected, true
	}
	if len(versions) == 1 {
		return versions[0], true
	}
	return ModVersion{}, false
}

func matchByFileName(versions []ModVersion, fileName string) (ModVersion, bool) {
	if fileName == "" {
		return ModVersion{}, false
	}
	base := strings.ToLower(filepath.Base(fileName))
	for _, version := range versions {
		if version.FileName != "" && strings.EqualFold(strings.ToLower(filepath.Base(version.FileName)), base) {
			return version, true
		}
	}
	return ModVersion{}, false
}

package presentation

import (
	"github.com/waxlight/waxlight-launcher/internal/recovery"
)

type ModChangeDTO struct {
	Name string `json:"name"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

type ConfigurationChangesDTO struct {
	GameVersionFrom string         `json:"gameVersionFrom,omitempty"`
	GameVersionTo   string         `json:"gameVersionTo,omitempty"`
	Updated         []ModChangeDTO `json:"updated"`
	Added           []ModChangeDTO `json:"added"`
	Removed         []ModChangeDTO `json:"removed"`
}

// LastKnownGoodDTO describes the Last Known Good marker of an instance and how
// the current configuration differs from it. SnapshotID is present only when a
// restorable snapshot captures the marker and one-click recovery is possible.
type LastKnownGoodDTO struct {
	RecordedAt     string                  `json:"recordedAt"`
	GameVersion    string                  `json:"gameVersion"`
	ModCount       int                     `json:"modCount"`
	SnapshotID     string                  `json:"snapshotId,omitempty"`
	SnapshotExists bool                    `json:"snapshotExists"`
	MatchesCurrent bool                    `json:"matchesCurrent"`
	ChangeCount    int                     `json:"changeCount"`
	Changes        ConfigurationChangesDTO `json:"changes"`
}

func lastKnownGoodDTO(status recovery.LastKnownGoodStatus) LastKnownGoodDTO {
	// No marker was recorded yet: the zero status must not render as a real
	// marker. An empty recordedAt signals the frontend that nothing exists.
	if status.RecordedAt.IsZero() {
		return LastKnownGoodDTO{
			Changes: emptyConfigurationChangesDTO(),
		}
	}
	return LastKnownGoodDTO{
		RecordedAt:     iso(status.RecordedAt),
		GameVersion:    status.GameVersion,
		ModCount:       status.ModCount,
		SnapshotID:     status.SnapshotID,
		SnapshotExists: status.SnapshotExists,
		MatchesCurrent: status.MatchesCurrent,
		ChangeCount:    status.Changes.Count(),
		Changes:        configurationChangesDTO(status.Changes),
	}
}

func configurationChangesDTO(changes recovery.ConfigurationChanges) ConfigurationChangesDTO {
	return ConfigurationChangesDTO{
		GameVersionFrom: changes.GameVersionFrom,
		GameVersionTo:   changes.GameVersionTo,
		Updated:         modChangesDTO(changes.Updated),
		Added:           modChangesDTO(changes.Added),
		Removed:         modChangesDTO(changes.Removed),
	}
}

func emptyConfigurationChangesDTO() ConfigurationChangesDTO {
	return ConfigurationChangesDTO{
		Updated: []ModChangeDTO{},
		Added:   []ModChangeDTO{},
		Removed: []ModChangeDTO{},
	}
}

func modChangesDTO(changes []recovery.ModChange) []ModChangeDTO {
	if len(changes) == 0 {
		return []ModChangeDTO{}
	}
	result := make([]ModChangeDTO, 0, len(changes))
	for _, change := range changes {
		result = append(result, ModChangeDTO{
			Name: change.Name,
			From: change.From,
			To:   change.To,
		})
	}
	return result
}

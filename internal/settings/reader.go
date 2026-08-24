package settings

import (
	"context"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/language"
)

// Reader is the immutable settings query service.
type Reader struct {
	repository Repository
}

func NewReader(repository Repository) *Reader {
	return &Reader{repository: repository}
}

func (reader *Reader) Get(ctx context.Context) (Settings, error) {
	value, err := reader.repository.GetSettings(ctx)
	if err != nil {
		return value, err
	}
	normalizedLanguage := language.NormalizeLanguage(value.Language)
	normalizedChannel, channelErr := normalizeUpdateChannel(value.UpdateChannel)
	normalizedLibrarySort := normalizeLibrarySort(value.LibrarySort)
	if channelErr != nil {
		normalizedChannel = "stable"
	}
	if value.GlobalLaunchArguments == nil {
		value.GlobalLaunchArguments = []string{}
	}
	if value.AutomaticSnapshotRetention < AutomaticSnapshotRetentionMin || value.AutomaticSnapshotRetention > AutomaticSnapshotRetentionMax {
		value.AutomaticSnapshotRetention = AutomaticSnapshotRetentionDefault
	}
	if value.Language != normalizedLanguage || value.UpdateChannel != normalizedChannel || value.LibrarySort != normalizedLibrarySort {
		value.Language = normalizedLanguage
		value.UpdateChannel = normalizedChannel
		value.LibrarySort = normalizedLibrarySort
		if err := reader.repository.SaveSettings(ctx, value); err != nil {
			return value, err
		}
	}
	return value, nil
}

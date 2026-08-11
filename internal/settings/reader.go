package settings

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/language"
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
	if channelErr != nil {
		normalizedChannel = "stable"
	}
	if value.GlobalLaunchArguments == nil {
		value.GlobalLaunchArguments = []string{}
	}
	if value.Language != normalizedLanguage || value.UpdateChannel != normalizedChannel {
		value.Language = normalizedLanguage
		value.UpdateChannel = normalizedChannel
		if err := reader.repository.SaveSettings(ctx, value); err != nil {
			return value, err
		}
	}
	return value, nil
}

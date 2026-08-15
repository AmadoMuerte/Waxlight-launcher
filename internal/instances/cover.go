package instances

import (
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"

	"github.com/waxlight/waxlight-launcher/internal/errs"
)

const maxCoverSize = 8 << 20

func (service *UpdateService) UpdateWithCover(ctx context.Context, instance Instance, sourcePath *string) (Instance, error) {
	if sourcePath == nil {
		return service.Update(ctx, instance)
	}
	previousPath := instance.CoverPath
	if *sourcePath == "" {
		instance.CoverPath = nil
		updated, updateErr := service.Update(ctx, instance)
		if updateErr == nil {
			removeManagedCover(instance.Directory, previousPath)
		}
		return updated, updateErr
	}

	source, err := os.Open(*sourcePath)
	if err != nil {
		return Instance{}, err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return Instance{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxCoverSize {
		return Instance{}, errs.NewError(errs.ErrValidation, "Cover must be a regular image no larger than 8 MiB")
	}
	if _, _, err = image.DecodeConfig(source); err != nil {
		return Instance{}, errs.NewError(errs.ErrValidation, "Cover must be a PNG, JPEG, or GIF image")
	}
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return Instance{}, err
	}

	cover, err := os.CreateTemp(instance.Directory, ".waxlight-cover-*")
	if err != nil {
		return Instance{}, err
	}
	coverPath := cover.Name()
	defer func() {
		_ = cover.Close()
	}()
	if _, err = io.Copy(cover, source); err != nil {
		_ = os.Remove(coverPath)
		return Instance{}, err
	}
	if err = cover.Close(); err != nil {
		_ = os.Remove(coverPath)
		return Instance{}, err
	}

	instance.CoverPath = &coverPath
	updated, err := service.Update(ctx, instance)
	if err != nil {
		_ = os.Remove(coverPath)
		return Instance{}, err
	}
	removeManagedCover(instance.Directory, previousPath)
	return updated, nil
}

func removeManagedCover(instanceDirectory string, coverPath *string) {
	if coverPath == nil {
		return
	}
	relative, err := filepath.Rel(instanceDirectory, *coverPath)
	if err == nil && relative != ".." && !filepath.IsAbs(relative) && !startsWithParent(relative) {
		_ = os.Remove(*coverPath)
	}
}

func startsWithParent(path string) bool {
	return len(path) > 3 && path[:3] == ".."+string(filepath.Separator)
}

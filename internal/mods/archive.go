package mods

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	vsmodinfo "github.com/AmadoMuerte/vintagestory-go/modinfo"
)

// ArchiveInfo is the modinfo.json metadata of a mod file.
type ArchiveInfo struct {
	ModID        string
	Name         string
	Version      string
	Dependencies map[string]string
}

// ReadModArchiveInfo inspects a mod file's modinfo.json. Archives without a
// modinfo.json entry and non-archive files return an empty ArchiveInfo with a
// nil error; corrupt, oversized, or unreadable archives return an
// INVALID_MOD_FILE error.
func ReadModArchiveInfo(filePath string) (ArchiveInfo, error) {
	info, err := vsmodinfo.ReadArchive(filePath)
	if err == nil {
		return ArchiveInfo{
			ModID:        info.ModID,
			Name:         info.Name,
			Version:      info.Version,
			Dependencies: info.Dependencies,
		}, nil
	}
	switch {
	case errors.Is(err, vsmodinfo.ErrNotAnArchive), errors.Is(err, vsmodinfo.ErrNoModInfo):
		// Keep supporting catalog entries that are not ZIP-based mods. Such
		// packages simply cannot advertise dependencies through modinfo.json.
		return ArchiveInfo{}, nil
	case errors.Is(err, vsmodinfo.ErrTooLarge):
		return ArchiveInfo{}, errs.NewError(ErrInvalidModFile, "modinfo.json is unexpectedly large")
	case errors.Is(err, vsmodinfo.ErrInvalidContent):
		return ArchiveInfo{}, &errs.AppError{
			Code:    ErrInvalidModFile,
			Message: "The downloaded mod contains an invalid modinfo.json",
			Cause:   err,
		}
	default:
		return ArchiveInfo{}, &errs.AppError{
			Code:    ErrInvalidModFile,
			Message: "Could not inspect the downloaded mod archive",
			Cause:   err,
		}
	}
}

// copyModFile copies a mod file into the library with cancellation support.
func copyModFile(ctx context.Context, sourcePath, destinationPath string) error {
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, &ContextReader{Ctx: ctx, Reader: source})
	closeErr := destination.Close()
	if copyErr != nil {
		if err := os.Remove(destinationPath); err != nil {
			slog.Debug("could not remove the incomplete mod copy", "path", destinationPath, "error", err)
		}
		return copyErr
	}
	if closeErr != nil {
		if err := os.Remove(destinationPath); err != nil {
			slog.Debug("could not remove the incomplete mod copy", "path", destinationPath, "error", err)
		}
		return closeErr
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

// ContextReader wraps a reader with cancellation checking, used by snapshot
// restore and library copies.
type ContextReader struct {
	Ctx    context.Context
	Reader io.Reader
}

func (reader *ContextReader) Read(buffer []byte) (int, error) {
	if err := reader.Ctx.Err(); err != nil {
		return 0, err
	}
	return reader.Reader.Read(buffer)
}

// friendlyInstallError renders a user-facing message for a failed mod file
// install.
func friendlyInstallError(err error) string {
	if errors.Is(err, os.ErrPermission) {
		return "Waxlight does not have permission to write to this instance"
	}
	if errors.Is(err, context.Canceled) {
		return "Mod installation was cancelled"
	}
	if errors.Is(err, os.ErrNotExist) {
		return "The downloaded mod file is missing. Download it again and retry."
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "mod source must be a file"):
		return "The downloaded mod is not a file"
	case strings.Contains(message, "unsupported mod file extension"):
		return "The downloaded file is not a supported mod archive"
	case strings.Contains(message, "mod file already exists"):
		return "A different mod file with this name already exists in the instance"
	}
	return "Could not install the mod in this instance"
}

// dependencyFailureMessage renders the user-facing failure message of a
// catalog download that could not resolve or install its dependencies.
func dependencyFailureMessage(err error) string {
	if errors.Is(err, context.Canceled) {
		return "Download cancelled"
	}
	var appErr *errs.AppError
	if errors.As(err, &appErr) && appErr.Message != "" {
		return appErr.Message
	}
	return "Could not resolve or install mod dependencies"
}

// isAppErrorCode reports whether the error chain contains an AppError with the
// given code.
func isAppErrorCode(err error, code string) bool {
	var appError *errs.AppError
	return errors.As(err, &appError) && appError.Code == code
}

package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	modsDirectory         = "Mods"
	disabledModsDirectory = "ModsDisabled"
)

type ModFileManager struct{}

func (ModFileManager) EnsureLayout(instanceDirectory string) error {
	if err := migrateDirectory(
		filepath.Join(instanceDirectory, "mods"),
		filepath.Join(instanceDirectory, modsDirectory),
	); err != nil {
		return err
	}

	if err := migrateDirectory(
		filepath.Join(instanceDirectory, "mods-disabled"),
		filepath.Join(instanceDirectory, disabledModsDirectory),
	); err != nil {
		return err
	}

	if err := os.MkdirAll(
		filepath.Join(instanceDirectory, modsDirectory),
		0o755,
	); err != nil {
		return err
	}

	return os.MkdirAll(
		filepath.Join(instanceDirectory, disabledModsDirectory),
		0o755,
	)
}

func (ModFileManager) Install(
	ctx context.Context,
	sourcePath string,
	instanceDirectory string,
) (string, int64, error) {
	extension := strings.ToLower(filepath.Ext(sourcePath))
	if extension != ".zip" && extension != ".cs" && extension != ".dll" {
		return "", 0, fmt.Errorf(
			"unsupported mod file extension: %s",
			extension,
		)
	}

	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", 0, err
	}
	if info.IsDir() {
		return "", 0, fmt.Errorf("mod source must be a file")
	}

	modsPath := filepath.Join(instanceDirectory, modsDirectory)
	if err := os.MkdirAll(modsPath, 0o755); err != nil {
		return "", 0, err
	}

	destinationPath := filepath.Join(modsPath, filepath.Base(sourcePath))
	if _, err := os.Stat(destinationPath); err == nil {
		return "", 0, fmt.Errorf("mod file already exists")
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return "", 0, err
	}
	defer source.Close()

	partialPath := destinationPath + ".partial"
	destination, err := os.OpenFile(
		partialPath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0o644,
	)
	if err != nil {
		return "", 0, err
	}

	_, copyErr := io.Copy(
		destination,
		&contextReader{ctx: ctx, reader: source},
	)
	closeErr := destination.Close()
	if copyErr != nil {
		_ = os.Remove(partialPath)
		return "", 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(partialPath)
		return "", 0, closeErr
	}

	if err := os.Rename(partialPath, destinationPath); err != nil {
		return "", 0, err
	}
	return destinationPath, info.Size(), nil
}

func (ModFileManager) InstallOrReplace(
	ctx context.Context,
	sourcePath string,
	instanceDirectory string,
	oldPath string,
) (string, int64, error) {
	if oldPath == "" {
		return (ModFileManager{}).Install(ctx, sourcePath, instanceDirectory)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", 0, err
	}
	if info.IsDir() {
		return "", 0, fmt.Errorf("mod source must be a file")
	}
	extension := strings.ToLower(filepath.Ext(sourcePath))
	if extension != ".zip" && extension != ".cs" && extension != ".dll" {
		return "", 0, fmt.Errorf("unsupported mod file extension: %s", extension)
	}
	modsPath := filepath.Join(instanceDirectory, modsDirectory)
	if err := os.MkdirAll(modsPath, 0o755); err != nil {
		return "", 0, err
	}
	destinationPath := filepath.Join(modsPath, filepath.Base(sourcePath))
	stagedPath := destinationPath + ".waxlight-new"
	staged, err := os.OpenFile(stagedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", 0, err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		_ = staged.Close()
		_ = os.Remove(stagedPath)
		return "", 0, err
	}
	_, copyErr := io.Copy(staged, &contextReader{ctx: ctx, reader: source})
	closeSourceErr := source.Close()
	closeStagedErr := staged.Close()
	if copyErr != nil || closeSourceErr != nil || closeStagedErr != nil {
		_ = os.Remove(stagedPath)
		if copyErr != nil {
			return "", 0, copyErr
		}
		if closeSourceErr != nil {
			return "", 0, closeSourceErr
		}
		return "", 0, closeStagedErr
	}

	backupPath := oldPath + ".waxlight-backup"
	if _, err := os.Stat(oldPath); err == nil {
		_ = os.Remove(backupPath)
		if err := os.Rename(oldPath, backupPath); err != nil {
			_ = os.Remove(stagedPath)
			return "", 0, err
		}
	}
	if destinationPath != oldPath {
		if _, err := os.Stat(destinationPath); err == nil {
			_ = os.Rename(backupPath, oldPath)
			_ = os.Remove(stagedPath)
			return "", 0, fmt.Errorf("mod file already exists")
		}
	}
	if err := os.Rename(stagedPath, destinationPath); err != nil {
		_ = os.Rename(backupPath, oldPath)
		_ = os.Remove(stagedPath)
		return "", 0, err
	}
	_ = os.Remove(backupPath)
	return destinationPath, info.Size(), nil
}

func (ModFileManager) SetEnabled(
	filePath string,
	instanceDirectory string,
	enabled bool,
) (string, error) {
	targetDirectory := disabledModsDirectory
	if enabled {
		targetDirectory = modsDirectory
	}

	targetRoot := filepath.Join(instanceDirectory, targetDirectory)
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return "", err
	}

	targetPath := filepath.Join(targetRoot, filepath.Base(filePath))
	if filePath == targetPath {
		return filePath, nil
	}
	if err := os.Rename(filePath, targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

func migrateDirectory(oldPath string, newPath string) error {
	oldInfo, err := os.Stat(oldPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !oldInfo.IsDir() {
		return fmt.Errorf("legacy mod path is not a directory: %s", oldPath)
	}

	newInfo, err := os.Stat(newPath)
	if os.IsNotExist(err) {
		return os.Rename(oldPath, newPath)
	} else if err != nil {
		return err
	}

	// Windows resolves `mods` and `Mods` to the same directory. Treat that as
	// an already-migrated layout instead of moving its entries onto themselves
	// and then trying to remove the still-populated directory.
	if os.SameFile(oldInfo, newInfo) {
		return nil
	}

	entries, err := os.ReadDir(oldPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(oldPath, entry.Name())
		destinationPath := filepath.Join(newPath, entry.Name())
		if _, err := os.Stat(destinationPath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(sourcePath, destinationPath); err != nil {
			return err
		}
	}

	return os.Remove(oldPath)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

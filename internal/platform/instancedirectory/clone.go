package instancedirectory

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type ClientSettingsSanitizer func([]byte) ([]byte, error)

type CloneStorage struct {
	sanitizeClientSettings ClientSettingsSanitizer
}

func NewCloneStorage(sanitizeClientSettings ClientSettingsSanitizer) CloneStorage {
	return CloneStorage{sanitizeClientSettings: sanitizeClientSettings}
}

func (storage CloneStorage) Copy(ctx context.Context, source, target string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	if sameClonePath(source, target) || clonePathWithin(source, target) || clonePathWithin(target, source) {
		return errors.New("source and clone directories overlap")
	}
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() || sourceInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("instance directory is not a regular directory and cannot be cloned")
	}
	targetInfo, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if !targetInfo.IsDir() || targetInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("clone directory is not a regular directory")
	}
	if hasPhysicalAncestor(source, targetInfo) || hasPhysicalAncestor(target, sourceInfo) {
		return errors.New("source and clone directories overlap")
	}
	sourceRoot, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()
	openedSourceInfo, err := sourceRoot.Stat(".")
	if err != nil || !os.SameFile(sourceInfo, openedSourceInfo) {
		return errors.New("instance directory changed while cloning started")
	}
	targetRoot, err := os.OpenRoot(target)
	if err != nil {
		return err
	}
	defer targetRoot.Close()
	openedTargetInfo, err := targetRoot.Stat(".")
	if err != nil || !os.SameFile(targetInfo, openedTargetInfo) {
		return errors.New("clone directory changed while cloning started")
	}

	return storage.copyDirectory(ctx, sourceRoot, targetRoot, ".", openedSourceInfo, openedTargetInfo)
}

func (storage CloneStorage) copyDirectory(
	ctx context.Context,
	sourceRoot, targetRoot *os.Root,
	relative string,
	expected, targetIdentity os.FileInfo,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	directory := sourceRoot
	if relative != "." {
		var err error
		directory, err = sourceRoot.OpenRoot(relative)
		if err != nil {
			return err
		}
		defer directory.Close()
	}
	openedInfo, err := directory.Stat(".")
	if err != nil || !openedInfo.IsDir() || !os.SameFile(expected, openedInfo) {
		return errors.New("instance directory changed while it was being cloned")
	}
	if os.SameFile(openedInfo, targetIdentity) {
		return errors.New("source and clone directories overlap")
	}
	entries, err := fs.ReadDir(directory.FS(), ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		child := entry.Name()
		if relative != "." {
			child = filepath.Join(relative, child)
		}
		if child == "SaveGame" || child == "Logs" {
			continue
		}
		info, err := sourceRoot.Lstat(child)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("instance directory contains a symbolic link and cannot be cloned")
		}
		if info.Name() == markerName || strings.HasSuffix(info.Name(), ".waxlight-auth-injection") {
			continue
		}
		if info.IsDir() {
			if err := ensureCloneDirectory(targetRoot, child, info.Mode().Perm()); err != nil {
				return err
			}
			if err := storage.copyDirectory(ctx, sourceRoot, targetRoot, child, info, targetIdentity); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return errors.New("instance directory contains a non-regular file and cannot be cloned")
		}
		if strings.EqualFold(child, "clientsettings.json") {
			if storage.sanitizeClientSettings == nil {
				return errors.New("client settings sanitizer is unavailable")
			}
			if err := copySanitizedClientSettings(ctx, sourceRoot, targetRoot, child, info, storage.sanitizeClientSettings); err != nil {
				return err
			}
			continue
		}
		if err := copyCloneFile(ctx, sourceRoot, targetRoot, child, info); err != nil {
			return err
		}
	}
	return nil
}

func hasPhysicalAncestor(path string, ancestor os.FileInfo) bool {
	for current := filepath.Dir(path); ; current = filepath.Dir(current) {
		info, err := os.Stat(current)
		if err == nil && os.SameFile(info, ancestor) {
			return true
		}
		next := filepath.Dir(current)
		if next == current {
			return false
		}
	}
}

func clonePathWithin(parent, candidate string) bool {
	relative, err := filepath.Rel(parent, candidate)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func sameClonePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func (CloneStorage) CopiedPath(source, target, path string) (string, bool) {
	relative, err := filepath.Rel(source, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", false
	}
	copied := filepath.Join(target, relative)
	info, err := os.Lstat(copied)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return copied, true
}

func copyCloneFile(ctx context.Context, sourceRoot, targetRoot *os.Root, relative string, expected os.FileInfo) error {
	input, err := sourceRoot.Open(relative)
	if err != nil {
		return err
	}
	defer input.Close()
	openedInfo, err := input.Stat()
	if err != nil {
		return err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(expected, openedInfo) {
		return errors.New("instance file changed while it was being cloned")
	}
	if err := ensureCloneDirectory(targetRoot, filepath.Dir(relative), 0o755); err != nil {
		return err
	}
	output, err := targetRoot.OpenFile(relative, os.O_WRONLY|os.O_CREATE|os.O_EXCL, expected.Mode().Perm())
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = output.Close()
		}
	}()
	buffer := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		read, readErr := input.Read(buffer)
		if read > 0 {
			if _, err := output.Write(buffer[:read]); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := output.Close(); err != nil {
		return err
	}
	closed = true
	return nil
}

func copySanitizedClientSettings(
	ctx context.Context,
	sourceRoot, targetRoot *os.Root,
	relative string,
	expected os.FileInfo,
	sanitize ClientSettingsSanitizer,
) error {
	if expected.Size() > 8<<20 {
		return errors.New("client settings file is too large to clone")
	}
	input, err := sourceRoot.Open(relative)
	if err != nil {
		return err
	}
	defer input.Close()
	openedInfo, err := input.Stat()
	if err != nil {
		return err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(expected, openedInfo) {
		return errors.New("client settings changed while it was being cloned")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	contents, err := io.ReadAll(io.LimitReader(input, (8<<20)+1))
	if err != nil {
		return err
	}
	if len(contents) > 8<<20 {
		return errors.New("client settings file is too large to clone")
	}
	sanitized, err := sanitize(contents)
	if err != nil {
		return err
	}
	if err := ensureCloneDirectory(targetRoot, filepath.Dir(relative), 0o755); err != nil {
		return err
	}
	output, err := targetRoot.OpenFile(relative, os.O_WRONLY|os.O_CREATE|os.O_EXCL, expected.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := output.Write(sanitized); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func ensureCloneDirectory(root *os.Root, relative string, mode os.FileMode) error {
	if relative == "." {
		return nil
	}
	currentPath := ""
	for _, part := range strings.Split(relative, string(os.PathSeparator)) {
		currentPath = filepath.Join(currentPath, part)
		if err := root.Mkdir(currentPath, mode); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := root.Lstat(currentPath)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("clone directory contains a symbolic link")
		}
	}
	return nil
}

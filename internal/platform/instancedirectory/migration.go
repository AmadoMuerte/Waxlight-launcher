package instancedirectory

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
)

const migrationSettingsLimit = 8 << 20

var gameVersionPattern = regexp.MustCompile(`(?i)(?:vintage story|game version)[^\r\n]{0,80}?\b(?:v)?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\b`)

type MigrationStorage struct {
	sanitize ClientSettingsSanitizer
}

func NewMigrationStorage(sanitize ClientSettingsSanitizer) MigrationStorage {
	return MigrationStorage{sanitize: sanitize}
}

func (MigrationStorage) Discover() []string {
	var paths []string
	if runtime.GOOS == "windows" {
		if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
			paths = append(paths, filepath.Join(appData, "VintagestoryData"))
		}
		return paths
	}
	home, _ := os.UserHomeDir()
	config := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if config == "" && home != "" {
		config = filepath.Join(home, ".config")
	}
	if config != "" {
		paths = append(paths, filepath.Join(config, "VintagestoryData"))
	}
	if home != "" {
		paths = append(paths,
			filepath.Join(home, ".var", "app", "at.vintagestory.VintageStory", "config", "VintagestoryData"),
			filepath.Join(home, ".var", "app", "com.vintagestory.VintageStory", "config", "VintagestoryData"))
	}
	return paths
}

func (storage MigrationStorage) Inspect(path string) (instances.MigrationCandidate, error) {
	path, info, err := migrationRoot(path)
	if err != nil {
		return instances.MigrationCandidate{}, err
	}
	candidate := instances.MigrationCandidate{Path: path, Warnings: []string{"Close Vintage Story before importing this data"}}
	plausible := false
	for _, name := range []string{"clientsettings.json", "Saves", "SaveGame", "Mods", "ModsDisabled", "ModConfig", "Config"} {
		if _, err := os.Lstat(filepath.Join(path, name)); err == nil {
			plausible = true
			break
		}
	}
	if !plausible {
		return candidate, errors.New("directory does not contain recognizable Vintage Story data")
	}
	candidate.WorldCount = countMigrationWorlds(path)
	candidate.ModCount = countMigrationMods(path)
	candidate.HasModConfig = regularDirectory(filepath.Join(path, "ModConfig")) || regularDirectory(filepath.Join(path, "Config"))
	settingsPath := filepath.Join(path, "clientsettings.json")
	if settings, settingsInfo, readErr := readRegularBounded(settingsPath, migrationSettingsLimit); readErr == nil {
		candidate.HasClientSettings = true
		if storage.sanitize == nil {
			candidate.Warnings = append(candidate.Warnings, "Client settings were skipped because sanitization is unavailable")
		} else if _, sanitizeErr := storage.sanitize(settings); sanitizeErr != nil {
			candidate.Warnings = append(candidate.Warnings, "Malformed client settings will be skipped")
		} else {
			candidate.TotalBytes += settingsInfo.Size()
			candidate.TotalFiles++
		}
	}
	err = filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == path {
			return nil
		}
		relative, relErr := filepath.Rel(path, current)
		if relErr != nil {
			return relErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			candidate.Warnings = append(candidate.Warnings, "Symbolic link will be skipped: "+relative)
			return nil
		}
		if rootMigrationExcluded(relative) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			candidate.Warnings = append(candidate.Warnings, "Symbolic link will be skipped: "+relative)
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode().IsRegular() && !strings.EqualFold(relative, "clientsettings.json") && info.Name() != markerName && !strings.HasSuffix(info.Name(), ".waxlight-auth-injection") {
			candidate.TotalBytes += info.Size()
			candidate.TotalFiles++
		}
		return nil
	})
	if err != nil {
		return candidate, err
	}
	candidate.DetectedGameVersion, candidate.VersionConfidence = detectMigrationVersion(filepath.Join(path, "Logs"))
	_ = info
	return candidate, nil
}

func (storage MigrationStorage) Copy(ctx context.Context, source, target string, progress func(instances.MigrationCopyProgress)) (instances.MigrationCopyResult, error) {
	source, sourceInfo, err := migrationRoot(source)
	if err != nil {
		return instances.MigrationCopyResult{}, err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return instances.MigrationCopyResult{}, err
	}
	targetInfo, err := os.Lstat(target)
	if err != nil || !targetInfo.IsDir() || targetInfo.Mode()&os.ModeSymlink != 0 {
		return instances.MigrationCopyResult{}, errors.New("import destination is not a regular directory")
	}
	if sameClonePath(source, target) || clonePathWithin(source, target) || clonePathWithin(target, source) ||
		hasPhysicalAncestor(source, targetInfo) || hasPhysicalAncestor(target, sourceInfo) {
		return instances.MigrationCopyResult{}, errors.New("source and import destination overlap")
	}
	sourceRoot, err := os.OpenRoot(source)
	if err != nil {
		return instances.MigrationCopyResult{}, err
	}
	defer sourceRoot.Close()
	targetRoot, err := os.OpenRoot(target)
	if err != nil {
		return instances.MigrationCopyResult{}, err
	}
	defer targetRoot.Close()
	openedSource, sourceErr := sourceRoot.Stat(".")
	openedTarget, targetErr := targetRoot.Stat(".")
	if sourceErr != nil || targetErr != nil || !os.SameFile(sourceInfo, openedSource) || !os.SameFile(targetInfo, openedTarget) {
		return instances.MigrationCopyResult{}, errors.New("source or import destination changed while import started")
	}
	result := instances.MigrationCopyResult{Warnings: []string{}}
	err = storage.copyRootDirectory(ctx, sourceRoot, targetRoot, ".", openedSource, openedTarget, &result, progress)
	return result, err
}

func (MigrationStorage) ValidateTarget(source, target string) error {
	source, sourceInfo, err := migrationRoot(source)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(strings.TrimSpace(target))
	if err != nil {
		return err
	}
	if sameClonePath(source, target) || clonePathWithin(source, target) || clonePathWithin(target, source) {
		return errors.New("source and import destination overlap")
	}
	current := target
	for {
		info, statErr := os.Stat(current)
		if statErr == nil {
			if os.SameFile(info, sourceInfo) || hasPhysicalAncestor(source, info) {
				return errors.New("source and import destination overlap")
			}
			break
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return nil
}

func (storage MigrationStorage) copyRootDirectory(ctx context.Context, sourceRoot, targetRoot *os.Root, relative string,
	expected, targetIdentity os.FileInfo, result *instances.MigrationCopyResult, progress func(instances.MigrationCopyProgress)) error {
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
	opened, err := directory.Stat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(expected, opened) || os.SameFile(opened, targetIdentity) {
		return errors.New("source directory changed or overlaps the destination")
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
		info, err := sourceRoot.Lstat(child)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			result.Warnings = append(result.Warnings, "Skipped symbolic link: "+child)
			continue
		}
		// Vintage Story recreates root caches and logs; copying either wastes space
		// and can import stale or privacy-sensitive diagnostics.
		if rootMigrationExcluded(child) {
			continue
		}
		if info.Name() == markerName || strings.HasSuffix(info.Name(), ".waxlight-auth-injection") {
			continue
		}
		if info.IsDir() {
			if err := ensureCloneDirectory(targetRoot, child, info.Mode().Perm()); err != nil {
				return err
			}
			if err := storage.copyRootDirectory(ctx, sourceRoot, targetRoot, child, info, targetIdentity, result, progress); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			result.Warnings = append(result.Warnings, "Skipped non-regular file: "+child)
			continue
		}
		var copied int64
		if strings.EqualFold(child, "clientsettings.json") {
			copied, err = storage.copyRootSettings(ctx, sourceRoot, targetRoot, child, info)
			if err != nil && !errors.Is(err, context.Canceled) {
				result.Warnings = append(result.Warnings, "Skipped malformed client settings")
				continue
			}
		} else {
			copied, err = copyMigrationRootFile(ctx, sourceRoot, targetRoot, child, info)
		}
		if err != nil {
			return err
		}
		result.Bytes += copied
		result.Files++
		if progress != nil {
			progress(instances.MigrationCopyProgress{Bytes: result.Bytes, Files: result.Files})
		}
	}
	return nil
}

func migrationRoot(path string) (string, os.FileInfo, error) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", nil, err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", nil, errors.New("Vintage Story data root must be a regular directory")
	}
	return abs, info, nil
}

func rootMigrationExcluded(relative string) bool {
	if strings.ContainsRune(relative, os.PathSeparator) {
		return false
	}
	return strings.EqualFold(relative, "Cache") || strings.EqualFold(relative, "Logs")
}

func countMigrationWorlds(root string) int {
	count := 0
	for _, name := range []string{"Saves", "SaveGame"} {
		entries, _ := os.ReadDir(filepath.Join(root, name))
		for _, entry := range entries {
			info, err := entry.Info()
			if err == nil && (info.IsDir() || info.Mode().IsRegular()) && info.Mode()&os.ModeSymlink == 0 && !strings.HasPrefix(entry.Name(), ".") {
				count++
			}
		}
	}
	return count
}

func countMigrationMods(root string) int {
	count := 0
	for _, name := range []string{"Mods", "ModsDisabled"} {
		entries, _ := os.ReadDir(filepath.Join(root, name))
		for _, entry := range entries {
			info, err := entry.Info()
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 && (ext == ".zip" || ext == ".cs" || ext == ".dll") {
				count++
			}
		}
	}
	return count
}

func regularDirectory(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

func readRegularBounded(path string, limit int64) ([]byte, os.FileInfo, error) {
	expected, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !expected.Mode().IsRegular() || expected.Mode()&os.ModeSymlink != 0 || expected.Size() > limit {
		return nil, expected, errors.New("file is not a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(expected, info) || info.Size() > limit {
		return nil, info, errors.New("file is not a bounded regular file")
	}
	contents, err := io.ReadAll(io.LimitReader(file, limit+1))
	return contents, info, err
}

func detectMigrationVersion(logs string) (string, string) {
	entries, err := os.ReadDir(logs)
	if err != nil {
		return "", ""
	}
	type logFile struct {
		path     string
		modified int64
	}
	files := []logFile{}
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if infoErr == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			files = append(files, logFile{filepath.Join(logs, entry.Name()), info.ModTime().UnixNano()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modified > files[j].modified })
	counts := map[string]int{}
	order := []string{}
	for index, file := range files {
		if index == 3 {
			break
		}
		contents, readErr := readTailBounded(file.path, 256<<10)
		if readErr != nil {
			continue
		}
		seen := map[string]bool{}
		for _, match := range gameVersionPattern.FindAllSubmatch(contents, 20) {
			version := string(match[1])
			if !seen[version] {
				if _, exists := counts[version]; !exists {
					order = append(order, version)
				}
				counts[version]++
				seen[version] = true
			}
		}
	}
	best, occurrences := "", 0
	for _, version := range order {
		if count := counts[version]; count > occurrences {
			best, occurrences = version, count
		}
	}
	if occurrences >= 2 {
		return best, "high"
	}
	if occurrences == 1 {
		return best, "medium"
	}
	return "", ""
}

func readTailBounded(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("log is not a regular file")
	}
	if info.Size() <= limit {
		return io.ReadAll(io.LimitReader(file, limit))
	}
	half := limit / 2
	head, err := io.ReadAll(io.LimitReader(file, half))
	if err != nil {
		return nil, err
	}
	if _, err := file.Seek(-half, io.SeekEnd); err != nil {
		return nil, err
	}
	tail, err := io.ReadAll(io.LimitReader(file, half))
	return append(head, tail...), err
}

func (storage MigrationStorage) copyRootSettings(ctx context.Context, sourceRoot, targetRoot *os.Root, relative string, expected os.FileInfo) (int64, error) {
	if storage.sanitize == nil {
		return 0, errors.New("client settings sanitizer is unavailable")
	}
	input, err := sourceRoot.Open(relative)
	if err != nil {
		return 0, err
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Size() > migrationSettingsLimit || !os.SameFile(expected, opened) {
		return 0, errors.New("client settings changed during import")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	contents, err := io.ReadAll(io.LimitReader(input, migrationSettingsLimit+1))
	if err != nil || int64(len(contents)) > migrationSettingsLimit {
		return 0, errors.New("client settings are too large")
	}
	sanitized, err := storage.sanitize(contents)
	if err != nil {
		return 0, err
	}
	if err := ensureCloneDirectory(targetRoot, filepath.Dir(relative), 0o755); err != nil {
		return 0, err
	}
	output, err := targetRoot.OpenFile(relative, os.O_CREATE|os.O_EXCL|os.O_WRONLY, expected.Mode().Perm())
	if err != nil {
		return 0, err
	}
	if _, err := output.Write(sanitized); err != nil {
		_ = output.Close()
		_ = targetRoot.Remove(relative)
		return 0, err
	}
	if err := output.Close(); err != nil {
		return 0, err
	}
	return int64(len(sanitized)), nil
}

func copyMigrationRootFile(ctx context.Context, sourceRoot, targetRoot *os.Root, relative string, expected os.FileInfo) (int64, error) {
	input, err := sourceRoot.Open(relative)
	if err != nil {
		return 0, err
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(expected, opened) {
		return 0, errors.New("source file changed during import")
	}
	if err := ensureCloneDirectory(targetRoot, filepath.Dir(relative), 0o755); err != nil {
		return 0, err
	}
	output, err := targetRoot.OpenFile(relative, os.O_CREATE|os.O_EXCL|os.O_WRONLY, expected.Mode().Perm())
	if err != nil {
		return 0, err
	}
	buffer := make([]byte, 32*1024)
	written, copyErr := io.CopyBuffer(output, &migrationContextReader{ctx: ctx, reader: input}, buffer)
	finalInfo, statErr := input.Stat()
	closeErr := output.Close()
	if copyErr != nil {
		_ = targetRoot.Remove(relative)
		return written, copyErr
	}
	if statErr != nil || !os.SameFile(expected, finalInfo) || finalInfo.Size() != expected.Size() || !finalInfo.ModTime().Equal(expected.ModTime()) || written != expected.Size() {
		_ = targetRoot.Remove(relative)
		return written, errors.New("source file changed during import")
	}
	return written, closeErr
}

type migrationContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *migrationContextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

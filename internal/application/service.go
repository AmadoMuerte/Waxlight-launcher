package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

type Service struct {
	store            Store
	accounts         *AccountService
	clientSettings   ClientSettingsPatcher
	installer        ArchiveInstaller
	versionCatalog   GameVersionCatalog
	downloader       Downloader
	packageInstaller GamePackageInstaller
	modFiles         ModFileManager
	launcher         ProcessLauncher
	dataRoot         string
	events           EventPublisher
	runningMu        sync.Mutex
	versionInstallMu sync.Mutex
	operationsMu     sync.Mutex
	operationCancels map[string]context.CancelFunc
	operationWG      sync.WaitGroup
	shutdownCtx      context.Context
	shutdownCancel   context.CancelFunc
	running          map[string]runningGame
}

type runningGame struct {
	process   RunningProcess
	sessionID string
	started   time.Time
	log       io.Closer
}

func NewService(
	store Store,
	installer ArchiveInstaller,
	modFiles ModFileManager,
	launcher ProcessLauncher,
	dataRoot string,
) *Service {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	return &Service{
		store:            store,
		installer:        installer,
		modFiles:         modFiles,
		launcher:         launcher,
		dataRoot:         dataRoot,
		operationCancels: make(map[string]context.CancelFunc),
		shutdownCtx:      shutdownCtx,
		shutdownCancel:   shutdownCancel,
		running:          make(map[string]runningGame),
	}
}

func (s *Service) ConfigureVersionDownloads(
	catalog GameVersionCatalog,
	downloader Downloader,
	installer GamePackageInstaller,
) {
	s.versionCatalog = catalog
	s.downloader = downloader
	s.packageInstaller = installer
}

func (s *Service) SetEventPublisher(publisher EventPublisher) {
	s.events = publisher
}

func (s *Service) ConfigureAuthentication(
	accounts *AccountService,
	clientSettings ClientSettingsPatcher,
) {
	s.accounts = accounts
	s.clientSettings = clientSettings
}

func (s *Service) emit(name string, payload any) {
	if s.events != nil {
		s.events.Publish(name, payload)
	}
}
func (s *Service) Close() error {
	s.shutdownCancel()
	s.operationWG.Wait()
	return s.store.Close()
}

func newID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
func cleanName(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", domain.NewError(domain.ErrValidation, "Name cannot be empty")
	}
	if len([]rune(v)) > 80 {
		return "", domain.NewError(domain.ErrValidation, "Name cannot exceed 80 characters")
	}
	return v, nil
}

func (s *Service) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	return s.store.ListAccounts(ctx)
}

func (s *Service) AddLocalAccount(
	ctx context.Context,
	username string,
	displayName string,
) (domain.Account, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return domain.Account{}, domain.NewError(domain.ErrValidation, "Enter a username")
	}
	if displayName == "" {
		displayName = username
	}
	displayName, _ = cleanName(displayName)
	now := time.Now().UTC()
	accounts, e := s.store.ListAccounts(ctx)
	if e != nil {
		return domain.Account{}, e
	}
	account := domain.Account{
		ID:          newID(),
		Username:    username,
		DisplayName: displayName,
		Status:      "local_profile",
		IsDefault:   len(accounts) == 0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if e = s.store.SaveAccount(ctx, account); e == nil {
		s.emit("account:added", account)
	}
	return account, e
}

func (s *Service) SetDefaultAccount(ctx context.Context, id string) error {
	if _, e := s.store.GetAccount(ctx, id); e != nil {
		return e
	}
	e := s.store.SetDefaultAccount(ctx, id)
	if e == nil {
		s.emit("account:updated", map[string]string{"id": id})
	}
	return e
}
func (s *Service) DeleteAccount(ctx context.Context, id string) error {
	e := s.store.DeleteAccount(ctx, id)
	if e == nil {
		s.emit("account:removed", map[string]string{"id": id})
	}
	return e
}

func (s *Service) ListVersions(ctx context.Context) ([]domain.GameVersion, error) {
	versions, err := s.store.ListVersions(ctx)
	if err != nil {
		return nil, err
	}

	for index := range versions {
		repairedVersion, repairErr := s.ensureVersionExecutable(
			ctx,
			versions[index],
		)
		if repairErr == nil {
			versions[index] = repairedVersion
		}
	}

	return versions, nil
}

func (s *Service) ensureVersionExecutable(
	ctx context.Context,
	version domain.GameVersion,
) (domain.GameVersion, error) {
	info, err := os.Stat(version.ExecutablePath)
	if err == nil && !info.IsDir() {
		return version, nil
	}

	executablePath, err := s.installer.FindExecutable(
		version.InstallationDir,
		"",
	)
	if err != nil {
		return version, err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(executablePath, 0o755); err != nil {
			return version, err
		}
	}

	version.ExecutablePath = executablePath
	version.Status = "installed"
	now := time.Now().UTC()
	version.VerifiedAt = &now
	if err := s.store.UpdateVersion(ctx, version); err != nil {
		return version, err
	}

	return version, nil
}

func (s *Service) InstallVersion(
	ctx context.Context,
	id string,
	name string,
	sourcePath string,
	executableRelativePath string,
	checksum string,
) (domain.Operation, error) {
	s.versionInstallMu.Lock()
	defer s.versionInstallMu.Unlock()

	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Operation{}, domain.NewError(domain.ErrValidation, "Enter a version ID")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = id
	}
	if sourcePath == "" {
		return domain.Operation{}, domain.NewError(domain.ErrValidation, "Select a game archive or directory")
	}
	if _, existingErr := s.store.GetVersion(ctx, id); existingErr == nil {
		return domain.Operation{}, domain.NewError(domain.ErrVersionExists, "This game version is already installed")
	} else {
		var appErr *domain.AppError
		if !errors.As(existingErr, &appErr) || appErr.Code != domain.ErrVersionNotFound {
			return domain.Operation{}, existingErr
		}
	}

	now := time.Now().UTC()
	resource := id
	operation := domain.Operation{
		ID:         newID(),
		Type:       "game_version_install",
		ResourceID: &resource,
		Title:      "Installing Vintage Story " + name,
		Status:     "running",
		Progress:   0.05,
		CreatedAt:  now,
		StartedAt:  &now,
	}
	_ = s.store.SaveOperation(ctx, operation)
	s.emit("operation:created", operation)

	target := filepath.Join(s.dataRoot, "versions", safeSegment(id))
	executable, size, e := s.installer.Install(
		ctx,
		sourcePath,
		target,
		executableRelativePath,
		checksum,
	)
	finished := time.Now().UTC()
	operation.FinishedAt = &finished
	if e != nil {
		operation.Status = "failed"
		code := domain.ErrArchiveInvalid
		if strings.Contains(strings.ToLower(e.Error()), "checksum") {
			code = domain.ErrChecksumMismatch
		}
		operation.ErrorCode = &code
		message := e.Error()
		operation.ErrorMessage = &message
		_ = s.store.SaveOperation(context.Background(), operation)
		s.emit("operation:failed", operation)
		return operation, &domain.AppError{
			Code:    code,
			Message: "Failed to install the game version",
			Cause:   e,
		}
	}

	version := domain.GameVersion{
		ID:              id,
		Name:            name,
		Channel:         "unknown",
		Platform:        runtime.GOOS,
		Architecture:    runtime.GOARCH,
		InstallationDir: target,
		ExecutablePath:  executable,
		Status:          "installed",
		InstalledAt:     finished,
		VerifiedAt:      &finished,
		SizeBytes:       size,
	}
	if e = os.WriteFile(filepath.Join(target, ".waxlight-version"), []byte(id), 0o600); e != nil {
		return operation, e
	}
	if e = s.store.SaveVersion(ctx, version); e != nil {
		return operation, e
	}

	operation.Status = "completed"
	operation.Progress = 1
	operation.TotalBytes = size
	operation.CurrentBytes = size
	_ = s.store.SaveOperation(ctx, operation)
	s.emit("operation:completed", operation)
	return operation, nil
}

func safeSegment(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "version"
	}
	return b.String()
}
func (s *Service) DeleteVersion(ctx context.Context, id string, deleteFiles bool) error {
	v, e := s.store.GetVersion(ctx, id)
	if e != nil {
		return e
	}
	instances, e := s.store.ListInstances(ctx)
	if e != nil {
		return e
	}
	for _, i := range instances {
		if i.GameVersionID == id {
			return domain.NewError(domain.ErrValidation, "The version is used by instance \""+i.Name+"\"")
		}
	}
	if deleteFiles {
		if e = safeRemoveAll(v.InstallationDir, s.dataRoot, ".waxlight-version"); e != nil {
			return e
		}
	}
	return s.store.DeleteVersion(ctx, id)
}

type CreateInstanceInput struct {
	Name             string
	Description      string
	GameVersionID    string
	Directory        string
	DefaultAccountID *string
	LaunchArguments  []string
}

func (s *Service) CreateInstance(ctx context.Context, in CreateInstanceInput) (domain.Instance, error) {
	name, e := cleanName(in.Name)
	if e != nil {
		return domain.Instance{}, e
	}
	if _, e = s.store.GetVersion(ctx, in.GameVersionID); e != nil {
		return domain.Instance{}, e
	}
	if in.DefaultAccountID != nil {
		if _, e = s.store.GetAccount(ctx, *in.DefaultAccountID); e != nil {
			return domain.Instance{}, e
		}
	}
	now := time.Now().UTC()
	id := newID()
	dir := strings.TrimSpace(in.Directory)
	if dir == "" {
		dir = filepath.Join(s.dataRoot, "instances", id)
	}
	dir, e = filepath.Abs(dir)
	if e != nil {
		return domain.Instance{}, e
	}
	used, e := s.store.IsDirectoryUsed(ctx, dir, "")
	if e != nil {
		return domain.Instance{}, e
	}
	if used {
		return domain.Instance{}, domain.NewError(domain.ErrDirectoryConflict, "The directory is already used by another instance")
	}
	if e = os.MkdirAll(dir, 0o755); e != nil {
		return domain.Instance{}, &domain.AppError{Code: domain.ErrFilePermission, Message: "Failed to create the instance directory", Cause: e}
	}
	if e = s.modFiles.EnsureLayout(dir); e != nil {
		return domain.Instance{}, e
	}
	_ = os.MkdirAll(filepath.Join(dir, "Logs"), 0o755)
	if e = os.WriteFile(filepath.Join(dir, ".waxlight-instance"), []byte(id), 0o600); e != nil {
		return domain.Instance{}, e
	}
	instance := domain.Instance{
		ID:               id,
		Name:             name,
		Description:      strings.TrimSpace(in.Description),
		GameVersionID:    in.GameVersionID,
		DefaultAccountID: in.DefaultAccountID,
		Directory:        dir,
		Status:           "ready",
		LaunchArguments:  in.LaunchArguments,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if e = s.store.SaveInstance(ctx, instance); e == nil {
		s.emit("instance:created", instance)
	}
	return instance, e
}

func (s *Service) ListInstances(ctx context.Context) ([]domain.Instance, error) {
	return s.store.ListInstances(ctx)
}
func (s *Service) GetInstance(ctx context.Context, id string) (domain.Instance, error) {
	return s.store.GetInstance(ctx, id)
}
func (s *Service) UpdateInstance(ctx context.Context, in domain.Instance) (domain.Instance, error) {
	old, e := s.store.GetInstance(ctx, in.ID)
	if e != nil {
		return in, e
	}
	in.Name, e = cleanName(in.Name)
	if e != nil {
		return in, e
	}
	if _, e = s.store.GetVersion(ctx, in.GameVersionID); e != nil {
		return in, e
	}
	in.Directory = old.Directory
	in.CreatedAt = old.CreatedAt
	in.LastPlayedAt = old.LastPlayedAt
	in.Status = old.Status
	in.UpdatedAt = time.Now().UTC()
	e = s.store.SaveInstance(ctx, in)
	if e == nil {
		s.emit("instance:updated", in)
	}
	return in, e
}
func (s *Service) DeleteInstance(ctx context.Context, id string, deleteFiles bool) error {
	s.runningMu.Lock()
	_, running := s.running[id]
	s.runningMu.Unlock()
	if running {
		return domain.NewError(domain.ErrInstanceRunning, "Stop the game before deleting this instance")
	}
	i, e := s.store.GetInstance(ctx, id)
	if e != nil {
		return e
	}
	if deleteFiles {
		if e = safeRemoveAll(i.Directory, s.dataRoot, ".waxlight-instance"); e != nil {
			return e
		}
	}
	if e = s.store.DeleteInstance(ctx, id); e != nil {
		return e
	}
	s.emit("instance:deleted", map[string]string{"id": id})
	return nil
}
func safeRemoveAll(path, dataRoot, marker string) error {
	abs, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	root, rootError := filepath.Abs(dataRoot)
	if rootError != nil {
		return rootError
	}
	home, _ := os.UserHomeDir()
	volumeRoot := filepath.VolumeName(abs) + string(os.PathSeparator)
	if abs == "/" || abs == volumeRoot || abs == home || abs == root || len(abs) < 5 {
		return domain.NewError(domain.ErrValidation, "Unsafe deletion path")
	}
	if _, e = os.Stat(filepath.Join(abs, marker)); e != nil {
		return domain.NewError(domain.ErrValidation, "The directory is not managed by Waxlight; no files were deleted")
	}
	return os.RemoveAll(abs)
}

func (s *Service) ListMods(ctx context.Context, instanceID string) ([]domain.InstalledMod, error) {
	instance, e := s.store.GetInstance(ctx, instanceID)
	if e != nil {
		return nil, e
	}
	if e = s.modFiles.EnsureLayout(instance.Directory); e != nil {
		return nil, e
	}
	mods, e := s.store.ListMods(ctx, instanceID)
	if e != nil {
		return nil, e
	}
	for index := range mods {
		if _, statErr := os.Stat(mods[index].FilePath); statErr == nil {
			continue
		}

		directory := "ModsDisabled"
		if mods[index].Enabled {
			directory = "Mods"
		}
		candidate := filepath.Join(instance.Directory, directory, mods[index].FileName)
		if _, statErr := os.Stat(candidate); statErr == nil {
			mods[index].FilePath = candidate
			mods[index].UpdatedAt = time.Now().UTC()
			_ = s.store.SaveMod(ctx, mods[index])
		}
	}
	return mods, nil
}
func (s *Service) InstallModFile(ctx context.Context, instanceID, sourcePath, name, version string) (domain.Operation, error) {
	i, e := s.store.GetInstance(ctx, instanceID)
	if e != nil {
		return domain.Operation{}, e
	}
	if sourcePath == "" {
		return domain.Operation{}, domain.NewError(domain.ErrValidation, "Select a mod file")
	}
	now := time.Now().UTC()
	resource := instanceID
	operation := domain.Operation{
		ID:         newID(),
		Type:       "mod_install",
		ResourceID: &resource,
		Title:      "Installing mod",
		Status:     "running",
		Progress:   0.1,
		CreatedAt:  now,
		StartedAt:  &now,
	}
	_ = s.store.SaveOperation(ctx, operation)

	path, size, e := s.modFiles.Install(ctx, sourcePath, i.Directory)
	finished := time.Now().UTC()
	operation.FinishedAt = &finished
	if e != nil {
		operation.Status = "failed"
		msg := e.Error()
		code := "MOD_INSTALL_FAILED"
		operation.ErrorCode = &code
		operation.ErrorMessage = &msg
		_ = s.store.SaveOperation(ctx, operation)
		return operation, e
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	}
	if version == "" {
		version = "unknown"
	}
	mod := domain.InstalledMod{
		ID:          newID(),
		InstanceID:  instanceID,
		Name:        name,
		Version:     version,
		FileName:    filepath.Base(path),
		FilePath:    path,
		Enabled:     true,
		Managed:     false,
		Source:      "local",
		SizeBytes:   size,
		InstalledAt: finished,
		UpdatedAt:   finished,
	}
	if e = s.store.SaveMod(ctx, mod); e != nil {
		return operation, e
	}

	operation.Status = "completed"
	operation.Progress = 1
	operation.CurrentBytes = size
	operation.TotalBytes = size
	_ = s.store.SaveOperation(ctx, operation)
	s.emit("mod:installed", mod)
	return operation, nil
}
func (s *Service) SetModEnabled(ctx context.Context, id string, enabled bool) (domain.InstalledMod, error) {
	m, e := s.store.GetMod(ctx, id)
	if e != nil {
		return m, e
	}
	i, e := s.store.GetInstance(ctx, m.InstanceID)
	if e != nil {
		return m, e
	}
	path, e := s.modFiles.SetEnabled(m.FilePath, i.Directory, enabled)
	if e != nil {
		return m, e
	}
	m.FilePath = path
	m.Enabled = enabled
	m.UpdatedAt = time.Now().UTC()
	e = s.store.SaveMod(ctx, m)
	if e == nil {
		event := "mod:disabled"
		if enabled {
			event = "mod:enabled"
		}
		s.emit(event, m)
	}
	return m, e
}
func (s *Service) DeleteMod(ctx context.Context, id string) error {
	m, e := s.store.GetMod(ctx, id)
	if e != nil {
		return e
	}
	if e = os.Remove(m.FilePath); e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if e = s.store.DeleteMod(ctx, id); e == nil {
		s.emit("mod:removed", map[string]string{"id": id, "instanceId": m.InstanceID})
	}
	return e
}

type LaunchValidation struct {
	Valid    bool
	Issues   []string
	Warnings []string
}

func (s *Service) ValidateLaunch(
	ctx context.Context,
	instanceID string,
	accountID *string,
) (LaunchValidation, error) {
	validation := LaunchValidation{
		Valid:    true,
		Issues:   []string{},
		Warnings: []string{},
	}

	instance, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return validation, err
	}

	version, err := s.store.GetVersion(ctx, instance.GameVersionID)
	if err != nil {
		validation.Valid = false
		validation.Issues = append(
			validation.Issues,
			"The selected game version is not installed",
		)
		return validation, nil
	}

	version, repairErr := s.ensureVersionExecutable(ctx, version)
	if repairErr != nil {
		validation.Valid = false
		validation.Issues = append(
			validation.Issues,
			"The Vintagestory executable could not be found",
		)
		return validation, nil
	}

	executableInfo, err := os.Stat(version.ExecutablePath)
	if err != nil || executableInfo.IsDir() {
		validation.Valid = false
		validation.Issues = append(
			validation.Issues,
			"The Vintagestory executable could not be found",
		)
	}

	chosen, chooseErr := s.resolveAccountID(ctx, instance, accountID)
	if chooseErr != nil {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "The selected account no longer exists")
	}
	if chosen == nil {
		validation.Warnings = append(
			validation.Warnings,
			"No account is selected. The game will start without authentication data.",
		)
	} else if account, accountErr := s.store.GetAccount(ctx, *chosen); accountErr != nil {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "The selected account no longer exists")
	} else if account.Status == domain.AccountStatusExpired || account.Status == domain.AccountStatusNeedsReauth {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "The selected account must be authenticated again")
	}

	s.runningMu.Lock()
	_, isRunning := s.running[instanceID]
	s.runningMu.Unlock()
	if isRunning {
		validation.Valid = false
		validation.Issues = append(
			validation.Issues,
			"This instance is already running",
		)
	}

	return validation, nil
}

func (s *Service) Launch(
	ctx context.Context,
	instanceID string,
	accountID *string,
) (domain.PlaySession, error) {
	validation, err := s.ValidateLaunch(ctx, instanceID, accountID)
	if err != nil {
		return domain.PlaySession{}, err
	}
	if !validation.Valid {
		return domain.PlaySession{}, domain.NewError(
			domain.ErrValidation,
			strings.Join(validation.Issues, "; "),
		)
	}

	instance, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return domain.PlaySession{}, err
	}
	version, err := s.store.GetVersion(ctx, instance.GameVersionID)
	if err != nil {
		return domain.PlaySession{}, err
	}
	version, err = s.ensureVersionExecutable(ctx, version)
	if err != nil {
		return domain.PlaySession{}, domain.NewError(
			domain.ErrValidation,
			"The Vintagestory executable could not be found",
		)
	}

	accountID, err = s.resolveAccountID(ctx, instance, accountID)
	if err != nil {
		return domain.PlaySession{}, err
	}

	clientSettingsPath := filepath.Join(instance.Directory, "clientsettings.json")
	if accountID != nil {
		if s.accounts == nil || s.clientSettings == nil {
			return domain.PlaySession{}, domain.NewError(domain.ErrValidation, "Account authentication is unavailable")
		}
		account, validateErr := s.accounts.ValidateAuthorizedAccount(ctx, *accountID)
		if validateErr != nil {
			return domain.PlaySession{}, validateErr
		}
		if patchErr := s.clientSettings.Patch(clientSettingsPath, account); patchErr != nil {
			return domain.PlaySession{}, &domain.AppError{
				Code:    domain.ErrClientSettings,
				Message: "Could not write authentication to the instance settings",
				Cause:   patchErr,
			}
		}
	} else if s.clientSettings != nil {
		if clearErr := s.clientSettings.Clear(clientSettingsPath); clearErr != nil {
			return domain.PlaySession{}, &domain.AppError{
				Code:    domain.ErrClientSettings,
				Message: "Could not clear authentication from the instance settings",
				Cause:   clearErr,
			}
		}
	}

	if err := s.modFiles.EnsureLayout(instance.Directory); err != nil {
		return domain.PlaySession{}, err
	}
	logsDirectory := filepath.Join(instance.Directory, "Logs")
	if err := os.MkdirAll(logsDirectory, 0o755); err != nil {
		return domain.PlaySession{}, err
	}

	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return domain.PlaySession{}, err
	}
	arguments := append([]string{}, settings.GlobalLaunchArguments...)
	arguments = append(arguments, instance.LaunchArguments...)
	arguments = append(arguments, "--dataPath", instance.Directory)

	logPath := filepath.Join(
		logsDirectory,
		time.Now().Format("20060102-150405")+".log",
	)
	logFile, err := os.OpenFile(
		logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0o644,
	)
	if err != nil {
		return domain.PlaySession{}, err
	}

	workingDirectory := filepath.Dir(version.ExecutablePath)
	process, err := s.launcher.Start(
		context.Background(),
		version.ExecutablePath,
		arguments,
		workingDirectory,
		map[string]string{"WAXLIGHT_INSTANCE_DIR": instance.Directory},
		logFile,
	)
	if err != nil {
		_ = logFile.Close()
		return domain.PlaySession{}, &domain.AppError{
			Code:    domain.ErrProcessStart,
			Message: "Failed to start Vintage Story",
			Cause:   err,
		}
	}

	now := time.Now().UTC()
	processID := process.PID()
	session := domain.PlaySession{
		ID:         newID(),
		InstanceID: instance.ID,
		AccountID:  accountID,
		VersionID:  version.ID,
		ProcessID:  &processID,
		StartedAt:  now,
	}
	if err := s.store.SaveSession(ctx, session); err != nil {
		_ = process.Kill()
		_ = logFile.Close()
		return session, err
	}

	instance.Status = "running"
	instance.LastPlayedAt = &now
	instance.UpdatedAt = now
	_ = s.store.SaveInstance(ctx, instance)

	s.runningMu.Lock()
	s.running[instance.ID] = runningGame{
		process:   process,
		sessionID: session.ID,
		started:   now,
		log:       logFile,
	}
	s.runningMu.Unlock()

	s.emit("game:started", session)
	go s.waitForGame(instance, process, session.ID, now, logFile)
	return session, nil
}

func (s *Service) resolveAccountID(
	ctx context.Context,
	instance domain.Instance,
	requested *string,
) (*string, error) {
	if requested != nil && strings.TrimSpace(*requested) != "" {
		if _, err := s.store.GetAccount(ctx, *requested); err != nil {
			return nil, err
		}
		value := *requested
		return &value, nil
	}
	if instance.DefaultAccountID != nil && strings.TrimSpace(*instance.DefaultAccountID) != "" {
		if _, err := s.store.GetAccount(ctx, *instance.DefaultAccountID); err != nil {
			return nil, err
		}
		value := *instance.DefaultAccountID
		return &value, nil
	}
	accounts, err := s.store.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		if account.IsDefault {
			value := account.ID
			return &value, nil
		}
	}
	return nil, nil
}

func (s *Service) waitForGame(
	instance domain.Instance,
	process RunningProcess,
	sessionID string,
	startedAt time.Time,
	logFile io.Closer,
) {
	exitCode, waitErr := process.Wait()
	_ = logFile.Close()

	durationSeconds := int64(time.Since(startedAt).Seconds())
	crashed := waitErr != nil || exitCode != 0
	_ = s.store.FinishSession(
		context.Background(),
		sessionID,
		exitCode,
		crashed,
		durationSeconds,
	)

	instance.Status = "ready"
	instance.UpdatedAt = time.Now().UTC()
	_ = s.store.SaveInstance(context.Background(), instance)

	s.runningMu.Lock()
	delete(s.running, instance.ID)
	s.runningMu.Unlock()

	s.emit("game:exited", map[string]any{
		"instanceId":      instance.ID,
		"sessionId":       sessionID,
		"exitCode":        exitCode,
		"crashed":         crashed,
		"durationSeconds": durationSeconds,
	})
}
func (s *Service) Stop(ctx context.Context, instanceID string, force bool) error {
	s.runningMu.Lock()
	r, ok := s.running[instanceID]
	s.runningMu.Unlock()
	if !ok {
		return domain.NewError(domain.ErrValidation, "The instance is not running")
	}
	var e error
	if force {
		e = r.process.Kill()
	} else {
		e = r.process.Stop()
	}
	if e != nil {
		return &domain.AppError{Code: domain.ErrProcessStop, Message: "Failed to stop Vintage Story", Cause: e}
	}
	return nil
}
func (s *Service) RunningInstanceIDs() []string {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	ids := make([]string, 0, len(s.running))
	for id := range s.running {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

type Statistics struct {
	TotalPlaytimeSeconds  int64
	LaunchCount           int
	AverageSessionSeconds int64
	MostPlayedInstanceID  *string
	RecentSessions        []domain.PlaySession
}

func (s *Service) GetStatistics(ctx context.Context) (Statistics, error) {
	sessions, e := s.store.ListSessions(ctx, "", 5000)
	if e != nil {
		return Statistics{}, e
	}
	st := Statistics{LaunchCount: len(sessions)}
	byInstance := map[string]int64{}
	for _, p := range sessions {
		st.TotalPlaytimeSeconds += p.DurationSec
		byInstance[p.InstanceID] += p.DurationSec
	}
	if len(sessions) > 0 {
		st.AverageSessionSeconds = st.TotalPlaytimeSeconds / int64(len(sessions))
	}
	var best string
	var bestValue int64
	for id, value := range byInstance {
		if value > bestValue {
			best = id
			bestValue = value
		}
	}
	if best != "" {
		st.MostPlayedInstanceID = &best
	}
	if len(sessions) > 10 {
		sessions = sessions[:10]
	}
	st.RecentSessions = sessions
	return st, nil
}
func (s *Service) GetInstancePlaytime(ctx context.Context, instanceID string) (int64, error) {
	sessions, err := s.store.ListSessions(ctx, instanceID, 5000)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, session := range sessions {
		total += session.DurationSec
	}
	return total, nil
}
func (s *Service) ListOperations(ctx context.Context) ([]domain.Operation, error) {
	return s.store.ListOperations(ctx, 100)
}
func (s *Service) GetSettings(ctx context.Context) (domain.Settings, error) {
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return settings, err
	}

	if settings.Language != "en" {
		settings.Language = "en"
		if err := s.store.SaveSettings(ctx, settings); err != nil {
			return settings, err
		}
	}

	return settings, nil
}
func (s *Service) SaveSettings(ctx context.Context, v domain.Settings) (domain.Settings, error) {
	if v.DownloadsParallel < 1 || v.DownloadsParallel > 10 {
		return v, domain.NewError(domain.ErrValidation, "Parallel downloads must be between 1 and 10")
	}
	if v.MinSessionDurationSec < 0 {
		return v, domain.NewError(domain.ErrValidation, "Session duration threshold cannot be negative")
	}
	v.Language = "en"
	return v, s.store.SaveSettings(ctx, v)
}

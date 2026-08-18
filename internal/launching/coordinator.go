package launching

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/gamelog"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/operations"
	"github.com/waxlight/waxlight-launcher/internal/platform/process"
	"github.com/waxlight/waxlight-launcher/internal/sessions"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
	"github.com/waxlight/waxlight-launcher/internal/telemetry"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

// gameStartupWindow is how long a game process must survive for its launch to
// be considered successful. Exits after this window are treated as a started
// game (a long play session followed by a crash is not a failed startup); a
// crashed exit inside the window is a failed startup. It is a variable so
// tests can shorten it. Launch snapshots the value once and hands it to the
// goroutines, so tests may restore it without racing background readers.
var gameStartupWindow = 60 * time.Second

// GameStartupWindow returns the current launch success window (for tests).
func GameStartupWindow() time.Duration {
	return gameStartupWindow
}

// SetGameStartupWindow replaces the launch success window (for tests).
func SetGameStartupWindow(window time.Duration) {
	gameStartupWindow = window
}

// Validation describes the result of a pre-launch check.
type Validation struct {
	Valid    bool
	Issues   []string
	Warnings []string
}

// Coordinator owns game-process orchestration: launch validation, starting,
// stopping, tracking, credential injection and cleanup, launcher-owned
// diagnostic logs, play-session persistence, and failed-startup handling.
type Coordinator struct {
	registry       *Registry
	gate           MutationGate
	instances      InstanceReader
	versions       VersionReader
	accounts       AccountReader
	clientSettings ClientSettingsPatcher
	settings       SettingsReader
	modLayout      ModLayout
	optimum        OptimumResolver
	mods           EnabledModChecker
	sessions       SessionRecorder
	launcher       ProcessLauncher
	logs           LaunchLogs
	events         Publisher
	telemetry      TelemetryReporter
	recovery       LaunchRecovery
	workers        WorkerGroup
	operations     OperationLister
	now            func() time.Time
	newID          func() string
}

func NewCoordinator(
	registry *Registry,
	gate MutationGate,
	instances InstanceReader,
	versions VersionReader,
	accounts AccountReader,
	clientSettings ClientSettingsPatcher,
	settings SettingsReader,
	modLayout ModLayout,
	optimum OptimumResolver,
	mods EnabledModChecker,
	sessions SessionRecorder,
	launcher ProcessLauncher,
	logs LaunchLogs,
	events Publisher,
	telemetry TelemetryReporter,
	recovery LaunchRecovery,
	workers WorkerGroup,
	operations OperationLister,
	now func() time.Time,
	newID func() string,
) *Coordinator {
	return &Coordinator{
		registry:       registry,
		gate:           gate,
		instances:      instances,
		versions:       versions,
		accounts:       accounts,
		clientSettings: clientSettings,
		settings:       settings,
		modLayout:      modLayout,
		optimum:        optimum,
		mods:           mods,
		sessions:       sessions,
		launcher:       launcher,
		logs:           logs,
		events:         events,
		telemetry:      telemetry,
		recovery:       recovery,
		workers:        workers,
		operations:     operations,
		now:            now,
		newID:          newID,
	}
}

// SetClientSettingsPatcher swaps the client-settings patcher. It is used by
// tests and by the composition root to share one patcher between the launch
// coordinator and the snapshot flow.
func (coordinator *Coordinator) SetClientSettingsPatcher(patcher ClientSettingsPatcher) {
	coordinator.clientSettings = patcher
}

// ValidateLaunch checks that an instance can be launched without modifying
// anything.
func (coordinator *Coordinator) ValidateLaunch(
	ctx context.Context,
	instanceID string,
	accountID *string,
) (Validation, error) {
	validation := Validation{
		Valid:    true,
		Issues:   []string{},
		Warnings: []string{},
	}

	instance, err := coordinator.instances.GetInstance(ctx, instanceID)
	if err != nil {
		return validation, err
	}

	version, err := coordinator.versions.ResolveExecutable(ctx, instance.GameVersionID)
	if err != nil {
		validation.Valid = false
		issue := "The Vintagestory executable could not be found"
		var appError *errs.AppError
		if errors.As(err, &appError) && appError.Code == errs.ErrVersionNotFound {
			issue = "The selected game version is not installed"
		}
		validation.Issues = append(validation.Issues, issue)
		return validation, nil
	}
	target, targetErr := coordinator.resolveLaunchTarget(ctx, instance, version)
	if targetErr != nil {
		validation.Valid = false
		validation.Issues = append(validation.Issues, launchIssue(targetErr))
		return validation, nil
	}

	executableInfo, err := os.Stat(target.Executable)
	if err != nil || executableInfo.IsDir() {
		validation.Valid = false
		if instance.GameClient == instances.GameClientOptimum {
			validation.Issues = append(validation.Issues, "The configured Optimum executable could not be found. Configure Optimum or switch this instance to Vanilla")
		} else {
			validation.Issues = append(validation.Issues, "The Vintagestory executable could not be found")
		}
	}
	chosen, chooseErr := coordinator.resolveAccountID(ctx, instance, accountID)
	if chooseErr != nil {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "The selected account no longer exists")
	}
	if chosen == nil {
		validation.Warnings = append(validation.Warnings, "No account is selected. The game will start without authentication data.")
	} else if account, accountErr := coordinator.accounts.GetAccount(ctx, *chosen); accountErr != nil {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "The selected account no longer exists")
	} else if account.Status == accounts.StatusExpired || account.Status == accounts.StatusNeedsReauth {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "The selected account must be authenticated again")
	}

	if coordinator.registry.Running(instanceID) {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "This instance is already running")
	}
	if instance.GameClient == instances.GameClientOptimum && coordinator.mods != nil {
		if found, modErr := coordinator.mods.HasEnabledMod(instance.Directory, "optitime"); modErr != nil {
			slog.Debug("could not check the instance for OptiTime", "instance", instance.Name, "error", modErr)
		} else if found {
			validation.Warnings = append(validation.Warnings, "OptiTime is installed in this instance. Optimum already provides the functionality of OptiTime and using both may cause problems.")
		}
	}

	return validation, nil
}

// Launch starts the game of an instance.
func (coordinator *Coordinator) Launch(
	ctx context.Context,
	instanceID string,
	accountID *string,
) (sessions.PlaySession, error) {
	return coordinator.launch(ctx, instanceID, accountID, "")
}

// LaunchServer starts an instance and connects it to the requested server.
func (coordinator *Coordinator) LaunchServer(
	ctx context.Context,
	instanceID string,
	accountID *string,
	address string,
) (sessions.PlaySession, error) {
	address = strings.TrimSpace(address)
	if address == "" || len(address) > 255 || strings.ContainsAny(address, " \t\r\n/?#") {
		return sessions.PlaySession{}, errs.NewError(errs.ErrValidation, "Enter a valid server address")
	}
	return coordinator.launch(ctx, instanceID, accountID, address)
}

func (coordinator *Coordinator) launch(
	ctx context.Context,
	instanceID string,
	accountID *string,
	serverAddress string,
) (sessions.PlaySession, error) {
	release, err := coordinator.beginMutation()
	if err != nil {
		return sessions.PlaySession{}, err
	}
	releaseOnReturn := true
	defer func() {
		if releaseOnReturn {
			release()
		}
	}()
	releaseLaunch := coordinator.registry.BeginLaunch()
	defer releaseLaunch()
	if coordinator.registry.Busy(instanceID) {
		return sessions.PlaySession{}, errs.NewError(snapshots.ErrSnapshotInProgress, "Wait for the running snapshot operation to finish")
	}
	validation, err := coordinator.ValidateLaunch(ctx, instanceID, accountID)
	if err != nil {
		return sessions.PlaySession{}, err
	}
	if !validation.Valid {
		return sessions.PlaySession{}, errs.NewError(errs.ErrValidation, strings.Join(validation.Issues, "; "))
	}

	instance, err := coordinator.instances.GetInstance(ctx, instanceID)
	if err != nil {
		return sessions.PlaySession{}, err
	}
	version, err := coordinator.versions.ResolveExecutable(ctx, instance.GameVersionID)
	if err != nil {
		return sessions.PlaySession{}, err
	}
	target, err := coordinator.resolveLaunchTarget(ctx, instance, version)
	if err != nil {
		return sessions.PlaySession{}, err
	}
	slog.Info("launching instance", "instance", instance.Name, "version", version.Name)
	accountID, err = coordinator.resolveAccountID(ctx, instance, accountID)
	if err != nil {
		return sessions.PlaySession{}, err
	}

	clientSettingsPath := filepath.Join(instance.Directory, "clientsettings.json")
	cleanupCredentials := func() error { return nil }
	if accountID != nil {
		if coordinator.accounts == nil || coordinator.clientSettings == nil {
			return sessions.PlaySession{}, errs.NewError(errs.ErrValidation, "Account authentication is unavailable")
		}
		account, validateErr := coordinator.accounts.ValidateAuthorizedAccount(ctx, *accountID)
		if validateErr != nil {
			return sessions.PlaySession{}, validateErr
		}
		cleanup, patchErr := coordinator.clientSettings.Inject(clientSettingsPath, account)
		if patchErr != nil {
			return sessions.PlaySession{}, &errs.AppError{
				Code:    errs.ErrClientSettings,
				Message: "Could not write authentication to the instance settings",
				Cause:   patchErr,
			}
		}
		cleanupCredentials = cleanup
	} else if coordinator.clientSettings != nil {
		if clearErr := coordinator.clientSettings.Clear(clientSettingsPath); clearErr != nil {
			return sessions.PlaySession{}, &errs.AppError{
				Code:    errs.ErrClientSettings,
				Message: "Could not clear authentication from the instance settings",
				Cause:   clearErr,
			}
		}
	}

	if err := coordinator.modLayout.EnsureLayout(instance.Directory); err != nil {
		coordinator.clearInjectedCredentials(cleanupCredentials, instance)
		return sessions.PlaySession{}, err
	}
	logsDirectory := filepath.Join(instance.Directory, "Logs")
	if err := coordinator.logs.Harden(logsDirectory); err != nil {
		coordinator.clearInjectedCredentials(cleanupCredentials, instance)
		return sessions.PlaySession{}, err
	}

	settings, err := coordinator.settings.Get(ctx)
	if err != nil {
		coordinator.clearInjectedCredentials(cleanupCredentials, instance)
		return sessions.PlaySession{}, err
	}
	arguments := buildLaunchArguments(
		settings.GlobalLaunchArguments,
		instance.LaunchArguments,
		instance.Directory,
		serverAddress,
	)

	logPath := filepath.Join(logsDirectory, coordinator.now().Format("20060102-150405.000000000")+".log")
	logFile, err := coordinator.logs.Open(logPath)
	if err != nil {
		coordinator.clearInjectedCredentials(cleanupCredentials, instance)
		return sessions.PlaySession{}, err
	}

	// Record the exact launch command so issues like a wrong data path can be
	// diagnosed from the instance log.
	if _, writeErr := fmt.Fprintf(
		logFile,
		"Executing: %s %s\n",
		target.Executable,
		strings.Join(quoteLaunchArguments(arguments), " "),
	); writeErr != nil {
		closeLaunchLog(logFile, instance.Name)
		coordinator.clearInjectedCredentials(cleanupCredentials, instance)
		return sessions.PlaySession{}, &errs.AppError{
			Code:    errs.ErrFilePermission,
			Message: "Could not write the launch command to the instance log",
			Cause:   writeErr,
		}
	}

	environment := make(map[string]string, len(instance.EnvironmentVariables)+1)
	for key, value := range instance.EnvironmentVariables {
		environment[key] = value
	}
	environment["WAXLIGHT_INSTANCE_DIR"] = instance.Directory

	process, err := coordinator.launcher.Start(
		context.Background(),
		target.Executable,
		arguments,
		target.WorkingDirectory,
		environment,
		logFile,
	)
	if err != nil {
		closeLaunchLog(logFile, instance.Name)
		coordinator.clearInjectedCredentials(cleanupCredentials, instance)
		coordinator.reportEvent(ctx, telemetry.EventGameLaunchFailed)
		coordinator.reportError(ctx, telemetry.ErrorGameLaunchFailed, telemetry.ComponentGameLauncher, telemetry.OperationLaunchGame)
		return sessions.PlaySession{}, &errs.AppError{
			Code:    errs.ErrProcessStart,
			Message: processStartMessage(instance.GameClient),
			Cause:   err,
		}
	}

	now := coordinator.now().UTC()
	processID := process.PID()
	session := sessions.PlaySession{
		ID:         coordinator.newID(),
		InstanceID: instance.ID,
		AccountID:  accountID,
		VersionID:  version.ID,
		ProcessID:  &processID,
		StartedAt:  now,
	}
	if err := coordinator.sessions.Create(ctx, session); err != nil {
		if killErr := process.Kill(); killErr != nil {
			slog.Debug("could not kill the game process after a failed session save", "error", killErr)
		}
		closeLaunchLog(logFile, instance.Name)
		coordinator.clearInjectedCredentials(cleanupCredentials, instance)
		return session, err
	}

	instance.Status = instances.StatusRunning
	instance.LastPlayedAt = &now
	instance.UpdatedAt = now
	if err := coordinator.instances.SaveInstance(ctx, instance); err != nil {
		slog.Warn("could not persist the running instance", "instance", instance.Name, "error", err)
	}

	coordinator.registry.Start(instance.ID, runningGame{
		process:   process,
		sessionID: session.ID,
		started:   now,
		client:    instance.GameClient,
		log:       logFile,
		cleanup:   cleanupCredentials,
	})

	coordinator.publish("game:started", session)
	coordinator.reportEvent(ctx, telemetry.EventGameLaunchSucceeded)
	slog.Info("game started", "instance", instance.Name)
	// Snapshot the startup window once on the caller goroutine; the launch
	// goroutines below read only this captured value (the package variable is
	// mutable in tests).
	startupWindow := gameStartupWindow
	coordinator.workers.Go(func(workerCtx context.Context) {
		coordinator.markLaunchEstablished(workerCtx, instance, session.ID, startupWindow)
	})
	releaseOnReturn = false
	go coordinator.waitForGame(instance, process, session.ID, now, logFile, cleanupCredentials, gamelog.Watch(instance.Name, logPath), startupWindow, release)
	return session, nil
}

func (coordinator *Coordinator) resolveLaunchTarget(
	ctx context.Context,
	instance instances.Instance,
	version versions.GameVersion,
) (OptimumTarget, error) {
	if instance.GameClient != instances.GameClientOptimum {
		return OptimumTarget{Executable: version.ExecutablePath, WorkingDirectory: filepath.Dir(version.ExecutablePath)}, nil
	}
	if coordinator.optimum == nil {
		return OptimumTarget{}, errs.NewError(errs.ErrValidation, "Optimum support is unavailable. Switch this instance to Vanilla")
	}
	settings, err := coordinator.settings.Get(ctx)
	if err != nil {
		return OptimumTarget{}, err
	}
	target, err := coordinator.optimum.Resolve(settings.OptimumPath, filepath.Dir(version.ExecutablePath))
	if err != nil {
		return OptimumTarget{}, err
	}
	if target.Exclusive && coordinator.registry.ClientRunning(instances.GameClientOptimum) {
		return OptimumTarget{}, errs.NewError(errs.ErrValidation, "This Optimum installation is already being used by another running instance")
	}
	return target, nil
}

func launchIssue(err error) string {
	var appError *errs.AppError
	if errors.As(err, &appError) && strings.TrimSpace(appError.Message) != "" {
		return appError.Message
	}
	return "Optimum could not be prepared for launch. Configure Optimum or switch this instance to Vanilla"
}

func processStartMessage(client instances.GameClient) string {
	if client == instances.GameClientOptimum {
		return "Failed to start Optimum. Check the configured installation or switch this instance to Vanilla"
	}
	return "Failed to start Vintage Story"
}

func quoteLaunchArguments(arguments []string) []string {
	result := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		if strings.ContainsAny(argument, " \t\"") {
			result = append(result, `"`+strings.ReplaceAll(argument, `"`, `\"`)+`"`)
		} else {
			result = append(result, argument)
		}
	}
	return result
}

func buildLaunchArguments(global, instance []string, dataPath, serverAddress string) []string {
	arguments := append([]string{}, global...)
	arguments = append(arguments, instance...)
	arguments = append(arguments, "--dataPath", dataPath)
	if serverAddress != "" {
		arguments = append(arguments, "--connect", serverAddress)
	}
	return arguments
}

func (coordinator *Coordinator) resolveAccountID(
	ctx context.Context,
	instance instances.Instance,
	requested *string,
) (*string, error) {
	if requested != nil && strings.TrimSpace(*requested) != "" {
		if coordinator.accounts == nil {
			return nil, errs.NewError(errs.ErrAccountNotFound, "Account not found")
		}
		if _, err := coordinator.accounts.GetAccount(ctx, *requested); err != nil {
			return nil, err
		}
		value := *requested
		return &value, nil
	}
	if instance.DefaultAccountID != nil && strings.TrimSpace(*instance.DefaultAccountID) != "" {
		if coordinator.accounts == nil {
			return nil, errs.NewError(errs.ErrAccountNotFound, "Account not found")
		}
		if _, err := coordinator.accounts.GetAccount(ctx, *instance.DefaultAccountID); err != nil {
			return nil, err
		}
		value := *instance.DefaultAccountID
		return &value, nil
	}
	if coordinator.accounts == nil {
		return nil, nil
	}
	storedAccounts, err := coordinator.accounts.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	for _, account := range storedAccounts {
		if account.IsDefault {
			value := account.ID
			return &value, nil
		}
	}
	return nil, nil
}

// closeLaunchLog best-effort closes the instance log file opened for a launch.
// Failures only affect cleanup, so they are logged at debug level.
func closeLaunchLog(logFile io.Closer, instanceName string) {
	if err := logFile.Close(); err != nil {
		slog.Debug("could not close the instance log file", "instance", instanceName, "error", err)
	}
}

// clearInjectedCredentials removes the session credentials injected into the
// instance client settings. A failure leaves credentials on disk, so it is
// logged as an error.
func (coordinator *Coordinator) clearInjectedCredentials(cleanup func() error, instance instances.Instance) {
	if err := cleanup(); err != nil {
		slog.Error("could not remove injected credentials", "instance", instance.Name, "error", err)
	}
}

// markLaunchEstablished records the Last Known Good state once a game process
// survives the startup window. The timer never fires for short-lived crashes
// because waitForGame removes the running entry when the process exits.
// startupWindow is the value captured by Launch so this goroutine never reads
// the mutable package variable.
func (coordinator *Coordinator) markLaunchEstablished(
	ctx context.Context,
	instance instances.Instance,
	sessionID string,
	startupWindow time.Duration,
) {
	timer := time.NewTimer(startupWindow)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return
	}
	running, ok := coordinator.registry.Get(instance.ID)
	if !ok || running.sessionID != sessionID {
		return
	}
	slog.Info("launch considered successful", "instance", instance.Name, "startupWindow", startupWindow.String())
	coordinator.recovery.RecordLastKnownGood(context.Background(), instance)
}

// RecordEstablishedOnShutdown records the Last Known Good state of every game
// that was still running past the startup window when the launcher shuts down.
// The game itself keeps running, so its configuration is a working one.
func (coordinator *Coordinator) RecordEstablishedOnShutdown() {
	var established []string
	for _, id := range coordinator.registry.RunningInstanceIDs() {
		running, ok := coordinator.registry.Get(id)
		if ok && time.Since(running.started) >= gameStartupWindow {
			established = append(established, id)
		}
	}
	for _, instanceID := range established {
		instance, err := coordinator.instances.GetInstance(context.Background(), instanceID)
		if err != nil {
			slog.Warn("could not read an instance for the last known good state", "instanceId", instanceID, "error", err)
			continue
		}
		slog.Info("launch considered successful", "instance", instance.Name, "launcherShutdown", true)
		coordinator.recovery.RecordLastKnownGood(context.Background(), instance)
	}
}

// waitForGame tracks a started game until it exits, then persists the finished
// session, restores the instance status, and releases the mutation gate that
// was held for the whole play session.
func (coordinator *Coordinator) waitForGame(
	instance instances.Instance,
	process process.Running,
	sessionID string,
	startedAt time.Time,
	logFile io.Closer,
	cleanupCredentials func() error,
	stopGameLog func(),
	startupWindow time.Duration,
	releaseMutation func(),
) {
	defer releaseMutation()
	exitCode, waitErr := process.Wait()
	// Let the tailer pick up the lines the process flushed right before
	// exiting, then stop it before the log file is closed.
	stopGameLog()
	if err := logFile.Close(); err != nil {
		slog.Debug("could not close the instance log file", "instance", instance.Name, "error", err)
	}
	coordinator.clearInjectedCredentials(cleanupCredentials, instance)

	durationSeconds := int64(time.Since(startedAt).Seconds())
	crashed := waitErr != nil || exitCode != 0
	if crashed {
		slog.Warn("game exited with an error", "instance", instance.Name, "exitCode", exitCode, "error", waitErr)
	} else {
		slog.Info("game exited", "instance", instance.Name, "exitCode", exitCode, "seconds", durationSeconds)
	}
	// A crash inside the startup window is a failed startup: compare the
	// current configuration with the Last Known Good state and offer a safe
	// recovery. A game that ran past the window (or exited normally) is not a
	// configuration failure, no matter how it ended.
	if crashed && time.Since(startedAt) < startupWindow {
		coordinator.recovery.HandleFailedLaunch(instance)
	}
	if err := coordinator.sessions.Finish(context.Background(), sessionID, exitCode, crashed, durationSeconds); err != nil {
		slog.Warn("could not persist the finished session", "instance", instance.Name, "sessionId", sessionID, "error", err)
	}

	instance.Status = instances.StatusReady
	instance.UpdatedAt = coordinator.now().UTC()
	if err := coordinator.instances.SaveInstance(context.Background(), instance); err != nil {
		slog.Warn("could not persist the instance after the game exited", "instance", instance.Name, "error", err)
	}

	coordinator.registry.Stop(instance.ID)

	coordinator.publish("game:exited", map[string]any{
		"instanceId":      instance.ID,
		"sessionId":       sessionID,
		"exitCode":        exitCode,
		"crashed":         crashed,
		"durationSeconds": durationSeconds,
	})
}

// Stop requests a running game to stop; force kills it instead.
func (coordinator *Coordinator) Stop(ctx context.Context, instanceID string, force bool) error {
	slog.Info("stopping instance", "instance", instanceID, "force", force)
	game, ok := coordinator.registry.Get(instanceID)
	if !ok {
		return errs.NewError(errs.ErrValidation, "The instance is not running")
	}
	var e error
	if force {
		e = game.process.Kill()
	} else {
		e = game.process.Stop()
	}
	if e != nil {
		return &errs.AppError{Code: errs.ErrProcessStop, Message: "Failed to stop Vintage Story", Cause: e}
	}
	return nil
}

// RunningInstanceIDs returns the sorted instance IDs with a running game.
func (coordinator *Coordinator) RunningInstanceIDs() []string {
	return coordinator.registry.RunningInstanceIDs()
}

// CheckDataRootRelocation rejects a move while a game or operation is running.
func (coordinator *Coordinator) CheckDataRootRelocation(ctx context.Context) error {
	if len(coordinator.registry.RunningInstanceIDs()) > 0 {
		return errs.NewError(instances.ErrInstanceRunning, "Stop the game before moving the data folder")
	}
	tracked, err := coordinator.operations.ListLimit(ctx, 1000)
	if err != nil {
		return err
	}
	for _, operation := range tracked {
		if operation.Status == operations.StatusRunning || operation.Status == operations.StatusQueued {
			return errs.NewError(errs.ErrDataFolderBusy, "Wait for running operations to finish before moving the data folder")
		}
	}
	return nil
}

func (coordinator *Coordinator) beginMutation() (func(), error) {
	if err := coordinator.gate.Begin(); err != nil {
		return nil, err
	}
	return coordinator.gate.End, nil
}

// ReconcileInjectedCredentials clears stale injected credentials from every
// instance after a crash or failed cleanup.
func (coordinator *Coordinator) ReconcileInjectedCredentials(ctx context.Context) error {
	if coordinator.clientSettings == nil {
		return nil
	}
	release, err := coordinator.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	instances, err := coordinator.instances.ListInstances(ctx)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		if err := coordinator.logs.Harden(filepath.Join(instance.Directory, "Logs")); err != nil {
			return err
		}
		if err := coordinator.clientSettings.Reconcile(filepath.Join(instance.Directory, "clientsettings.json")); err != nil {
			return &errs.AppError{Code: errs.ErrClientSettings, Message: "Could not clear stale instance authentication", Cause: err}
		}
	}
	return nil
}

// ClearAccountFromInstances removes account credentials from every instance
// and clears the default-account association.
func (coordinator *Coordinator) ClearAccountFromInstances(ctx context.Context, accountID string) error {
	if coordinator.clientSettings == nil {
		return nil
	}
	release, err := coordinator.beginMutation()
	if err != nil {
		return err
	}
	defer release()
	instances, err := coordinator.instances.ListInstances(ctx)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		if err := coordinator.clientSettings.Clear(filepath.Join(instance.Directory, "clientsettings.json")); err != nil {
			return &errs.AppError{Code: errs.ErrClientSettings, Message: "Could not clear account authentication from an instance", Cause: err}
		}
		if instance.DefaultAccountID != nil && *instance.DefaultAccountID == accountID {
			instance.DefaultAccountID = nil
			instance.UpdatedAt = coordinator.now().UTC()
			if err := coordinator.instances.SaveInstance(ctx, instance); err != nil {
				return err
			}
		}
	}
	return nil
}

func (coordinator *Coordinator) publish(name string, payload any) {
	if coordinator.events != nil {
		coordinator.events.Publish(name, payload)
	}
}

func (coordinator *Coordinator) reportEvent(ctx context.Context, name string) {
	if coordinator.telemetry != nil {
		coordinator.telemetry.Event(ctx, name)
	}
}

func (coordinator *Coordinator) reportError(ctx context.Context, code, component, operation string) {
	if coordinator.telemetry != nil {
		coordinator.telemetry.Error(ctx, code, component, operation)
	}
}

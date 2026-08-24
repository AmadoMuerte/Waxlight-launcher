package wails

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/atomicfile"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/logging"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/nativefs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/version"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/versions"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// maxFrontendLogMessage caps how large a single frontend-provided log message
// may be so the UI cannot flood the log buffer.
const maxFrontendLogMessage = 4096

// maxFrontendLogAttrs caps how many key/value pairs the frontend may attach to
// one log message.
const maxFrontendLogAttrs = 16

type versionLister interface {
	List(context.Context) ([]versions.GameVersion, error)
}

type instanceLister interface {
	List(context.Context) ([]instances.Instance, error)
}

// LogController exposes the launcher's in-memory log console and lets the user
// export the recent logs plus a system summary for support.
type LogController struct {
	instances instanceLister
	mods      *mods.Service
	versions  versionLister
	lifecycle lifecycle
}

func NewLogController(instances instanceLister, mods *mods.Service, versionService versionLister, lifecycle lifecycle) *LogController {
	return &LogController{instances: instances, mods: mods, versions: versionService, lifecycle: lifecycle}
}

// ListLogs returns the most recent log lines rendered for the console. The
// newest lines come last. Lines are ANSI-colored so an xterm-style viewer can
// display them directly.
func (controller *LogController) ListLogs(limit int) ([]string, error) {
	lines := logging.SnapshotLines()
	if limit > 0 && len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return lines, nil
}

// WriteLog routes a log entry reported from the frontend into the launcher's
// logging pipeline (in-memory console, live push, session file, support
// export). The level must be one of "debug", "info", "warn" or "error";
// messages are length-capped and sensitive values are redacted like any other
// log line. Attrs are attached as key/value pairs; the entry is marked with
// source=frontend so UI-originated lines are distinguishable.
func (controller *LogController) WriteLog(level string, message string, attrs map[string]string) error {
	slogLevel, err := frontendLogLevel(level)
	if err != nil {
		return err
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return errs.NewError(errs.ErrValidation, "Log message must not be empty")
	}
	if len(message) > maxFrontendLogMessage {
		message = message[:maxFrontendLogMessage]
	}
	args := make([]any, 0, 2+len(attrs)*2)
	args = append(args, "source", "frontend")
	for key, value := range attrs {
		if len(args) >= 2+maxFrontendLogAttrs*2 {
			break
		}
		args = append(args, key, value)
	}
	ctx := context.Background()
	if controller.lifecycle != nil {
		ctx = controller.lifecycle.Context()
	}
	slog.Log(ctx, slogLevel, message, args...)
	return nil
}

// frontendLogLevel maps a frontend level name onto a slog level. An unknown
// level is a validation error so a buggy UI cannot corrupt the log stream.
func frontendLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, errs.NewError(errs.ErrValidation, "Unsupported log level: "+level)
	}
}

// OpenLogsDirectory opens the launcher's rolling log directory in the native
// file manager, creating it when needed.
func (controller *LogController) OpenLogsDirectory() error {
	dir := logging.LogDirectory()
	if dir == "" {
		return errs.NewError(errs.ErrValidation, "Log directory is not configured")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return &errs.AppError{Code: errs.ErrFilePermission, Message: "Could not create the log directory", Cause: err}
	}
	if err := (nativefs.Opener{}).OpenDirectory(dir); err != nil {
		return &errs.AppError{Code: errs.ErrFilePermission, Message: "Could not open the log directory", Cause: err}
	}
	return nil
}

// ExportLogs asks the user for a destination and writes a support log file
// containing a system summary and the recent launcher log. It returns the
// saved path, or an empty string when the user cancels the dialog. Sensitive
// values never reach the file: the log buffer is redacted on write and the
// summary carries no credentials or machine-specific paths.
func (controller *LogController) ExportLogs() (string, error) {
	suggested := "waxlight-logs-" + time.Now().UTC().Format("20060102-150405") + ".txt"
	path, err := wruntime.SaveFileDialog(
		controller.lifecycle.Context(),
		wruntime.SaveDialogOptions{
			Title:           "Export Waxlight launcher logs",
			DefaultFilename: suggested,
			Filters: []wruntime.FileFilter{
				{
					DisplayName: "Text files (*.txt)",
					Pattern:     "*.txt",
				},
			},
		},
	)
	if err != nil || path == "" {
		return "", nil
	}
	report, err := controller.buildSupportLog(controller.lifecycle.Context())
	if err != nil {
		return "", err
	}
	if err := atomicfile.Write(path, []byte(report), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

type supportLogInstance struct {
	Name            string
	GameVersionID   string
	ModCount        int
	EnabledModCount int
}

type supportLogData struct {
	GeneratedAt string
	Version     string
	Platform    string
	GoVersion   string
	Versions    []versions.GameVersion
	Instances   []supportLogInstance
}

func (controller *LogController) gatherSupportLogData(ctx context.Context) (supportLogData, error) {
	data := supportLogData{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Version:     version.Version(),
		Platform:    runtime.GOOS + "/" + runtime.GOARCH,
		GoVersion:   runtime.Version(),
	}
	var err error
	data.Versions, err = controller.versions.List(ctx)
	if err != nil {
		return data, err
	}
	instances, err := controller.instances.List(ctx)
	if err != nil {
		return data, err
	}
	for _, instance := range instances {
		installedMods, modsErr := controller.mods.ListMods(ctx, instance.ID)
		if modsErr != nil {
			slog.Warn("could not count mods for the support log", "instance", instance.Name, "error", modsErr)
		}
		enabled := 0
		for _, mod := range installedMods {
			if mod.Enabled {
				enabled++
			}
		}
		data.Instances = append(data.Instances, supportLogInstance{
			Name:            instance.Name,
			GameVersionID:   instance.GameVersionID,
			ModCount:        len(installedMods),
			EnabledModCount: enabled,
		})
	}
	return data, nil
}

func (controller *LogController) buildSupportLog(ctx context.Context) (string, error) {
	data, err := controller.gatherSupportLogData(ctx)
	if err != nil {
		return "", err
	}
	return formatSupportLog(data, logging.Snapshot()), nil
}

// formatSupportLog renders the export file. It deliberately contains no
// credentials, emails, account data, or absolute paths to user directories.
func formatSupportLog(data supportLogData, entries []logging.Entry) string {
	var builder strings.Builder
	builder.WriteString("Waxlight Launcher support log\n")
	fmt.Fprintf(&builder, "Generated: %s\n", data.GeneratedAt)
	fmt.Fprintf(&builder, "Version: %s\n", data.Version)
	fmt.Fprintf(&builder, "Platform: %s\n", data.Platform)
	fmt.Fprintf(&builder, "Go: %s\n", data.GoVersion)

	builder.WriteString("\nInstalled game versions:\n")
	if len(data.Versions) == 0 {
		builder.WriteString("  none\n")
	}
	for _, item := range data.Versions {
		fmt.Fprintf(&builder, "  - %s (%s)\n", item.Name, item.ID)
	}

	builder.WriteString("\nInstances:\n")
	if len(data.Instances) == 0 {
		builder.WriteString("  none\n")
	}
	for _, item := range data.Instances {
		fmt.Fprintf(
			&builder,
			"  - %s [%s] mods %d enabled %d\n",
			item.Name, item.GameVersionID, item.ModCount, item.EnabledModCount,
		)
	}

	builder.WriteString("\nLauncher log:\n")
	for _, entry := range entries {
		fmt.Fprintf(
			&builder,
			"%s | %-5s | %s\n",
			entry.Time.Format("2006-01-02 15:04:05.000"),
			entry.Level,
			entry.Message,
		)
	}

	builder.WriteString("\nCredentials, account data, and private paths are never included.\n")
	return builder.String()
}

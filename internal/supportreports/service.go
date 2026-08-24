package supportreports

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/operations"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/sessions"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/version"
)

type Store interface {
	GetInstance(context.Context, string) (instances.Instance, error)
	ListMods(context.Context, string) ([]mods.InstalledMod, error)
}

type OperationReader interface {
	ListLimit(context.Context, int) ([]operations.Operation, error)
}

type SessionReader interface {
	ListSessions(context.Context, string, int) ([]sessions.PlaySession, error)
}

type RecoveryReader interface {
	Summary(context.Context, string) (bool, int)
}

type LogReader interface {
	Lines() []string
}

type Identity interface {
	InstallationID(context.Context) string
}

type Sender interface {
	SendSupportReport(context.Context, Report) (Result, error)
}

type Service struct {
	store      Store
	operations OperationReader
	sessions   SessionReader
	recovery   RecoveryReader
	logs       LogReader
	identity   Identity
	sender     Sender
	mu         sync.Mutex
	snapshots  map[string]reportSnapshot
	now        func() time.Time
}

type reportSnapshot struct {
	report    Report
	expiresAt time.Time
}

func NewService(store Store, operations OperationReader, sessions SessionReader, recovery RecoveryReader, logs LogReader, identity Identity, sender Sender) *Service {
	return &Service{store: store, operations: operations, sessions: sessions, recovery: recovery, logs: logs, identity: identity, sender: sender, snapshots: make(map[string]reportSnapshot), now: time.Now}
}

func (s *Service) Preview(ctx context.Context, description, instanceID string) (Preview, error) {
	report, err := s.collect(ctx, description, instanceID)
	if err != nil {
		return Preview{}, err
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return Preview{}, err
	}
	snapshotID, err := newSnapshotID()
	if err != nil {
		return Preview{}, err
	}
	s.mu.Lock()
	for id, snapshot := range s.snapshots {
		if !snapshot.expiresAt.After(s.now()) {
			delete(s.snapshots, id)
		}
	}
	s.snapshots[snapshotID] = reportSnapshot{report: report, expiresAt: s.now().Add(15 * time.Minute)}
	s.mu.Unlock()
	return Preview{SnapshotID: snapshotID, Payload: string(encoded)}, nil
}

func (s *Service) Submit(ctx context.Context, snapshotID string) (Result, error) {
	s.mu.Lock()
	snapshot, ok := s.snapshots[snapshotID]
	s.mu.Unlock()
	if !ok || !snapshot.expiresAt.After(s.now()) {
		return Result{}, errs.NewError(errs.ErrValidation, "The support report preview expired; review it again")
	}
	report := snapshot.report
	encoded, err := json.Marshal(report)
	if err != nil {
		return Result{}, err
	}
	if len(encoded) > MaxPayloadBytes {
		return Result{}, errs.NewError(errs.ErrSupportReportTooLarge, "The support report is too large")
	}
	result, err := s.sender.SendSupportReport(ctx, report)
	if err == nil {
		s.mu.Lock()
		delete(s.snapshots, snapshotID)
		s.mu.Unlock()
	}
	return result, err
}

func newSnapshotID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (s *Service) collect(ctx context.Context, description, instanceID string) (Report, error) {
	description = strings.TrimSpace(description)
	if description == "" || len([]rune(description)) > MaxDescriptionLength {
		return Report{}, errs.NewError(errs.ErrValidation, "Describe the problem in 1 to 2000 characters")
	}
	report := Report{
		SchemaVersion:  SchemaVersion,
		InstallationID: s.identity.InstallationID(ctx),
		Description:    SanitizeText(description),
		Launcher:       Launcher{Version: version.Version(), Platform: runtime.GOOS, Arch: runtime.GOARCH},
		System:         System{OS: runtime.GOOS, Arch: runtime.GOARCH, GoVersion: runtime.Version()},
		Mods:           []Mod{}, Operations: []Operation{}, Logs: Logs{Launcher: boundedLines(s.logs.Lines())},
	}
	if instanceID != "" {
		s.collectInstance(ctx, &report, instanceID)
	}
	s.collectOperations(ctx, &report)
	s.collectLaunch(ctx, &report, instanceID)
	return report, nil
}

func (s *Service) collectInstance(ctx context.Context, report *Report, instanceID string) {
	instance, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return
	}
	item := &Instance{ID: instance.ID, Name: SanitizeText(instance.Name), GameVersion: instance.GameVersionID, Client: string(instance.GameClient), Status: instance.Status, LaunchArguments: sanitizeStrings(instance.LaunchArguments, MaxLaunchArguments), EnvironmentVariables: sanitizeEnvironment(instance.EnvironmentVariables)}
	installed, err := s.store.ListMods(ctx, instanceID)
	if err == nil {
		for _, installedMod := range installed[:min(len(installed), MaxMods)] {
			modID, source := safeSource(installedMod.Source)
			report.Mods = append(report.Mods, Mod{ModID: modID, Version: SanitizeText(installedMod.Version), Enabled: installedMod.Enabled, Source: source, UpdatePolicy: string(mods.NormalizeUpdatePolicy(installedMod.UpdatePolicy))})
			item.ModCount++
			if installedMod.Enabled {
				item.EnabledModCount++
			}
		}
	}
	report.Instance = item
	if s.recovery != nil {
		exists, count := s.recovery.Summary(ctx, instanceID)
		report.Recovery = &Recovery{LastKnownGoodExists: exists, SnapshotCount: count}
	}
}

func (s *Service) collectOperations(ctx context.Context, report *Report) {
	items, err := s.operations.ListLimit(ctx, MaxOperations)
	if err != nil {
		return
	}
	for _, item := range items {
		mapped := Operation{Type: item.Type, Status: item.Status, Progress: item.Progress, CurrentBytes: item.CurrentBytes, TotalBytes: item.TotalBytes, CreatedAt: item.CreatedAt, StartedAt: item.StartedAt, FinishedAt: item.FinishedAt}
		if item.ErrorCode != nil {
			mapped.ErrorCode = *item.ErrorCode
		}
		if item.ErrorMessage != nil {
			mapped.Error = SanitizeText(*item.ErrorMessage)
		}
		report.Operations = append(report.Operations, mapped)
	}
}

func (s *Service) collectLaunch(ctx context.Context, report *Report, instanceID string) {
	items, err := s.sessions.ListSessions(ctx, instanceID, 1)
	if err != nil || len(items) == 0 {
		return
	}
	item := items[0]
	report.Launch = &Launch{GameVersion: item.VersionID, StartedAt: item.StartedAt, EndedAt: item.EndedAt, DurationSec: item.DurationSec, ExitCode: item.ExitCode, StartupFailed: item.Crashed && item.DurationSec < 30}
}

func sanitizeStrings(values []string, limit int) []string {
	if len(values) > limit {
		values = values[:limit]
	}
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = SanitizeText(value)
	}
	return result
}

func sanitizeEnvironment(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > MaxEnvironment {
		names = names[:MaxEnvironment]
	}
	limited := make(map[string]string, len(names))
	for _, name := range names {
		limited[name] = values[name]
	}
	return SanitizeEnvironment(limited)
}

func boundedLines(lines []string) []string {
	if len(lines) > MaxLogLines {
		lines = lines[len(lines)-MaxLogLines:]
	}
	result := make([]string, 0, len(lines))
	bytes := 0
	for i := len(lines) - 1; i >= 0; i-- {
		line := SanitizeText(lines[i])
		if bytes+len(line) > MaxLogBytes {
			break
		}
		result = append(result, line)
		bytes += len(line)
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

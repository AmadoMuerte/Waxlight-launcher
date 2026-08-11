package telemetry

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mods"
	"github.com/waxlight/waxlight-launcher/internal/settings"
	"github.com/waxlight/waxlight-launcher/internal/version"
)

// HeartbeatInterval is the minimum time between heartbeat transmissions. The
// launcher sends at most approximately one heartbeat per installation per day.
const HeartbeatInterval = 24 * time.Hour

// lastHeartbeatKey stores the UTC time of the last successful heartbeat inside
// the normal Waxlight settings storage.
const lastHeartbeatKey = "telemetry_last_heartbeat"

// Store contains only authoritative instance and mod count sources.
type Store interface {
	ListInstances(context.Context) ([]instances.Instance, error)
	ListMods(context.Context, string) ([]mods.InstalledMod, error)
}

type SettingsReader interface {
	Get(context.Context) (settings.Settings, error)
}

// Sender delivers telemetry payloads. The HTTP Client implements it; tests
// substitute fakes.
type Sender interface {
	SendHeartbeat(context.Context, Heartbeat) error
	SendEvent(context.Context, Event) error
	SendError(context.Context, ErrorEvent) error
}

// Service coordinates telemetry collection, scheduling, and delivery.
//
// Telemetry is strictly best-effort: it is never required for launcher
// functionality, deliveries are asynchronous, failures are silently dropped
// (debug-level logs only), and nothing is ever retried aggressively.
type Service struct {
	sender           Sender
	store            Store
	settings         SettingsReader
	values           settings.ValueRepository
	identity         *identity
	now              func() time.Time
	deliveryMu       sync.RWMutex
	heartbeatMu      sync.Mutex
	heartbeatPending bool
}

// SynchronizeConsent makes a persisted preference change atomic with respect to
// starting a telemetry request. Callers must use it for writes to settings.
func (s *Service) SynchronizeConsent(change func() error) error {
	s.deliveryMu.Lock()
	defer s.deliveryMu.Unlock()
	return change()
}

func NewService(sender Sender, reader SettingsReader, values settings.ValueRepository, store Store) *Service {
	return &Service{
		sender:   sender,
		store:    store,
		settings: reader,
		values:   values,
		identity: newIdentity(values),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// Enabled reports whether the user has opted into telemetry. It fails closed:
// any settings error is treated as disabled, and nothing is ever transmitted
// while the setting forbids it.
func (s *Service) Enabled(ctx context.Context) bool {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		slog.Debug("telemetry: settings unavailable, treating as disabled", "error", err)
		return false
	}
	return settings.TelemetryEnabled
}

// Event reports an allowlisted lifecycle event asynchronously. The caller's
// operation never depends on telemetry delivery. Unknown event names are
// dropped: the allowlist is the only path into the backend.
func (s *Service) Event(ctx context.Context, name string) {
	if _, allowed := allowedEvents[name]; !allowed {
		slog.Debug("telemetry: unknown event dropped", "event", name)
		return
	}
	if !s.Enabled(ctx) {
		return
	}
	go s.sendEvent(context.Background(), name)
}

func (s *Service) sendEvent(ctx context.Context, name string) {
	if !s.Enabled(ctx) {
		return
	}
	installationID := s.identity.ID(ctx)
	if installationID == "" {
		return
	}
	s.deliveryMu.RLock()
	defer s.deliveryMu.RUnlock()
	if !s.Enabled(ctx) {
		return
	}
	err := s.sender.SendEvent(ctx, Event{
		InstallationID: installationID,
		AppVersion:     version.Version(),
		OS:             normalizeOS(runtime.GOOS),
		Arch:           normalizeArch(runtime.GOARCH),
		Event:          name,
	})
	if err != nil {
		slog.Debug("telemetry: event delivery failed", "event", name)
	}
}

// Error reports a structured error category asynchronously. Only allowlisted
// codes, components, and operations are transmitted; raw error text, stack
// traces, paths, and response bodies never enter the payload.
func (s *Service) Error(ctx context.Context, code, component, operation string) {
	if _, allowedCode := allowedErrorCodes[code]; !allowedCode {
		slog.Debug("telemetry: error code outside taxonomy dropped", "code", code)
		return
	}
	if _, allowedComponent := allowedComponents[component]; !allowedComponent {
		slog.Debug("telemetry: component outside taxonomy dropped", "component", component)
		return
	}
	if _, allowedOperation := allowedOperations[operation]; !allowedOperation {
		slog.Debug("telemetry: operation outside taxonomy dropped", "operation", operation)
		return
	}
	if !s.Enabled(ctx) {
		return
	}
	go s.sendError(context.Background(), code, component, operation)
}

func (s *Service) sendError(ctx context.Context, code, component, operation string) {
	if !s.Enabled(ctx) {
		return
	}
	installationID := s.identity.ID(ctx)
	if installationID == "" {
		return
	}
	s.deliveryMu.RLock()
	defer s.deliveryMu.RUnlock()
	if !s.Enabled(ctx) {
		return
	}
	err := s.sender.SendError(ctx, ErrorEvent{
		InstallationID: installationID,
		AppVersion:     version.Version(),
		OS:             normalizeOS(runtime.GOOS),
		Arch:           normalizeArch(runtime.GOARCH),
		ErrorCode:      code,
		Component:      component,
		Operation:      operation,
	})
	if err != nil {
		slog.Debug("telemetry: error delivery failed", "code", code)
	}
}

// MaybeSendHeartbeat transmits a heartbeat if telemetry is enabled and the
// last successful heartbeat is older than HeartbeatInterval. It is called at
// application startup and whenever telemetry is enabled. The mutex and the
// pending flag guarantee that concurrent callers never send multiple
// heartbeats, and a failed send leaves the persisted timestamp untouched so
// the installation stays eligible for the next heartbeat opportunity.
func (s *Service) MaybeSendHeartbeat() {
	ctx := context.Background()
	if !s.Enabled(ctx) {
		return
	}
	s.heartbeatMu.Lock()
	defer s.heartbeatMu.Unlock()
	if raw, err := s.values.GetSettingValue(ctx, lastHeartbeatKey); err == nil {
		if last, parseErr := time.Parse(time.RFC3339, raw); parseErr == nil {
			if s.now().Sub(last) < HeartbeatInterval {
				return
			}
		}
	}
	if s.heartbeatPending {
		return
	}
	s.heartbeatPending = true
	go func() {
		defer func() {
			s.heartbeatMu.Lock()
			s.heartbeatPending = false
			s.heartbeatMu.Unlock()
		}()
		s.sendHeartbeat(context.Background())
	}()
}

func (s *Service) sendHeartbeat(ctx context.Context) {
	if !s.Enabled(ctx) {
		return
	}
	installationID := s.identity.ID(ctx)
	if installationID == "" {
		return
	}
	instances, err := s.store.ListInstances(ctx)
	if err != nil {
		slog.Debug("telemetry: heartbeat skipped, instance count unavailable", "error", err)
		return
	}
	// mods_count semantics: total number of mods currently installed across
	// all Waxlight instances, from the authoritative store. The shared
	// download cache is deliberately not counted so installed copies are not
	// double-counted.
	modsCount := 0
	for _, instance := range instances {
		mods, listErr := s.store.ListMods(ctx, instance.ID)
		if listErr != nil {
			continue
		}
		modsCount += len(mods)
	}
	s.deliveryMu.RLock()
	defer s.deliveryMu.RUnlock()
	if !s.Enabled(ctx) {
		return
	}
	err = s.sender.SendHeartbeat(ctx, Heartbeat{
		InstallationID: installationID,
		AppVersion:     version.Version(),
		OS:             normalizeOS(runtime.GOOS),
		Arch:           normalizeArch(runtime.GOARCH),
		InstancesCount: len(instances),
		ModsCount:      modsCount,
	})
	if err != nil {
		slog.Debug("telemetry: heartbeat delivery failed")
		return
	}
	if err := s.values.SetSettingValue(ctx, lastHeartbeatKey, s.now().Format(time.RFC3339)); err != nil {
		slog.Debug("telemetry: could not persist heartbeat time", "error", err)
	}
	slog.Debug("telemetry: heartbeat sent")
}

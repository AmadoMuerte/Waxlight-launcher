package settings

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

const progressInterval = 150 * time.Millisecond

// ErrDataFolderNotWritable is wrapped around a relocation-target write failure
// so the feature layer can map it to a user-facing permission error. It is
// implemented by the dataroot adapter.
var ErrDataFolderNotWritable = errors.New("data folder target is not writable")

// DataRootService coordinates relocation without depending on Wails or a host platform.
type DataRootService struct {
	root    DataRoot
	gate    RelocationGate
	checker RelocationChecker
	workers WorkerGroup
	events  Publisher
	quitter Quitter
	now     func() time.Time
}

func NewDataRootService(
	root DataRoot,
	gate RelocationGate,
	checker RelocationChecker,
	workers WorkerGroup,
	events Publisher,
	quitter Quitter,
) *DataRootService {
	return &DataRootService{root: root, gate: gate, checker: checker, workers: workers, events: events, quitter: quitter, now: time.Now}
}

func (service *DataRootService) Get() (DataFolder, error) {
	current, err := service.root.Current()
	if err != nil {
		return DataFolder{}, err
	}
	lastError, err := service.root.ReadError()
	if err != nil {
		return DataFolder{}, err
	}
	return DataFolder{CurrentPath: current, DefaultPath: service.root.Home(), LastError: lastError}, nil
}

// Check verifies that a target can be used as the launcher data folder,
// including write access, without starting a relocation. A write failure maps
// to a permission error so the frontend can show guidance before the move is
// confirmed.
func (service *DataRootService) Check(_ context.Context, target string) error {
	err := service.root.CheckTarget(target)
	if errors.Is(err, ErrDataFolderNotWritable) {
		return errs.NewError(errs.ErrFilePermission, "Waxlight has no write access to this folder")
	}
	return err
}

func (service *DataRootService) Move(ctx context.Context, target string) error {
	if err := service.gate.BeginRelocation(); err != nil {
		return err
	}
	release := true
	defer func() {
		if release {
			service.gate.EndRelocation()
		}
	}()
	if err := service.checker.CheckDataRootRelocation(ctx); err != nil {
		return err
	}
	relocation, err := service.root.PrepareRelocation(target)
	if err != nil {
		if errors.Is(err, ErrDataFolderNotWritable) {
			return errs.NewError(errs.ErrFilePermission, "Waxlight has no write access to this folder")
		}
		return err
	}
	service.events.Publish("data-folder:progress", RelocationProgress{Phase: "preparing"})
	if !service.workers.Go(func(workerCtx context.Context) { service.run(workerCtx, relocation) }) {
		return errs.NewError(errs.ErrDataFolderBusy, "The launcher is shutting down")
	}
	release = false
	return nil
}

func (service *DataRootService) run(ctx context.Context, relocation Relocation) {
	var progressMu sync.Mutex
	lastEmit := time.Time{}
	err := relocation.Run(ctx, func(copied, total int64) {
		progressMu.Lock()
		defer progressMu.Unlock()
		progress := 0.0
		if total > 0 {
			progress = float64(copied) / float64(total)
		}
		now := service.now()
		if now.Sub(lastEmit) < progressInterval && progress < 1 {
			return
		}
		lastEmit = now
		service.events.Publish("data-folder:progress", RelocationProgress{
			CopiedBytes: copied,
			TotalBytes:  total,
			Progress:    progress,
			Phase:       "moving",
		})
	})
	if err != nil {
		service.gate.EndRelocation()
		service.events.Publish("data-folder:error", map[string]string{"message": err.Error()})
		return
	}
	service.events.Publish("data-folder:progress", RelocationProgress{Progress: 1, Phase: "relaunching"})
	select {
	case <-time.After(500 * time.Millisecond):
		service.quitter.Quit(ctx)
	case <-ctx.Done():
	}
}

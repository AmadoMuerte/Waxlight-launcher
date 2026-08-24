package settings

import (
	"context"
	"sync"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

const progressInterval = 150 * time.Millisecond

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

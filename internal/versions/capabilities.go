package versions

import (
	"time"

	"github.com/waxlight/waxlight-launcher/internal/operations"
)

// Capabilities is a concrete aggregate for hosts which expose all version
// features. Each embedded component remains independently constructible and
// consumable through a narrow interface.
type Capabilities struct {
	*QueryService
	*LocalInstallService
	*CatalogInstallService
	*RemovalService
}

func NewCapabilities(
	query *QueryService,
	local *LocalInstallService,
	catalog *CatalogInstallService,
	removal *RemovalService,
) *Capabilities {
	return &Capabilities{QueryService: query, LocalInstallService: local, CatalogInstallService: catalog, RemovalService: removal}
}

// InstallRuntime contains only mechanics shared by local and catalog install
// pipelines; it exposes no query, catalog, or removal capability.
type InstallRuntime struct {
	filesystem Filesystem
	gate       MutationGate
	operations *operations.Manager
	now        func() time.Time
	newID      func() string
}

func NewInstallRuntime(
	filesystem Filesystem,
	gate MutationGate,
	operationManager *operations.Manager,
	now func() time.Time,
	newID func() string,
) *InstallRuntime {
	return &InstallRuntime{filesystem: filesystem, gate: gate, operations: operationManager, now: now, newID: newID}
}

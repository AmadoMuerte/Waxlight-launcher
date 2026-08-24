package servers

import (
	"context"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
)

// QueryRepository is the read-only favorite-server persistence surface.
type QueryRepository interface {
	ListFavoriteServers(context.Context) ([]FavoriteServer, error)
	GetFavoriteServer(context.Context, string) (FavoriteServer, error)
}

// MutationRepository is the favorite-server persistence surface for writes.
type MutationRepository interface {
	SaveFavoriteServer(context.Context, FavoriteServer) error
	DeleteFavoriteServer(context.Context, string) error
}

// Repository is the complete favorite-server persistence surface.
type Repository interface {
	QueryRepository
	MutationRepository
}

// InstanceReader resolves the instance a favorite server is associated with.
type InstanceReader interface {
	GetInstance(context.Context, string) (instances.Instance, error)
}

// Catalog lists public server listings from the Vintage Story catalog.
type Catalog interface {
	List(context.Context) ([]PublicServer, error)
}

// MutationGate coordinates launcher-wide writes with data-root relocation.
type MutationGate interface {
	Begin() error
	End()
}

// Publisher emits launcher events.
type Publisher interface {
	Publish(string, any)
}

// PublishFunc adapts a publish function to the Publisher port.
type PublishFunc func(string, any)

func (publish PublishFunc) Publish(name string, payload any) {
	publish(name, payload)
}

// Clock returns the current wall-clock time.
type Clock func() time.Time

// IDGenerator creates feature-owned record identifiers.
type IDGenerator func() string

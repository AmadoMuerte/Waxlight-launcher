package servers

import (
	"context"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/errs"
)

// Service owns favorite-server persistence, validation, instance
// association, and update events.
type Service struct {
	repository Repository
	instances  InstanceReader
	gate       MutationGate
	events     Publisher
	now        Clock
	newID      IDGenerator
}

func NewService(
	repository Repository,
	instances InstanceReader,
	gate MutationGate,
	events Publisher,
	now Clock,
	newID IDGenerator,
) *Service {
	return &Service{
		repository: repository,
		instances:  instances,
		gate:       gate,
		events:     events,
		now:        now,
		newID:      newID,
	}
}

// List returns favorite servers ordered by most recently updated.
func (service *Service) List(ctx context.Context) ([]FavoriteServer, error) {
	return service.repository.ListFavoriteServers(ctx)
}

// Get returns a single favorite server by identifier.
func (service *Service) Get(ctx context.Context, id string) (FavoriteServer, error) {
	return service.repository.GetFavoriteServer(ctx, id)
}

// Save creates or updates a favorite server. Whitelist-only listings may omit
// the address; every other entry needs a name and a space-free address. An
// existing server keeps its creation timestamp.
func (service *Service) Save(ctx context.Context, input SaveInput) (FavoriteServer, error) {
	if err := service.gate.Begin(); err != nil {
		return FavoriteServer{}, err
	}
	defer service.gate.End()

	name := strings.TrimSpace(input.Name)
	address := strings.TrimSpace(input.Address)
	if name == "" || len(name) > 100 || len(address) > 255 || strings.ContainsAny(address, "\r\n\t ") {
		return FavoriteServer{}, errs.NewError(errs.ErrValidation, "Enter a server name and an address without spaces")
	}
	if input.InstanceID != nil {
		if _, err := service.instances.GetInstance(ctx, *input.InstanceID); err != nil {
			return FavoriteServer{}, err
		}
	}
	now := service.now().UTC()
	server := FavoriteServer{ID: input.ID, Name: name, Address: address, InstanceID: input.InstanceID, UpdatedAt: now}
	if server.ID == "" {
		server.ID = service.newID()
		server.CreatedAt = now
	} else if previous, err := service.repository.GetFavoriteServer(ctx, server.ID); err != nil {
		return FavoriteServer{}, err
	} else {
		server.CreatedAt = previous.CreatedAt
	}
	if err := service.repository.SaveFavoriteServer(ctx, server); err != nil {
		return FavoriteServer{}, err
	}
	if service.events != nil {
		service.events.Publish("favorite-server:updated", server)
	}
	return server, nil
}

// Delete removes a favorite server and emits the removal event.
func (service *Service) Delete(ctx context.Context, id string) error {
	if err := service.gate.Begin(); err != nil {
		return err
	}
	defer service.gate.End()
	if err := service.repository.DeleteFavoriteServer(ctx, id); err != nil {
		return err
	}
	if service.events != nil {
		service.events.Publish("favorite-server:removed", map[string]string{"id": id})
	}
	return nil
}

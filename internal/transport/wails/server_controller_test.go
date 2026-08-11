package wails

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/app"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/instances"
	"github.com/waxlight/waxlight-launcher/internal/mutations"
	"github.com/waxlight/waxlight-launcher/internal/servers"
)

type favoriteRepository struct {
	items []servers.FavoriteServer
	err   error
}

func (repository *favoriteRepository) ListFavoriteServers(context.Context) ([]servers.FavoriteServer, error) {
	return repository.items, repository.err
}

func (repository *favoriteRepository) GetFavoriteServer(_ context.Context, id string) (servers.FavoriteServer, error) {
	if repository.err != nil {
		return servers.FavoriteServer{}, repository.err
	}
	for _, server := range repository.items {
		if server.ID == id {
			return server, nil
		}
	}
	return servers.FavoriteServer{}, domain.NewError(domain.ErrServerNotFound, "Favorite server not found")
}

func (repository *favoriteRepository) SaveFavoriteServer(_ context.Context, server servers.FavoriteServer) error {
	if repository.err != nil {
		return repository.err
	}
	for index, existing := range repository.items {
		if existing.ID == server.ID {
			repository.items[index] = server
			return nil
		}
	}
	repository.items = append(repository.items, server)
	return nil
}

func (repository *favoriteRepository) DeleteFavoriteServer(_ context.Context, id string) error {
	if repository.err != nil {
		return repository.err
	}
	for index, server := range repository.items {
		if server.ID == id {
			repository.items = append(repository.items[:index], repository.items[index+1:]...)
			return nil
		}
	}
	return domain.NewError(domain.ErrServerNotFound, "Favorite server not found")
}

type instanceReader struct{}

func (instanceReader) GetInstance(context.Context, string) (instances.Instance, error) {
	return instances.Instance{ID: "instance"}, nil
}

func newServerController() (*ServerController, *favoriteRepository) {
	repository := &favoriteRepository{}
	lifecycle := app.NewLifecycle()
	lifecycle.Startup(context.Background())
	favorites := servers.NewService(
		repository,
		instanceReader{},
		&mutations.Gate{},
		nil,
		func() time.Time { return time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC) },
		func() string { return "server-id" },
	)
	catalog := servers.NewCatalogService(catalogStub{})
	return NewServerController(favorites, catalog, lifecycle), repository
}

type catalogStub struct{}

func (catalogStub) List(context.Context) ([]servers.PublicServer, error) {
	return []servers.PublicServer{{
		Name: "Example", Address: "example.org:42420", Description: "A friendly server.",
		Players: 12, ModCount: 7, Joinable: true,
	}}, nil
}

func TestServerControllerListsFavoriteServers(t *testing.T) {
	controller, repository := newServerController()
	repository.items = []servers.FavoriteServer{{ID: "one", Name: "Cozy", Address: "example.org:42420"}}

	result, err := controller.ListFavoriteServers()
	if err != nil || len(result) != 1 || result[0].ID != "one" || result[0].Name != "Cozy" {
		t.Fatalf("ListFavoriteServers() = %+v, %v", result, err)
	}
}

func TestServerControllerListsPublicServers(t *testing.T) {
	controller, _ := newServerController()

	result, err := controller.ListPublicServers()
	if err != nil || len(result) != 1 {
		t.Fatalf("ListPublicServers() = %+v, %v", result, err)
	}
	if result[0].Name != "Example" || result[0].Address != "example.org:42420" || result[0].Players != 12 {
		t.Fatalf("unexpected public server DTO: %+v", result[0])
	}
	if result[0].AccessRestricted {
		t.Fatal("public server DTO access restriction mismatch")
	}
}

func TestServerControllerSavesAndDeletesFavoriteServers(t *testing.T) {
	controller, repository := newServerController()

	saved, err := controller.SaveFavoriteServer(SaveFavoriteServerRequest{Name: "Whitelist server"})
	if err != nil || saved.ID != "server-id" || saved.Address != "" {
		t.Fatalf("SaveFavoriteServer() = %+v, %v", saved, err)
	}
	if err := controller.DeleteFavoriteServer(saved.ID); err != nil {
		t.Fatal(err)
	}
	if len(repository.items) != 0 {
		t.Fatalf("server was not deleted: %#v", repository.items)
	}
}

func TestServerControllerDTORoundTrip(t *testing.T) {
	instanceID := "instance"
	favorite := servers.FavoriteServer{ID: "one", Name: "Cozy", Address: "example.org:42420", InstanceID: &instanceID}
	dto := favoriteServerDTO(favorite)
	if dto.ID != "one" || dto.Name != "Cozy" || dto.Address != "example.org:42420" || dto.InstanceID == nil || *dto.InstanceID != "instance" {
		t.Fatalf("unexpected favorite server DTO: %+v", dto)
	}
	public := servers.PublicServer{
		Name: "Private", Address: "example.org:42421", Description: "Restricted",
		Players: 4, ModCount: 2, RequiresWhitelist: true, PasswordProtected: true, Joinable: false,
	}
	publicDTO := publicServerDTO(public)
	if publicDTO.AccessRestricted != true || publicDTO.RequiresWhitelist != true || publicDTO.Joinable {
		t.Fatalf("unexpected public server DTO: %+v", publicDTO)
	}
	if !reflect.DeepEqual(publicDTO, PublicServerDTO{
		Name: "Private", Address: "example.org:42421", Description: "Restricted",
		Players: 4, ModCount: 2, RequiresWhitelist: true, AccessRestricted: true, Joinable: false,
	}) {
		t.Fatalf("public server DTO mismatch: %+v", publicDTO)
	}
}

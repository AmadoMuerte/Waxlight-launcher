package servers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/instances"
)

type repository struct {
	items []FavoriteServer
	err   error
}

func (repository *repository) ListFavoriteServers(context.Context) ([]FavoriteServer, error) {
	return repository.items, repository.err
}

func (repository *repository) GetFavoriteServer(_ context.Context, id string) (FavoriteServer, error) {
	if repository.err != nil {
		return FavoriteServer{}, repository.err
	}
	for _, server := range repository.items {
		if server.ID == id {
			return server, nil
		}
	}
	return FavoriteServer{}, domain.NewError(domain.ErrServerNotFound, "Favorite server not found")
}

func (repository *repository) SaveFavoriteServer(_ context.Context, server FavoriteServer) error {
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

func (repository *repository) DeleteFavoriteServer(_ context.Context, id string) error {
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

type instanceReader struct {
	err error
}

func (reader instanceReader) GetInstance(context.Context, string) (instances.Instance, error) {
	if reader.err != nil {
		return instances.Instance{}, reader.err
	}
	return instances.Instance{ID: "instance"}, nil
}

type blockedGate struct {
	err error
}

func (gate blockedGate) Begin() error { return gate.err }
func (blockedGate) End()              {}

type recordingPublisher struct {
	events []event
}

type event struct {
	name    string
	payload any
}

func (publisher *recordingPublisher) Publish(name string, payload any) {
	publisher.events = append(publisher.events, event{name: name, payload: payload})
}

func newTestService(repository *repository) (*Service, *recordingPublisher) {
	publisher := &recordingPublisher{}
	service := NewService(
		repository,
		instanceReader{},
		blockedGate{},
		publisher,
		func() time.Time { return time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC) },
		func() string { return "server-id" },
	)
	return service, publisher
}

func TestServiceListsFavoriteServers(t *testing.T) {
	repository := &repository{items: []FavoriteServer{{ID: "one", Name: "Cozy"}}}
	service, _ := newTestService(repository)

	servers, err := service.List(context.Background())
	if err != nil || len(servers) != 1 || servers[0].Name != "Cozy" {
		t.Fatalf("List() = %+v, %v", servers, err)
	}
	server, err := service.Get(context.Background(), "one")
	if err != nil || server.ID != "one" {
		t.Fatalf("Get() = %+v, %v", server, err)
	}
}

func TestServiceSaveCreatesServerWithGeneratedIDAndEvent(t *testing.T) {
	repository := &repository{}
	service, publisher := newTestService(repository)

	server, err := service.Save(context.Background(), SaveInput{Name: "Whitelist server"})
	if err != nil {
		t.Fatal(err)
	}
	if server.ID != "server-id" || server.Address != "" || server.Name != "Whitelist server" {
		t.Fatalf("unexpected saved server: %#v", server)
	}
	if server.CreatedAt.IsZero() || server.UpdatedAt.IsZero() {
		t.Fatalf("timestamps were not set: %#v", server)
	}
	if len(publisher.events) != 1 || publisher.events[0].name != "favorite-server:updated" {
		t.Fatalf("unexpected events: %#v", publisher.events)
	}
}

func TestServiceSaveTrimsNameAndAddress(t *testing.T) {
	repository := &repository{}
	service, _ := newTestService(repository)

	server, err := service.Save(context.Background(), SaveInput{Name: "  Cozy server  ", Address: "  example.org:42420  "})
	if err != nil {
		t.Fatal(err)
	}
	if server.Name != "Cozy server" || server.Address != "example.org:42420" {
		t.Fatalf("input was not trimmed: %#v", server)
	}
}

func TestServiceSaveRejectsInvalidInput(t *testing.T) {
	service, _ := newTestService(&repository{})
	cases := []SaveInput{
		{Name: "", Address: "example.org:42420"},
		{Name: "Long name", Address: "example.org:42420 with spaces"},
		{Name: "Long name", Address: "example.org:\t42420"},
	}
	for _, input := range cases {
		if _, err := service.Save(context.Background(), input); err == nil {
			t.Fatalf("Save(%#v) succeeded, want validation error", input)
		}
	}
}

func TestServiceSaveRejectsUnknownInstance(t *testing.T) {
	repository := &repository{}
	service := NewService(
		repository,
		instanceReader{err: domain.NewError(instances.ErrInstanceNotFound, "Instance not found")},
		blockedGate{},
		nil,
		func() time.Time { return time.Now() },
		func() string { return "server-id" },
	)
	instanceID := "missing"
	if _, err := service.Save(context.Background(), SaveInput{Name: "Cozy", Address: "example.org:42420", InstanceID: &instanceID}); err == nil {
		t.Fatal("Save() succeeded, want instance validation error")
	}
}

func TestServiceSaveUpdatesExistingServerKeepingCreatedAt(t *testing.T) {
	created := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	repository := &repository{items: []FavoriteServer{{ID: "server-id", Name: "Old", CreatedAt: created}}}
	service, publisher := newTestService(repository)

	server, err := service.Save(context.Background(), SaveInput{ID: "server-id", Name: "New", Address: "example.org:42420"})
	if err != nil {
		t.Fatal(err)
	}
	if server.Name != "New" || !server.CreatedAt.Equal(created) {
		t.Fatalf("created timestamp was not preserved: %#v", server)
	}
	if len(publisher.events) != 1 || publisher.events[0].name != "favorite-server:updated" {
		t.Fatalf("unexpected events: %#v", publisher.events)
	}
}

func TestServiceSavePropagatesRepositoryErrors(t *testing.T) {
	want := errors.New("database unavailable")
	repository := &repository{err: want}
	service, _ := newTestService(repository)

	if _, err := service.Save(context.Background(), SaveInput{Name: "Cozy", Address: "example.org:42420"}); !errors.Is(err, want) {
		t.Fatalf("Save() error = %v, want %v", err, want)
	}
}

func TestServiceDeleteRemovesServerAndEmitsEvent(t *testing.T) {
	repository := &repository{items: []FavoriteServer{{ID: "server-id", Name: "Cozy"}}}
	service, publisher := newTestService(repository)

	if err := service.Delete(context.Background(), "server-id"); err != nil {
		t.Fatal(err)
	}
	if len(repository.items) != 0 {
		t.Fatalf("server was not deleted: %#v", repository.items)
	}
	if len(publisher.events) != 1 || publisher.events[0].name != "favorite-server:removed" {
		t.Fatalf("unexpected events: %#v", publisher.events)
	}
	if payload, ok := publisher.events[0].payload.(map[string]string); !ok || payload["id"] != "server-id" {
		t.Fatalf("unexpected removal payload: %#v", publisher.events[0].payload)
	}
}

func TestServiceDeletePropagatesRepositoryErrors(t *testing.T) {
	want := errors.New("database unavailable")
	repository := &repository{err: want}
	service, _ := newTestService(repository)

	if err := service.Delete(context.Background(), "server-id"); !errors.Is(err, want) {
		t.Fatalf("Delete() error = %v, want %v", err, want)
	}
}

func TestServiceHonorsMutationGate(t *testing.T) {
	repository := &repository{}
	service := NewService(
		repository,
		instanceReader{},
		blockedGate{err: domain.NewError(domain.ErrDataFolderBusy, "The data folder is being moved")},
		nil,
		func() time.Time { return time.Now() },
		func() string { return "server-id" },
	)

	if _, err := service.Save(context.Background(), SaveInput{Name: "Cozy", Address: "example.org:42420"}); err == nil {
		t.Fatal("Save() succeeded, want mutation gate error")
	}
	if err := service.Delete(context.Background(), "server-id"); err == nil {
		t.Fatal("Delete() succeeded, want mutation gate error")
	}
}

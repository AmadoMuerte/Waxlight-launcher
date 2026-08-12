package servers

import (
	"context"
	"errors"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/errs"
)

type catalog struct {
	servers []PublicServer
	err     error
}

func (catalog catalog) List(context.Context) ([]PublicServer, error) {
	return catalog.servers, catalog.err
}

func TestCatalogServiceListsPublicServers(t *testing.T) {
	service := NewCatalogService(catalog{servers: []PublicServer{{Name: "Example", Players: 12}}})

	servers, err := service.List(context.Background())
	if err != nil || len(servers) != 1 || servers[0].Name != "Example" {
		t.Fatalf("List() = %+v, %v", servers, err)
	}
}

func TestCatalogServicePropagatesCatalogFailures(t *testing.T) {
	want := errors.New("catalog unavailable")
	service := NewCatalogService(catalog{err: want})

	if _, err := service.List(context.Background()); !errors.Is(err, want) {
		t.Fatalf("List() error = %v, want %v", err, want)
	}
}

func TestCatalogServiceReportsUnavailableWhenNil(t *testing.T) {
	service := NewCatalogService(nil)

	_, err := service.List(context.Background())
	if err == nil {
		t.Fatal("List() succeeded, want unavailable error")
	}
	var appError *errs.AppError
	if !errors.As(err, &appError) || appError.Code != errs.ErrValidation {
		t.Fatalf("List() error = %v, want validation error", err)
	}
}

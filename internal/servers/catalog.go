package servers

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

// CatalogService lists public server listings behind the catalog port. The
// catalog dependency is immutable at construction; a nil catalog keeps the
// historical "unavailable" behavior instead of panicking.
type CatalogService struct {
	catalog Catalog
}

func NewCatalogService(catalog Catalog) *CatalogService {
	return &CatalogService{catalog: catalog}
}

// List returns the public server catalog sorted by player count.
func (service *CatalogService) List(ctx context.Context) ([]PublicServer, error) {
	if service.catalog == nil {
		return nil, domain.NewError(domain.ErrValidation, "Public server catalog is unavailable")
	}
	return service.catalog.List(ctx)
}

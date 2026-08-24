package wails

import (
	"github.com/AmadoMuerte/Waxlight-launcher/internal/servers"
)

// ServerController exposes favorite-server and public-catalog operations to
// the frontend. It stays limited to DTO conversion and feature invocation.
type ServerController struct {
	favorites *servers.Service
	catalog   *servers.CatalogService
	lifecycle lifecycle
}

func NewServerController(
	favorites *servers.Service,
	catalog *servers.CatalogService,
	lifecycle lifecycle,
) *ServerController {
	return &ServerController{favorites: favorites, catalog: catalog, lifecycle: lifecycle}
}

type SaveFavoriteServerRequest struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Address    string  `json:"address"`
	InstanceID *string `json:"instanceId,omitempty"`
}

type FavoriteServerDTO struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Address    string  `json:"address"`
	InstanceID *string `json:"instanceId,omitempty"`
}

type PublicServerDTO struct {
	Name              string `json:"name"`
	Address           string `json:"address"`
	Description       string `json:"description"`
	Players           int    `json:"players"`
	ModCount          int    `json:"modCount"`
	RequiresWhitelist bool   `json:"requiresWhitelist"`
	AccessRestricted  bool   `json:"accessRestricted"`
	Joinable          bool   `json:"joinable"`
}

func (controller *ServerController) ListFavoriteServers() ([]FavoriteServerDTO, error) {
	servers, err := controller.favorites.List(controller.lifecycle.Context())
	if err != nil {
		return nil, err
	}
	result := make([]FavoriteServerDTO, 0, len(servers))
	for _, server := range servers {
		result = append(result, favoriteServerDTO(server))
	}
	return result, nil
}

func (controller *ServerController) ListPublicServers() ([]PublicServerDTO, error) {
	servers, err := controller.catalog.List(controller.lifecycle.Context())
	if err != nil {
		return nil, err
	}
	result := make([]PublicServerDTO, 0, len(servers))
	for _, server := range servers {
		result = append(result, publicServerDTO(server))
	}
	return result, nil
}

func (controller *ServerController) SaveFavoriteServer(request SaveFavoriteServerRequest) (FavoriteServerDTO, error) {
	server, err := controller.favorites.Save(controller.lifecycle.Context(), servers.SaveInput{
		ID: request.ID, Name: request.Name, Address: request.Address, InstanceID: request.InstanceID,
	})
	return favoriteServerDTO(server), err
}

func (controller *ServerController) DeleteFavoriteServer(id string) error {
	return controller.favorites.Delete(controller.lifecycle.Context(), id)
}

func favoriteServerDTO(server servers.FavoriteServer) FavoriteServerDTO {
	return FavoriteServerDTO{ID: server.ID, Name: server.Name, Address: server.Address, InstanceID: server.InstanceID}
}

func publicServerDTO(server servers.PublicServer) PublicServerDTO {
	return PublicServerDTO{
		Name: server.Name, Address: server.Address, Description: server.Description,
		Players: server.Players, ModCount: server.ModCount,
		RequiresWhitelist: server.RequiresWhitelist, AccessRestricted: server.PasswordProtected,
		Joinable: server.Joinable,
	}
}

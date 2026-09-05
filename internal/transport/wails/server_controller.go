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

// SaveFavoriteServerRequest defines a favorite server and its optional instance association.
type SaveFavoriteServerRequest struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Address    string  `json:"address"`
	InstanceID *string `json:"instanceId,omitempty"`
}

// FavoriteServerDTO describes a saved multiplayer destination for the frontend.
type FavoriteServerDTO struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Address    string  `json:"address"`
	InstanceID *string `json:"instanceId,omitempty"`
}

// PublicServerDTO describes a catalog server and whether the current client can join it.
type PublicServerDTO struct {
	ID                string         `json:"id"`
	URL               string         `json:"url"`
	Name              string         `json:"name"`
	Address           string         `json:"address"`
	Description       string         `json:"description"`
	FullDescription   string         `json:"fullDescription"`
	DescriptionHTML   string         `json:"descriptionHtml"`
	ImageURL          string         `json:"imageUrl"`
	BannerURL         string         `json:"bannerUrl"`
	GameVersion       string         `json:"gameVersion"`
	Players           int            `json:"players"`
	MaxPlayers        int            `json:"maxPlayers"`
	ModCount          int            `json:"modCount"`
	Location          string         `json:"location"`
	Languages         []string       `json:"languages"`
	Operator          string         `json:"operator"`
	OperatorURL       string         `json:"operatorUrl"`
	Modified          bool           `json:"modified"`
	RequiresWhitelist bool           `json:"requiresWhitelist"`
	AccessRestricted  bool           `json:"accessRestricted"`
	Joinable          bool           `json:"joinable"`
	Mods              []ServerModDTO `json:"mods"`
}

// ServerModDTO describes a mod reported by a server detail page.
type ServerModDTO struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`
}

// ListFavoriteServers returns the multiplayer servers saved by the user.
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

// ListPublicServers returns the current public server catalog with joinability information.
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

// GetPublicServer returns the full public details for one catalog server.
func (controller *ServerController) GetPublicServer(id string) (PublicServerDTO, error) {
	server, err := controller.catalog.Get(controller.lifecycle.Context(), id)
	if err != nil {
		return PublicServerDTO{}, err
	}
	return publicServerDTO(server), nil
}

// SaveFavoriteServer creates or updates a favorite server and its optional instance association.
func (controller *ServerController) SaveFavoriteServer(request SaveFavoriteServerRequest) (FavoriteServerDTO, error) {
	server, err := controller.favorites.Save(controller.lifecycle.Context(), servers.SaveInput{
		ID: request.ID, Name: request.Name, Address: request.Address, InstanceID: request.InstanceID,
	})
	return favoriteServerDTO(server), err
}

// DeleteFavoriteServer removes a server from the user's favorites.
func (controller *ServerController) DeleteFavoriteServer(id string) error {
	return controller.favorites.Delete(controller.lifecycle.Context(), id)
}

func favoriteServerDTO(server servers.FavoriteServer) FavoriteServerDTO {
	return FavoriteServerDTO{ID: server.ID, Name: server.Name, Address: server.Address, InstanceID: server.InstanceID}
}

func publicServerDTO(server servers.PublicServer) PublicServerDTO {
	return PublicServerDTO{
		ID:   server.ID,
		URL:  server.URL,
		Name: server.Name, Address: server.Address, Description: server.Description,
		FullDescription: server.FullDescription, DescriptionHTML: server.DescriptionHTML,
		ImageURL: server.ImageURL, BannerURL: server.BannerURL, GameVersion: server.GameVersion,
		Players: server.Players, MaxPlayers: server.MaxPlayers, ModCount: server.ModCount,
		Location: server.Location, Languages: append([]string(nil), server.Languages...),
		Operator: server.Operator, OperatorURL: server.OperatorURL, Modified: server.Modified,
		RequiresWhitelist: server.RequiresWhitelist, AccessRestricted: server.PasswordProtected,
		Joinable: server.Joinable, Mods: serverModsDTO(server.Mods),
	}
}

func serverModsDTO(items []servers.ServerMod) []ServerModDTO {
	if len(items) == 0 {
		return nil
	}
	result := make([]ServerModDTO, 0, len(items))
	for _, item := range items {
		result = append(result, ServerModDTO{Name: item.Name, Version: item.Version, URL: item.URL})
	}
	return result
}

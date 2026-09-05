// Package servercatalog maps public Vintage Story catalog listings into
// Waxlight's domain model.
package servercatalog

import (
	"context"
	"net/http"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/servers"
	vsgservers "github.com/AmadoMuerte/vintagestory-go/servers"
)

type Client struct {
	client *vsgservers.Client
}

func NewClient(httpClient *http.Client) *Client {
	return &Client{client: vsgservers.NewClient(httpClient)}
}

func NewClientWithURL(httpClient *http.Client, endpoint string) *Client {
	return &Client{client: vsgservers.NewClientWithURL(httpClient, endpoint)}
}

func (client *Client) List(ctx context.Context) ([]servers.PublicServer, error) {
	listings, err := client.client.List(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		errs.LogFailure("public server catalog request failed", err)
		return nil, &errs.AppError{Code: errs.ErrServerCatalogUnavailable, Message: "Could not load the public server catalog", Retryable: true, Cause: err}
	}
	result := make([]servers.PublicServer, 0, len(listings))
	for _, server := range listings {
		result = append(result, mapPublicServer(server))
	}
	return result, nil
}

func (client *Client) Get(ctx context.Context, id string) (servers.PublicServer, error) {
	server, err := client.client.Get(ctx, id)
	if err != nil {
		if ctx.Err() != nil {
			return servers.PublicServer{}, ctx.Err()
		}
		errs.LogFailure("public server detail request failed", err)
		return servers.PublicServer{}, &errs.AppError{Code: errs.ErrServerCatalogUnavailable, Message: "Could not load public server details", Retryable: true, Cause: err}
	}
	return mapPublicServer(server), nil
}

func mapPublicServer(server vsgservers.Server) servers.PublicServer {
	return servers.PublicServer{
		ID:                server.ID,
		URL:               server.URL,
		Name:              server.Name,
		Address:           server.Address,
		Description:       server.Description,
		FullDescription:   server.FullDescription,
		DescriptionHTML:   server.DescriptionHTML,
		ImageURL:          server.ImageURL,
		BannerURL:         server.BannerURL,
		GameVersion:       server.GameVersion,
		Players:           server.Players,
		MaxPlayers:        server.MaxPlayers,
		ModCount:          server.ModCount,
		Location:          server.Location,
		Languages:         append([]string(nil), server.Languages...),
		Operator:          server.Operator,
		OperatorURL:       server.OperatorURL,
		Modified:          server.Modified,
		RequiresWhitelist: server.RequiresWhitelist,
		PasswordProtected: server.PasswordProtected,
		Joinable:          server.Joinable,
		Mods:              mapMods(server.Mods),
	}
}

func mapMods(items []vsgservers.Mod) []servers.ServerMod {
	if len(items) == 0 {
		return nil
	}
	result := make([]servers.ServerMod, 0, len(items))
	for _, item := range items {
		result = append(result, servers.ServerMod{Name: item.Name, Version: item.Version, URL: item.URL})
	}
	return result
}

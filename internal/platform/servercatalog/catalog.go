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

func mapPublicServer(server vsgservers.Server) servers.PublicServer {
	return servers.PublicServer{
		Name:              server.Name,
		Address:           server.Address,
		Description:       server.Description,
		Players:           server.Players,
		ModCount:          server.ModCount,
		RequiresWhitelist: server.RequiresWhitelist,
		PasswordProtected: server.PasswordProtected,
		Joinable:          server.Joinable,
	}
}

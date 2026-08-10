// Package servercatalog maps public Vintage Story catalog listings into
// Waxlight's domain model.
package servercatalog

import (
	"context"
	"net/http"

	"github.com/AmadoMuerte/vintagestory-go/servers"
	"github.com/waxlight/waxlight-launcher/internal/domain"
)

type Client struct {
	client *servers.Client
}

func NewClient(httpClient *http.Client) *Client {
	return &Client{client: servers.NewClient(httpClient)}
}

func NewClientWithURL(httpClient *http.Client, endpoint string) *Client {
	return &Client{client: servers.NewClientWithURL(httpClient, endpoint)}
}

func (client *Client) List(ctx context.Context) ([]domain.PublicServer, error) {
	servers, err := client.client.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.PublicServer, 0, len(servers))
	for _, server := range servers {
		result = append(result, mapPublicServer(server))
	}
	return result, nil
}

func mapPublicServer(server servers.Server) domain.PublicServer {
	return domain.PublicServer{
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

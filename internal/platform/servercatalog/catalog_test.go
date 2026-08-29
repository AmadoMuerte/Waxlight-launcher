package servercatalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	vsgservers "github.com/AmadoMuerte/vintagestory-go/servers"
	"github.com/AmadoMuerte/vintagestory-go/vshttp"
)

func TestClientMapsPublicServerListings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`<div class="server"><b>12 players</b> on <a href="vintagestoryjoin://example.org:42420">Example</a><img title="7 mods installed"><div class="serverdesc">A <strong>friendly</strong> server.</div></div><div class="server"><b>4 players</b> on <abbr title="Whitelisted players only">Private</abbr></div>`))
	}))
	defer server.Close()

	servers, err := NewClientWithURL(server.Client(), server.URL).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}
	if got := servers[0]; got.Name != "Example" || got.Address != "example.org:42420" || got.Players != 12 || got.ModCount != 7 || !got.Joinable || got.Description != "A friendly server." {
		t.Fatalf("unexpected public server: %#v", got)
	}
	if got := servers[1]; got.Name != "Private" || !got.RequiresWhitelist || got.Joinable {
		t.Fatalf("unexpected restricted server: %#v", got)
	}
}

func TestClientMapsCatalogFailureToAppError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "no", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := NewClientWithURL(server.Client(), server.URL).List(context.Background())
	var appErr *errs.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T: %v", err, err)
	}
	if appErr.Code != errs.ErrServerCatalogUnavailable || !appErr.Retryable {
		t.Fatalf("unexpected AppError: %#v", appErr)
	}
	// The library error chain must survive for diagnostics.
	if !errors.Is(err, vsgservers.ErrUnavailable) {
		t.Fatalf("library sentinel lost: %v", err)
	}
	var apiErr *vshttp.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("API details lost: %v", err)
	}
}

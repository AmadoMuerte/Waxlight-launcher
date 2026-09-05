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

func TestClientMapsPublicServerDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/s/42" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte(`<aside><div id="server-info"><img alt="Thumbnail" src="/files/thumb.png"><span>Players:</span><span>12 / 40</span><span>Game Version:</span><span>1.22.7</span><span>Location:</span><span>United States</span><span>Languages:</span><span><span class="tag" title="English">en</span></span><span>Operated By:</span><a href="/u/operator">Owner</a><a href="vintagestoryjoin://example.org:42420">Join</a></div></aside><main class="server" data-sid="42"><h1>Example</h1><img alt="Banner" src="/files/banner.png"><div class="text-section"><p>Full description</p></div><ul><li><a class="external" href="https://mods.vintagestory.at/show/mod/1">Example Mod@1.2.3</a></li></ul></main>`))
	}))
	defer server.Close()

	got, err := NewClientWithURL(server.Client(), server.URL).Get(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "42" || got.Name != "Example" || got.Players != 12 || got.MaxPlayers != 40 || got.GameVersion != "1.22.7" || got.Location != "United States" || got.ImageURL != server.URL+"/files/thumb.png" || got.BannerURL != server.URL+"/files/banner.png" || got.Operator != "Owner" || len(got.Languages) != 1 || got.Languages[0] != "English" || len(got.Mods) != 1 || got.Mods[0].Version != "1.2.3" {
		t.Fatalf("unexpected server details: %#v", got)
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

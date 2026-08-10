package servercatalog

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestParseServers(t *testing.T) {
	root, err := html.Parse(strings.NewReader(`<div class="server"><b>12 players</b> on <a href="vintagestoryjoin://example.org:42420">Example</a><img title="7 mods installed"><div class="serverdesc">A <strong>friendly</strong> server.</div></div><div class="server"><b>4 players</b> on <abbr title="Whitelisted players only">Private</abbr></div>`))
	if err != nil {
		t.Fatal(err)
	}
	servers := parseServers(root)
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

// Package servercatalog reads Vintage Story's public HTML server catalog.
package servercatalog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

const catalogURL = "https://servers.vintagestory.at/"

var (
	playersPattern = regexp.MustCompile(`^(\d+)\s+players?$`)
	modsPattern    = regexp.MustCompile(`^(\d+)\s+mods?\s+installed$`)
)

type Client struct {
	httpClient *http.Client
	mu         sync.Mutex
	servers    []domain.PublicServer
	fetchedAt  time.Time
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{httpClient: httpClient}
}

func (client *Client) List(ctx context.Context) ([]domain.PublicServer, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if time.Since(client.fetchedAt) < 5*time.Minute && client.servers != nil {
		return append([]domain.PublicServer(nil), client.servers...), nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch public server catalog: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("public server catalog returned HTTP %d", response.StatusCode)
	}

	root, err := html.Parse(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("parse public server catalog: %w", err)
	}
	servers := parseServers(root)
	if len(servers) == 0 {
		return nil, fmt.Errorf("public server catalog contained no server listings")
	}
	client.servers = servers
	client.fetchedAt = time.Now()
	return append([]domain.PublicServer(nil), servers...), nil
}

func parseServers(root *html.Node) []domain.PublicServer {
	var servers []domain.PublicServer
	visit(root, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "div" || attribute(node, "class") != "server" {
			return
		}
		server, ok := parseServer(node)
		if ok {
			servers = append(servers, server)
		}
	})
	sort.SliceStable(servers, func(i, j int) bool { return servers[i].Players > servers[j].Players })
	return servers
}

func parseServer(node *html.Node) (domain.PublicServer, bool) {
	server := domain.PublicServer{}
	visit(node, func(current *html.Node) {
		if current.Type != html.ElementNode {
			return
		}
		switch current.Data {
		case "b":
			if matches := playersPattern.FindStringSubmatch(nodeText(current)); len(matches) == 2 {
				server.Players, _ = strconv.Atoi(matches[1])
			}
		case "a":
			if address := strings.TrimPrefix(attribute(current, "href"), "vintagestoryjoin://"); address != attribute(current, "href") {
				server.Name, server.Address, server.Joinable = nodeText(current), address, true
			}
		case "abbr":
			if server.Name == "" {
				server.Name = nodeText(current)
			}
			switch strings.ToLower(attribute(current, "title")) {
			case "whitelisted players only":
				server.RequiresWhitelist = true
			case "password protected":
				server.PasswordProtected = true
			}
		case "img":
			if matches := modsPattern.FindStringSubmatch(attribute(current, "title")); len(matches) == 2 {
				server.ModCount, _ = strconv.Atoi(matches[1])
			}
		case "div":
			if attribute(current, "class") == "serverdesc" {
				server.Description = nodeText(current)
			}
		}
	})
	server.Name = strings.TrimSpace(server.Name)
	server.Address = strings.TrimSpace(server.Address)
	return server, server.Name != ""
}

func visit(node *html.Node, fn func(*html.Node)) {
	fn(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		visit(child, fn)
	}
}

func attribute(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ""
}

func nodeText(node *html.Node) string {
	var values []string
	visit(node, func(current *html.Node) {
		if current.Type == html.TextNode {
			values = append(values, current.Data)
		}
	})
	return strings.Join(strings.Fields(strings.Join(values, " ")), " ")
}

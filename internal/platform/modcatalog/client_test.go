package modcatalog

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestClientMapsFiltersAndSortsCatalog(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/mods" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		return jsonResponse(`{"statuscode":"200","mods":[
			{"modid":1,"downloads":20,"name":"Server Tools","summary":"Admin helpers","author":"Ada","side":"server","type":"mod","tags":["Utility"],"modidstrs":["servertools"],"lastreleased":"2026-08-01 12:00:00"},
			{"modid":2,"downloads":200,"name":"Warm Light","summary":"Soft lamps","author":"Bea","side":"client","type":"mod","tags":["Graphics"],"modidstrs":["warmlight"],"lastreleased":"2026-08-02 12:00:00"},
			{"modid":3,"downloads":999,"name":"External","summary":"skip","author":"Cat","side":"both","type":"externaltool","tags":[],"lastreleased":"2026-08-03 12:00:00"}
		]}`), nil
	})
	client := NewClientWithURL(&http.Client{Transport: transport}, "https://catalog.test")
	result, err := client.Search(context.Background(), mods.ModSearchQuery{
		Text: "ada", Side: mods.ModSideServer, Sort: "downloads", Page: 1, PageSize: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalItems != 1 || result.Items[0].ID != "1" {
		t.Fatalf("unexpected search result: %#v", result)
	}
}

func TestClientListsCatalogWithModIDStrings(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(`{"statuscode":"200","mods":[
			{"modid":51,"downloads":42,"name":"Player Corpse","summary":"Corpse","author":"Ada","side":"both","type":"mod","modidstrs":["playercorpse","oldcorpse"],"lastreleased":"2026-08-01 10:00:00"},
			{"modid":3,"downloads":999,"name":"External","summary":"skip","author":"Cat","side":"both","type":"externaltool","lastreleased":"2026-08-03 12:00:00"}
		]}`), nil
	})
	client := NewClientWithURL(&http.Client{Transport: transport}, "https://catalog.test")
	items, err := client.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected catalog list: %#v", items)
	}
	if len(items[0].ModIDStrings) != 2 || items[0].ModIDStrings[0] != "playercorpse" || items[0].ModIDStrings[1] != "oldcorpse" {
		t.Fatalf("modid strings were not mapped: %#v", items[0].ModIDStrings)
	}
}

func TestClientMapsDetailsWithoutComments(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(`{"statuscode":"200","mod":{
			"modid":51,"name":"Player Corpse","text":"<p>Safe description</p>","author":"Ada","side":"both","type":"mod","downloads":42,
			"created":"2026-01-01 10:00:00","lastreleased":"2026-08-01 10:00:00","tags":["Utility"],"comments":[{"text":"must not map"}],
			"releases":[{"releaseid":7,"mainfile":"https://cdn.test/mod.zip","filename":"mod.zip","tags":["1.21.0"],"modversion":"2.0.0","created":"2026-08-01 10:00:00","changelog":"<p>New</p>"}],
			"screenshots":["https://cdn.test/screen.png"]
		}}`), nil
	})
	client := NewClientWithURL(&http.Client{Transport: transport}, "https://catalog.test")
	details, err := client.Get(context.Background(), "51")
	if err != nil {
		t.Fatal(err)
	}
	if details.LatestVersion != "2.0.0" || len(details.Versions) != 1 || len(details.Screenshots) != 1 {
		t.Fatalf("unexpected details: %#v", details)
	}
	if details.Versions[0].DownloadURL != "https://cdn.test/mod.zip" {
		t.Fatal("download URL was not mapped")
	}
}

func TestClientListsTagsWithCounts(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/mods" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		return jsonResponse(`{"statuscode":"200","mods":[
			{"modid":1,"downloads":20,"name":"Server Tools","summary":"Admin helpers","author":"Ada","side":"server","type":"mod","tags":["Utility"],"lastreleased":"2026-08-01 12:00:00"},
			{"modid":2,"downloads":200,"name":"Warm Light","summary":"Soft lamps","author":"Bea","side":"client","type":"mod","tags":["graphics"],"lastreleased":"2026-08-02 12:00:00"},
			{"modid":3,"downloads":999,"name":"Glow Lamps","summary":"More lamps","author":"Bea","side":"both","type":"mod","tags":["Graphics","Utility"],"lastreleased":"2026-08-03 12:00:00"}
		]}`), nil
	})
	client := NewClientWithURL(&http.Client{Transport: transport}, "https://catalog.test")
	tags, err := client.ListTags(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Fatalf("unexpected tags: %#v", tags)
	}
	if tags[0].Name != "graphics" || tags[0].Count != 2 {
		t.Fatalf("unexpected first tag: %#v", tags[0])
	}
	if tags[1].Name != "Utility" || tags[1].Count != 2 {
		t.Fatalf("unexpected second tag: %#v", tags[1])
	}
}

func TestSearchPreservesSummaryWhenFilteringByGameVersion(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/mods":
			return jsonResponse(`{"statuscode":"200","mods":[
				{"modid":51,"downloads":42,"name":"Player Corpse","summary":"Creates a recoverable corpse after death.","author":"Ada","side":"both","type":"mod","lastreleased":"2026-08-01 10:00:00"}
			]}`), nil
		case "/mod/51":
			return jsonResponse(`{"statuscode":"200","mod":{
				"modid":51,"name":"Player Corpse","text":"<p>Full description</p>","author":"Ada","side":"both","type":"mod","downloads":42,
				"lastreleased":"2026-08-01 10:00:00","releases":[{"releaseid":7,"tags":["1.22.6"],"modversion":"2.0.0"}]
			}}`), nil
		default:
			t.Fatalf("unexpected path %s", request.URL.Path)
			return nil, nil
		}
	})
	client := NewClientWithURL(&http.Client{Transport: transport}, "https://catalog.test")
	result, err := client.Search(context.Background(), mods.ModSearchQuery{
		GameVersion: "1.22.6", Page: 1, PageSize: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("unexpected search result: %#v", result)
	}
	if result.Items[0].Summary != "Creates a recoverable corpse after death." {
		t.Fatalf("summary was lost after compatibility enrichment: %q", result.Items[0].Summary)
	}
}

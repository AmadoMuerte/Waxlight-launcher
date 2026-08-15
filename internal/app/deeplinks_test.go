package app

import (
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/events"
	"github.com/waxlight/waxlight-launcher/internal/platform/deeplink"
)

func TestDeepLinksQueuesColdStartThenDispatches(t *testing.T) {
	var published []deeplink.Target
	links := NewDeepLinks(events.PublisherFunc(func(name string, payload any) {
		if name != "deeplink:open" {
			t.Errorf("event = %q", name)
			return
		}
		published = append(published, payload.(deeplink.Target))
	}))

	links.ReceiveArgs([]string{"waxlight://mod/optimum"})
	if len(published) != 0 {
		t.Fatal("cold-start link was dispatched before the frontend was ready")
	}
	pending := links.Consume()
	if len(pending) != 1 || pending[0].ModID != "optimum" {
		t.Fatalf("pending = %#v", pending)
	}

	links.ReceiveArgs([]string{"waxlight://mod/betterruins"})
	if len(published) != 1 || published[0].ModID != "betterruins" {
		t.Fatalf("published = %#v", published)
	}
}

package errs

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/AmadoMuerte/vintagestory-go/vshttp"
)

func TestFailureAttrsExtractsAPIDetails(t *testing.T) {
	cause := errors.New("connection reset")
	err := fmt.Errorf("wrapper: %w", &vshttp.APIError{
		Operation:  "moddb: list catalog",
		Method:     "GET",
		Endpoint:   "https://mods.vintagestory.at/api/mods",
		StatusCode: 503,
		Kind:       vshttp.KindServerError,
		Retryable:  true,
		RetryAfter: 2 * time.Second,
		Cause:      cause,
	})
	attrs := FailureAttrs(err)
	got := map[string]any{}
	for i := 0; i < len(attrs); i += 2 {
		got[attrs[i].(string)] = attrs[i+1]
	}
	if got["kind"] != string(vshttp.KindServerError) {
		t.Fatalf("kind: %v", got["kind"])
	}
	if got["status"] != 503 {
		t.Fatalf("status: %v", got["status"])
	}
	if got["endpoint"] != "https://mods.vintagestory.at/api/mods" {
		t.Fatalf("endpoint: %v", got["endpoint"])
	}
	if got["retryable"] != true || got["retry_after"] != "2s" {
		t.Fatalf("retry attrs: %v", got)
	}
	if !errors.Is(got["error"].(error), cause) {
		t.Fatalf("full chain not attached: %v", got["error"])
	}
}

func TestFailureAttrsFallsBackToErrorOnly(t *testing.T) {
	attrs := FailureAttrs(errors.New("plain failure"))
	if len(attrs) != 2 || attrs[0] != "error" {
		t.Fatalf("%v", attrs)
	}
}

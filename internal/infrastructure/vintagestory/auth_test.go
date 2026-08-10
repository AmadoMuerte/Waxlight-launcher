package vintagestory

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	vsauth "github.com/AmadoMuerte/vintagestory-go/auth"
	"github.com/waxlight/waxlight-launcher/internal/application"
)

func TestAuthClientMapsTOTPChallengeAndErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"valid":0,"reason":"requiretotpcode","prelogintoken":"pre"}`)
	}))
	defer server.Close()

	client := NewAuthClientWithURLs(server.Client(), server.URL, server.URL)
	_, challenge, err := client.Login(context.Background(), "player@example.com", "password", "", "")
	if !errors.Is(err, application.ErrTOTPRequired) {
		t.Fatalf("expected TOTP error, got %v", err)
	}
	if challenge == nil || challenge.PreLoginToken != "pre" {
		t.Fatalf("unexpected challenge: %#v", challenge)
	}
}

func TestMapAuthErrorPreservesApplicationCategories(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want error
	}{
		{name: "credentials", err: vsauth.ErrInvalidCredentials, want: application.ErrInvalidCredentials},
		{name: "ip changed", err: vsauth.ErrIPChanged, want: application.ErrIPChanged},
		{name: "blocked", err: vsauth.ErrTemporarilyBlocked, want: application.ErrTemporarilyBlocked},
		{name: "invalid response", err: vsauth.ErrInvalidAuthReply, want: application.ErrInvalidAuthReply},
		{name: "network", err: vsauth.ErrNetwork, want: application.ErrAuthNetwork},
		{name: "server", err: vsauth.ErrServer, want: application.ErrAuthServer},
		{name: "unknown", err: errors.New("unknown"), want: application.ErrUnknownAuth},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !errors.Is(mapAuthError(test.err), test.want) {
				t.Fatalf("unexpected mapped error: %v", mapAuthError(test.err))
			}
		})
	}
}

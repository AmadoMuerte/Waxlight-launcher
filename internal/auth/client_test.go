package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoginScenarios(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError error
		wantTOTP  bool
	}{
		{name: "success", body: `{"valid":1,"sessionkey":"key","sessionsignature":"sig","uid":"uid","playername":"Ada"}`},
		{name: "invalid credentials", body: `{"valid":0,"reason":"invalidemailorpassword"}`, wantError: ErrInvalidCredentials},
		{name: "ip changed", body: `{"valid":0,"reason":"ipchanged"}`, wantError: ErrIPChanged},
		{name: "temporarily blocked", body: `{"valid":0,"reason":"temporarilyblocked"}`, wantError: ErrTemporarilyBlocked},
		{name: "totp", body: `{"valid":0,"reason":"requiretotpcode","prelogintoken":"pre"}`, wantError: ErrTOTPRequired, wantTOTP: true},
		{name: "totp missing token", body: `{"valid":0,"reason":"requiretotpcode"}`, wantError: ErrInvalidAuthReply},
		{name: "unknown reason", body: `{"valid":0,"reason":"surprise"}`, wantError: ErrUnknown},
		{name: "invalid json", body: `{`, wantError: ErrInvalidAuthReply},
		{name: "success missing key", body: `{"valid":1,"sessionsignature":"sig","uid":"uid","playername":"Ada"}`, wantError: ErrInvalidAuthReply},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodPost {
					t.Fatalf("unexpected method %s", request.Method)
				}
				if err := request.ParseForm(); err != nil {
					t.Fatal(err)
				}
				if request.Form.Get("email") != "ada@example.com" || request.Form.Get("password") != "secret" {
					t.Fatal("login form fields were not sent")
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(writer, test.body)
			}))
			defer server.Close()

			client := NewClientWithURLs(server.Client(), server.URL, server.URL)
			session, challenge, err := client.Login(context.Background(), "ada@example.com", "secret", "", "")
			if !errors.Is(err, test.wantError) {
				t.Fatalf("got error %v, want %v", err, test.wantError)
			}
			if test.wantTOTP != (challenge != nil) {
				t.Fatalf("unexpected challenge: %#v", challenge)
			}
			if test.wantError == nil && (session.UID != "uid" || session.PlayerName != "Ada") {
				t.Fatalf("unexpected session: %#v", session)
			}
		})
	}
}

func TestCompleteTOTPSendsFlowFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("totpcode") != "123456" || request.Form.Get("prelogintoken") != "pre" {
			t.Fatalf("unexpected form: %v", request.Form)
		}
		_, _ = io.WriteString(writer, `{"valid":1,"sessionkey":"key","sessionsignature":"sig","uid":"uid","playername":"Ada"}`)
	}))
	defer server.Close()

	client := NewClientWithURLs(server.Client(), server.URL, server.URL)
	_, _, err := client.Login(context.Background(), "ada@example.com", "secret", "123456", "pre")
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoginHTTPAndTimeoutErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "unavailable", http.StatusInternalServerError)
	}))
	client := NewClientWithURLs(server.Client(), server.URL, server.URL)
	_, _, err := client.Login(context.Background(), "a@b.test", "secret", "", "")
	server.Close()
	if !errors.Is(err, ErrServer) {
		t.Fatalf("expected server error, got %v", err)
	}

	timeoutServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		time.Sleep(100 * time.Millisecond)
		_, _ = io.WriteString(writer, `{"valid":1}`)
	}))
	defer timeoutServer.Close()
	timeoutClient := NewClientWithURLs(&http.Client{Timeout: 10 * time.Millisecond}, timeoutServer.URL, timeoutServer.URL)
	_, _, err = timeoutClient.Login(context.Background(), "a@b.test", "secret", "", "")
	if !errors.Is(err, ErrNetwork) {
		t.Fatalf("expected network error, got %v", err)
	}
}

func TestValidateUsesPOSTForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", request.Method)
		}
		if request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("unexpected content type %q", request.Header.Get("Content-Type"))
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("uid") != "player-uid" || request.Form.Get("sessionkey") != "session-key" {
			t.Fatalf("unexpected form: %v", request.Form)
		}
		_, _ = io.WriteString(writer, `{"valid":1}`)
	}))
	defer server.Close()

	client := NewClientWithURLs(server.Client(), server.URL, server.URL)
	valid, err := client.Validate(context.Background(), "player-uid", "session-key")
	if err != nil || !valid {
		t.Fatalf("validate returned %v, %v", valid, err)
	}
}

func TestValidateFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want bool
		err  error
	}{
		{name: "invalid", body: `{"valid":0}`, want: false},
		{name: "invalid json", body: `{`, err: ErrInvalidAuthReply},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(writer, test.body)
			}))
			defer server.Close()
			client := NewClientWithURLs(server.Client(), server.URL, server.URL)
			valid, err := client.Validate(context.Background(), "uid", "key")
			if valid != test.want || !errors.Is(err, test.err) {
				t.Fatalf("got (%v, %v), want (%v, %v)", valid, err, test.want, test.err)
			}
		})
	}
}

func TestValidateNetworkFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(writer, `{"valid":1}`)
	}))
	client := NewClientWithURLs(server.Client(), server.URL, server.URL)
	server.Close()
	if _, err := client.Validate(context.Background(), "uid", "key"); !errors.Is(err, ErrNetwork) {
		t.Fatalf("expected network error, got %v", err)
	}
}

func TestErrorsDoNotContainSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{`)
	}))
	defer server.Close()
	client := NewClientWithURLs(server.Client(), server.URL, server.URL)
	_, _, err := client.Login(context.Background(), "a@b.test", "do-not-leak", "", "")
	if err == nil || strings.Contains(err.Error(), "do-not-leak") {
		t.Fatalf("secret leaked in error: %v", err)
	}
}

func TestClientRejectsRedirectOversizedAndInvalidContent(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"redirect": func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, "https://different-origin.example/login", http.StatusFound)
		},
		"oversized": func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, strings.Repeat(" ", maxResponseBytes)+`{"valid":0}`)
		},
		"invalid-content": func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(writer, `{"valid":0}`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			defer server.Close()
			client := NewClientWithURLs(server.Client(), server.URL, server.URL)
			if _, _, err := client.Login(context.Background(), "a@b.test", "secret", "", ""); err == nil {
				t.Fatal("expected hardened response rejection")
			}
		})
	}
}

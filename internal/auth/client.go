package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	VintageStoryLoginURL    = "https://auth3.vintagestory.at/v2/gamelogin"
	VintageStoryValidateURL = "https://auth3.vintagestory.at/clientvalidate"
)

const maxResponseBytes = 1 << 20

type Client struct {
	httpClient  *http.Client
	loginURL    string
	validateURL string
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 20 * time.Second,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				ExpectContinueTimeout: time.Second,
			},
		}
	}
	clone := *httpClient
	clone.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{
		httpClient:  &clone,
		loginURL:    VintageStoryLoginURL,
		validateURL: VintageStoryValidateURL,
	}
}

// NewClientWithURLs is intended for tests and controlled integrations.
func NewClientWithURLs(httpClient *http.Client, loginURL, validateURL string) *Client {
	client := NewClient(httpClient)
	client.loginURL = loginURL
	client.validateURL = validateURL
	return client
}

func (c *Client) Login(
	ctx context.Context,
	email string,
	password string,
	totpCode string,
	preLoginToken string,
) (Session, *TOTPChallenge, error) {
	form := url.Values{}
	form.Set("email", email)
	form.Set("password", password)
	if totpCode != "" || preLoginToken != "" {
		form.Set("totpcode", totpCode)
		form.Set("prelogintoken", preLoginToken)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.loginURL,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return Session{}, nil, fmt.Errorf("create login request: %w", ErrNetwork)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var response loginResponse
	if err := c.doJSON(request, &response); err != nil {
		return Session{}, nil, err
	}

	if bool(response.Valid) {
		if response.SessionKey == nil || *response.SessionKey == "" ||
			response.SessionSignature == nil || *response.SessionSignature == "" ||
			response.UID == nil || *response.UID == "" ||
			response.PlayerName == nil || *response.PlayerName == "" {
			return Session{}, nil, ErrInvalidAuthReply
		}

		return Session{
			SessionKey:       *response.SessionKey,
			SessionSignature: *response.SessionSignature,
			UID:              *response.UID,
			PlayerName:       *response.PlayerName,
		}, nil, nil
	}

	reason := ""
	if response.Reason != nil {
		reason = strings.ToLower(strings.TrimSpace(*response.Reason))
	}
	switch reason {
	case "requiretotpcode":
		if response.PreLoginToken == nil || *response.PreLoginToken == "" {
			return Session{}, nil, ErrInvalidAuthReply
		}
		slog.Info("auth server requested a TOTP challenge")
		return Session{}, &TOTPChallenge{PreLoginToken: *response.PreLoginToken}, ErrTOTPRequired
	case "invalidemailorpassword", "invalidtotpcode":
		slog.Warn("auth login rejected with invalid credentials")
		return Session{}, nil, ErrInvalidCredentials
	case "ipchanged":
		slog.Warn("auth login rejected because the session IP changed")
		return Session{}, nil, ErrIPChanged
	case "temporarilyblocked":
		slog.Warn("auth login temporarily blocked")
		return Session{}, nil, ErrTemporarilyBlocked
	default:
		slog.Warn("auth login returned an unknown response")
		return Session{}, nil, ErrUnknown
	}
}

// Validate uses POST form data. This is the format currently accepted by the
// Vintage Story endpoint; GET parameters were rejected by the live endpoint.
func (c *Client) Validate(ctx context.Context, uid, sessionKey string) (bool, error) {
	form := url.Values{}
	form.Set("uid", uid)
	form.Set("sessionkey", sessionKey)
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.validateURL,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return false, fmt.Errorf("create validation request: %w", ErrNetwork)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var response validateResponse
	if err := c.doJSON(request, &response); err != nil {
		return false, err
	}
	if !bool(response.Valid) {
		slog.Warn("auth session validation was rejected")
	}
	return bool(response.Valid), nil
}

func (c *Client) doJSON(request *http.Request, target any) error {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request failed: %w", ErrNetwork)
	}
	defer response.Body.Close()

	contentType := response.Header.Get("Content-Type")

	contents, err := io.ReadAll(
		io.LimitReader(response.Body, maxResponseBytes+1),
	)
	if err != nil {
		return &InvalidResponseError{
			StatusCode:  response.StatusCode,
			ContentType: contentType,
			BodySize:    0,
			Cause:       ErrInvalidAuthReply,
		}
	}

	if len(contents) > maxResponseBytes {
		return &InvalidResponseError{
			StatusCode:  response.StatusCode,
			ContentType: contentType,
			BodySize:    len(contents),
			Cause:       ErrInvalidAuthReply,
		}
	}

	if response.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf(
			"authentication server returned HTTP %d: %w",
			response.StatusCode,
			ErrServer,
		)
	}

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf(
			"authentication server returned HTTP %d: %w",
			response.StatusCode,
			ErrServer,
		)
	}

	trimmed := bytes.TrimSpace(contents)
	if len(trimmed) == 0 {
		return &InvalidResponseError{
			StatusCode:  response.StatusCode,
			ContentType: contentType,
			BodySize:    0,
			Cause:       ErrInvalidAuthReply,
		}
	}

	if !json.Valid(trimmed) {
		return &InvalidResponseError{
			StatusCode:  response.StatusCode,
			ContentType: contentType,
			BodySize:    len(trimmed),
			Cause:       ErrInvalidAuthReply,
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))

	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf(
			"decode authentication response: %w",
			&InvalidResponseError{
				StatusCode:  response.StatusCode,
				ContentType: contentType,
				BodySize:    len(trimmed),
				Cause:       ErrInvalidAuthReply,
			},
		)
	}

	var trailing any
	err = decoder.Decode(&trailing)

	switch {
	case errors.Is(err, io.EOF):
		return nil

	case err != nil:
		return fmt.Errorf(
			"decode trailing authentication response: %w",
			&InvalidResponseError{
				StatusCode:  response.StatusCode,
				ContentType: contentType,
				BodySize:    len(trimmed),
				Cause:       ErrInvalidAuthReply,
			},
		)

	default:
		return &InvalidResponseError{
			StatusCode:  response.StatusCode,
			ContentType: contentType,
			BodySize:    len(trimmed),
			Cause:       ErrInvalidAuthReply,
		}
	}
}

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{
		httpClient:  httpClient,
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

	if response.Valid == 1 {
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
		return Session{}, &TOTPChallenge{PreLoginToken: *response.PreLoginToken}, ErrTOTPRequired
	case "invalidemailorpassword", "invalidtotpcode":
		return Session{}, nil, ErrInvalidCredentials
	case "ipchanged":
		return Session{}, nil, ErrIPChanged
	case "temporarilyblocked":
		return Session{}, nil, ErrTemporarilyBlocked
	default:
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
	return response.Valid == 1, nil
}

func (c *Client) doJSON(request *http.Request, target any) error {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request failed: %w", ErrNetwork)
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusInternalServerError {
		return ErrServer
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ErrServer
	}

	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", ErrInvalidAuthReply)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode trailing response: %w", ErrInvalidAuthReply)
	} else if err == nil {
		return ErrInvalidAuthReply
	}
	return nil
}

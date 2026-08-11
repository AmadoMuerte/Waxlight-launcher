package servers

import "time"

// FavoriteServer is a locally saved server address. Credentials are never
// stored here; Vintage Story owns its own authenticated connection flow.
type FavoriteServer struct {
	ID         string
	Name       string
	Address    string
	InstanceID *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// PublicServer is a listing published by Vintage Story's public server catalog.
// It contains no account or connection credentials.
type PublicServer struct {
	Name              string
	Address           string
	Description       string
	Players           int
	ModCount          int
	RequiresWhitelist bool
	PasswordProtected bool
	Joinable          bool
}

// SaveInput is the feature-owned input for creating or updating a favorite
// server. An empty ID creates a new server; a present ID updates it.
type SaveInput struct {
	ID         string
	Name       string
	Address    string
	InstanceID *string
}

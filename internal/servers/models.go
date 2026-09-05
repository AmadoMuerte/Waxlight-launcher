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
	ID                string
	URL               string
	Name              string
	Address           string
	Description       string
	FullDescription   string
	DescriptionHTML   string
	ImageURL          string
	BannerURL         string
	GameVersion       string
	Players           int
	MaxPlayers        int
	ModCount          int
	Location          string
	Languages         []string
	Operator          string
	OperatorURL       string
	Modified          bool
	RequiresWhitelist bool
	PasswordProtected bool
	Joinable          bool
	Mods              []ServerMod
}

// ServerMod is a mod reported by a public server's detail page.
type ServerMod struct {
	Name    string
	Version string
	URL     string
}

// SaveInput is the feature-owned input for creating or updating a favorite
// server. An empty ID creates a new server; a present ID updates it.
type SaveInput struct {
	ID         string
	Name       string
	Address    string
	InstanceID *string
}

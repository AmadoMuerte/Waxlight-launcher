package instances

import (
	"context"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/versions"
)

type QueryRepository interface {
	ListInstances(context.Context) ([]Instance, error)
	GetInstance(context.Context, string) (Instance, error)
}

type CreateRepository interface {
	ListInstances(context.Context) ([]Instance, error)
	SaveInstance(context.Context, Instance) error
	IsDirectoryUsed(context.Context, string, string) (bool, error)
}

type Repository interface {
	QueryRepository
	CreateRepository
	DeleteInstance(context.Context, string) error
}

type VersionReader interface {
	Get(context.Context, string) (versions.GameVersion, error)
}

type AccountReader interface {
	GetAccount(context.Context, string) (accounts.Account, error)
}

type MutationGate interface {
	Begin() error
	End()
}

type DirectoryStorage interface {
	Allocate(directory, instanceID string) (DirectoryAllocation, error)
}

type DirectoryAllocation interface {
	Directory() string
	Commit()
	Rollback() error
}

type Publisher interface {
	Publish(string, any)
}

type LanguageFunc func(context.Context) (string, error)
type TelemetryFunc func(context.Context, string)
type Clock func() time.Time
type IDGenerator func() string

package instances

import (
	"context"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
	"github.com/waxlight/waxlight-launcher/internal/domain"
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

type UpdateRepository interface {
	GetInstance(context.Context, string) (Instance, error)
	SaveInstance(context.Context, Instance) error
}

type DeleteRepository interface {
	GetInstance(context.Context, string) (Instance, error)
	DeleteInstance(context.Context, string) error
}

type CloneRepository interface {
	GetInstance(context.Context, string) (Instance, error)
	SaveInstance(context.Context, Instance) error
	DeleteInstance(context.Context, string) error
}

type CloneModRepository interface {
	ListMods(context.Context, string) ([]domain.InstalledMod, error)
	SaveMod(context.Context, domain.InstalledMod) error
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

type InstanceCreator interface {
	Create(context.Context, CreateInput) (Instance, error)
}

type CloneStorage interface {
	Copy(context.Context, string, string) error
	CopiedPath(string, string, string) (string, bool)
}

type Publisher interface {
	Publish(string, any)
}

type PublishFunc func(string, any)

func (publish PublishFunc) Publish(name string, payload any) {
	publish(name, payload)
}

type VersionChangePreparer func(context.Context, Instance, Instance) (func(), error)
type ClientSettingsClearer func(string) error
type DeleteGuard func(string) (func(), error)
type CloneGuard func(string) (func(), error)
type DirectoryRemover func(string) error
type RecoveryCleaner func(context.Context, string) error
type LanguageFunc func(context.Context) (string, error)
type TelemetryFunc func(context.Context, string)
type Clock func() time.Time
type IDGenerator func() string

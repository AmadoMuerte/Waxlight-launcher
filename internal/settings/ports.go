package settings

import "context"

type Repository interface {
	GetSettings(context.Context) (Settings, error)
	SaveSettings(context.Context, Settings) error
}

type ValueRepository interface {
	GetSettingValue(context.Context, string) (string, error)
	SetSettingValue(context.Context, string, string) error
}

type ConsentSynchronizer interface {
	SynchronizeConsent(func() error) error
}

type Heartbeat interface {
	MaybeSendHeartbeat()
}

type DownloadLimiter interface {
	SetLimit(int)
}

type RelocationGate interface {
	BeginRelocation() error
	EndRelocation()
}

type RelocationChecker interface {
	CheckDataRootRelocation(context.Context) error
}

type DataRoot interface {
	Current() (string, error)
	Home() string
	ReadError() (string, error)
	PrepareRelocation(string) (Relocation, error)
	CheckTarget(string) error
}

type Relocation interface {
	Run(context.Context, func(copied, total int64)) error
}

type WorkerGroup interface {
	Go(func(context.Context)) bool
}

type Quitter interface {
	Quit(context.Context)
}

type Publisher interface {
	Publish(string, any)
}

package wails

import "context"

// lifecycle is the minimal application-context and worker contract the Wails
// transport needs. *app.Lifecycle implements it; keeping the transport
// independent of the composition-root package avoids an import cycle between
// the wire and the controllers it binds.
type lifecycle interface {
	Context() context.Context
	Go(func(context.Context)) bool
}

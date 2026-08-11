// Package events defines framework-neutral semantic event delivery.
package events

// Publisher delivers a named feature event and its semantic payload.
type Publisher interface {
	Publish(name string, payload any)
}

// PublisherFunc adapts a function to Publisher.
type PublisherFunc func(name string, payload any)

func (publish PublisherFunc) Publish(name string, payload any) {
	publish(name, payload)
}

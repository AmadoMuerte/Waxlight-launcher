//go:build !linux

package credentials

import keyring "github.com/zalando/go-keyring"

type systemBackend struct{}

func (systemBackend) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (systemBackend) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

func (systemBackend) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

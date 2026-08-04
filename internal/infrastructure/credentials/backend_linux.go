//go:build linux

package credentials

import (
	"fmt"

	dbus "github.com/godbus/dbus/v5"
	keyring "github.com/zalando/go-keyring"
	secretservice "github.com/zalando/go-keyring/secret_service"
)

const (
	secretServiceName      = "org.freedesktop.secrets"
	defaultCollectionAlias = dbus.ObjectPath("/org/freedesktop/secrets/aliases/default")
)

// systemBackend uses the active default collection. go-keyring's Linux
// provider prefers a collection literally named "login" when the service
// advertises one. Some Secret Service implementations keep a stale login path
// while a localized collection is the real default, making that path unusable.
type systemBackend struct{}

func (systemBackend) Get(service, user string) (string, error) {
	svc, err := secretservice.NewSecretService()
	if err != nil {
		return "", err
	}

	item, err := findDefaultCollectionItem(svc, service, user)
	if err != nil {
		return "", err
	}

	session, err := svc.OpenSession()
	if err != nil {
		return "", err
	}
	defer svc.Close(session)

	if err := svc.Unlock(item); err != nil {
		return "", err
	}
	secret, err := svc.GetSecret(item, session.Path())
	if err != nil {
		return "", err
	}
	return string(secret.Value), nil
}

func (systemBackend) Set(service, user, password string) error {
	svc, err := secretservice.NewSecretService()
	if err != nil {
		return err
	}

	session, err := svc.OpenSession()
	if err != nil {
		return err
	}
	defer svc.Close(session)

	collection := svc.Object(secretServiceName, defaultCollectionAlias)
	if err := svc.Unlock(defaultCollectionAlias); err != nil {
		return err
	}
	attributes := map[string]string{"username": user, "service": service}
	secret := secretservice.NewSecret(session.Path(), password)
	return svc.CreateItem(
		collection,
		fmt.Sprintf("Password for '%s' on '%s'", user, service),
		attributes,
		secret,
	)
}

func (systemBackend) Delete(service, user string) error {
	svc, err := secretservice.NewSecretService()
	if err != nil {
		return err
	}
	item, err := findDefaultCollectionItem(svc, service, user)
	if err != nil {
		return err
	}
	return svc.Delete(item)
}

func findDefaultCollectionItem(svc *secretservice.SecretService, service, user string) (dbus.ObjectPath, error) {
	collection := svc.Object(secretServiceName, defaultCollectionAlias)
	if err := svc.Unlock(defaultCollectionAlias); err != nil {
		return "", err
	}
	items, err := svc.SearchItems(collection, map[string]string{
		"username": user,
		"service":  service,
	})
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", keyring.ErrNotFound
	}
	return items[0], nil
}

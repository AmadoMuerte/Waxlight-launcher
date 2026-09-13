//go:build linux

package credentials

import (
	"errors"
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
		return "", wrapLinuxStoreError(nativeStoreErrorOperationService, nativeStoreErrorTargetService, err)
	}

	item, err := findDefaultCollectionItem(svc, service, user)
	if err != nil {
		return "", err
	}

	session, err := svc.OpenSession()
	if err != nil {
		return "", wrapLinuxStoreError(nativeStoreErrorOperationService, nativeStoreErrorTargetService, err)
	}
	defer svc.Close(session)

	if err := svc.Unlock(item); err != nil {
		return "", wrapLinuxStoreError(nativeStoreErrorOperationUnlock, nativeStoreErrorTargetItem, err)
	}
	secret, err := svc.GetSecret(item, session.Path())
	if err != nil {
		return "", wrapLinuxStoreError(nativeStoreErrorOperationRead, nativeStoreErrorTargetItem, err)
	}
	return string(secret.Value), nil
}

func (systemBackend) Set(service, user, password string) error {
	svc, err := secretservice.NewSecretService()
	if err != nil {
		return wrapLinuxStoreError(nativeStoreErrorOperationService, nativeStoreErrorTargetService, err)
	}

	session, err := svc.OpenSession()
	if err != nil {
		return wrapLinuxStoreError(nativeStoreErrorOperationService, nativeStoreErrorTargetService, err)
	}
	defer svc.Close(session)

	collection := svc.Object(secretServiceName, defaultCollectionAlias)
	if err := svc.Unlock(defaultCollectionAlias); err != nil {
		return wrapLinuxStoreError(nativeStoreErrorOperationUnlock, nativeStoreErrorTargetCollection, err)
	}
	attributes := map[string]string{"username": user, "service": service}
	secret := secretservice.NewSecret(session.Path(), password)
	err = svc.CreateItem(
		collection,
		fmt.Sprintf("Password for '%s' on '%s'", user, service),
		attributes,
		secret,
	)
	if err != nil {
		return wrapLinuxStoreError(nativeStoreErrorOperationCreate, nativeStoreErrorTargetCollection, err)
	}
	return nil
}

func (systemBackend) Delete(service, user string) error {
	svc, err := secretservice.NewSecretService()
	if err != nil {
		return wrapLinuxStoreError(nativeStoreErrorOperationService, nativeStoreErrorTargetService, err)
	}
	item, err := findDefaultCollectionItem(svc, service, user)
	if err != nil {
		return err
	}
	if err := svc.Delete(item); err != nil {
		return wrapLinuxStoreError(nativeStoreErrorOperationDelete, nativeStoreErrorTargetItem, err)
	}
	return nil
}

func findDefaultCollectionItem(svc *secretservice.SecretService, service, user string) (dbus.ObjectPath, error) {
	collection := svc.Object(secretServiceName, defaultCollectionAlias)
	if err := svc.Unlock(defaultCollectionAlias); err != nil {
		return "", wrapLinuxStoreError(nativeStoreErrorOperationUnlock, nativeStoreErrorTargetCollection, err)
	}
	items, err := svc.SearchItems(collection, map[string]string{
		"username": user,
		"service":  service,
	})
	if err != nil {
		return "", wrapLinuxStoreError(nativeStoreErrorOperationSearch, nativeStoreErrorTargetCollection, err)
	}
	if len(items) == 0 {
		return "", wrapLinuxStoreError(nativeStoreErrorOperationSearch, nativeStoreErrorTargetItem, keyring.ErrNotFound)
	}
	return items[0], nil
}

func wrapLinuxStoreError(operation nativeStoreErrorOperation, target nativeStoreErrorTarget, err error) error {
	if err == nil {
		return nil
	}

	kind := nativeStoreErrorUnknown
	if errors.Is(err, keyring.ErrNotFound) {
		kind = nativeStoreErrorMissing
	}
	var dbusErr *dbus.Error
	if errors.As(err, &dbusErr) {
		switch dbusErr.Name {
		case "org.freedesktop.Secret.Error.IsLocked":
			kind = nativeStoreErrorLocked
		case "org.freedesktop.DBus.Error.AccessDenied", "org.freedesktop.DBus.Error.AuthFailed":
			kind = nativeStoreErrorPermission
		case "org.freedesktop.Secret.Error.NoSession", "org.freedesktop.DBus.Error.NoReply", "org.freedesktop.DBus.Error.Disconnected":
			kind = nativeStoreErrorDesktopSession
		case "org.freedesktop.DBus.Error.ServiceUnknown", "org.freedesktop.DBus.Error.NameHasNoOwner":
			kind = nativeStoreErrorUnavailable
		case "org.freedesktop.Secret.Error.NoSuchObject":
			if target == nativeStoreErrorTargetItem {
				kind = nativeStoreErrorMissing
			} else {
				kind = nativeStoreErrorUnavailable
			}
		}
	} else if operation == nativeStoreErrorOperationUnlock {
		kind = nativeStoreErrorUnlock
	}
	return &nativeStoreError{kind: kind, operation: operation, target: target, cause: err}
}

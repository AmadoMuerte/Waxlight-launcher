package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/atomicfile"
)

const storeVersion = 1

type fileData struct {
	Version int                     `json:"version"`
	Secrets map[string]storedSecret `json:"secrets"`
}

type storedSecret struct {
	SessionKey       string `json:"sessionkey"`
	SessionSignature string `json:"sessionsignature"`
}

// FileStore is a temporary cross-platform fallback until the system keyring
// adapter is introduced. The file is separate from metadata and is written
// atomically with owner-only permissions where the OS supports POSIX modes.
type FileStore struct {
	path string
	mu   sync.Mutex
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (store *FileStore) Get(accountID string) (application.Secret, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	data, err := store.load()
	if err != nil {
		return application.Secret{}, err
	}
	secret, ok := data.Secrets[accountID]
	if !ok {
		return application.Secret{}, application.ErrSecretNotFound
	}
	return application.Secret{
		SessionKey:       secret.SessionKey,
		SessionSignature: secret.SessionSignature,
	}, nil
}

func (store *FileStore) Set(accountID string, secret application.Secret) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	data, err := store.load()
	if err != nil {
		return err
	}
	data.Secrets[accountID] = storedSecret{
		SessionKey:       secret.SessionKey,
		SessionSignature: secret.SessionSignature,
	}
	return store.save(data)
}

func (store *FileStore) Delete(accountID string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	data, err := store.load()
	if err != nil {
		return err
	}
	delete(data.Secrets, accountID)
	return store.save(data)
}

func (store *FileStore) load() (fileData, error) {
	data := fileData{Version: storeVersion, Secrets: map[string]storedSecret{}}
	contents, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return data, nil
	}
	if err != nil {
		return data, fmt.Errorf("read credential store: %w", err)
	}
	if err := json.Unmarshal(contents, &data); err != nil || data.Version != storeVersion || data.Secrets == nil {
		backupPath := fmt.Sprintf("%s.corrupt-%d", store.path, time.Now().UTC().Unix())
		_ = atomicfile.Write(backupPath, contents, 0o600)
		return fileData{}, errors.New("credential store is corrupted; a backup was created")
	}
	return data, nil
}

func (store *FileStore) save(data fileData) error {
	contents, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return errors.New("encode credential store")
	}
	contents = append(contents, '\n')
	if err := atomicfile.Write(store.path, contents, 0o600); err != nil {
		return fmt.Errorf("write credential store: %w", err)
	}
	return nil
}

func DefaultPath(dataRoot string) string {
	return filepath.Join(dataRoot, "account-secrets.json")
}

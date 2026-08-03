package credentials

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/atomicfile"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/securefs"
)

type pendingCommits struct {
	Version int      `json:"version"`
	IDs     []string `json:"pendingAccountIds"`
}

func (store *Store) MarkPending(ctx context.Context, accountID string) error {
	if err := validateRequest(ctx, accountID); err != nil {
		return err
	}
	store.pendingMu.Lock()
	defer store.pendingMu.Unlock()
	state, err := store.loadPending()
	if err != nil {
		return err
	}
	for _, id := range state.IDs {
		if id == accountID {
			return nil
		}
	}
	state.IDs = append(state.IDs, accountID)
	return store.savePending(state)
}

func (store *Store) ClearPending(_ context.Context, accountID string) error {
	store.pendingMu.Lock()
	defer store.pendingMu.Unlock()
	state, err := store.loadPending()
	if err != nil {
		return err
	}
	filtered := state.IDs[:0]
	for _, id := range state.IDs {
		if id != accountID {
			filtered = append(filtered, id)
		}
	}
	state.IDs = filtered
	if len(state.IDs) == 0 {
		if err := os.Remove(store.pendingPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return store.savePending(state)
}

func (store *Store) ReconcilePending(ctx context.Context, accountIDs []string) error {
	store.pendingMu.Lock()
	defer store.pendingMu.Unlock()
	state, err := store.loadPending()
	if err != nil {
		return err
	}
	known := make(map[string]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		known[id] = struct{}{}
	}
	for _, id := range state.IDs {
		if _, ok := known[id]; ok {
			continue
		}
		if err := store.Delete(ctx, id); err != nil && !errors.Is(err, application.ErrSecretNotFound) {
			return err
		}
	}
	if err := os.Remove(store.pendingPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (store *Store) loadPending() (pendingCommits, error) {
	state := pendingCommits{Version: 1, IDs: []string{}}
	if store.pendingPath == "" {
		return state, application.ErrStoreUnavailable
	}
	info, err := os.Lstat(store.pendingPath)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 64*1024 {
		return state, errors.New("invalid pending credential journal")
	}
	if err := validatePendingPermissions(info); err != nil {
		return state, err
	}
	contents, err := os.ReadFile(store.pendingPath)
	if err != nil {
		return state, err
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return state, errors.New("invalid pending credential journal")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) || state.Version != 1 || state.IDs == nil {
		return state, errors.New("invalid pending credential journal")
	}
	for _, id := range state.IDs {
		if err := validateRequest(context.Background(), id); err != nil {
			return state, errors.New("invalid pending credential journal")
		}
	}
	return state, nil
}

func (store *Store) savePending(state pendingCommits) error {
	if err := os.MkdirAll(filepath.Dir(store.pendingPath), 0o700); err != nil {
		return err
	}
	if err := securefs.Apply(filepath.Dir(store.pendingPath), 0o700, true); err != nil {
		return err
	}
	contents, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return atomicfile.Write(store.pendingPath, append(contents, '\n'), 0o600)
}

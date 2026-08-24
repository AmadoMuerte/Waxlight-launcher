//go:build !windows

package credentials

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
)

func TestMigrationRejectsUnsafePermissionsAndSymlink(t *testing.T) {
	for name, setup := range map[string]func(*testing.T, string){
		"unsafe-permissions": func(t *testing.T, root string) { writeLegacy(t, root, validLegacy, 0o644) },
		"symlink": func(t *testing.T, root string) {
			target := filepath.Join(root, "target.json")
			if err := os.WriteFile(target, []byte(validLegacy), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(root, "account-secrets.json")); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			setup(t, root)
			store := &migrationStore{values: map[string]accounts.Credential{}}
			if err := NewMigrator(root, store).Run(context.Background(), []string{"account-1", "account-2"}); err == nil {
				t.Fatal("expected rejection")
			}
			if len(store.values) != 0 {
				t.Fatal("unsafe source was ingested")
			}
		})
	}
}

package wails

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
)

var prohibitedPublicNames = []string{"password", "totpcode", "prelogintoken", "sessionkey", "sessionsignature", "credentialstorepath"}

func TestPublicAuthenticationDTOsAreAllowListed(t *testing.T) {
	for _, value := range []any{AccountDTO{}, LoginResultDTO{}} {
		typeOf := reflect.TypeOf(value)
		for index := 0; index < typeOf.NumField(); index++ {
			field := typeOf.Field(index)
			candidate := strings.ToLower(field.Name + " " + field.Tag.Get("json"))
			for _, prohibited := range prohibitedPublicNames {
				if strings.Contains(candidate, prohibited) {
					t.Fatalf("%s exposes prohibited field %s", typeOf.Name(), field.Name)
				}
			}
		}
	}
	sentinel := accounts.Account{SessionKey: "WAXLIGHT_TEST_SESSION_KEY_DO_NOT_LEAK", SessionSignature: "WAXLIGHT_TEST_SIGNATURE_DO_NOT_LEAK"}
	encoded, err := json.Marshal(accountDTO(sentinel))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "WAXLIGHT_TEST_") {
		t.Fatalf("DTO leaked a secret: %s", encoded)
	}
}

func TestGeneratedWailsBindingsContainNoSecretFields(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	for _, relative := range []string{
		"frontend/src/wailsjs/go/models.ts",
		"frontend/src/wailsjs/go/wails/AccountController.d.ts",
	} {
		contents, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(contents))
		for _, prohibited := range prohibitedPublicNames {
			if strings.Contains(lower, prohibited) {
				t.Fatalf("%s contains prohibited public name %s", relative, prohibited)
			}
		}
	}
}

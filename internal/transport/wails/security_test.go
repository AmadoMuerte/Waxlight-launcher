package wails

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
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

func TestExportInstanceRequestHasNoAuthorMetadata(t *testing.T) {
	typeOf := reflect.TypeOf(ExportInstanceRequest{})
	for index := 0; index < typeOf.NumField(); index++ {
		field := typeOf.Field(index)
		if strings.Contains(strings.ToLower(field.Name+" "+field.Tag.Get("json")), "author") {
			t.Fatalf("%s exposes removed author field %s", typeOf.Name(), field.Name)
		}
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

func TestGeneratedWailsDocumentationContainsNoSecretFields(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "generated", "wails-api.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Types []struct {
			Name   string `json:"name"`
			Fields []struct {
				Name     string `json:"name"`
				JSONName string `json:"jsonName"`
			} `json:"fields"`
		} `json:"types"`
	}
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	for _, typ := range document.Types {
		for _, field := range typ.Fields {
			candidate := strings.ToLower(typ.Name + " " + field.Name + " " + field.JSONName)
			for _, prohibited := range prohibitedPublicNames {
				if strings.Contains(candidate, prohibited) {
					t.Fatalf("generated API documentation exposes prohibited field %s.%s", typ.Name, field.Name)
				}
			}
		}
	}
}

package wails

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/AmadoMuerte/wailsdoc/inventory"
	"github.com/AmadoMuerte/wailsdoc/renderer/markdown"
	"github.com/AmadoMuerte/wailsdoc/scanner"
	"github.com/AmadoMuerte/wailsdoc/schema"
)

// repoRoot returns the repository root relative to this test file.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate the test file")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repository root %s has no go.mod: %v", root, err)
	}
	return root
}

func loadInventory(t *testing.T) inventory.Inventory {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "wails-api-inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result inventory.Inventory
	if err := json.Unmarshal(contents, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func scanTransportAPI(t *testing.T) schema.API {
	t.Helper()
	result, err := scanner.Scan(context.Background(), scanner.Options{
		Dir: repoRoot(t), Packages: []string{"./internal/transport/wails"}, Generator: "wailsdoc",
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// TestWailsAPIInventoryIsInSync proves the checked-in inventory in
// docs/wails-api-inventory.json matches the transport controllers. Regenerate
// it with `make api-inventory` after adding or renaming controller methods.
func TestWailsAPIInventoryIsInSync(t *testing.T) {
	checkedIn := loadInventory(t)
	current := inventory.FromAPI(scanTransportAPI(t))
	if checkedIn.SchemaVersion != current.SchemaVersion {
		t.Fatalf("inventory schema version %d does not match %d", checkedIn.SchemaVersion, current.SchemaVersion)
	}
	expectedJSON, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	actualJSON, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "wails-api-inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(actualJSON) != string(expectedJSON)+"\n" {
		t.Fatalf(
			"docs/wails-api-inventory.json is out of date. Run `make api-inventory`.\nExpected:\n%s",
			expectedJSON,
		)
	}
}

func TestWailsAPIDocumentationIsInSync(t *testing.T) {
	current := scanTransportAPI(t)
	expectedJSON, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	actualJSON, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "generated", "wails-api.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(actualJSON) != string(expectedJSON)+"\n" {
		t.Fatal("docs/generated/wails-api.json is out of date. Run `make api-docs`.")
	}

	expectedDir := filepath.Join(t.TempDir(), "wails-api")
	if _, err := markdown.RenderTitle(current, expectedDir, "Waxlight Backend API"); err != nil {
		t.Fatal(err)
	}
	actualDir := filepath.Join(repoRoot(t), "docs", "generated", "wails-api")
	expectedFiles := markdownFiles(t, expectedDir)
	actualFiles := markdownFiles(t, actualDir)
	if strings.Join(expectedFiles, "\n") != strings.Join(actualFiles, "\n") {
		t.Fatalf("generated Markdown file set is out of date. Run `make api-docs`.\nExpected: %v\nActual: %v", expectedFiles, actualFiles)
	}
	for _, relative := range expectedFiles {
		expected, err := os.ReadFile(filepath.Join(expectedDir, relative))
		if err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(filepath.Join(actualDir, relative))
		if err != nil {
			t.Fatal(err)
		}
		if string(expected) != string(actual) {
			t.Fatalf("docs/generated/wails-api/%s is out of date. Run `make api-docs`.", relative)
		}
	}
}

func markdownFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".md" {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// TestWailsBindingsMatchInventory proves the generated frontend bindings
// expose exactly the controllers and methods recorded in the inventory.
func TestWailsBindingsMatchInventory(t *testing.T) {
	inventory := loadInventory(t)
	root := repoRoot(t)
	for controller, methods := range inventory.Controllers {
		bindingPath := filepath.Join(root, "frontend", "src", "wailsjs", "go", "wails", controller+".js")
		contents, err := os.ReadFile(bindingPath)
		if err != nil {
			t.Fatalf("controller %s has no generated binding: %v", controller, err)
		}
		for _, method := range methods {
			if !regexp.MustCompile(`export function ` + regexp.QuoteMeta(method) + `\b`).Match(contents) {
				t.Fatalf("binding %s is missing method %s", controller, method)
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "frontend", "src", "wailsjs", "go", "wails"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".js")
		if strings.HasSuffix(entry.Name(), ".js") {
			if _, known := inventory.Controllers[name]; !known {
				t.Fatalf("generated binding %s has no matching inventory controller", name)
			}
		}
	}
}

// TestFrontendBackendCompatibility proves every backend call the frontend
// makes targets a controller and method that exist in the transport
// inventory, and that no legacy presentation namespace references remain.
func TestFrontendBackendCompatibility(t *testing.T) {
	inventory := loadInventory(t)
	root := repoRoot(t)
	apiDir := filepath.Join(root, "frontend", "src", "shared", "api")

	callPattern := regexp.MustCompile(`call(?:<[^>]*>)?\(\s*"([A-Za-z]+Controller)"\s*,\s*"([A-Za-z]+)"`)
	importPattern := regexp.MustCompile(`from "\.\./\.\./wailsjs/go/wails/([A-Za-z]+Controller)"`)
	methodImportPattern := regexp.MustCompile(`import\s*\{([^}]+)\}\s*from "\.\./\.\./wailsjs/go/wails/`)

	entries, err := os.ReadDir(apiDir)
	if err != nil {
		t.Fatal(err)
	}
	foundAPIReferences := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ts") {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(apiDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		source := string(contents)
		for _, match := range callPattern.FindAllStringSubmatch(source, -1) {
			foundAPIReferences++
			assertInventoryMethod(t, inventory, match[1], match[2], entry.Name())
		}
		if match := importPattern.FindStringSubmatch(source); match != nil {
			foundAPIReferences++
			controller := match[1]
			methods, known := inventory.Controllers[controller]
			if !known {
				t.Fatalf("%s imports unknown controller %s", entry.Name(), controller)
			}
			if named := methodImportPattern.FindStringSubmatch(source); named != nil {
				for _, method := range strings.Split(named[1], ",") {
					method = strings.TrimSpace(method)
					if method == "" {
						continue
					}
					if !containsString(methods, method) {
						t.Fatalf("%s imports unknown method %s.%s", entry.Name(), controller, method)
					}
				}
			}
		}
	}
	if foundAPIReferences == 0 {
		t.Fatal("no frontend backend references found; the check may be broken")
	}

	modelsPath := filepath.Join(root, "frontend", "src", "wailsjs", "go", "models.ts")
	models, err := os.ReadFile(modelsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(models), "export namespace wails {") {
		t.Fatal("generated models.ts is missing the wails namespace")
	}
}

func assertInventoryMethod(t *testing.T, inventory inventory.Inventory, controller, method, sourceFile string) {
	t.Helper()
	methods, known := inventory.Controllers[controller]
	if !known {
		t.Fatalf("%s calls unknown controller %s", sourceFile, controller)
	}
	if !containsString(methods, method) {
		t.Fatalf("%s calls unknown method %s.%s", sourceFile, controller, method)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

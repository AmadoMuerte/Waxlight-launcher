package wails

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
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

type inventoryFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Controllers   map[string][]string `json:"controllers"`
}

func loadInventory(t *testing.T) inventoryFile {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "wails-api-inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result inventoryFile
	if err := json.Unmarshal(contents, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// buildTransportInventory parses the transport package source and collects
// every exported type named <name>Controller with its exported methods. The
// rule mirrors how Wails binds controllers: exported methods on exported
// struct types are bound.
func buildTransportInventory(t *testing.T) inventoryFile {
	t.Helper()
	result := inventoryFile{SchemaVersion: 1, Controllers: map[string][]string{}}
	dir := filepath.Join(repoRoot(t), "internal", "transport", "wails")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	for _, path := range files {
		source, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, declaration := range source.Decls {
			typeDeclaration, ok := declaration.(*ast.GenDecl)
			if !ok || typeDeclaration.Tok != token.TYPE {
				continue
			}
			for _, spec := range typeDeclaration.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !strings.HasSuffix(typeSpec.Name.Name, "Controller") || !typeSpec.Name.IsExported() {
					continue
				}
				result.Controllers[typeSpec.Name.Name] = controllerMethodsFromFiles(t, files, typeSpec.Name.Name)
			}
		}
	}
	for _, methods := range result.Controllers {
		sort.Strings(methods)
	}
	return result
}

func controllerMethodsFromFiles(t *testing.T, files []string, typeName string) []string {
	t.Helper()
	var methods []string
	for _, path := range files {
		source, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, declaration := range source.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil || function.Name == nil || !function.Name.IsExported() {
				continue
			}
			for _, receiver := range function.Recv.List {
				if receiverTypeName(receiver.Type) == typeName {
					methods = append(methods, function.Name.Name)
				}
			}
		}
	}
	return methods
}

func receiverTypeName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.StarExpr:
		return receiverTypeName(typed.X)
	case *ast.Ident:
		return typed.Name
	case *ast.IndexExpr:
		return receiverTypeName(typed.X)
	case *ast.IndexListExpr:
		return receiverTypeName(typed.X)
	default:
		return ""
	}
}

// TestWailsAPIInventoryIsInSync proves the checked-in inventory in
// docs/wails-api-inventory.json matches the transport controllers. Regenerate
// it with `make api-inventory` after adding or renaming controller methods.
func TestWailsAPIInventoryIsInSync(t *testing.T) {
	checkedIn := loadInventory(t)
	current := buildTransportInventory(t)
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

func assertInventoryMethod(t *testing.T, inventory inventoryFile, controller, method, sourceFile string) {
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

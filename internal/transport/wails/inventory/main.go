// Command inventory generates the checked-in Wails API inventory from the
// transport controllers. Every exported type named <name>Controller in the
// transport package is a bound Wails controller; the inventory records its
// exported methods.
//
// Run from the repository root or the internal/transport/wails directory:
//
//	go run ./internal/transport/wails/inventory
//
// or with the Makefile target:
//
//	make api-inventory
//
// The inventory is validated against the transport source and the frontend by
// TestWailsAPIInventoryIsInSync in internal/transport/wails.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// inventoryPath is the checked-in inventory file relative to the repository
// root.
const inventoryPath = "docs/wails-api-inventory.json"

// transportDir is the directory that owns the Wails controllers relative to
// the repository root.
const transportDir = "internal/transport/wails"

func main() {
	root := findRoot()
	inventory, err := buildInventory(filepath.Join(root, transportDir))
	if err != nil {
		fatalf("build inventory: %v", err)
	}
	contents, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		fatalf("encode inventory: %v", err)
	}
	contents = append(contents, '\n')
	target := filepath.Join(root, inventoryPath)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fatalf("create docs directory: %v", err)
	}
	if err := os.WriteFile(target, contents, 0o644); err != nil {
		fatalf("write inventory: %v", err)
	}
	fmt.Printf("wrote %s\n", target)
}

type inventory struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Controllers   map[string][]string `json:"controllers"`
}

// findRoot locates the repository root from the current working directory or
// the executable directory.
func findRoot() string {
	for _, start := range []string{currentDir(), executableDir()} {
		dir := start
		for {
			if fileExists(filepath.Join(dir, "go.mod")) {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	fatalf("could not locate the repository root (no go.mod found)")
	return ""
}

func buildInventory(dir string) (inventory, error) {
	result := inventory{SchemaVersion: 1, Controllers: map[string][]string{}}
	files, err := os.ReadDir(dir)
	if err != nil {
		return result, err
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") {
			continue
		}
		source, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, file.Name()), nil, 0)
		if err != nil {
			return result, fmt.Errorf("parse %s: %w", file.Name(), err)
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
				methods := controllerMethods(files, dir, typeSpec.Name.Name)
				result.Controllers[typeSpec.Name.Name] = methods
			}
		}
	}
	for _, methods := range result.Controllers {
		sort.Strings(methods)
	}
	return result, nil
}

// controllerMethods collects every exported method declared on the controller
// receiver across all files of the package.
func controllerMethods(files []os.DirEntry, dir string, typeName string) []string {
	var methods []string
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") {
			continue
		}
		source, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, file.Name()), nil, 0)
		if err != nil {
			continue
		}
		for _, declaration := range source.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil || function.Name == nil || !function.Name.IsExported() {
				continue
			}
			for _, receiver := range function.Recv.List {
				receiverName := receiverTypeName(receiver.Type)
				if receiverName == typeName {
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

func currentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

func executableDir() string {
	dir, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(dir)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "inventory: "+format+"\n", args...)
	os.Exit(1)
}

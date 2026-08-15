package wails

import (
	"context"
	"runtime"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type DialogAdapter struct {
	lifecycle lifecycle
}

func NewDialogAdapter(lifecycle lifecycle) *DialogAdapter {
	return &DialogAdapter{lifecycle: lifecycle}
}

func (adapter *DialogAdapter) SelectDataFolder() (string, error) {
	return wruntime.OpenDirectoryDialog(adapter.lifecycle.Context(), wruntime.OpenDialogOptions{Title: "Select the launcher data folder"})
}

func (adapter *DialogAdapter) SelectGameArchive() (string, error) {
	return wruntime.OpenFileDialog(adapter.lifecycle.Context(), wruntime.OpenDialogOptions{
		Title:   "Select a Vintage Story archive",
		Filters: []wruntime.FileFilter{{DisplayName: "Game archives (*.zip, *.tar.gz, *.tgz)", Pattern: "*.zip;*.tar.gz;*.tgz"}},
	})
}

func (adapter *DialogAdapter) SelectGameDirectory() (string, error) {
	return wruntime.OpenDirectoryDialog(adapter.lifecycle.Context(), wruntime.OpenDialogOptions{Title: "Select a Vintage Story directory"})
}

func (adapter *DialogAdapter) SelectOptimumInstallation() (string, error) {
	if runtime.GOOS == "windows" {
		return wruntime.OpenFileDialog(adapter.lifecycle.Context(), wruntime.OpenDialogOptions{
			Title:   "Select Optimum.exe",
			Filters: []wruntime.FileFilter{{DisplayName: "Optimum (Optimum.exe)", Pattern: "Optimum.exe"}},
		})
	}
	return wruntime.OpenDirectoryDialog(adapter.lifecycle.Context(), wruntime.OpenDialogOptions{Title: "Select the Optimum installation folder"})
}

func (adapter *DialogAdapter) SelectModFile() (string, error) {
	return wruntime.OpenFileDialog(adapter.lifecycle.Context(), wruntime.OpenDialogOptions{
		Title:   "Select a mod file",
		Filters: []wruntime.FileFilter{{DisplayName: "Vintage Story mods", Pattern: "*.zip;*.cs;*.dll"}},
	})
}

func (adapter *DialogAdapter) SelectModFiles() ([]string, error) {
	return wruntime.OpenMultipleFilesDialog(adapter.lifecycle.Context(), wruntime.OpenDialogOptions{
		Title:   "Select mod files",
		Filters: []wruntime.FileFilter{{DisplayName: "Vintage Story mods", Pattern: "*.zip;*.cs;*.dll"}},
	})
}

type QuitAdapter struct{}

func (QuitAdapter) Quit(ctx context.Context) {
	wruntime.Quit(ctx)
}

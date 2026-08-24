package wails

import (
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
	vsmodpack "github.com/AmadoMuerte/vintagestory-go/modpack"
)

func TestInstanceModUpdateReportDTO(t *testing.T) {
	report := mods.ModUpdateReport{
		Build: vsmodpack.Build{GameVersion: "1.20"},
		Mods: []vsmodpack.ModUpdate{
			{
				ModID:            "stonequarry",
				Name:             "Stone Quarry",
				InstalledVersion: "1.2.0",
				TargetVersionID:  "v2",
				TargetVersion:    "1.3.0",
				Status:           vsmodpack.StatusUpdateAvailable,
				Changelog:        "Fixed a crash.",
				Compatible:       false,
				AddedDeps:        []vsmodpack.Dependency{{ModID: "newdep", Name: "New"}},
			},
			{
				ModID:  "mylocal",
				Name:   "My Local",
				Status: vsmodpack.StatusNotUpdatable,
				Reason: vsmodpack.ReasonLocalMod,
			},
		},
		Summary: vsmodpack.Summary{
			TotalMods: 2, UpdatesAvailable: 1, Incompatible: 1, NotUpdatableLocal: 1,
		},
	}
	dto := instanceModUpdateReportDTO(report)
	if dto.GameVersion != "1.20" {
		t.Fatalf("unexpected game version %q", dto.GameVersion)
	}
	if len(dto.Mods) != 2 {
		t.Fatalf("expected two mods, got %d", len(dto.Mods))
	}
	updatable := dto.Mods[0]
	if updatable.Status != "update_available" || updatable.TargetVersion != "1.3.0" {
		t.Fatalf("unexpected updatable mod: %+v", updatable)
	}
	if updatable.Compatible {
		t.Fatalf("expected the incompatible flag to map through")
	}
	if len(updatable.AddedDeps) != 1 || updatable.AddedDeps[0].ModID != "newdep" {
		t.Fatalf("unexpected added deps: %+v", updatable.AddedDeps)
	}
	if len(updatable.RemovedDeps) != 0 {
		t.Fatalf("expected empty removed deps to become an empty slice")
	}
	local := dto.Mods[1]
	if local.Status != "not_updatable" || local.Reason != "local_mod" {
		t.Fatalf("unexpected local mod: %+v", local)
	}
	if dto.Summary.UpdatesAvailable != 1 || dto.Summary.Incompatible != 1 || dto.Summary.NotUpdatableLocal != 1 {
		t.Fatalf("unexpected summary: %+v", dto.Summary)
	}
}

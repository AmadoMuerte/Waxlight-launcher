package supportreports

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/operations"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/sessions"
)

type fakeStore struct {
	instance instances.Instance
	mods     []mods.InstalledMod
}

func (f fakeStore) GetInstance(context.Context, string) (instances.Instance, error) {
	return f.instance, nil
}
func (f fakeStore) ListMods(context.Context, string) ([]mods.InstalledMod, error) { return f.mods, nil }

type fakeOperations []operations.Operation

func (f fakeOperations) ListLimit(_ context.Context, limit int) ([]operations.Operation, error) {
	return f[:min(limit, len(f))], nil
}

type fakeSessions []sessions.PlaySession

func (f fakeSessions) ListSessions(context.Context, string, int) ([]sessions.PlaySession, error) {
	return f, nil
}

type fakeLogs []string

func (f fakeLogs) Lines() []string { return f }

type fakeIdentity struct{}

func (fakeIdentity) InstallationID(context.Context) string { return "installation-1" }

type fakeSender struct{ report Report }

func (f *fakeSender) SendSupportReport(_ context.Context, report Report) (Result, error) {
	f.report = report
	return Result{ReportID: "WL-R-A7F31C", Status: "received"}, nil
}

func TestServiceCollectsSanitizedBoundedReport(t *testing.T) {
	ops := make(fakeOperations, MaxOperations+5)
	for i := range ops {
		ops[i] = operations.Operation{Type: "download", Status: "failed", CreatedAt: time.Now()}
	}
	logs := make(fakeLogs, MaxLogLines+5)
	for i := range logs {
		logs[i] = fmt.Sprintf("%d /home/alice/log token=secret", i)
	}
	sender := &fakeSender{}
	service := NewService(fakeStore{
		instance: instances.Instance{ID: "i1", Name: "Pack", GameVersionID: "1.20", GameClient: instances.GameClientOptimum, EnvironmentVariables: map[string]string{"SAFE": "1", "GITHUB_TOKEN": "secret"}},
		mods:     []mods.InstalledMod{{Version: "1.2.3", Enabled: true, Source: "moddb:examplemod:7", UpdatePolicy: mods.UpdatePolicyPinned}},
	}, ops, fakeSessions{{VersionID: "1.20", StartedAt: time.Now(), Crashed: true}}, nil, logs, fakeIdentity{}, sender)

	preview, err := service.Preview(context.Background(), "Bearer abcdefgh", "i1")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"/home/alice", "abcdefgh", "GITHUB_TOKEN\": \"secret"} {
		if strings.Contains(preview.Payload, forbidden) {
			t.Fatalf("preview contains %q", forbidden)
		}
	}
	result, err := service.Submit(context.Background(), preview.SnapshotID)
	if err != nil || result.ReportID != "WL-R-A7F31C" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(sender.report.Operations) != MaxOperations || len(sender.report.Logs.Launcher) != MaxLogLines {
		t.Fatal("limits not enforced")
	}
	if sender.report.Instance.EnvironmentVariables["SAFE"] != "1" || sender.report.Instance.EnvironmentVariables["GITHUB_TOKEN"] != "<redacted>" {
		t.Fatal("environment not sanitized")
	}
	if sender.report.Mods[0].ModID != "examplemod" || sender.report.Mods[0].Source != "moddb" {
		t.Fatal("mod not mapped")
	}
}

func TestServiceWorksWithoutInstance(t *testing.T) {
	service := NewService(fakeStore{}, fakeOperations{}, fakeSessions{}, nil, fakeLogs{}, fakeIdentity{}, &fakeSender{})
	preview, err := service.Preview(context.Background(), "Launcher does not start", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(preview.Payload, `"instance"`) {
		t.Fatal("unexpected instance")
	}
}

func TestSubmitUsesPreviewedSnapshotAndKeepsItAfterFailure(t *testing.T) {
	sender := &failingOnceSender{}
	service := NewService(fakeStore{}, fakeOperations{}, fakeSessions{}, nil, fakeLogs{"first"}, fakeIdentity{}, sender)
	preview, err := service.Preview(context.Background(), "Original", "")
	if err != nil {
		t.Fatal(err)
	}
	service.logs = fakeLogs{"changed"}
	if _, err := service.Submit(context.Background(), preview.SnapshotID); err == nil {
		t.Fatal("expected first failure")
	}
	if _, err := service.Submit(context.Background(), preview.SnapshotID); err != nil {
		t.Fatal(err)
	}
	if sender.report.Description != "Original" || sender.report.Logs.Launcher[0] != "first" {
		t.Fatalf("report changed: %+v", sender.report)
	}
}

type failingOnceSender struct {
	report Report
	failed bool
}

func (f *failingOnceSender) SendSupportReport(_ context.Context, report Report) (Result, error) {
	f.report = report
	if !f.failed {
		f.failed = true
		return Result{}, fmt.Errorf("network")
	}
	return Result{ReportID: "WL-R-A7F31C", Status: "received"}, nil
}

func TestSanitizeTextAndEnvironment(t *testing.T) {
	text := `Bearer abc.def password=secret /home/alice/a C:\Users\Alice\AppData "sessionkey":"value"`
	sanitized := SanitizeText(text)
	for _, forbidden := range []string{"abc.def", "secret", "/home/alice", `C:\Users\Alice`, `"value"`} {
		if strings.Contains(sanitized, forbidden) {
			t.Fatalf("contains %q: %s", forbidden, sanitized)
		}
	}
	if !strings.Contains(sanitized, "$HOME/a") || !strings.Contains(sanitized, `%USERPROFILE%\AppData`) {
		t.Fatal(sanitized)
	}
	env := SanitizeEnvironment(map[string]string{"SAFE": "yes", "API_KEY": "no"})
	if env["SAFE"] != "yes" || env["API_KEY"] != "<redacted>" {
		t.Fatalf("env=%v", env)
	}
}

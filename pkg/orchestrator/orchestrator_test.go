package orchestrator

import (
	"context"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/cperrin88/gotya/pkg/download"
	dlmocks "github.com/cperrin88/gotya/pkg/download/mocks"
	"github.com/cperrin88/gotya/pkg/index"
	ocmocks "github.com/cperrin88/gotya/pkg/orchestrator/mocks"
	"go.uber.org/mock/gomock"
)

func TestSyncAll_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	u1, _ := url.Parse("https://example.com/repo1/index.json")
	u2, _ := url.Parse("https://example.com/repo2/index.json")
	repos := []*index.Repository{{Name: "repo1", URL: u1}, {Name: "repo2", URL: u2}}

	expectedDir := t.TempDir()

	dl := dlmocks.NewMockManager(ctrl)
	dl.EXPECT().FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, items []download.Item, opts download.Options) (map[string]string, error) {
			if len(items) != 2 {
				t.Fatalf("expected 2 items, got %d", len(items))
			}
			if items[0].ID != "repo1" || items[0].Filename != "repo1.json" {
				t.Fatalf("unexpected first item: %+v", items[0])
			}
			if items[1].ID != "repo2" || items[1].Filename != "repo2.json" {
				t.Fatalf("unexpected second item: %+v", items[1])
			}
			if opts.Dir != expectedDir {
				t.Fatalf("expected dir %s, got %s", expectedDir, opts.Dir)
			}
			return map[string]string{"repo1": filepath.Join(expectedDir, "repo1.json"), "repo2": filepath.Join(expectedDir, "repo2.json")}, nil
		},
	).Times(1)

	orch := &Orchestrator{DL: dl}

	if err := orch.SyncAll(context.Background(), repos, expectedDir, Options{Concurrency: 3}); err != nil {
		t.Fatalf("SyncAll failed: %v", err)
	}
}

func TestSyncAll_NoReposOrNilURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dl := dlmocks.NewMockManager(ctrl)
	orch := &Orchestrator{DL: dl}

	// No repos
	if err := orch.SyncAll(context.Background(), nil, t.TempDir(), Options{}); err != nil {
		t.Fatalf("expected nil error for nil repos, got %v", err)
	}
	// Repos with nil URLs
	repos := []*index.Repository{{Name: "r1", URL: nil}}
	if err := orch.SyncAll(context.Background(), repos, t.TempDir(), Options{}); err != nil {
		t.Fatalf("expected nil error for repos with nil URL, got %v", err)
	}
}

func TestSyncAll_NoDownloadManager(t *testing.T) {
	orch := &Orchestrator{}
	if err := orch.SyncAll(context.Background(), nil, t.TempDir(), Options{}); err == nil {
		t.Fatalf("expected error when DL is nil")
	}
}

func TestInstall_DryRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	idx := ocmocks.NewMockIndexPlanner(ctrl)
	orch := &Orchestrator{Index: idx}

	// plan with two steps
	s1url, _ := url.Parse("https://example.com/a.tgz")
	plan := index.InstallPlan{Steps: []index.InstallStep{
		{ID: "pkgA@1.0.0", Name: "pkgA", Version: "1.0.0", OS: "linux", Arch: "amd64", SourceURL: s1url, Checksum: "abc"},
		{ID: "pkgB@2.0.0", Name: "pkgB", Version: "2.0.0", OS: "linux", Arch: "amd64"},
	}}

	req := index.InstallRequest{Name: "pkgA", Version: ">= 0.0.0", OS: "linux", Arch: "amd64"}

	idx.EXPECT().Plan(gomock.Any(), req).Return(plan, nil).Times(1)

	var phases []string
	var msgs []string
	hooks := Hooks{OnEvent: func(e Event) {
		phases = append(phases, e.Phase)
		msgs = append(msgs, e.Msg)
	}}

	if err := orch.Install(context.Background(), req, Options{DryRun: true}, hooks); err != nil {
		t.Fatalf("Install dry-run failed: %v", err)
	}

	// Expect planning, planning (steps), done
	if len(phases) < 3 || phases[0] != "planning" || phases[len(phases)-1] != "done" {
		t.Fatalf("unexpected events: phases=%v msgs=%v", phases, msgs)
	}
}

func TestInstall_PrefetchAndInstall_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	idx := ocmocks.NewMockIndexPlanner(ctrl)
	art := ocmocks.NewMockArtifactInstaller(ctrl)

	tmp := t.TempDir() // absolute
	dl := dlmocks.NewMockManager(ctrl)
	dl.EXPECT().FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, items []download.Item, opts download.Options) (map[string]string, error) {
			if len(items) != 1 || items[0].ID != "pkgA@1.0.0" {
				t.Fatalf("unexpected items: %+v", items)
			}
			if opts.Dir != tmp || opts.Concurrency != 2 {
				t.Fatalf("unexpected opts: %+v", opts)
			}
			return map[string]string{items[0].ID: filepath.Join(tmp, "pkgA-1.0.0.tgz")}, nil
		},
	).Times(1)

	orch := &Orchestrator{Index: idx, DL: dl, Artifact: art}

	sURL, _ := url.Parse("https://example.com/pkgA-1.0.0.tgz")
	step := index.InstallStep{ID: "pkgA@1.0.0", Name: "pkgA", Version: "1.0.0", OS: "linux", Arch: "amd64", SourceURL: sURL, Checksum: "deadbeef"}
	plan := index.InstallPlan{Steps: []index.InstallStep{step}}
	req := index.InstallRequest{Name: "pkgA", Version: "1.0.0", OS: "linux", Arch: "amd64"}

	idx.EXPECT().Plan(gomock.Any(), req).Return(plan, nil).Times(1)

	art.EXPECT().InstallArtifact(gomock.Any(), gomock.Any(), filepath.Join(tmp, "pkgA-1.0.0.tgz")).Return(nil).Times(1)

	var gotDone bool
	hooks := Hooks{OnEvent: func(e Event) {
		if e.Phase == "done" {
			gotDone = true
		}
	}}

	if err := orch.Install(context.Background(), req, Options{CacheDir: tmp, Concurrency: 2}, hooks); err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if !gotDone {
		t.Fatalf("expected done event")
	}
}

func TestInstall_MissingLocalFile_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	idx := ocmocks.NewMockIndexPlanner(ctrl)
	art := ocmocks.NewMockArtifactInstaller(ctrl)

	tmp := t.TempDir() // absolute
	dl := dlmocks.NewMockManager(ctrl)
	dl.EXPECT().FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, items []download.Item, opts download.Options) (map[string]string, error) {
			// Return empty map so orchestrator can't find local file
			return map[string]string{}, nil
		},
	).Times(1)

	orch := &Orchestrator{Index: idx, DL: dl, Artifact: art}

	sURL, _ := url.Parse("https://example.com/pkgA-1.0.0.tgz")
	step := index.InstallStep{ID: "pkgA@1.0.0", Name: "pkgA", Version: "1.0.0", OS: "linux", Arch: "amd64", SourceURL: sURL}
	plan := index.InstallPlan{Steps: []index.InstallStep{step}}
	req := index.InstallRequest{Name: "pkgA", Version: "1.0.0", OS: "linux", Arch: "amd64"}

	idx.EXPECT().Plan(gomock.Any(), req).Return(plan, nil).Times(1)

	// Artifact should NOT be called

	err := orch.Install(context.Background(), req, Options{CacheDir: tmp}, Hooks{})
	if err == nil {
		t.Fatalf("expected error due to missing local file")
	}
}

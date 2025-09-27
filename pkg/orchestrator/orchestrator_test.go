package orchestrator

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/cperrin88/gotya/pkg/download"
	"github.com/cperrin88/gotya/pkg/index"
	"github.com/cperrin88/gotya/pkg/model"
	mocks "github.com/cperrin88/gotya/pkg/orchestrator/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSyncAll_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	u1, _ := url.Parse("https://example.com/repo1/index.json")
	u2, _ := url.Parse("https://example.com/repo2/index.json")
	repos := []*index.Repository{{Name: "repo1", URL: u1}, {Name: "repo2", URL: u2}}

	expectedDir := t.TempDir()

	dl := mocks.NewMockDownloader(ctrl)
	dl.EXPECT().FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, items []download.Item, opts download.Options) (map[string]string, error) {
			assert.Len(t, items, 2, "should have 2 items to fetch")
			assert.Equal(t, "repo1", items[0].ID, "first item ID should be 'repo1'")
			assert.Equal(t, "repo1.json", items[0].Filename, "first item filename should be 'repo1.json'")
			assert.Equal(t, "repo2", items[1].ID, "second item ID should be 'repo2'")
			assert.Equal(t, "repo2.json", items[1].Filename, "second item filename should be 'repo2.json'")
			assert.Equal(t, expectedDir, opts.Dir, "download directory should match")

			return map[string]string{
				"repo1": filepath.Join(expectedDir, "repo1.json"),
				"repo2": filepath.Join(expectedDir, "repo2.json"),
			}, nil
		},
	).Times(1)

	orch := &Orchestrator{DL: dl}

	err := orch.SyncAll(context.Background(), repos, expectedDir, Options{Concurrency: 3})
	require.NoError(t, err, "SyncAll should not return an error")
}

func TestSyncAll_NoReposOrNilURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dl := mocks.NewMockDownloader(ctrl)
	dl.EXPECT().FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).Times(0) // No fetch should happen

	orch := &Orchestrator{DL: dl}

	// Test with nil repos
	t.Run("nil repos", func(t *testing.T) {
		err := orch.SyncAll(context.Background(), nil, t.TempDir(), Options{})
		assert.NoError(t, err, "should not return error for nil repos")
	})

	// Test with repos containing nil URL
	t.Run("nil URL in repos", func(t *testing.T) {
		repos := []*index.Repository{{Name: "r1", URL: nil}}
		err := orch.SyncAll(context.Background(), repos, t.TempDir(), Options{})
		assert.NoError(t, err, "should not return error for repos with nil URL")
	})
}

func TestSyncAll_NoDownloadManager(t *testing.T) {
	orch := &Orchestrator{} // DL is nil
	err := orch.SyncAll(context.Background(), nil, t.TempDir(), Options{})
	require.Error(t, err, "should return error when DL is nil")
	assert.Contains(t, err.Error(), "download manager is not configured", "error message should indicate missing download manager")
}

func TestSyncAll_DownloadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dl := mocks.NewMockDownloader(ctrl)
	expectedErr := fmt.Errorf("download failed")
	dl.EXPECT().FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, expectedErr).
		Times(1)

	orch := &Orchestrator{DL: dl}
	u, _ := url.Parse("https://example.com/repo/index.json")
	err := orch.SyncAll(
		context.Background(),
		[]*index.Repository{{Name: "repo", URL: u}},
		t.TempDir(),
		Options{},
	)

	require.Error(t, err, "should return error when download fails")
	// Since we're not using a custom error type, we can just check the error message
	assert.ErrorIs(t, err, expectedErr, "should return the expected error")
}

func TestSyncAll_EmptyRepos(t *testing.T) {
	ctrl := gomock.NewController(t)
	dl := mocks.NewMockDownloader(ctrl)
	dl.EXPECT().FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	orch := &Orchestrator{DL: dl}
	err := orch.SyncAll(
		context.Background(),
		[]*index.Repository{},
		t.TempDir(),
		Options{},
	)

	assert.NoError(t, err, "should not return error for empty repos")
}

func TestInstall_DryRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	s1url, _ := url.Parse("https://example.com/pkgA-1.0.0.tgz")
	req := model.ResolveRequest{
		Name:               "pkgA",
		Version:            ">= 0.0.0",
		OS:                 "linux",
		Arch:               "amd64",
		InstalledArtifacts: []*model.InstalledArtifact{},
	}

	plan := model.ResolvedArtifacts{
		Artifacts: []model.ResolvedArtifact{
			{
				ID:        "pkgA@1.0.0",
				Name:      "pkgA",
				Version:   "1.0.0",
				OS:        "linux",
				Arch:      "amd64",
				SourceURL: s1url,
				Checksum:  "abc",
			},
			{
				ID:      "pkgB@2.0.0",
				Name:    "pkgB",
				Version: "2.0.0",
				OS:      "linux",
				Arch:    "amd64",
			},
		},
	}

	// Setup mocks
	idx := mocks.NewMockArtifactResolver(ctrl)
	idx.EXPECT().
		Resolve(gomock.Any(), req).
		Return(plan, nil).
		Times(1)

	orch := &Orchestrator{Index: idx}

	// Setup hooks to capture events
	var events []Event
	orch.Hooks = Hooks{
		OnEvent: func(e Event) {
			events = append(events, e)
		},
	}

	// Execute dry run
	err := orch.Install(
		context.Background(),
		req,
		InstallOptions{DryRun: true},
	)

	// Verify results
	require.NoError(t, err, "Install dry-run should not return an error")
	require.GreaterOrEqual(t, len(events), 3, "should have at least 3 events (planning, steps, done)")

	// Check first event is planning
	assert.Equal(t, "planning", events[0].Phase, "first event should be planning phase")
	assert.Equal(t, req.Name, events[0].Msg, "planning message should include package name")

	// Check last event is done
	lastEvent := events[len(events)-1]
	assert.Equal(t, "done", lastEvent.Phase, "last event should be done phase")
	assert.Equal(t, "dry-run", lastEvent.Msg, "done message should indicate dry run")

	// Check that we have events for each step
	stepEvents := events[1 : len(events)-1] // Exclude first and last events
	assert.Len(t, stepEvents, len(plan.Artifacts), "should have one event per step")

	for i, step := range plan.Artifacts {
		event := stepEvents[i]
		expectedMsg := fmt.Sprintf("%s@%s", step.Name, step.Version)

		assert.Equal(t, "planning", event.Phase, "step event should be in planning phase")
		assert.Equal(t, step.ID, event.ID, "step event ID should match step ID")
		assert.Equal(t, expectedMsg, event.Msg, "step event message should include name and version")
	}
}

func TestInstall_PrefetchAndInstall_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	tmp := t.TempDir()
	sURL, _ := url.Parse("https://example.com/pkgA-1.0.0.tgz")
	req := model.ResolveRequest{
		Name:               "pkgA",
		Version:            "1.0.0",
		OS:                 "linux",
		Arch:               "amd64",
		InstalledArtifacts: []*model.InstalledArtifact{},
	}

	step := model.ResolvedArtifact{
		ID:        "pkgA@1.0.0",
		Name:      "pkgA",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL,
		Checksum:  "deadbeef",
	}
	plan := model.ResolvedArtifacts{Artifacts: []model.ResolvedArtifact{step}}

	// Setup mocks
	dl := mocks.NewMockDownloader(ctrl)
	dl.EXPECT().
		FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, items []download.Item, opts download.Options) (map[string]string, error) {
			require.Len(t, items, 1, "should have one item to fetch")
			assert.Equal(t, step.ID, items[0].ID, "item ID should match step ID")
			assert.Equal(t, tmp, opts.Dir, "cache directory should match")
			assert.Equal(t, 2, opts.Concurrency, "concurrency should be 2")

			return map[string]string{
				items[0].ID: filepath.Join(tmp, "pkgA-1.0.0.tgz"),
			}, nil
		}).
		Times(1)

	idx := mocks.NewMockArtifactResolver(ctrl)
	idx.EXPECT().
		Resolve(gomock.Any(), req).
		Return(plan, nil).
		Times(1)

	art := mocks.NewMockArtifactManager(ctrl)
	art.EXPECT().
		GetInstalledArtifacts().
		Return([]*model.InstalledArtifact{}, nil).
		Times(1)
	expectedArtifactPath := filepath.Join(tmp, "pkgA-1.0.0.tgz")
	art.EXPECT().
		InstallArtifact(gomock.Any(), gomock.Any(), expectedArtifactPath, gomock.Any()).
		DoAndReturn(func(_ context.Context, desc *model.IndexArtifactDescriptor, path string, reason model.InstallationReason) error {
			assert.Equal(t, step.Name, desc.Name, "artifact name should match")
			assert.Equal(t, step.Version, desc.Version, "artifact version should match")
			assert.Equal(t, step.OS, desc.OS, "artifact OS should match")
			assert.Equal(t, step.Arch, desc.Arch, "artifact arch should match")
			assert.Equal(t, step.Checksum, desc.Checksum, "artifact checksum should match")
			assert.Equal(t, sURL.String(), desc.URL, "artifact URL should match")
			return nil
		}).
		Times(1)

	// Setup orchestrator and hooks
	orch := &Orchestrator{
		Index:           idx,
		DL:              dl,
		ArtifactManager: art,
	}

	var gotDone bool
	orch.Hooks = Hooks{
		OnEvent: func(e Event) {
			if e.Phase == "done" {
				gotDone = true
			}
		},
	}

	// Execute the test
	testOpts := InstallOptions{
		CacheDir:    tmp,
		Concurrency: 2, // Match the expected concurrency in the test
	}
	err := orch.Install(
		context.Background(),
		req,
		testOpts,
	)

	// Verify results
	require.NoError(t, err, "Install should not return an error")
	assert.True(t, gotDone, "should have received done event")
}

func TestNew(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	idx := mocks.NewMockArtifactResolver(ctrl)
	reverseIdx := mocks.NewMockArtifactReverseResolver(ctrl)
	dl := mocks.NewMockDownloader(ctrl)
	am := mocks.NewMockArtifactManager(ctrl)

	// Call the constructor
	orch := New(idx, reverseIdx, dl, am, Hooks{})

	// Verify all fields are properly initialized
	assert.Same(t, idx, orch.Index, "Index field should be set to the provided mock")
	assert.Same(t, dl, orch.DL, "DL field should be set to the provided mock")
	assert.Same(t, am, orch.ArtifactManager, "ArtifactManager field should be set to the provided mock")
}

func TestEmit(t *testing.T) {
	t.Run("with hooks", func(t *testing.T) {
		var called bool
		var event Event

		hooks := Hooks{
			OnEvent: func(e Event) {
				called = true
				event = e
			},
		}

		// Execute test
		emit(hooks, Event{Phase: "test", Msg: "message"})

		// Verify results
		require.True(t, called, "OnEvent hook should be called")
		require.Equal(t, "test", event.Phase, "event phase should match")
		require.Equal(t, "message", event.Msg, "event message should match")
	})

	t.Run("with nil hooks", func(t *testing.T) {
		// This should not panic
		require.NotPanics(t, func() {
			emit(Hooks{}, Event{Phase: "test2"})
		}, "emit with nil hooks should not panic")
	})

	t.Run("with nil OnEvent function", func(t *testing.T) {
		// This should not panic
		require.NotPanics(t, func() {
			emit(Hooks{OnEvent: nil}, Event{Phase: "test3"})
		}, "emit with nil OnEvent function should not panic")
	})
}

func TestInstall_NoDownloadManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	sURL, _ := url.Parse("https://example.com/pkgA-1.0.0.tgz")
	req := model.ResolveRequest{
		Name:    "pkgA",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	step := model.ResolvedArtifact{
		ID:        "pkgA@1.0.0",
		Name:      "pkgA",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL,
	}

	plan := model.ResolvedArtifacts{Artifacts: []model.ResolvedArtifact{step}}

	// Setup test data
	req = model.ResolveRequest{
		Name:               "pkgA",
		Version:            "1.0.0",
		OS:                 "linux",
		Arch:               "amd64",
		InstalledArtifacts: []*model.InstalledArtifact{},
	}

	// Setup mocks
	idx := mocks.NewMockArtifactResolver(ctrl)
	idx.EXPECT().
		Resolve(gomock.Any(), req).
		Return(plan, nil).
		Times(1)

	art := mocks.NewMockArtifactManager(ctrl)
	art.EXPECT().
		GetInstalledArtifacts().
		Return([]*model.InstalledArtifact{}, nil).
		Times(1)

	// Create orchestrator without download manager
	orch := &Orchestrator{
		Index:           idx,
		ArtifactManager: art,
		Hooks:           Hooks{},
		// DL is intentionally nil
	}
	// Execute test
	err := orch.Install(
		context.Background(),
		req,
		InstallOptions{
			CacheDir: t.TempDir(),
		},
	)

	// Verify results
	require.Error(t, err, "should return error when download is required but DL is nil")
	errMsg := err.Error()
	assert.Contains(t, errMsg, "no local file available",
		"error message should indicate missing local file")
	assert.Contains(t, errMsg, step.ID,
		"error message should include the step ID")
}

func TestInstall_NoIndexPlanner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create an orchestrator with nil Index
	testReq := model.ResolveRequest{
		Name:    "pkgA",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	testOpts := InstallOptions{
		CacheDir: t.TempDir(),
	}

	torch := &Orchestrator{
		DL:              mocks.NewMockDownloader(ctrl),
		ArtifactManager: mocks.NewMockArtifactManager(ctrl),
		Hooks:           Hooks{},
		// Index is intentionally nil
	}

	// Execute test
	err := torch.Install(
		context.Background(),
		testReq,
		testOpts,
	)

	// Verify results
	require.Error(t, err, "should return error when Index is nil")
	errMsg := err.Error()
	assert.Contains(t, errMsg, "index planner is not configured",
		"error message should indicate missing index planner")
}

func TestInstall_PlanError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	testReq := model.ResolveRequest{
		Name:               "pkgA",
		Version:            "1.0.0",
		InstalledArtifacts: []*model.InstalledArtifact{},
	}

	expectedErr := fmt.Errorf("planning failed")

	// Setup mocks
	idx := mocks.NewMockArtifactResolver(ctrl)
	idx.EXPECT().
		Resolve(gomock.Any(), testReq).
		Return(model.ResolvedArtifacts{}, expectedErr).
		Times(1)

	// Create orchestrator with only Index set
	torch := &Orchestrator{
		Index: idx,
		Hooks: Hooks{},
	}
	err := torch.Install(
		context.Background(),
		testReq,
		InstallOptions{},
	)

	// Verify results
	require.Error(t, err, "should return error when planning fails")
	assert.Same(t, expectedErr, err, "should return the exact error from Resolve")
}

func TestInstall_ArtifactInstallError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "pkgA-1.0.0.tgz")
	require.NoError(t, os.WriteFile(tmpFile, []byte("test"), 0644), "failed to create temp file")

	sURL, _ := url.Parse("https://example.com/pkgA-1.0.0.tgz")
	testReq := model.ResolveRequest{
		Name:               "pkgA",
		Version:            "1.0.0",
		OS:                 "linux",
		Arch:               "amd64",
		InstalledArtifacts: []*model.InstalledArtifact{},
	}

	step := model.ResolvedArtifact{
		ID:        "pkgA@1.0.0",
		Name:      "pkgA",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL,
		Checksum:  "abc123",
	}

	plan := model.ResolvedArtifacts{Artifacts: []model.ResolvedArtifact{step}}

	// Setup mocks
	idx := mocks.NewMockArtifactResolver(ctrl)
	idx.EXPECT().
		Resolve(gomock.Any(), testReq).
		Return(plan, nil).
		Times(1)

	dl := mocks.NewMockDownloader(ctrl)
	dl.EXPECT().
		FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]string{step.ID: tmpFile}, nil).
		Times(1)

	expectedErr := fmt.Errorf("installation failed")
	art := mocks.NewMockArtifactManager(ctrl)
	art.EXPECT().
		GetInstalledArtifacts().
		Return([]*model.InstalledArtifact{}, nil).
		Times(1)
	art.EXPECT().
		InstallArtifact(gomock.Any(), gomock.Any(), tmpFile, gomock.Any()).
		DoAndReturn(func(_ context.Context, desc *model.IndexArtifactDescriptor, path string, reason model.InstallationReason) error {
			assert.Equal(t, step.Name, desc.Name, "artifact name should match")
			assert.Equal(t, step.Version, desc.Version, "artifact version should match")
			assert.Equal(t, step.OS, desc.OS, "artifact OS should match")
			assert.Equal(t, step.Arch, desc.Arch, "artifact arch should match")
			assert.Equal(t, step.Checksum, desc.Checksum, "artifact checksum should match")
			return expectedErr
		}).
		Times(1)

	// Create orchestrator
	torch := &Orchestrator{
		Index:           idx,
		DL:              dl,
		ArtifactManager: art,
	}

	// Execute test
	err := torch.Install(
		context.Background(),
		testReq,
		InstallOptions{
			CacheDir: tmpDir,
		},
	)

	// Verify results
	require.Error(t, err, "should return error when artifact installation fails")
	assert.Same(t, expectedErr, err, "should return the exact error from InstallArtifact")
}

func TestInstall_MissingLocalFile_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	tmpDir := t.TempDir()
	testReq := model.ResolveRequest{
		Name:               "pkgA",
		Version:            "1.0.0",
		OS:                 "linux",
		Arch:               "amd64",
		InstalledArtifacts: []*model.InstalledArtifact{},
	}

	sURL, _ := url.Parse("https://example.com/pkgA-1.0.0.tgz")
	step := model.ResolvedArtifact{
		ID:        "pkgA@1.0.0",
		Name:      "pkgA",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL,
		Checksum:  "abc123",
	}

	plan := model.ResolvedArtifacts{Artifacts: []model.ResolvedArtifact{step}}

	// Setup mocks
	idx := mocks.NewMockArtifactResolver(ctrl)
	idx.EXPECT().
		Resolve(gomock.Any(), testReq).
		Return(plan, nil).
		Times(1)

	dl := mocks.NewMockDownloader(ctrl)
	dl.EXPECT().
		FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]string{"pkgA@1.0.0": ""}, nil).
		Times(1)

	art := mocks.NewMockArtifactManager(ctrl)
	art.EXPECT().
		GetInstalledArtifacts().
		Return([]*model.InstalledArtifact{}, nil).
		Times(1)

	// Create orchestrator
	torch := &Orchestrator{
		Index:           idx,
		DL:              dl,
		ArtifactManager: art,
	}

	// Execute test
	err := torch.Install(
		context.Background(),
		testReq,
		InstallOptions{
			CacheDir: tmpDir,
		},
	)

	// Verify results
	require.Error(t, err, "should return error when local file is missing")
	errMsg := err.Error()
	assert.Contains(t, errMsg, "no local file available",
		"error message should indicate missing local file")
	assert.Contains(t, errMsg, step.ID,
		"error message should include the step ID")
}

func TestUninstall_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	testReq := model.ResolveRequest{
		Name:    "test-artifact",
		Version: "1.0.0",
	}

	// Create mocks
	reverseIdx := mocks.NewMockArtifactReverseResolver(ctrl)
	am := mocks.NewMockArtifactManager(ctrl)

	// Setup expectations
	reverseIdx.EXPECT().
		ReverseResolve(gomock.Any(), testReq).
		Return(model.ResolvedArtifacts{
			Artifacts: []model.ResolvedArtifact{
				{ID: "dep1@1.0.0", Name: "dep1", Version: "1.0.0"},
				{ID: "test-artifact@1.0.0", Name: "test-artifact", Version: "1.0.0"},
			},
		}, nil)

	am.EXPECT().
		UninstallArtifact(gomock.Any(), "dep1", false).
		Return(nil)

	am.EXPECT().
		UninstallArtifact(gomock.Any(), "test-artifact", false).
		Return(nil)

	// Create orchestrator with mocks
	orch := &Orchestrator{
		ReverseIndex:    reverseIdx,
		ArtifactManager: am,
	}

	// Execute test
	err := orch.Uninstall(context.Background(), testReq, UninstallOptions{})

	// Verify results
	require.NoError(t, err, "uninstall should not return an error")
}

func TestUninstall_DryRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	testReq := model.ResolveRequest{
		Name:    "test-artifact",
		Version: "1.0.0",
	}

	// Create mocks
	reverseIdx := mocks.NewMockArtifactReverseResolver(ctrl)

	// Setup expectations - should not call any ArtifactManager methods in dry-run mode
	reverseIdx.EXPECT().
		ReverseResolve(gomock.Any(), testReq).
		Return(model.ResolvedArtifacts{
			Artifacts: []model.ResolvedArtifact{
				{ID: "dep1@1.0.0", Name: "dep1", Version: "1.0.0"},
				{ID: "test-artifact@1.0.0", Name: "test-artifact", Version: "1.0.0"},
			},
		}, nil)

	// Create orchestrator with mocks
	orch := &Orchestrator{
		ReverseIndex: reverseIdx,
		// No ArtifactManager needed for dry-run
	}

	// Execute test with dry-run
	err := orch.Uninstall(context.Background(), testReq, UninstallOptions{DryRun: true})

	// Verify results
	require.NoError(t, err, "uninstall with dry-run should not return an error")
}

func TestUninstall_NoCascade_WithDependencies(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	testReq := model.ResolveRequest{
		Name:    "test-artifact",
		Version: "1.0.0",
	}

	// Create mocks
	reverseIdx := mocks.NewMockArtifactReverseResolver(ctrl)

	// Setup expectations - return multiple artifacts to trigger cascade check
	reverseIdx.EXPECT().
		ReverseResolve(gomock.Any(), testReq).
		Return(model.ResolvedArtifacts{
			Artifacts: []model.ResolvedArtifact{
				{ID: "dep1@1.0.0", Name: "dep1", Version: "1.0.0"},
				{ID: "test-artifact@1.0.0", Name: "test-artifact", Version: "1.0.0"},
			},
		}, nil)

	// Create orchestrator with mocks
	orch := &Orchestrator{
		ReverseIndex: reverseIdx,
	}

	// Execute test with NoCascade option
	err := orch.Uninstall(context.Background(), testReq, UninstallOptions{NoCascade: true})

	// Verify results
	require.Error(t, err, "should return error when NoCascade is true and there are dependencies")
	assert.Contains(t, err.Error(), "has 1 reverse dependencies", "error message should mention reverse dependencies")
}

func TestUninstall_ForceWithNoCascade(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	testReq := model.ResolveRequest{
		Name:    "test-artifact",
		Version: "1.0.0",
	}

	// Create mocks
	am := mocks.NewMockArtifactManager(ctrl)
	reverseIdx := mocks.NewMockArtifactReverseResolver(ctrl)

	// Setup expectations - with Force and NoCascade, it should create a minimal artifact list
	// and only uninstall the requested artifact
	am.EXPECT().
		UninstallArtifact(gomock.Any(), "test-artifact", false).
		Return(nil)

	// Create orchestrator with mocks
	orch := &Orchestrator{
		ArtifactManager: am,
		ReverseIndex:    reverseIdx, // Required even with Force + NoCascade
	}

	// Execute test with both Force and NoCascade
	err := orch.Uninstall(context.Background(), testReq, UninstallOptions{
		NoCascade: true,
		Force:     true,
	})

	// Verify results
	require.NoError(t, err, "uninstall with Force and NoCascade should not return an error")
}

func TestUninstall_NoReverseIndex(t *testing.T) {
	// Setup test data
	testReq := model.ResolveRequest{
		Name:    "test-artifact",
		Version: "1.0.0",
	}

	// Create orchestrator without ReverseIndex
	orch := &Orchestrator{
		ReverseIndex: nil,
	}

	// Execute test
	err := orch.Uninstall(context.Background(), testReq, UninstallOptions{})

	// Verify results
	require.Error(t, err, "should return error when ReverseIndex is nil")
	assert.Contains(t, err.Error(), "reverse index resolver is not configured",
		"error message should indicate missing reverse index resolver")
}

func TestUninstall_NoArtifactManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	testReq := model.ResolveRequest{
		Name:    "test-artifact",
		Version: "1.0.0",
	}

	// Create mocks
	reverseIdx := mocks.NewMockArtifactReverseResolver(ctrl)

	// Setup expectations
	reverseIdx.EXPECT().
		ReverseResolve(gomock.Any(), testReq).
		Return(model.ResolvedArtifacts{
			Artifacts: []model.ResolvedArtifact{
				{ID: "test-artifact@1.0.0", Name: "test-artifact", Version: "1.0.0"},
			},
		}, nil)

	// Create orchestrator with mocks but without ArtifactManager
	orch := &Orchestrator{
		ReverseIndex: reverseIdx,
		// ArtifactManager is nil
	}

	// Execute test
	err := orch.Uninstall(context.Background(), testReq, UninstallOptions{})

	// Verify results
	require.Error(t, err, "should return error when ArtifactManager is nil")
	assert.Contains(t, err.Error(), "artifact uninstaller is not configured",
		"error message should indicate missing artifact uninstaller")
}

func TestUninstall_ArtifactUninstallError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	testReq := model.ResolveRequest{
		Name:    "test-artifact",
		Version: "1.0.0",
	}

	expectedErr := fmt.Errorf("failed to uninstall artifact")

	// Create mocks
	reverseIdx := mocks.NewMockArtifactReverseResolver(ctrl)
	am := mocks.NewMockArtifactManager(ctrl)

	// Setup expectations
	reverseIdx.EXPECT().
		ReverseResolve(gomock.Any(), testReq).
		Return(model.ResolvedArtifacts{
			Artifacts: []model.ResolvedArtifact{
				{ID: "test-artifact@1.0.0", Name: "test-artifact", Version: "1.0.0"},
			},
		}, nil)

	am.EXPECT().
		UninstallArtifact(gomock.Any(), "test-artifact", false).
		Return(expectedErr)

	// Create orchestrator with mocks
	orch := &Orchestrator{
		ReverseIndex:    reverseIdx,
		ArtifactManager: am,
	}

	// Execute test
	err := orch.Uninstall(context.Background(), testReq, UninstallOptions{})

	// Verify results
	require.Error(t, err, "should return error when uninstall fails")
	assert.Equal(t, expectedErr, err, "should return the error from ArtifactManager")
}

func TestInstall_InstallationReason_FirstArtifactManual(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data - single artifact that should be manual
	sURL, _ := url.Parse("https://example.com/pkgA-1.0.0.tgz")
	req := model.ResolveRequest{
		Name:               "pkgA",
		Version:            "1.0.0",
		OS:                 "linux",
		Arch:               "amd64",
		InstalledArtifacts: []*model.InstalledArtifact{},
	}

	step := model.ResolvedArtifact{
		ID:        "pkgA@1.0.0",
		Name:      "pkgA",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL,
		Checksum:  "abc123",
	}

	plan := model.ResolvedArtifacts{Artifacts: []model.ResolvedArtifact{step}}

	// Setup mocks
	idx := mocks.NewMockArtifactResolver(ctrl)
	dl := mocks.NewMockDownloader(ctrl)
	am := mocks.NewMockArtifactManager(ctrl)

	// Setup expectations
	idx.EXPECT().
		Resolve(gomock.Any(), req).
		Return(plan, nil).
		Times(1)

	dl.EXPECT().
		FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]string{step.ID: "/tmp/pkgA-1.0.0.tgz"}, nil).
		Times(1)

	am.EXPECT().
		GetInstalledArtifacts().
		Return([]*model.InstalledArtifact{}, nil).
		Times(1)

	// Expect InstallArtifact call with InstallationReasonManual for the first (and only) artifact
	am.EXPECT().
		InstallArtifact(gomock.Any(), gomock.Any(), "/tmp/pkgA-1.0.0.tgz", model.InstallationReasonManual).
		DoAndReturn(func(_ context.Context, desc *model.IndexArtifactDescriptor, path string, reason model.InstallationReason) error {
			// Verify that the reason is Manual for the primary artifact
			assert.Equal(t, model.InstallationReasonManual, reason, "first artifact should have InstallationReasonManual")
			assert.Equal(t, step.Name, desc.Name, "artifact name should match")
			assert.Equal(t, step.Version, desc.Version, "artifact version should match")
			return nil
		}).
		Times(1)

	// Create orchestrator
	orch := &Orchestrator{
		Index:           idx,
		DL:              dl,
		ArtifactManager: am,
	}

	// Execute test
	err := orch.Install(
		context.Background(),
		req,
		InstallOptions{
			CacheDir: t.TempDir(),
		},
	)

	// Verify results
	require.NoError(t, err, "install should succeed")
}

func TestInstall_InstallationReason_DependenciesAutomatic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data - main artifact with dependency
	sURL, _ := url.Parse("https://example.com/pkgA-1.0.0.tgz")
	depURL, _ := url.Parse("https://example.com/dep-1.0.0.tgz")
	req := model.ResolveRequest{
		Name:               "pkgA",
		Version:            "1.0.0",
		OS:                 "linux",
		Arch:               "amd64",
		InstalledArtifacts: []*model.InstalledArtifact{},
	}

	mainStep := model.ResolvedArtifact{
		ID:        "pkgA@1.0.0",
		Name:      "pkgA",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL,
		Checksum:  "abc123",
	}

	depStep := model.ResolvedArtifact{
		ID:        "dep@1.0.0",
		Name:      "dep",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: depURL,
		Checksum:  "def456",
	}

	plan := model.ResolvedArtifacts{Artifacts: []model.ResolvedArtifact{mainStep, depStep}}

	// Setup mocks
	idx := mocks.NewMockArtifactResolver(ctrl)
	dl := mocks.NewMockDownloader(ctrl)
	am := mocks.NewMockArtifactManager(ctrl)

	// Setup expectations
	idx.EXPECT().
		Resolve(gomock.Any(), req).
		Return(plan, nil).
		Times(1)

	dl.EXPECT().
		FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]string{
			mainStep.ID: "/tmp/pkgA-1.0.0.tgz",
			depStep.ID:  "/tmp/dep-1.0.0.tgz",
		}, nil).
		Times(1)

	am.EXPECT().
		GetInstalledArtifacts().
		Return([]*model.InstalledArtifact{}, nil).
		Times(1)

	// Expect InstallArtifact calls with correct installation reasons
	// First artifact (main) should be manual
	am.EXPECT().
		InstallArtifact(gomock.Any(), gomock.Any(), "/tmp/pkgA-1.0.0.tgz", model.InstallationReasonManual).
		DoAndReturn(func(_ context.Context, desc *model.IndexArtifactDescriptor, path string, reason model.InstallationReason) error {
			assert.Equal(t, model.InstallationReasonManual, reason, "main artifact should have InstallationReasonManual")
			return nil
		}).
		Times(1)

	// Second artifact (dependency) should be automatic
	am.EXPECT().
		InstallArtifact(gomock.Any(), gomock.Any(), "/tmp/dep-1.0.0.tgz", model.InstallationReasonAutomatic).
		DoAndReturn(func(_ context.Context, desc *model.IndexArtifactDescriptor, path string, reason model.InstallationReason) error {
			assert.Equal(t, model.InstallationReasonAutomatic, reason, "dependency should have InstallationReasonAutomatic")
			return nil
		}).
		Times(1)

	// Create orchestrator
	orch := &Orchestrator{
		Index:           idx,
		DL:              dl,
		ArtifactManager: am,
	}

	// Execute test
	err := orch.Install(
		context.Background(),
		req,
		InstallOptions{
			CacheDir: t.TempDir(),
		},
	)

	// Verify results
	require.NoError(t, err, "install should succeed")
}

func TestInstall_InstallationReason_MultipleArtifacts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data - multiple artifacts where only the first matching the request name should be manual
	sURL1, _ := url.Parse("https://example.com/pkgA-1.0.0.tgz")
	sURL2, _ := url.Parse("https://example.com/pkgB-1.0.0.tgz")
	sURL3, _ := url.Parse("https://example.com/pkgC-1.0.0.tgz")

	req := model.ResolveRequest{
		Name:               "pkgA", // Only pkgA should be manual
		Version:            "1.0.0",
		OS:                 "linux",
		Arch:               "amd64",
		InstalledArtifacts: []*model.InstalledArtifact{},
	}

	artifactA := model.ResolvedArtifact{
		ID:        "pkgA@1.0.0",
		Name:      "pkgA",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL1,
		Checksum:  "abc123",
	}

	artifactB := model.ResolvedArtifact{
		ID:        "pkgB@1.0.0",
		Name:      "pkgB",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL2,
		Checksum:  "def456",
	}

	artifactC := model.ResolvedArtifact{
		ID:        "pkgC@1.0.0",
		Name:      "pkgC",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL3,
		Checksum:  "ghi789",
	}

	plan := model.ResolvedArtifacts{Artifacts: []model.ResolvedArtifact{artifactA, artifactB, artifactC}}

	// Setup mocks
	idx := mocks.NewMockArtifactResolver(ctrl)
	dl := mocks.NewMockDownloader(ctrl)
	am := mocks.NewMockArtifactManager(ctrl)

	// Setup expectations
	idx.EXPECT().
		Resolve(gomock.Any(), req).
		Return(plan, nil).
		Times(1)

	dl.EXPECT().
		FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]string{
			artifactA.ID: "/tmp/pkgA-1.0.0.tgz",
			artifactB.ID: "/tmp/pkgB-1.0.0.tgz",
			artifactC.ID: "/tmp/pkgC-1.0.0.tgz",
		}, nil).
		Times(1)

	am.EXPECT().
		GetInstalledArtifacts().
		Return([]*model.InstalledArtifact{}, nil).
		Times(1)

	// pkgA should be manual (matches request name)
	am.EXPECT().
		InstallArtifact(gomock.Any(), gomock.Any(), "/tmp/pkgA-1.0.0.tgz", model.InstallationReasonManual).
		DoAndReturn(func(_ context.Context, desc *model.IndexArtifactDescriptor, path string, reason model.InstallationReason) error {
			assert.Equal(t, model.InstallationReasonManual, reason, "pkgA should have InstallationReasonManual")
			return nil
		}).
		Times(1)

	// pkgB and pkgC should be automatic (dependencies)
	am.EXPECT().
		InstallArtifact(gomock.Any(), gomock.Any(), "/tmp/pkgB-1.0.0.tgz", model.InstallationReasonAutomatic).
		DoAndReturn(func(_ context.Context, desc *model.IndexArtifactDescriptor, path string, reason model.InstallationReason) error {
			assert.Equal(t, model.InstallationReasonAutomatic, reason, "pkgB should have InstallationReasonAutomatic")
			return nil
		}).
		Times(1)

	am.EXPECT().
		InstallArtifact(gomock.Any(), gomock.Any(), "/tmp/pkgC-1.0.0.tgz", model.InstallationReasonAutomatic).
		DoAndReturn(func(_ context.Context, desc *model.IndexArtifactDescriptor, path string, reason model.InstallationReason) error {
			assert.Equal(t, model.InstallationReasonAutomatic, reason, "pkgC should have InstallationReasonAutomatic")
			return nil
		}).
		Times(1)

	// Create orchestrator
	orch := &Orchestrator{
		Index:           idx,
		DL:              dl,
		ArtifactManager: am,
	}

	// Execute test
	err := orch.Install(
		context.Background(),
		req,
		InstallOptions{
			CacheDir: t.TempDir(),
		},
	)

	// Verify results
	require.NoError(t, err, "install should succeed")
}

// TestCleanup_Success tests successful cleanup of orphaned automatic artifacts
func TestCleanup_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	am := mocks.NewMockArtifactManager(ctrl)

	// Set up expectations
	am.EXPECT().
		GetOrphanedAutomaticArtifacts().
		Return([]string{"orphaned1", "orphaned2"}, nil)

	am.EXPECT().
		UninstallArtifact(gomock.Any(), "orphaned1", true).
		Return(nil)

	am.EXPECT().
		UninstallArtifact(gomock.Any(), "orphaned2", true).
		Return(nil)

	// Create orchestrator with hooks to capture events
	var events []Event
	hooks := Hooks{
		OnEvent: func(e Event) {
			events = append(events, e)
		},
	}

	orch := New(nil, nil, nil, am, hooks)

	// Execute cleanup
	cleaned, err := orch.Cleanup(context.Background())

	// Verify results
	require.NoError(t, err)
	require.Len(t, cleaned, 2)
	assert.Contains(t, cleaned, "orphaned1")
	assert.Contains(t, cleaned, "orphaned2")

	// Verify events were emitted
	require.Len(t, events, 3) // 2 cleanup events + 1 done event
	assert.Equal(t, "cleanup", events[0].Phase)
	assert.Equal(t, "orphaned1", events[0].ID)
	assert.Equal(t, "cleanup", events[1].Phase)
	assert.Equal(t, "orphaned2", events[1].ID)
	assert.Equal(t, "done", events[2].Phase)
	assert.Contains(t, events[2].Msg, "cleaned up 2 orphaned artifacts")
}

// TestCleanup_NoOrphanedArtifacts tests cleanup when no orphaned artifacts exist
func TestCleanup_NoOrphanedArtifacts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	am := mocks.NewMockArtifactManager(ctrl)

	// Set up expectations - no orphaned artifacts
	am.EXPECT().
		GetOrphanedAutomaticArtifacts().
		Return([]string{}, nil)

	// Create orchestrator
	orch := New(nil, nil, nil, am, Hooks{})

	// Execute cleanup
	cleaned, err := orch.Cleanup(context.Background())

	// Verify results
	require.NoError(t, err)
	require.Nil(t, cleaned)
}

// TestCleanup_NoArtifactManager tests cleanup when ArtifactManager is not configured
func TestCleanup_NoArtifactManager(t *testing.T) {
	// Create orchestrator without ArtifactManager
	orch := New(nil, nil, nil, nil, Hooks{})

	// Execute cleanup
	cleaned, err := orch.Cleanup(context.Background())

	// Verify results
	require.Error(t, err)
	assert.Contains(t, err.Error(), "artifact manager is not configured")
	require.Nil(t, cleaned)
}

// TestCleanup_GetOrphanedError tests cleanup when getting orphaned artifacts fails
func TestCleanup_GetOrphanedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	am := mocks.NewMockArtifactManager(ctrl)

	// Set up expectations - getting orphaned artifacts fails
	expectedError := fmt.Errorf("database connection failed")
	am.EXPECT().
		GetOrphanedAutomaticArtifacts().
		Return(nil, expectedError)

	// Create orchestrator
	orch := New(nil, nil, nil, am, Hooks{})

	// Execute cleanup
	cleaned, err := orch.Cleanup(context.Background())

	// Verify results
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get orphaned artifacts")
	require.Nil(t, cleaned)
}

// TestCleanup_UninstallError tests cleanup when uninstall fails for some artifacts
func TestCleanup_UninstallError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mocks
	am := mocks.NewMockArtifactManager(ctrl)

	// Set up expectations
	am.EXPECT().
		GetOrphanedAutomaticArtifacts().
		Return([]string{"orphaned1", "orphaned2"}, nil)

	am.EXPECT().
		UninstallArtifact(gomock.Any(), "orphaned1", true).
		Return(nil)

	// Second uninstall fails
	uninstallError := fmt.Errorf("permission denied")
	am.EXPECT().
		UninstallArtifact(gomock.Any(), "orphaned2", true).
		Return(uninstallError)

	// Create orchestrator with hooks to capture events
	var events []Event
	hooks := Hooks{
		OnEvent: func(e Event) {
			events = append(events, e)
		},
	}

	orch := New(nil, nil, nil, am, hooks)

	// Execute cleanup
	cleaned, err := orch.Cleanup(context.Background())

	// Verify results - should succeed but only return successfully cleaned artifacts
	require.NoError(t, err)
	require.Len(t, cleaned, 1)
	assert.Contains(t, cleaned, "orphaned1")
	assert.NotContains(t, cleaned, "orphaned2")

	// Verify events were emitted including error
	require.Len(t, events, 4) // 2 cleanup events + 1 error event + 1 done event
	assert.Equal(t, "cleanup", events[0].Phase)
	assert.Equal(t, "orphaned1", events[0].ID)
	assert.Equal(t, "cleanup", events[1].Phase)
	assert.Equal(t, "orphaned2", events[1].ID)
	assert.Equal(t, "error", events[2].Phase)
	assert.Equal(t, "orphaned2", events[2].ID)
	assert.Contains(t, events[2].Msg, "failed to cleanup")
	assert.Equal(t, "done", events[3].Phase)
	assert.Contains(t, events[3].Msg, "cleaned up 1 orphaned artifacts")
}

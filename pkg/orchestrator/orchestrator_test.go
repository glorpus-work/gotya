package orchestrator

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/cperrin88/gotya/pkg/download"
	dlmocks "github.com/cperrin88/gotya/pkg/download/mocks"
	"github.com/cperrin88/gotya/pkg/index"
	"github.com/cperrin88/gotya/pkg/model"
	ocmocks "github.com/cperrin88/gotya/pkg/orchestrator/mocks"
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

	dl := dlmocks.NewMockManager(ctrl)
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

	dl := dlmocks.NewMockManager(ctrl)
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

	dl := dlmocks.NewMockManager(ctrl)
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
	dl := dlmocks.NewMockManager(ctrl)
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
	s1url, _ := url.Parse("https://example.com/a.tgz")
	req := index.InstallRequest{
		Name:    "pkgA",
		Version: ">= 0.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	plan := index.InstallPlan{
		Steps: []index.InstallStep{
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
	idx := ocmocks.NewMockIndexPlanner(ctrl)
	idx.EXPECT().
		Plan(gomock.Any(), req).
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
		Options{DryRun: true},
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
	assert.Len(t, stepEvents, len(plan.Steps), "should have one event per step")

	for i, step := range plan.Steps {
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
	req := index.InstallRequest{
		Name:    "pkgA",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	step := index.InstallStep{
		ID:        "pkgA@1.0.0",
		Name:      "pkgA",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL,
		Checksum:  "deadbeef",
	}
	plan := index.InstallPlan{Steps: []index.InstallStep{step}}

	// Setup mocks
	dl := dlmocks.NewMockManager(ctrl)
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

	idx := ocmocks.NewMockIndexPlanner(ctrl)
	idx.EXPECT().
		Plan(gomock.Any(), req).
		Return(plan, nil).
		Times(1)

	art := ocmocks.NewMockArtifactInstaller(ctrl)
	expectedArtifactPath := filepath.Join(tmp, "pkgA-1.0.0.tgz")
	art.EXPECT().
		InstallArtifact(gomock.Any(), gomock.Any(), expectedArtifactPath).
		DoAndReturn(func(_ context.Context, desc *model.IndexArtifactDescriptor, path string) error {
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
		Index:    idx,
		DL:       dl,
		Artifact: art,
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
	err := orch.Install(
		context.Background(),
		req,
		Options{
			CacheDir:    tmp,
			Concurrency: 2,
		},
	)

	// Verify results
	require.NoError(t, err, "Install should not return an error")
	assert.True(t, gotDone, "should have received done event")
}

func TestNew(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	idx := ocmocks.NewMockIndexPlanner(ctrl)
	dl := dlmocks.NewMockManager(ctrl)
	am := ocmocks.NewMockArtifactInstaller(ctrl)

	// Call the constructor
	orch := New(idx, dl, am, Hooks{})

	// Verify all fields are properly initialized
	assert.Same(t, idx, orch.Index, "Index field should be set to the provided mock")
	assert.Same(t, dl, orch.DL, "DL field should be set to the provided mock")
	assert.Same(t, am, orch.Artifact, "Artifact field should be set to the provided mock")
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
	req := index.InstallRequest{
		Name:    "pkgA",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	step := index.InstallStep{
		ID:        "pkgA@1.0.0",
		Name:      "pkgA",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL,
	}

	plan := index.InstallPlan{Steps: []index.InstallStep{step}}

	// Setup mocks
	idx := ocmocks.NewMockIndexPlanner(ctrl)
	idx.EXPECT().
		Plan(gomock.Any(), req).
		Return(plan, nil).
		Times(1)

	art := ocmocks.NewMockArtifactInstaller(ctrl)

	// Create orchestrator without download manager
	orch := &Orchestrator{
		Index:    idx,
		Artifact: art,
		Hooks:    Hooks{},
		// DL is intentionally nil
	}
	// Execute test
	err := orch.Install(
		context.Background(),
		req,
		Options{
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
	testReq := index.InstallRequest{
		Name:    "pkgA",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	testOpts := Options{
		CacheDir: t.TempDir(),
	}

	torch := &Orchestrator{
		DL:       dlmocks.NewMockManager(ctrl),
		Artifact: ocmocks.NewMockArtifactInstaller(ctrl),
		Hooks:    Hooks{},
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
	testReq := index.InstallRequest{
		Name:    "pkgA",
		Version: "1.0.0",
	}

	expectedErr := fmt.Errorf("planning failed")

	// Setup mocks
	idx := ocmocks.NewMockIndexPlanner(ctrl)
	idx.EXPECT().
		Plan(gomock.Any(), testReq).
		Return(index.InstallPlan{}, expectedErr).
		Times(1)

	// Create orchestrator with only Index set
	torch := &Orchestrator{
		Index: idx,
		Hooks: Hooks{},
	}
	err := torch.Install(
		context.Background(),
		testReq,
		Options{},
	)

	// Verify results
	require.Error(t, err, "should return error when planning fails")
	assert.Same(t, expectedErr, err, "should return the exact error from Plan")
}

func TestInstall_ArtifactInstallError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "pkgA-1.0.0.tgz")
	require.NoError(t, os.WriteFile(tmpFile, []byte("test"), 0644), "failed to create temp file")

	sURL, _ := url.Parse("https://example.com/pkgA-1.0.0.tgz")
	testReq := index.InstallRequest{
		Name:    "pkgA",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	step := index.InstallStep{
		ID:        "pkgA@1.0.0",
		Name:      "pkgA",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL,
		Checksum:  "abc123",
	}

	plan := index.InstallPlan{Steps: []index.InstallStep{step}}

	// Setup mocks
	idx := ocmocks.NewMockIndexPlanner(ctrl)
	idx.EXPECT().
		Plan(gomock.Any(), testReq).
		Return(plan, nil).
		Times(1)

	dl := dlmocks.NewMockManager(ctrl)
	dl.EXPECT().
		FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]string{step.ID: tmpFile}, nil).
		Times(1)

	expectedErr := fmt.Errorf("installation failed")
	art := ocmocks.NewMockArtifactInstaller(ctrl)
	art.EXPECT().
		InstallArtifact(gomock.Any(), gomock.Any(), tmpFile).
		DoAndReturn(func(_ context.Context, desc *model.IndexArtifactDescriptor, path string) error {
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
		Index:    idx,
		DL:       dl,
		Artifact: art,
	}

	// Execute test
	err := torch.Install(
		context.Background(),
		testReq,
		Options{
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
	testReq := index.InstallRequest{
		Name:    "pkgA",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	sURL, _ := url.Parse("https://example.com/pkgA-1.0.0.tgz")
	step := index.InstallStep{
		ID:        "pkgA@1.0.0",
		Name:      "pkgA",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
		SourceURL: sURL,
		Checksum:  "abc123",
	}

	plan := index.InstallPlan{Steps: []index.InstallStep{step}}

	// Setup mocks
	idx := ocmocks.NewMockIndexPlanner(ctrl)
	idx.EXPECT().
		Plan(gomock.Any(), testReq).
		Return(plan, nil).
		Times(1)

	dl := dlmocks.NewMockManager(ctrl)
	dl.EXPECT().
		FetchAll(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]string{"pkgA@1.0.0": ""}, nil).
		Times(1)

	art := ocmocks.NewMockArtifactInstaller(ctrl)

	// Create orchestrator
	torch := &Orchestrator{
		Index:    idx,
		DL:       dl,
		Artifact: art,
	}

	// Execute test
	err := torch.Install(
		context.Background(),
		testReq,
		Options{
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

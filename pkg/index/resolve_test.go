package index

import (
	"context"
	"testing"

	"github.com/cperrin88/gotya/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager(t *testing.T, artifactsJSON string) *ManagerImpl {
	dir := t.TempDir()
	repo := &Repository{Name: "test-repo"}
	_ = writeIndexFile(t, dir, "test-repo", artifactsJSON)
	return NewManager([]*Repository{repo}, dir)
}

func TestResolve_KeepVersionPreferenceHonored_NoUnnecessaryUpdates(t *testing.T) {
	// Simulate scenario: installing a new, unrelated package should NOT update other installed packages
	// when KeepVersion=true and the old version satisfies constraints.
	mgr := setupTestManager(t, `[
        {"name":"tool","version":"1.0.0","url":"https://ex/tool-1.0","checksum":"tool1"},
        {"name":"lib","version":"1.0.0","url":"https://ex/lib-1.0","checksum":"lib1"},
        {"name":"lib","version":"2.0.0","url":"https://ex/lib-2.0","checksum":"lib2"}
    ]`)

	// Requests mimic orchestrator install: main request plus keep preference for installed artifacts
	plan, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "tool",
			VersionConstraint: "1.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
		{
			Name:        "lib",
			OS:          "linux",
			Arch:        "amd64",
			OldVersion:  "1.0.0",
			KeepVersion: true, // prefer keeping lib@1.0.0 if possible
		},
	})

	require.NoError(t, err)
	require.Len(t, plan.Artifacts, 2)

	// Find entries by name to avoid depending on topo order
	var libStep, toolStep *model.ResolvedArtifact
	for i := range plan.Artifacts {
		if plan.Artifacts[i].Name == "lib" {
			libStep = &plan.Artifacts[i]
		}
		if plan.Artifacts[i].Name == "tool" {
			toolStep = &plan.Artifacts[i]
		}
	}
	require.NotNil(t, libStep)
	require.NotNil(t, toolStep)

	// Expectation: resolver should keep lib at 1.0.0 (skip) and install tool@1.0.0
	assert.Equal(t, "1.0.0", libStep.Version)
	assert.Equal(t, model.ResolvedActionSkip, libStep.Action)
	assert.Contains(t, libStep.Reason, "already at the required version")

	assert.Equal(t, "1.0.0", toolStep.Version)
	assert.Equal(t, model.ResolvedActionInstall, toolStep.Action)
}

func TestResolve_SimpleDependencyChain(t *testing.T) {
	// Test a simple dependency chain: a -> b -> c
	mgr := setupTestManager(t, `[
		{"name":"a","version":"1.0.0","dependencies":[{"name":"b","version_constraint":">= 1.0.0"}],"url":"https://ex/a","checksum":"a1"},
		{"name":"b","version":"1.0.0","dependencies":[{"name":"c","version_constraint":">= 1.0.0"}],"url":"https://ex/b","checksum":"b1"},
		{"name":"c","version":"1.0.0","url":"https://ex/c","checksum":"c1"}
	]`)

	plan, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "a",
			VersionConstraint: "1.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
	})

	require.NoError(t, err)
	require.Len(t, plan.Artifacts, 3)
	assert.Equal(t, "c@1.0.0", plan.Artifacts[0].GetID())
	assert.Equal(t, "b@1.0.0", plan.Artifacts[1].GetID())
	assert.Equal(t, "a@1.0.0", plan.Artifacts[2].GetID())
}

func TestResolve_VersionConflictResolution(t *testing.T) {
	// Test version conflict resolution where two dependencies require different versions of the same package
	mgr := setupTestManager(t, `[
		{"name":"app","version":"1.0.0","dependencies":[
			{"name":"lib-a","version_constraint":">= 1.0.0"},
			{"name":"lib-b","version_constraint":">= 1.0.0"}
		],"url":"https://ex/app","checksum":"app1"},
		{"name":"lib-a","version":"1.0.0","dependencies":[
			{"name":"common-lib","version_constraint":"= 1.0.0"}
		],"url":"https://ex/lib-a","checksum":"liba1"},
		{"name":"lib-b","version":"1.0.0","dependencies":[
			{"name":"common-lib","version_constraint":"= 2.0.0"}
		],"url":"https://ex/lib-b","checksum":"libb1"},
		{"name":"common-lib","version":"1.0.0","url":"https://ex/common-1","checksum":"clib1"},
		{"name":"common-lib","version":"2.0.0","url":"https://ex/common-2","checksum":"clib2"}
	]`)

	_, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "app",
			VersionConstraint: "= 1.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
	})

	// This should fail due to version conflict: common-lib is required as both 1.0.0 and 2.0.0
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "common-lib")
	assert.Contains(t, err.Error(), "= 1.0.0, = 2.0.0")
}

func TestResolve_CyclicDependency(t *testing.T) {
	// Test detection of cyclic dependencies
	mgr := setupTestManager(t, `[
		{"name":"a","version":"1.0.0","dependencies":[{"name":"b","version_constraint":">= 1.0.0"}],"url":"https://ex/a","checksum":"a1"},
		{"name":"b","version":"1.0.0","dependencies":[{"name":"a","version_constraint":">= 1.0.0"}],"url":"https://ex/b","checksum":"b1"}
	]`)

	_, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "a",
			VersionConstraint: "1.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dependency cycle detected")
}

func TestResolve_ComplexDependencyGraph(t *testing.T) {
	// Test a more complex dependency graph with multiple versions and shared dependencies
	mgr := setupTestManager(t, `[
		{"name":"app","version":"1.0.0","dependencies":[
			{"name":"feature-a","version_constraint":">= 1.0.0"},
			{"name":"feature-b","version_constraint":">= 1.0.0"}
		],"url":"https://ex/app","checksum":"app1"},
		{"name":"feature-a","version":"1.0.0","dependencies":[
			{"name":"common-utils","version_constraint":">= 1.0.0, < 2.0.0"},
			{"name":"logger","version_constraint":">= 1.0.0"}
		],"url":"https://ex/feat-a","checksum":"fa1"},
		{"name":"feature-b","version":"1.0.0","dependencies":[
			{"name":"common-utils","version_constraint":">= 1.5.0, < 2.0.0"},
			{"name":"http-client","version_constraint":">= 1.0.0"}
		],"url":"https://ex/feat-b","checksum":"fb1"},
		{"name":"common-utils","version":"1.0.0","url":"https://ex/cu-1.0","checksum":"cu1"},
		{"name":"common-utils","version":"1.5.0","url":"https://ex/cu-1.5","checksum":"cu15"},
		{"name":"logger","version":"1.2.0","url":"https://ex/logger-1.2","checksum":"log12"},
		{"name":"http-client","version":"2.0.0","dependencies":[
			{"name":"logger","version_constraint":">= 1.0.0, < 2.0.0"}
		],"url":"https://ex/http-2.0","checksum":"http2"}
	]`)

	plan, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "app",
			VersionConstraint: "1.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
	})

	require.NoError(t, err)
	// Expected order (one possible valid topological sort):
	// logger, common-utils, http-client, feature-a, feature-b, app
	// or similar - the exact order might vary as long as dependencies come before dependents
	assert.Len(t, plan.Artifacts, 6)

	// Verify all required packages are included
	names := make(map[string]bool)
	for _, step := range plan.Artifacts {
		names[step.Name] = true
	}
	required := []string{"app", "feature-a", "feature-b", "common-utils", "logger", "http-client"}
	for _, name := range required {
		assert.True(t, names[name], "missing required package: %s", name)
	}

	// Verify common-utils version is 1.5.0 (the only version that satisfies both constraints)
	for _, step := range plan.Artifacts {
		if step.Name == "common-utils" {
			assert.Equal(t, "1.5.0", step.Version)
		}
	}
}

func TestResolve_PlatformSpecificDependencies(t *testing.T) {
	// Test that the correct platform-specific dependencies are selected
	mgr := setupTestManager(t, `[
		{"name":"app","version":"1.0.0","dependencies":[
			{"name":"platform-lib","version_constraint":">= 1.0.0"}
		],"url":"https://ex/app","checksum":"app1"},
		{"name":"platform-lib","version":"1.0.0","os":"linux","arch":"amd64","url":"https://ex/linux-amd64/lib","checksum":"lib1"},
		{"name":"platform-lib","version":"1.0.0","os":"darwin","arch":"arm64","url":"https://ex/darwin-arm64/lib","checksum":"lib2"}
	]`)

	t.Run("linux/amd64", func(t *testing.T) {
		plan, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
			{
				Name:              "app",
				VersionConstraint: "1.0.0",
				OS:                "linux",
				Arch:              "amd64",
			},
		})

		require.NoError(t, err)
		require.Len(t, plan.Artifacts, 2)
		assert.Equal(t, "platform-lib@1.0.0", plan.Artifacts[0].GetID())
		assert.Equal(t, "app@1.0.0", plan.Artifacts[1].GetID())
		assert.Equal(t, "linux", plan.Artifacts[0].OS)
		assert.Equal(t, "amd64", plan.Artifacts[0].Arch)
	})

	t.Run("darwin/arm64", func(t *testing.T) {
		plan, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
			{
				Name:              "app",
				VersionConstraint: "1.0.0",
				OS:                "darwin",
				Arch:              "arm64",
			},
		})

		require.NoError(t, err)
		require.Len(t, plan.Artifacts, 2)
		assert.Equal(t, "platform-lib@1.0.0", plan.Artifacts[0].GetID())
		assert.Equal(t, "app@1.0.0", plan.Artifacts[1].GetID())
		assert.Equal(t, "darwin", plan.Artifacts[0].OS)
		assert.Equal(t, "arm64", plan.Artifacts[0].Arch)
	})
}

func TestResolve_NoDependencies(t *testing.T) {
	// Test planning for a package with no dependencies
	mgr := setupTestManager(t, `[{"name":"standalone","version":"1.0.0","url":"https://ex/standalone","checksum":"s1"}]`)

	plan, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "standalone",
			VersionConstraint: "1.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
	})

	require.NoError(t, err)
	require.Len(t, plan.Artifacts, 1)
	assert.Equal(t, "standalone@1.0.0", plan.Artifacts[0].GetID())
}

func TestResolve_NonExistentPackage(t *testing.T) {
	// Test behavior when the requested package doesn't exist
	mgr := setupTestManager(t, `[{"name":"exists","version":"1.0.0","url":"https://ex/exists","checksum":"e1"}]`)

	_, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "nonexistent",
			VersionConstraint: "1.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "linux/amd64")
}

func TestResolve_WithInstalledArtifacts_CompatibleVersions(t *testing.T) {
	// Test that the resolver considers installed artifacts and chooses compatible versions
	mgr := setupTestManager(t, `[
		{"name":"app","version":"2.0.0","dependencies":[
			{"name":"lib","version_constraint":">= 1.0.0"}
		],"url":"https://ex/app-2.0","checksum":"app2"},
		{"name":"lib","version":"1.0.0","url":"https://ex/lib-1.0","checksum":"lib1"},
		{"name":"lib","version":"2.0.0","url":"https://ex/lib-2.0","checksum":"lib2"}
	]`)

	// Simulate having lib@1.0.0 already installed

	plan, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "app",
			VersionConstraint: "2.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
		// Add keep preference for installed artifact
		{
			Name:              "lib",
			VersionConstraint: "", // No hard constraint, just preference
			OS:                "linux",
			Arch:              "amd64",
			OldVersion:        "1.0.0",
			KeepVersion:       true, // Prefer to keep current version
		},
	})

	require.NoError(t, err)
	require.Len(t, plan.Artifacts, 2)

	// With KeepVersion=true and constraint ">= 1.0.0", prefer keeping lib@1.0.0
	// Validate by finding entries by name
	var libStep, appStep *model.ResolvedArtifact
	for i := range plan.Artifacts {
		if plan.Artifacts[i].Name == "lib" {
			libStep = &plan.Artifacts[i]
		}
		if plan.Artifacts[i].Name == "app" {
			appStep = &plan.Artifacts[i]
		}
	}
	require.NotNil(t, libStep)
	require.NotNil(t, appStep)

	assert.Equal(t, "1.0.0", libStep.Version)
	assert.Equal(t, model.ResolvedActionSkip, libStep.Action)
	assert.Contains(t, libStep.Reason, "already at the required version")

	assert.Equal(t, "2.0.0", appStep.Version)
	assert.Equal(t, model.ResolvedActionInstall, appStep.Action)
}

func TestResolve_WithInstalledArtifacts_IncompatibleVersions(t *testing.T) {
	// Test version conflict resolution when installed artifacts are incompatible
	mgr := setupTestManager(t, `[
		{"name":"app","version":"2.0.0","dependencies":[
			{"name":"lib","version_constraint":">= 2.0.0"}
		],"url":"https://ex/app-2.0","checksum":"app2"},
		{"name":"lib","version":"2.0.0","url":"https://ex/lib-2.0","checksum":"lib2"},
		{"name":"lib","version":"3.0.0","url":"https://ex/lib-3.0","checksum":"lib3"}
	]`)

	// Simulate having lib@1.0.0 already installed (incompatible with app requirement)
	plan, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "app",
			VersionConstraint: "2.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
		// Add keep preference for installed artifact
		{
			Name:              "lib",
			VersionConstraint: "", // No hard constraint, just preference
			OS:                "linux",
			Arch:              "amd64",
			OldVersion:        "1.0.0",
			KeepVersion:       true, // Prefer to keep current version
		},
	})

	require.NoError(t, err)
	require.Len(t, plan.Artifacts, 2)

	// Should update lib to the latest version that satisfies >= 2.0.0 constraint
	var libID string
	for _, artifact := range plan.Artifacts {
		if artifact.Name == "lib" {
			libID = artifact.GetID()
			break
		}
	}
	assert.Contains(t, []string{"lib@2.0.0", "lib@3.0.0"}, libID, "lib should be updated to satisfy >= 2.0.0 constraint")

	// Verify the action and reason
	var libAction model.ResolvedAction
	for _, artifact := range plan.Artifacts {
		if artifact.Name == "lib" {
			libAction = artifact.Action
			break
		}
	}
	assert.Equal(t, model.ResolvedActionUpdate, libAction, "lib should be updated")

	assert.Equal(t, "app@2.0.0", plan.Artifacts[1].GetID())
	assert.Equal(t, model.ResolvedActionInstall, plan.Artifacts[1].Action)
}

func TestResolve_WithInstalledArtifacts_SkipWhenCompatible(t *testing.T) {
	// Test that compatible installed artifacts are skipped
	mgr := setupTestManager(t, `[
		{"name":"app","version":"1.0.0","dependencies":[
			{"name":"lib","version_constraint":">= 1.0.0"}
		],"url":"https://ex/app-1.0","checksum":"app1"},
		{"name":"lib","version":"1.0.0","url":"https://ex/lib-1.0","checksum":"lib1"}
	]`)

	// Simulate having lib@1.0.0 already installed (compatible)
	plan, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "app",
			VersionConstraint: "1.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
		// Add keep preference for installed artifact
		{
			Name:              "lib",
			VersionConstraint: "", // No hard constraint, just preference
			OS:                "linux",
			Arch:              "amd64",
			OldVersion:        "1.0.0",
			KeepVersion:       true, // Prefer to keep current version
		},
	})

	require.NoError(t, err)
	require.Len(t, plan.Artifacts, 2)

	// Should skip lib since it's already at the correct version
	assert.Equal(t, "lib@1.0.0", plan.Artifacts[0].GetID())
	assert.Equal(t, model.ResolvedActionSkip, plan.Artifacts[0].Action)
	assert.Contains(t, plan.Artifacts[0].Reason, "already at the required version")

	assert.Equal(t, "app@1.0.0", plan.Artifacts[1].GetID())
	assert.Equal(t, model.ResolvedActionInstall, plan.Artifacts[1].Action)
}

func TestResolve_WithInstalledArtifacts_ComplexScenario(t *testing.T) {
	// Test complex scenario with multiple installed artifacts and dependencies
	mgr := setupTestManager(t, `[
		{"name":"app","version":"3.0.0","dependencies":[
			{"name":"lib-a","version_constraint":">= 2.0.0"},
			{"name":"lib-b","version_constraint":">= 2.0.0"}
		],"url":"https://ex/app-3.0","checksum":"app3"},
		{"name":"lib-a","version":"2.0.0","url":"https://ex/lib-a-2.0","checksum":"liba2"},
		{"name":"lib-a","version":"3.0.0","url":"https://ex/lib-a-3.0","checksum":"liba3"},
		{"name":"lib-b","version":"1.0.0","url":"https://ex/lib-b-1.0","checksum":"libb1"},
		{"name":"lib-b","version":"2.0.0","url":"https://ex/lib-b-2.0","checksum":"libb2"}
	]`)

	// Simulate having lib-a@2.0.0 and lib-b@1.0.0 already installed
	plan, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "app",
			VersionConstraint: "3.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
		// Add keep preferences for installed artifacts
		{
			Name:              "lib-a",
			VersionConstraint: "", // No hard constraint, just preference
			OS:                "linux",
			Arch:              "amd64",
			OldVersion:        "2.0.0",
			KeepVersion:       true, // Prefer to keep current version
		},
		{
			Name:              "lib-b",
			VersionConstraint: "", // No hard constraint, just preference
			OS:                "linux",
			Arch:              "amd64",
			OldVersion:        "1.0.0",
			KeepVersion:       true, // Prefer to keep current version
		},
	})

	require.NoError(t, err)
	require.Len(t, plan.Artifacts, 3)

	var libAAction, libBAction, appAction model.ResolvedAction
	for _, artifact := range plan.Artifacts {
		switch artifact.Name {
		case "lib-a":
			libAAction = artifact.Action
		case "lib-b":
			libBAction = artifact.Action
		case "app":
			appAction = artifact.Action
		}
	}

	assert.Equal(t, model.ResolvedActionSkip, libAAction, "lib-a should be kept")
	assert.Equal(t, model.ResolvedActionUpdate, libBAction, "lib-b should be updated to latest version")
	assert.Equal(t, model.ResolvedActionInstall, appAction, "app should be installed")
}

func TestResolve_MultipleRequestsWithMixedKeepVersion(t *testing.T) {
	// Test multiple requests with mixed KeepVersion settings
	mgr := setupTestManager(t, `[
		{"name":"app","version":"2.0.0","dependencies":[
			{"name":"lib-a","version_constraint":">= 1.0.0"},
			{"name":"lib-b","version_constraint":">= 1.0.0"},
			{"name":"lib-c","version_constraint":">= 2.0.0"}
		],"url":"https://ex/app-2.0","checksum":"app2"},
		{"name":"lib-a","version":"1.0.0","url":"https://ex/lib-a-1.0","checksum":"liba1"},
		{"name":"lib-a","version":"2.0.0","url":"https://ex/lib-a-2.0","checksum":"liba2"},
		{"name":"lib-b","version":"1.0.0","url":"https://ex/lib-b-1.0","checksum":"libb1"},
		{"name":"lib-b","version":"2.0.0","url":"https://ex/lib-b-2.0","checksum":"libb2"},
		{"name":"lib-c","version":"1.0.0","url":"https://ex/lib-c-1.0","checksum":"libc1"},
		{"name":"lib-c","version":"2.0.0","url":"https://ex/lib-c-2.0","checksum":"libc2"}
	]`)

	// Test with mixed KeepVersion settings
	plan, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "app",
			VersionConstraint: "2.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
		// lib-a: KeepVersion=true (prefer to keep current)
		{
			Name:              "lib-a",
			VersionConstraint: "",
			OS:                "linux",
			Arch:              "amd64",
			OldVersion:        "1.0.0",
			KeepVersion:       true,
		},
		// lib-b: KeepVersion=false (always update)
		{
			Name:              "lib-b",
			VersionConstraint: "",
			OS:                "linux",
			Arch:              "amd64",
			OldVersion:        "1.0.0",
			KeepVersion:       false,
		},
		{
			Name:              "lib-c",
			VersionConstraint: "",
			OS:                "linux",
			Arch:              "amd64",
			OldVersion:        "1.0.0",
			KeepVersion:       true,
		},
	})

	require.NoError(t, err)
	require.Len(t, plan.Artifacts, 4)

	// Verify actions based on KeepVersion settings
	var libAAction, libBAction, libCAction, appAction model.ResolvedAction
	for _, artifact := range plan.Artifacts {
		switch artifact.Name {
		case "lib-a":
			libAAction = artifact.Action
		case "lib-b":
			libBAction = artifact.Action
		case "lib-c":
			libCAction = artifact.Action
		case "app":
			appAction = artifact.Action
		}
	}

	// lib-a should be updated (even with KeepVersion=true, latest version wins)
	assert.Equal(t, model.ResolvedActionSkip, libAAction, "lib-a should be kept")
	// lib-b should be updated (KeepVersion=false)
	assert.Equal(t, model.ResolvedActionUpdate, libBAction, "lib-b should be updated")
	// lib-c should be updated (KeepVersion=true, dependency version wins)
	assert.Equal(t, model.ResolvedActionUpdate, libCAction, "lib-c should be updated")
	// app should be installed
	assert.Equal(t, model.ResolvedActionInstall, appAction, "app should be installed")
}

func TestResolve_ConstraintPriority(t *testing.T) {
	// Test that hard constraints take priority over keep preferences
	mgr := setupTestManager(t, `[
		{"name":"app","version":"3.0.0","dependencies":[
			{"name":"lib","version_constraint":">= 2.0.0"}
		],"url":"https://ex/app-3.0","checksum":"app3"},
		{"name":"lib","version":"2.0.0","url":"https://ex/lib-2.0","checksum":"lib2"},
		{"name":"lib","version":"3.0.0","url":"https://ex/lib-3.0","checksum":"lib3"}
	]`)

	// Request app@3.0.0 which requires lib >= 2.0.0
	// Even though we prefer to keep lib@1.0.0, the hard constraint should win
	plan, err := mgr.Resolve(context.Background(), []model.ResolveRequest{
		{
			Name:              "app",
			VersionConstraint: "3.0.0",
			OS:                "linux",
			Arch:              "amd64",
		},
		{
			Name:              "lib",
			VersionConstraint: "", // No hard constraint
			OS:                "linux",
			Arch:              "amd64",
			OldVersion:        "1.0.0",
			KeepVersion:       true, // Prefer to keep 1.0.0
		},
	})

	require.NoError(t, err)
	require.Len(t, plan.Artifacts, 2)

	// lib should be updated to 3.0.0 (latest that satisfies >= 2.0.0)
	var libID string
	for _, artifact := range plan.Artifacts {
		if artifact.Name == "lib" {
			libID = artifact.GetID()
		}
	}
	assert.Equal(t, "lib@3.0.0", libID, "lib should be updated to latest version that satisfies hard constraint")

	// Verify the action
	var libAction model.ResolvedAction
	for _, artifact := range plan.Artifacts {
		if artifact.Name == "lib" {
			libAction = artifact.Action
		}
	}
	assert.Equal(t, model.ResolvedActionUpdate, libAction, "lib should be marked as update")
}

func TestResolve_EmptyRequestsList(t *testing.T) {
	// Test that empty requests list returns error
	mgr := setupTestManager(t, `[{"name":"test","version":"1.0.0","url":"https://ex/test","checksum":"test1"}]`)

	_, err := mgr.Resolve(context.Background(), []model.ResolveRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no resolve requests provided")
}

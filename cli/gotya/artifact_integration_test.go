//go:build integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArtifact_CreateAndVerify_Success(t *testing.T) {
	tempDir := t.TempDir()
	src := createSampleArtifactSource(t, tempDir)
	outDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(outDir, 0o755))

	name := "sample"
	version := "1.2.3"
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Capture stdout for create
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"artifact", "create",
		"--source", src,
		"--name", name,
		"--version", version,
		"--os", goos,
		"--arch", goarch,
		"--output", outDir,
	})
	err := cmd.ExecuteContext(context.Background())

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	require.NoError(t, err, "artifact create should succeed")

	// Parse created file path and assert it exists
	createdPath := parseCreatedArtifactPath(t, out)
	if _, statErr := os.Stat(createdPath); statErr != nil {
		t.Fatalf("expected created artifact to exist at %s: %v\ncreate output: %s", createdPath, statErr, out)
	}

	// Verify using flag -f
	verifyCmd := newRootCmd()
	verifyCmd.SetArgs([]string{"artifact", "verify", "-f", createdPath})
	require.NoError(t, verifyCmd.ExecuteContext(context.Background()))

	// Also verify using positional argument
	verifyCmd2 := newRootCmd()
	verifyCmd2.SetArgs([]string{"artifact", "verify", createdPath})
	require.NoError(t, verifyCmd2.ExecuteContext(context.Background()))
}

func TestArtifact_Verify_FailsForMissingFile(t *testing.T) {
	// Use a path we know does not exist
	missing := filepath.Join(t.TempDir(), "does-not-exist.gotya")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"artifact", "verify", "-f", missing})
	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestArtifact_Create_FailsForInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	// Build a source directory with an invalid top-level file
	src := filepath.Join(tempDir, "badsrc")
	require.NoError(t, os.MkdirAll(src, 0o755))
	// Add a forbidden file at the root (only meta/ and data/ are allowed)
	require.NoError(t, os.WriteFile(filepath.Join(src, "FORBIDDEN.txt"), []byte("nope"), 0o644))

	outDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(outDir, 0o755))

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"artifact", "create",
		"--source", src,
		"--name", "bad",
		"--version", "0.0.1",
		"--os", runtime.GOOS,
		"--arch", runtime.GOARCH,
		"--output", outDir,
	})
	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "not allowed")

	// Also validate that nothing got created in outDir matching the expected name
	expected := filepath.Join(outDir, fmt.Sprintf("%s_%s_%s_%s.gotya", "bad", "0.0.1", runtime.GOOS, runtime.GOARCH))
	if _, statErr := os.Stat(expected); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact to be created at %s; statErr=%v", expected, statErr)
	}
}

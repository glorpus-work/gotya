package archive

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveManager_ExtractAll(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create test files and directories
	testFiles := map[string]string{
		"meta/artifact.json":    `{"name":"test","version":"1.0.0"}`,
		"data/file1.txt":        "Hello World",
		"data/subdir/file2.txt": "Hello World 2",
	}

	// Create source directory structure
	sourceDir := filepath.Join(tempDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(sourceDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Create archive manager
	am := NewManager()

	// Create archive
	archivePath := filepath.Join(tempDir, "test.tar.gz")
	ctx := context.Background()
	if err := am.Create(ctx, sourceDir, archivePath); err != nil {
		t.Fatalf("Failed to create archive: %v", err)
	}

	// Verify archive was created
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatalf("Archive was not created")
	}

	// Extract archive
	extractDir := filepath.Join(tempDir, "extracted")
	if err := am.ExtractAll(ctx, archivePath, extractDir); err != nil {
		t.Fatalf("Failed to extract archive: %v", err)
	}

	// Verify extracted files
	for path, expectedContent := range testFiles {
		fullPath := filepath.Join(extractDir, path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("File %s was not extracted", path)
			continue
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("Failed to read extracted file %s: %v", path, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("File %s has wrong content. Expected: %s, Got: %s", path, expectedContent, string(content))
		}
	}
}

func TestArchiveManager_ExtractFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create test files and directories
	testFiles := map[string]string{
		"meta/artifact.json": `{"name":"test","version":"1.0.0"}`,
		"data/file1.txt":     "Hello World",
		"data/file2.txt":     "Hello World 2",
	}

	// Create source directory structure
	sourceDir := filepath.Join(tempDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(sourceDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Create archive manager
	am := NewManager()

	// Create archive
	archivePath := filepath.Join(tempDir, "test.tar.gz")
	ctx := context.Background()
	if err := am.Create(ctx, sourceDir, archivePath); err != nil {
		t.Fatalf("Failed to create archive: %v", err)
	}

	// Extract specific file
	extractPath := filepath.Join(tempDir, "extracted_file.txt")
	if err := am.ExtractFile(ctx, archivePath, "data/file1.txt", extractPath); err != nil {
		t.Fatalf("Failed to extract file: %v", err)
	}

	// Verify extracted file
	expectedContent := "Hello World"
	content, err := os.ReadFile(extractPath)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}

	if string(content) != expectedContent {
		t.Errorf("Extracted file has wrong content. Expected: %s, Got: %s", expectedContent, string(content))
	}
}

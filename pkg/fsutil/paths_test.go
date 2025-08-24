package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnsureDirs(t *testing.T) {
	// Clean up after test
	t.Cleanup(func() {
		cacheDir, _ := GetCacheDir()
		dataDir, _ := GetDataDir()
		os.RemoveAll(cacheDir)
		os.RemoveAll(dataDir)
	})

	err := EnsureDirs()
	assert.NoError(t, err)

	// Verify directories were created
	cacheDir, _ := GetCacheDir()
	dataDir, _ := GetDataDir()

	assert.DirExists(t, filepath.Join(cacheDir, "packages"))
	assert.DirExists(t, filepath.Join(dataDir, "installed"))
	assert.DirExists(t, filepath.Join(dataDir, "meta"))
}

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
		cacheDir, err := GetCacheDir()
		if err != nil {
			t.Errorf("failed to get cache dir: %v", err)
			return
		}
		dataDir, err := GetDataDir()
		if err != nil {
			t.Errorf("failed to get data dir: %v", err)
			return
		}
		_ = os.RemoveAll(cacheDir) // Best effort cleanup
		_ = os.RemoveAll(dataDir)  // Best effort cleanup
	})

	err := EnsureDirs()
	assert.NoError(t, err)

	// Verify directories were created
	cacheDir, err := GetCacheDir()
	if !assert.NoError(t, err, "should get cache directory") {
		t.FailNow()
	}

	dataDir, err := GetDataDir()
	if !assert.NoError(t, err, "should get data directory") {
		t.FailNow()
	}

	assert.DirExists(t, filepath.Join(cacheDir, "packages"))
	assert.DirExists(t, filepath.Join(dataDir, "installed"))
	assert.DirExists(t, filepath.Join(dataDir, "meta"))
}

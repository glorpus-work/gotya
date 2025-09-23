package index

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/cperrin88/gotya/pkg/fsutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeIndexFile(t *testing.T, dir, repoName string, artifactsJSON string) string {
	indexPath := filepath.Join(dir, repoName+".json")
	require.NoError(t, os.MkdirAll(filepath.Dir(indexPath), fsutil.DirModeDefault))
	data := []byte(`{
  "format_version": "1.0",
  "last_update": "2024-08-16T10:00:00Z",
  "packages": ` + artifactsJSON + `
}`)
	require.NoError(t, os.WriteFile(indexPath, data, fsutil.FileModeDefault))
	return indexPath
}

func TestManager_ListRepositories(t *testing.T) {
	repos := []*Repository{{Name: "r1"}, {Name: "r2"}}
	m := NewManager(repos, t.TempDir())
	got := m.ListRepositories()
	require.Len(t, got, 2)
	assert.Equal(t, "r1", got[0].Name)
	assert.Equal(t, "r2", got[1].Name)
}

func TestManager_GetIndex_ErrorWhenMissing(t *testing.T) {
	dir := t.TempDir()
	repos := []*Repository{{Name: "r1"}}
	m := NewManager(repos, dir)
	idx, err := m.GetIndex("r1")
	assert.Nil(t, idx)
	assert.Error(t, err)
}

func TestManager_FindArtifacts_SingleRepo(t *testing.T) {
	dir := t.TempDir()
	repo := &Repository{Name: "repo1"}
	_ = writeIndexFile(t, dir, "repo1", `[
    {"name":"foo","version":"1.0.0","description":"d","url":"https://ex/","checksum":"x"},
    {"name":"bar","version":"0.1.0","description":"d","url":"https://ex/","checksum":"y"}
  ]`)
	m := NewManager([]*Repository{repo}, dir)

	pkgs, err := m.FindArtifacts("foo")
	require.NoError(t, err)
	require.Contains(t, pkgs, "repo1")
	require.Len(t, pkgs["repo1"], 1)
	assert.Equal(t, "foo", pkgs["repo1"][0].Name)
}

func TestManager_ResolveArtifact_Basic(t *testing.T) {
	dir := t.TempDir()
	repo := &Repository{
		Name:     "test-repo",
		URL:      &url.URL{Scheme: "https", Host: "example.com", Path: "/index.json"},
		Priority: 1,
		Enabled:  true,
	}
	_ = writeIndexFile(t, dir, "test-repo", `[
    {"name":"test-artifact","version":"1.0.0","description":"d","url":"https://ex/1","checksum":"c1"}
  ]`)

	mgr := NewManager([]*Repository{repo}, dir)
	pkg, err := mgr.ResolveArtifact("test-artifact", ">= 0.0.0", "linux", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "test-artifact", pkg.Name)
	assert.Equal(t, "1.0.0", pkg.Version)
}

func TestManager_ResolveArtifact_VersionAcrossPriorities(t *testing.T) {
	dir := t.TempDir()
	repoHi := &Repository{Name: "hi", Priority: 2}
	repoLo := &Repository{Name: "lo", Priority: 1}
	_ = writeIndexFile(t, dir, "hi", `[
    {"name":"a","version":"1.0.0","description":"","url":"https://ex/","checksum":"c"}
  ]`)
	_ = writeIndexFile(t, dir, "lo", `[
    {"name":"a","version":"2.0.0","description":"","url":"https://ex/","checksum":"c"}
  ]`)
	m := NewManager([]*Repository{repoHi, repoLo}, dir)

	pkg, err := m.ResolveArtifact("a", ">= 0.0.0", "linux", "amd64")
	require.NoError(t, err)
	// Given current implementation, lower-priority repo can win if version is newer
	assert.Equal(t, "2.0.0", pkg.Version)
}

func TestManager_ResolveArtifact_OSArchFilter(t *testing.T) {
	dir := t.TempDir()
	repo := &Repository{Name: "r"}
	_ = writeIndexFile(t, dir, "r", `[
    {"name":"a","version":"1.0.0","os":"linux","arch":"amd64","description":"","url":"https://ex/","checksum":"c"}
  ]`)
	m := NewManager([]*Repository{repo}, dir)

	_, err := m.ResolveArtifact("a", ">= 0.0.0", "darwin", "arm64")
	assert.ErrorIs(t, err, ErrArtifactNotFound)

	pkg, err := m.ResolveArtifact("a", ">= 0.0.0", "linux", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", pkg.Version)
}

func TestManager_Reload(t *testing.T) {
	dir := t.TempDir()
	repo := &Repository{Name: "r"}
	path := writeIndexFile(t, dir, "r", `[{"name":"a","version":"1.0.0","description":"","url":"https://ex/","checksum":"c"}]`)
	m := NewManager([]*Repository{repo}, dir)

	pkg, err := m.ResolveArtifact("a", ">= 0.0.0", "linux", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", pkg.Version)

	// overwrite index with newer version and reload
	_ = os.WriteFile(path, []byte(`{
  "format_version": "1.0",
  "last_update": "2024-08-16T10:00:00Z",
  "packages": [
    {"name":"a","version":"1.2.3","description":"","url":"https://ex/","checksum":"c"}
  ]
}`), fsutil.FileModeDefault)
	require.NoError(t, m.Reload())

	pkg2, err := m.ResolveArtifact("a", ">= 0.0.0", "linux", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", pkg2.Version)
}

package artifact

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	mockhttp "github.com/cperrin88/gotya/pkg/http/mocks"
	mockindex "github.com/cperrin88/gotya/pkg/index/mocks"
	"github.com/cperrin88/gotya/pkg/model"
	"github.com/cperrin88/gotya/pkg/platform"
	"github.com/mholt/archives"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIndexMgr := mockindex.NewMockManager(ctrl)
	mockHttpClient := mockhttp.NewMockClient(ctrl)

	mgr := NewManager(mockIndexMgr, mockHttpClient, platform.OSLinux, platform.ArchAMD64, t.TempDir())

	assert.NotNil(t, mgr)
	assert.Equal(t, mockIndexMgr, mgr.indexManager)
}

func TestInstallArtifact_Success(t *testing.T) {
	t.Skip()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	pkgName := "test-package"
	version := "1.0.0"
	osName := "linux"
	archName := "amd64"
	pkgURL := "https://example.com/pkg/test-package_1.0.0_linux_amd64.gotya"

	// Create mocks
	mockPkg := &model.IndexArtifactDescriptor{
		Name:    pkgName,
		Version: version,
		OS:      osName,
		Arch:    archName,
		URL:     pkgURL,
	}

	mockIndexMgr := mockindex.NewMockManager(ctrl)
	mockHTTPClient := mockhttp.NewMockClient(ctrl)

	// Set up expectations
	mockIndexMgr.EXPECT().
		ResolveArtifact(pkgName, version, osName, archName).
		Return(mockPkg, nil)

	parsedURL, err := url.Parse(pkgURL)
	if err != nil {
		t.Fatalf("failed to parse package URL: %v", err)
	}
	mockHTTPClient.EXPECT().
		DownloadArtifact(gomock.Any(), parsedURL, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ *url.URL, outPath string) error {
			// Create a minimal valid archive with meta/artifact.json
			temp := t.TempDir()
			metaDir := filepath.Join(temp, artifactMetaDir)
			if err := os.MkdirAll(metaDir, 0o755); err != nil {
				return err
			}
			meta := &Metadata{
				Name:    pkgName,
				Version: version,
				OS:      osName,
				Arch:    archName,
				Hashes:  map[string]string{},
			}
			f, err := os.Create(filepath.Join(metaDir, metadataFile))
			if err != nil {
				return err
			}
			if err := json.NewEncoder(f).Encode(meta); err != nil {
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			ctx := context.Background()
			files, err := archives.FilesFromDisk(ctx, nil, map[string]string{temp + "/": ""})
			if err != nil {
				return err
			}
			outFile, err := os.Create(outPath)
			if err != nil {
				return err
			}
			defer outFile.Close()
			format := archives.CompressedArchive{Compression: archives.Gz{}, Archival: archives.Tar{}}
			return format.Archive(ctx, outFile, files)
		})

	// Create manager with mocks
	mgr := &ManagerImpl{
		indexManager:     mockIndexMgr,
		httpClient:       mockHTTPClient,
		os:               osName,
		arch:             archName,
		artifactCacheDir: t.TempDir(),
	}

	// Test
	err = mgr.InstallArtifact(context.Background(), pkgName, version, false)

	// Assert
	assert.NoError(t, err)
}

func TestInstallArtifact_ResolveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	expectedErr := errors.New("package not found")

	// Create mocks
	mockIndexMgr := mockindex.NewMockManager(ctrl)

	// Set up expectations
	mockIndexMgr.EXPECT().
		ResolveArtifact("invalid-artifact", "1.0.0", "linux", "amd64").
		Return(nil, expectedErr)

	// Create manager with mocks
	mgr := &ManagerImpl{
		indexManager: mockIndexMgr,
		httpClient:   mockhttp.NewMockClient(ctrl),
		os:           "linux",
		arch:         "amd64",
	}

	// Test
	err := mgr.InstallArtifact(context.Background(), "invalid-artifact", "1.0.0", false)

	// Assert
	assert.EqualError(t, err, expectedErr.Error())
}

func TestInstallArtifact_DownloadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	pkgName := "test-package"
	version := "1.0.0"
	os := "linux"
	arch := "amd64"
	pkgURL := "https://example.com/pkg/test-package-1.0.0-linux-amd64.tar.gz"
	downloadErr := errors.New("download failed")

	// Create mocks
	mockPkg := &model.IndexArtifactDescriptor{
		Name:    pkgName,
		Version: version,
		OS:      os,
		Arch:    arch,
		URL:     pkgURL,
	}

	mockIndexMgr := mockindex.NewMockManager(ctrl)
	mockHTTPClient := mockhttp.NewMockClient(ctrl)

	// Set up expectations
	mockIndexMgr.EXPECT().
		ResolveArtifact(pkgName, version, os, arch).
		Return(mockPkg, nil)

	parsedURL, err := url.Parse(pkgURL)
	if err != nil {
		t.Fatalf("failed to parse package URL: %v", err)
	}
	mockHTTPClient.EXPECT().
		DownloadArtifact(gomock.Any(), parsedURL, gomock.Any()).
		Return(downloadErr)

	// Create manager with mocks
	mgr := &ManagerImpl{
		indexManager: mockIndexMgr,
		httpClient:   mockHTTPClient,
		os:           os,
		arch:         arch,
	}

	// Test
	err = mgr.InstallArtifact(context.Background(), pkgName, version, false)

	// Assert
	assert.EqualError(t, err, downloadErr.Error())
}

package pkg

import (
	"context"
	"errors"
	"net/url"
	"testing"

	mockhttp "github.com/cperrin88/gotya/pkg/http/mocks"
	"github.com/cperrin88/gotya/pkg/index"
	mockindex "github.com/cperrin88/gotya/pkg/index/mocks"
	"github.com/cperrin88/gotya/pkg/platform"
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

func TestInstallPackage_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	pkgName := "test-package"
	version := "1.0.0"
	os := "linux"
	arch := "amd64"
	pkgURL := "https://example.com/pkg/test-package-1.0.0-linux-amd64.tar.gz"

	// Create mocks
	mockPkg := &index.Package{
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
		ResolvePackage(pkgName, version, os, arch).
		Return(mockPkg, nil)

	parsedURL, _ := url.Parse(pkgURL)
	mockHTTPClient.EXPECT().
		DownloadPackage(gomock.Any(), parsedURL, "").
		Return(nil)

	// Create manager with mocks
	mgr := &ManagerImpl{
		indexManager: mockIndexMgr,
		httpClient:   mockHTTPClient,
		os:           os,
		arch:         arch,
	}

	// Test
	err := mgr.InstallPackage(context.Background(), pkgName, version, false)

	// Assert
	assert.NoError(t, err)
}

func TestInstallPackage_ResolveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup test data
	expectedErr := errors.New("package not found")

	// Create mocks
	mockIndexMgr := mockindex.NewMockManager(ctrl)

	// Set up expectations
	mockIndexMgr.EXPECT().
		ResolvePackage("invalid-pkg", "1.0.0", "linux", "amd64").
		Return(nil, expectedErr)

	// Create manager with mocks
	mgr := &ManagerImpl{
		indexManager: mockIndexMgr,
		httpClient:   mockhttp.NewMockClient(ctrl),
		os:           "linux",
		arch:         "amd64",
	}

	// Test
	err := mgr.InstallPackage(context.Background(), "invalid-pkg", "1.0.0", false)

	// Assert
	assert.EqualError(t, err, expectedErr.Error())
}

func TestInstallPackage_DownloadError(t *testing.T) {
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
	mockPkg := &index.Package{
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
		ResolvePackage(pkgName, version, os, arch).
		Return(mockPkg, nil)

	parsedURL, _ := url.Parse(pkgURL)
	mockHTTPClient.EXPECT().
		DownloadPackage(gomock.Any(), parsedURL, "").
		Return(downloadErr)

	// Create manager with mocks
	mgr := &ManagerImpl{
		indexManager: mockIndexMgr,
		httpClient:   mockHTTPClient,
		os:           os,
		arch:         arch,
	}

	// Test
	err := mgr.InstallPackage(context.Background(), pkgName, version, false)

	// Assert
	assert.EqualError(t, err, downloadErr.Error())
}

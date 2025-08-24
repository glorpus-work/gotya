package pkg

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/cperrin88/gotya/pkg/config"
	mockhttp "github.com/cperrin88/gotya/pkg/http/mocks"
	"github.com/cperrin88/gotya/pkg/index"
	mockindex "github.com/cperrin88/gotya/pkg/index/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Settings: config.Settings{
			HTTPTimeout: 30,
		},
	}
	mockIndexMgr := mockindex.NewMockManager(ctrl)

	mgr := NewManager(mockIndexMgr, cfg)

	assert.NotNil(t, mgr)
	assert.Equal(t, mockIndexMgr, mgr.indexManager)
	assert.Equal(t, cfg, mgr.config)
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
		config:       &config.Config{},
		httpClient:   mockHTTPClient,
	}

	// Test
	err := mgr.InstallPackage(context.Background(), pkgName, version, os, arch, false)

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
		config:       &config.Config{},
		httpClient:   mockhttp.NewMockClient(ctrl),
	}

	// Test
	err := mgr.InstallPackage(context.Background(), "invalid-pkg", "1.0.0", "linux", "amd64", false)

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
		config:       &config.Config{},
		httpClient:   mockHTTPClient,
	}

	// Test
	err := mgr.InstallPackage(context.Background(), pkgName, version, os, arch, false)

	// Assert
	assert.EqualError(t, err, downloadErr.Error())
}

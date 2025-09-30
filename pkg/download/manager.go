package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	pkgerrors "github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/fsutil"
)

// ManagerImpl is a simple HTTP-based download manager with optional checksum verification
// and basic de-duplication. It is intentionally minimal and can be extended later
// with retries, backoff, mirror selection, and content-addressed storage.
type ManagerImpl struct {
	client    *http.Client
	userAgent string
}

// NewManager creates a new download manager with the given timeout and user agent.
func NewManager(timeout time.Duration, userAgent string) *ManagerImpl {
	if userAgent == "" {
		userAgent = "gotya/1.0"
	}
	return &ManagerImpl{
		client:    &http.Client{Timeout: timeout},
		userAgent: userAgent,
	}
}

// FetchAll downloads multiple items concurrently and returns a map of item IDs to downloaded file paths.
func (m *ManagerImpl) FetchAll(ctx context.Context, items []Item, opts Options) (map[string]string, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = max(2, runtime.NumCPU()/2)
	}
	if opts.Dir == "" || !filepath.IsAbs(opts.Dir) {
		return nil, fmt.Errorf("download dir must be absolute: %w: %s", pkgerrors.ErrInvalidPath, opts.Dir)
	}
	if err := os.MkdirAll(opts.Dir, fsutil.DirModeSecure); err != nil {
		return nil, pkgerrors.Wrap(err, "could not create download dir")
	}

	byURL, err := buildURLIndex(items)
	if err != nil {
		return nil, err
	}
	results, err := m.runDownloadWorkers(ctx, items, byURL, opts)
	if err != nil {
		return nil, err
	}
	return mapResultsByID(items, results), nil
}

func buildURLIndex(items []Item) (map[string][]int, error) {
	byURL := make(map[string][]int)
	for i, it := range items {
		if it.URL == nil {
			return nil, fmt.Errorf("item %d has nil URL: %w", i, pkgerrors.ErrDownloadFailed)
		}
		key := it.URL.String()
		byURL[key] = append(byURL[key], i)
	}
	return byURL, nil
}

func mapResultsByID(items []Item, results []string) map[string]string {
	out := make(map[string]string, len(items))
	for i, it := range items {
		out[it.ID] = results[i]
	}
	return out
}

// Fetch downloads a single item and returns the path to the downloaded file.
func (m *ManagerImpl) Fetch(ctx context.Context, item Item, opts Options) (string, error) {
	if opts.Dir == "" || !filepath.IsAbs(opts.Dir) {
		return "", fmt.Errorf("download dir must be absolute: %s: %w", opts.Dir, pkgerrors.ErrInvalidPath)
	}
	if err := os.MkdirAll(opts.Dir, fsutil.DirModeSecure); err != nil {
		return "", pkgerrors.Wrap(err, "could not create download dir")
	}
	return m.fetchOne(ctx, item, opts)
}

func (m *ManagerImpl) runDownloadWorkers(ctx context.Context, items []Item, byURL map[string][]int, opts Options) ([]string, error) {
	results := make([]string, len(items))
	var firstErr error
	var mu sync.Mutex

	tasks := make(chan string)
	var wg sync.WaitGroup

	for w := 0; w < opts.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for urlStr := range tasks {
				idx := byURL[urlStr][0]
				path, err := m.fetchOne(ctx, items[idx], opts)
				mu.Lock()
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
					for _, i := range byURL[urlStr] {
						results[i] = ""
					}
					mu.Unlock()
					continue
				}
				for _, i := range byURL[urlStr] {
					results[i] = path
				}
				mu.Unlock()
			}
		}()
	}

	for _, urlStr := range rangeKeys(byURL) {
		tasks <- urlStr
	}
	close(tasks)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

func (m *ManagerImpl) fetchOne(ctx context.Context, item Item, opts Options) (string, error) {
	if item.URL == nil {
		return "", fmt.Errorf("nil URL: %w", pkgerrors.ErrDownloadFailed)
	}
	filename := selectFilename(item)
	absPath := filepath.Join(opts.Dir, filename)
	if reuse, ok := tryReuseExisting(absPath, item.Checksum); ok {
		return reuse, nil
	}
	resp, err := m.doRequest(ctx, item)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	tmpPath, err := writeBodyToTemp(resp, absPath)
	if err != nil {
		return "", err
	}
	if item.Checksum != "" {
		ok, err := verifySHA256(tmpPath, item.Checksum)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("checksum mismatch for %s: %w", item.URL, pkgerrors.ErrFileHashMismatch)
		}
	}
	if err := finalizeFile(tmpPath, absPath); err != nil {
		return "", err
	}
	return absPath, nil
}

func selectFilename(item Item) string {
	if item.Filename != "" {
		return item.Filename
	}
	if item.Checksum != "" {
		return item.Checksum
	}
	h := sha256.Sum256([]byte(item.URL.String()))
	return hex.EncodeToString(h[:])
}

func tryReuseExisting(absPath, checksum string) (string, bool) {
	if st, err := os.Stat(absPath); err == nil && st.Size() > 0 {
		if checksum == "" {
			return absPath, true
		}
		ok, err := verifySHA256(absPath, checksum)
		if err == nil && ok {
			return absPath, true
		}
	}
	return "", false
}

func (m *ManagerImpl) doRequest(ctx context.Context, item Item) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL.String(), http.NoBody)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "failed to create request")
	}
	req.Header.Set("User-Agent", m.userAgent)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "download failed")
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d: %w", resp.StatusCode, pkgerrors.ErrDownloadFailed)
	}
	return resp, nil
}

func writeBodyToTemp(resp *http.Response, absPath string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(absPath), fsutil.DirModeSecure); err != nil {
		return "", pkgerrors.Wrap(err, "could not create download dir")
	}
	tmp, err := os.CreateTemp(filepath.Dir(absPath), "dl-*.tmp")
	if err != nil {
		return "", pkgerrors.Wrap(err, "could not create temp file")
	}
	tmpPath := tmp.Name()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return "", pkgerrors.Wrap(err, "could not write file")
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", pkgerrors.Wrap(err, "could not sync file")
	}
	if err := tmp.Close(); err != nil {
		return "", pkgerrors.Wrap(err, "could not close file")
	}
	return tmpPath, nil
}

func finalizeFile(tmpPath, absPath string) error {
	if err := fsutil.Move(tmpPath, absPath); err != nil {
		return pkgerrors.Wrap(err, "could not finalize file")
	}
	if err := os.Chmod(absPath, fsutil.FileModeSecure); err != nil {
		return pkgerrors.Wrap(err, "could not set permissions")
	}
	return nil
}

func verifySHA256(path string, wantHex string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, pkgerrors.Wrap(err, "open for checksum")
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, pkgerrors.Wrap(err, "hashing")
	}
	got := hex.EncodeToString(h.Sum(nil))
	return got == normalizeHex(wantHex), nil
}

func normalizeHex(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func rangeKeys(m map[string][]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

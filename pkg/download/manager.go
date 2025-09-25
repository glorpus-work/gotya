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

func (m *ManagerImpl) FetchAll(ctx context.Context, items []Item, opts Options) (map[string]string, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = max(2, runtime.NumCPU()/2)
	}
	if opts.Dir == "" || !filepath.IsAbs(opts.Dir) {
		return nil, fmt.Errorf("download dir must be absolute: %s", opts.Dir)
	}
	if err := os.MkdirAll(opts.Dir, fsutil.DirModeSecure); err != nil {
		return nil, pkgerrors.Wrap(err, "could not create download dir")
	}

	// de-duplicate by URL string to avoid downloading the same resource multiple times in a batch
	byURL := make(map[string][]int)
	for i, it := range items {
		if it.URL == nil {
			return nil, fmt.Errorf("item %d has nil URL", i)
		}
		key := it.URL.String()
		byURL[key] = append(byURL[key], i)
	}

	results := make([]string, len(items))
	var firstErr error
	var mu sync.Mutex

	tasks := make(chan string)
	var wg sync.WaitGroup

	// worker pool
	for w := 0; w < opts.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for urlStr := range tasks {
				// pick the first representative index for this URL
				idx := byURL[urlStr][0]
				path, err := m.fetchOne(ctx, items[idx], opts)
				mu.Lock()
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
					// still mark all duplicates with empty to signal failure
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

	out := make(map[string]string, len(items))
	for i, it := range items {
		out[it.ID] = results[i]
	}
	return out, nil
}

func (m *ManagerImpl) Fetch(ctx context.Context, item Item, opts Options) (string, error) {
	if opts.Dir == "" || !filepath.IsAbs(opts.Dir) {
		return "", fmt.Errorf("download dir must be absolute: %s", opts.Dir)
	}
	if err := os.MkdirAll(opts.Dir, fsutil.DirModeSecure); err != nil {
		return "", pkgerrors.Wrap(err, "could not create download dir")
	}
	return m.fetchOne(ctx, item, opts)
}

func (m *ManagerImpl) fetchOne(ctx context.Context, item Item, opts Options) (string, error) {
	if item.URL == nil {
		return "", fmt.Errorf("nil URL")
	}

	// choose a filename: prefer provided, then checksum-based, then hash of URL path
	filename := item.Filename
	if filename == "" {
		if item.Checksum != "" {
			filename = item.Checksum
		} else {
			h := sha256.Sum256([]byte(item.URL.String()))
			filename = hex.EncodeToString(h[:])
		}
	}
	absPath := filepath.Join(opts.Dir, filename)

	// If file exists and checksum matches (if provided), reuse it
	if st, err := os.Stat(absPath); err == nil && st.Size() > 0 {
		if item.Checksum == "" {
			return absPath, nil
		}
		ok, err := verifySHA256(absPath, item.Checksum)
		if err == nil && ok {
			return absPath, nil
		}
		// otherwise, re-download
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL.String(), http.NoBody)
	if err != nil {
		return "", pkgerrors.Wrap(err, "failed to create request")
	}
	req.Header.Set("User-Agent", m.userAgent)

	resp, err := m.client.Do(req)
	if err != nil {
		return "", pkgerrors.Wrap(err, "download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// write to temp file then atomically rename
	if err := os.MkdirAll(filepath.Dir(absPath), fsutil.DirModeSecure); err != nil {
		return "", pkgerrors.Wrap(err, "could not create download dir")
	}
	tmp, err := os.CreateTemp(filepath.Dir(absPath), "dl-*.tmp")
	if err != nil {
		return "", pkgerrors.Wrap(err, "could not create temp file")
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

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

	// verify checksum if provided
	if item.Checksum != "" {
		ok, err := verifySHA256(tmpPath, item.Checksum)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("checksum mismatch for %s", item.URL)
		}
	}

	if err := os.Rename(tmpPath, absPath); err != nil {
		return "", pkgerrors.Wrap(err, "could not finalize file")
	}
	if err := os.Chmod(absPath, fsutil.FileModeSecure); err != nil {
		return "", pkgerrors.Wrap(err, "could not set permissions")
	}

	return absPath, nil
}

func verifySHA256(path string, wantHex string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, pkgerrors.Wrap(err, "open for checksum")
	}
	defer f.Close()
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

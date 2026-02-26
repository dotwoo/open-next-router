package logx

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const archiveTimeLayout = "20060102-150405.000000000"

type AccessLogRotateOptions struct {
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
	Now        func() time.Time
}

type AccessRotateWriter struct {
	mu sync.Mutex

	path string
	dir  string
	base string

	maxSizeBytes int64
	maxBackups   int
	maxAgeDays   int
	compress     bool
	now          func() time.Time

	f           *os.File
	currentSize int64
	currentDay  string
	closed      bool
}

type accessArchiveFile struct {
	path string
	when time.Time
}

func NewAccessRotateWriter(opts AccessLogRotateOptions) (*AccessRotateWriter, error) {
	path := strings.TrimSpace(opts.Path)
	if path == "" {
		return nil, errors.New("access log rotate path is empty")
	}
	if opts.MaxSizeMB <= 0 {
		return nil, errors.New("max_size_mb must be > 0")
	}
	if opts.MaxBackups <= 0 {
		return nil, errors.New("max_backups must be > 0")
	}
	if opts.MaxAgeDays < 0 {
		return nil, errors.New("max_age_days must be >= 0")
	}

	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	dir := filepath.Dir(path)
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	if dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
	}

	f, size, err := openActiveLogFile(path)
	if err != nil {
		return nil, err
	}

	now := nowFn().In(time.Local)
	w := &AccessRotateWriter{
		path:         path,
		dir:          dir,
		base:         filepath.Base(path),
		maxSizeBytes: int64(opts.MaxSizeMB) * 1024 * 1024,
		maxBackups:   opts.MaxBackups,
		maxAgeDays:   opts.MaxAgeDays,
		compress:     opts.Compress,
		now:          nowFn,
		f:            f,
		currentSize:  size,
		currentDay:   dayKey(now),
	}
	return w, nil
}

func (w *AccessRotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, os.ErrClosed
	}
	if err := w.rotateIfNeededLocked(len(p)); err != nil {
		return 0, err
	}
	if w.f == nil {
		return 0, errors.New("access log writer is not initialized")
	}
	n, err := w.f.Write(p)
	w.currentSize += int64(n)
	return n, err
}

func (w *AccessRotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

func (w *AccessRotateWriter) rotateIfNeededLocked(incomingBytes int) error {
	now := w.now().In(time.Local)
	needDayRotate := dayKey(now) != w.currentDay
	needSizeRotate := w.currentSize > 0 && (w.currentSize+int64(incomingBytes) > w.maxSizeBytes)
	if !needDayRotate && !needSizeRotate {
		return nil
	}
	return w.rotateLocked(now)
}

func (w *AccessRotateWriter) rotateLocked(now time.Time) error {
	if w.f != nil {
		if err := w.f.Close(); err != nil {
			return err
		}
		w.f = nil
	}

	archivePath := fmt.Sprintf("%s.%s", w.path, now.Format(archiveTimeLayout))
	renamed := false
	if err := os.Rename(w.path, archivePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			if openErr := w.reopenActiveLocked(now); openErr != nil {
				return openErr
			}
			return err
		}
	} else {
		renamed = true
	}

	if renamed && w.compress {
		if err := compressArchiveFile(archivePath); err != nil {
			return err
		}
	}

	if err := w.reopenActiveLocked(now); err != nil {
		return err
	}
	_ = w.cleanupArchivesLocked(now)
	return nil
}

func (w *AccessRotateWriter) reopenActiveLocked(now time.Time) error {
	f, size, err := openActiveLogFile(w.path)
	if err != nil {
		return err
	}
	w.f = f
	w.currentSize = size
	w.currentDay = dayKey(now)
	return nil
}

func (w *AccessRotateWriter) cleanupArchivesLocked(now time.Time) error {
	files, err := w.listArchiveFilesLocked()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	toDelete := map[string]struct{}{}
	for i := w.maxBackups; i < len(files); i++ {
		toDelete[files[i].path] = struct{}{}
	}

	if w.maxAgeDays > 0 {
		cutoff := now.AddDate(0, 0, -w.maxAgeDays)
		for _, f := range files {
			if f.when.Before(cutoff) {
				toDelete[f.path] = struct{}{}
			}
		}
	}

	for p := range toDelete {
		_ = os.Remove(p)
	}
	return nil
}

func (w *AccessRotateWriter) listArchiveFilesLocked() ([]accessArchiveFile, error) {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return nil, err
	}
	prefix := w.base + "."
	files := make([]accessArchiveFile, 0)
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		ts := strings.TrimPrefix(name, prefix)
		ts = strings.TrimSuffix(ts, ".gz")
		parsed, err := time.ParseInLocation(archiveTimeLayout, ts, time.Local)
		if err != nil {
			continue
		}
		files = append(files, accessArchiveFile{
			path: filepath.Join(w.dir, name),
			when: parsed,
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].when.After(files[j].when)
	})
	return files, nil
}

func openActiveLogFile(path string) (*os.File, int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, 0, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	return f, st.Size(), nil
}

func compressArchiveFile(path string) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = src.Close()
	}()

	tmpPath := path + ".gz.tmp"
	dst, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	gz := gzip.NewWriter(dst)
	_, copyErr := io.Copy(gz, src)
	closeGzErr := gz.Close()
	closeDstErr := dst.Close()
	if copyErr != nil || closeGzErr != nil || closeDstErr != nil {
		_ = os.Remove(tmpPath)
		if copyErr != nil {
			return copyErr
		}
		if closeGzErr != nil {
			return closeGzErr
		}
		return closeDstErr
	}

	finalPath := path + ".gz"
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Remove(path)
}

func dayKey(ts time.Time) string {
	return ts.In(time.Local).Format("20060102")
}

package primitivestest

import (
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"aelum-installer/backend/install/primitives"
)

// Compile-time guard: MemFS must satisfy the primitives.FileSystem seam.
var _ primitives.FileSystem = (*MemFS)(nil)

// MemFS is an in-memory primitives.FileSystem for unit tests. Reads of unknown
// paths return os.ErrNotExist, so the missing-file branches are exercised by
// default; preload files with WithFile. Write/append/remove/stat can be forced
// to fail per path so error paths stay deterministic.
type MemFS struct {
	mu        sync.Mutex
	files     map[string][]byte
	modes     map[string]os.FileMode
	dirs      map[string]bool
	readErr   map[string]error
	writeErr  map[string]error
	appendErr map[string]error
	statErr   map[string]error
	removeErr map[string]error
	chmodErr  map[string]error
	mkdirErr  map[string]error
	writes    map[string]int
}

// NewMemFS returns an empty in-memory filesystem.
func NewMemFS() *MemFS {
	return &MemFS{
		files:     map[string][]byte{},
		modes:     map[string]os.FileMode{},
		dirs:      map[string]bool{},
		readErr:   map[string]error{},
		writeErr:  map[string]error{},
		appendErr: map[string]error{},
		statErr:   map[string]error{},
		removeErr: map[string]error{},
		chmodErr:  map[string]error{},
		mkdirErr:  map[string]error{},
		writes:    map[string]int{},
	}
}

// WithFile preloads a file so reads of it succeed.
func (fs *MemFS) WithFile(path, content string) *MemFS {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.files[path] = []byte(content)
	fs.modes[path] = 0o644
	return fs
}

// WithDir registers an otherwise-empty directory so Stat succeeds for it.
func (fs *MemFS) WithDir(path string) *MemFS {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.dirs[path] = true
	return fs
}

// WithReadError forces ReadFile(path) to fail.
func (fs *MemFS) WithReadError(path string, err error) *MemFS {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.readErr[path] = err
	return fs
}

// WithWriteError forces WriteFile(path, …) to fail.
func (fs *MemFS) WithWriteError(path string, err error) *MemFS {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.writeErr[path] = err
	return fs
}

// WithAppendError forces AppendFile(path, …) to fail.
func (fs *MemFS) WithAppendError(path string, err error) *MemFS {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.appendErr[path] = err
	return fs
}

// WithStatError forces Stat(path) to fail.
func (fs *MemFS) WithStatError(path string, err error) *MemFS {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.statErr[path] = err
	return fs
}

// WithRemoveError forces RemoveAll(path) to fail.
func (fs *MemFS) WithRemoveError(path string, err error) *MemFS {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.removeErr[path] = err
	return fs
}

// WithChmodError forces Chmod(path, …) to fail.
func (fs *MemFS) WithChmodError(path string, err error) *MemFS {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.chmodErr[path] = err
	return fs
}

// WithMkdirError forces MkdirAll(path, …) to fail.
func (fs *MemFS) WithMkdirError(path string, err error) *MemFS {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.mkdirErr[path] = err
	return fs
}

// Read returns the current content of path, if present.
func (fs *MemFS) Read(path string) (string, bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	data, ok := fs.files[path]
	return string(data), ok
}

// Exists reports whether path is a known file or directory (or a parent of one).
func (fs *MemFS) Exists(path string) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.existsLocked(path)
}

// WriteCount returns how many times path was written or appended.
func (fs *MemFS) WriteCount(path string) int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.writes[path]
}

// Mode returns the permission bits recorded for path (0 if unknown).
func (fs *MemFS) Mode(path string) os.FileMode {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.modes[path]
}

// Paths returns every known file path, sorted.
func (fs *MemFS) Paths() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	paths := make([]string, 0, len(fs.files))
	for p := range fs.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

func (fs *MemFS) existsLocked(p string) bool {
	if _, ok := fs.files[p]; ok {
		return true
	}
	if fs.dirs[p] {
		return true
	}
	prefix := p + "/"
	for f := range fs.files {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}

// ReadFile implements primitives.FileSystem.
func (fs *MemFS) ReadFile(p string) ([]byte, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if err, ok := fs.readErr[p]; ok {
		return nil, err
	}
	data, ok := fs.files[p]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

// WriteFile implements primitives.FileSystem.
func (fs *MemFS) WriteFile(p string, data []byte, perm os.FileMode) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if err, ok := fs.writeErr[p]; ok {
		return err
	}
	fs.files[p] = append([]byte(nil), data...)
	fs.modes[p] = perm
	fs.writes[p]++
	return nil
}

// AppendFile implements primitives.FileSystem.
func (fs *MemFS) AppendFile(p string, data []byte, perm os.FileMode) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if err, ok := fs.appendErr[p]; ok {
		return err
	}
	fs.files[p] = append(fs.files[p], data...)
	fs.modes[p] = perm
	fs.writes[p]++
	return nil
}

// MkdirAll implements primitives.FileSystem.
func (fs *MemFS) MkdirAll(p string, _ os.FileMode) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if err, ok := fs.mkdirErr[p]; ok {
		return err
	}
	fs.dirs[p] = true
	return nil
}

// Stat implements primitives.FileSystem.
func (fs *MemFS) Stat(p string) (os.FileInfo, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if err, ok := fs.statErr[p]; ok {
		return nil, err
	}
	if data, ok := fs.files[p]; ok {
		return memFileInfo{name: path.Base(p), size: int64(len(data)), mode: fs.modes[p]}, nil
	}
	if fs.existsLocked(p) {
		return memFileInfo{name: path.Base(p), dir: true, mode: os.ModeDir | 0o755}, nil
	}
	return nil, os.ErrNotExist
}

// RemoveAll implements primitives.FileSystem.
func (fs *MemFS) RemoveAll(p string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if err, ok := fs.removeErr[p]; ok {
		return err
	}
	prefix := p + "/"
	for f := range fs.files {
		if f == p || strings.HasPrefix(f, prefix) {
			delete(fs.files, f)
			delete(fs.modes, f)
		}
	}
	for d := range fs.dirs {
		if d == p || strings.HasPrefix(d, prefix) {
			delete(fs.dirs, d)
		}
	}
	return nil
}

// Chmod implements primitives.FileSystem.
func (fs *MemFS) Chmod(p string, mode os.FileMode) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if err, ok := fs.chmodErr[p]; ok {
		return err
	}
	fs.modes[p] = mode
	return nil
}

// memFileInfo is a minimal os.FileInfo for MemFS entries.
type memFileInfo struct {
	name string
	size int64
	mode os.FileMode
	dir  bool
}

func (fi memFileInfo) Name() string       { return fi.name }
func (fi memFileInfo) Size() int64        { return fi.size }
func (fi memFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi memFileInfo) ModTime() time.Time { return time.Time{} }
func (fi memFileInfo) IsDir() bool        { return fi.dir }
func (fi memFileInfo) Sys() any           { return nil }

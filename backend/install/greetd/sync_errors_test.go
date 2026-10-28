package greetd

import (
	"errors"
	"strings"
	"testing"
)

// scriptFS extends the greetd test fake with fault injection: it can fail the
// nth RunChroot call and/or force `test -d` to succeed, so every error branch
// in SyncDmsGreeter can be exercised without a real system.
type scriptFS struct {
	*fakeFS
	failAt         int
	count          int
	dirsOK         bool
	writeErr       error
	lastFailedArgs []string
}

func (f *scriptFS) RunChroot(args ...string) ([]byte, error) {
	f.count++
	failed := f.failAt > 0 && f.count == f.failAt
	if failed {
		f.lastFailedArgs = append([]string(nil), args...)
	}
	if f.dirsOK && len(args) >= 2 && args[0] == "test" && args[1] == "-d" {
		return nil, nil
	}
	if failed {
		return []byte("boom"), errors.New("command failed")
	}
	return f.fakeFS.RunChroot(args...)
}

func (f *scriptFS) WriteFile(path, content string) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	return f.fakeFS.WriteFile(path, content)
}

// TestSyncDmsGreeter_PropagatesRunErrors injects a failure at every RunChroot
// call in turn and asserts the error is returned rather than swallowed. The
// dirsOK variant also reaches the shell-dir chgrp/chmod branches, which are
// skipped while `test -d` reports the directory as missing.
func TestSyncDmsGreeter_PropagatesRunErrors(t *testing.T) {
	for _, dirsOK := range []bool{false, true} {
		for n := 1; n <= 80; n++ {
			fs := &scriptFS{fakeFS: newFakeFS(), failAt: n, dirsOK: dirsOK}
			err := SyncDmsGreeter(fs, testContext())

			if fs.lastFailedArgs == nil {
				continue // n is past the last call; nothing was injected
			}
			// A failing `test -d` only makes the loop continue, so no error is
			// expected to propagate from that call.
			if fs.lastFailedArgs[0] == "test" {
				continue
			}
			if err == nil {
				t.Errorf("dirsOK=%v: failure at RunChroot call %d (%v) was swallowed",
					dirsOK, n, fs.lastFailedArgs)
			}
		}
	}
}

// TestSyncDmsGreeter_PropagatesWriteError covers the memory.json write failure.
func TestSyncDmsGreeter_PropagatesWriteError(t *testing.T) {
	fs := &scriptFS{fakeFS: newFakeFS(), writeErr: errors.New("disk full")}

	err := SyncDmsGreeter(fs, testContext())
	if err == nil || !strings.Contains(err.Error(), "memory.json") {
		t.Fatalf("SyncDmsGreeter() error = %v, want memory.json failure", err)
	}
}

package greetd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

// fakeFS is an in-memory, chroot-relative filesystem used to exercise the
// greetd sync without touching the real (mounted) system.
type fakeFS struct {
	files map[string]string
	dirs  map[string]bool
	calls []string
}

func newFakeFS() *fakeFS {
	return &fakeFS{files: map[string]string{}, dirs: map[string]bool{}}
}

func (f *fakeFS) mkfileAbs(home, rel, content string) {
	f.files[filepath.Join(home, rel)] = content
}

func (f *fakeFS) seedUser(home string) {
	// DMS config files that the /etc/skel seeding during install would ship.
	f.mkfileAbs(home, ".config/DankMaterialShell/settings.json",
		`{"currentThemeName":"purple","customThemeFile":"~/.config/DankMaterialShell/themes/my-theme.json"}`)
	f.mkfileAbs(home, ".config/DankMaterialShell/themes/my-theme.json",
		`{"fg":"#ffffff","bg":"#101010"}`)
	f.mkfileAbs(home, ".local/state/DankMaterialShell/session.json",
		`{"wallpaperPath":"~/Pictures/wall.jpg","isLightMode":false}`)
	f.mkfileAbs(home, ".cache/DankMaterialShell/dms-colors.json",
		`{"colors":{"dark":{"primary":"#7b5cbff"}}}`)
	f.mkfileAbs(home, "Pictures/wall.jpg", "\xff\xd8\xff\xe0JPGDATA")
}

func (f *fakeFS) RunChroot(args ...string) ([]byte, error) {
	f.calls = append(f.calls, strings.Join(args, " "))
	switch args[0] {
	case "test":
		// args: test -e/-f/-d path
		path := args[2]
		if args[1] == "-d" {
			return nil, os.ErrNotExist // dirs are auto-created by mkdir -p below
		}
		if _, ok := f.files[path]; ok {
			return nil, nil
		}
		if f.dirs[path] {
			return nil, nil
		}
		return nil, os.ErrNotExist
	case "mkdir":
		f.ensureDir(args[1])
		return nil, nil
	case "rm":
		delete(f.files, args[2])
		return nil, nil
	case "ln":
		// args: ln [-sfn|-sf] <target> <link>
		if len(args) >= 4 && args[0] == "ln" {
			f.files[args[3]] = args[2] // record the symlink target
			return nil, nil
		}
		return nil, nil
	case "chown", "chmod", "chgrp", "setfacl", "usermod", "groupadd":
		// ownership/permission changes are no-ops for the fake FS
		return nil, nil
	}
	return nil, nil
}

func (f *fakeFS) ensureDir(path string) {
	f.dirs[path] = true
	if parent := filepath.Dir(path); parent != path {
		f.ensureDir(parent)
	}
}

func (f *fakeFS) WriteFile(path, content string) error {
	f.files[path] = content
	f.ensureDir(filepath.Dir(path))
	return nil
}

func (f *fakeFS) Logger() shared.Logger { return nil }

func testContext() *InstallContext {
	return &InstallContext{Username: "aelum"}
}

func TestSyncCreatesSymlinksToLiveConfig(t *testing.T) {
	fs := newFakeFS()
	home := "/home/aelum"
	fs.seedUser(home)

	ctx := testContext()
	if err := SyncDmsGreeter(fs, ctx); err != nil {
		t.Fatalf("SyncDmsGreeter: %v", err)
	}

	slot := filepath.Join("/var/cache/dms-greeter", "users", "aelum")
	homeSettings := filepath.Join(home, ".config", "DankMaterialShell", "settings.json")
	homeSession := filepath.Join(home, ".local", "state", "DankMaterialShell", "session.json")
	homeColors := filepath.Join(home, ".cache", "DankMaterialShell", "dms-colors.json")

	// Each config in the slot must be a symlink pointing at the live home file,
	// not a static copy. This is what lets the greeter track post-install
	// wallpaper changes.
	cases := []struct {
		link   string
		target string
	}{
		{filepath.Join(slot, "settings.json"), homeSettings},
		{filepath.Join(slot, "session.json"), homeSession},
		{filepath.Join(slot, "colors.json"), homeColors},
	}
	for _, c := range cases {
		got, ok := fs.files[c.link]
		if !ok {
			t.Fatalf("%s symlink not created in slot", c.link)
		}
		if got != c.target {
			t.Errorf("%s -> %q, want %q", c.link, got, c.target)
		}
	}

	// The greeter reads the symlinked session.json, so resolving the symlink to
	// the live file must expose the user's chosen wallpaper.
	sessionTarget := fs.files[filepath.Join(slot, "session.json")]
	liveSession, ok := fs.files[sessionTarget]
	if !ok {
		t.Fatalf("symlinked session target %s missing", sessionTarget)
	}
	if !strings.Contains(liveSession, "wall.jpg") {
		t.Errorf("live session.json should reference user wallpaper, got %q", liveSession)
	}
}

func TestSyncCreatesSymlinksEvenWhenConfigMissing(t *testing.T) {
	fs := newFakeFS()
	// No DMS config files seeded (home not yet populated by DMS).

	ctx := testContext()
	if err := SyncDmsGreeter(fs, ctx); err != nil {
		t.Fatalf("SyncDmsGreeter: %v", err)
	}

	slot := filepath.Join("/var/cache/dms-greeter", "users", "aelum")
	// Symlinks must still be created (dangling is fine) so the greeter picks up
	// the config the moment DMS writes it after first login.
	for _, name := range []string{"settings.json", "session.json", "colors.json"} {
		link := filepath.Join(slot, name)
		if _, ok := fs.files[link]; !ok {
			t.Errorf("%s symlink should be created even when target is missing", name)
		}
	}
}

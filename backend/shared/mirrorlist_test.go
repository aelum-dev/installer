package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetEmbeddedMirrorList_KnownRegion(t *testing.T) {
	data, err := GetEmbeddedMirrorList("en_US")
	if err != nil {
		t.Fatalf("GetEmbeddedMirrorList(en_US): %v", err)
	}
	if len(data) == 0 {
		t.Fatal("en_US mirror list is empty")
	}
	if !strings.Contains(string(data), MirrorListCommentedServerPrefix) {
		t.Error("expected commented server lines in the en_US mirror list")
	}
}

func TestGetEmbeddedMirrorList_FallsBackToDefault(t *testing.T) {
	data, err := GetEmbeddedMirrorList("xx_ZZ")
	if err != nil {
		t.Fatalf("expected the worldwide fallback for an unknown region, got error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("fallback mirror list is empty")
	}
}

// TestEmbeddedMirrorListForEveryRegion ensures each selectable region has a
// usable mirror list (its own file or the worldwide fallback).
func TestEmbeddedMirrorListForEveryRegion(t *testing.T) {
	for code := range Regions {
		data, err := GetEmbeddedMirrorList(code)
		if err != nil {
			t.Errorf("region %q: %v", code, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("region %q has an empty mirror list", code)
		}
	}
}

func TestParseMirrorList(t *testing.T) {
	const content = `##
## Germany
##
#Server = https://de.example.org/$repo/os/$arch
#Server = https://de2.example.org/$repo/os/$arch
##
## Atlantis
##
#Server = https://atlantis.example.org/$repo/os/$arch
##
## Worldwide
##
#Server = https://world.example.org/$repo/os/$arch
`
	path := filepath.Join(t.TempDir(), "mirrors.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	sections, worldwide, err := parseMirrorList(path)
	if err != nil {
		t.Fatalf("parseMirrorList: %v", err)
	}

	// "Germany" maps to de_DE; "Atlantis" is unknown and must be dropped.
	if len(sections) != 1 {
		t.Fatalf("got %d sections, want 1: %+v", len(sections), sections)
	}
	sec := sections[0]
	if len(sec.Locales) != 1 || sec.Locales[0] != "de_DE" {
		t.Errorf("section locales = %v, want [de_DE]", sec.Locales)
	}
	if len(sec.Servers) != 2 {
		t.Errorf("German servers = %d, want 2", len(sec.Servers))
	}
	if len(worldwide) != 1 || worldwide[0] != "#Server = https://world.example.org/$repo/os/$arch" {
		t.Errorf("worldwide servers = %v", worldwide)
	}
}

func TestParseMirrorList_MissingFile(t *testing.T) {
	if _, _, err := parseMirrorList(filepath.Join(t.TempDir(), "nope.txt")); err == nil {
		t.Error("expected an error for a missing mirrorlist file")
	}
}

func TestWriteRegionMirrorList(t *testing.T) {
	dir := t.TempDir()
	servers := []string{
		"#Server = https://a.example.org/$repo/os/$arch",
		"#Server = https://b.example.org/$repo/os/$arch",
	}
	if err := writeRegionMirrorList("de_DE", servers, dir); err != nil {
		t.Fatalf("writeRegionMirrorList: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "de_DE.list"))
	if err != nil {
		t.Fatalf("read written list: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "Arch Linux mirrorlist for de_DE") {
		t.Errorf("header missing from %q", body)
	}
	for _, s := range servers {
		if !strings.Contains(body, s) {
			t.Errorf("server %q missing from %q", s, body)
		}
	}
}

// TestEnsureMirrorLists_Idempotent exercises the real generated-data path: the
// repo ships every per-region .list, so the call must be a no-op rather than
// downloading or rewriting anything.
func TestEnsureMirrorLists_Idempotent(t *testing.T) {
	before := map[string][]byte{}
	entries, err := os.ReadDir(mirrorListsDir)
	if err != nil {
		t.Fatalf("read mirrorlist dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(mirrorListsDir, e.Name()))
		if err != nil {
			t.Fatalf("snapshot %s: %v", e.Name(), err)
		}
		before[e.Name()] = data
	}

	if err := EnsureMirrorLists(); err != nil {
		t.Fatalf("EnsureMirrorLists: %v", err)
	}

	after, err := os.ReadDir(mirrorListsDir)
	if err != nil {
		t.Fatalf("re-read mirrorlist dir: %v", err)
	}
	for _, e := range after {
		if e.IsDir() {
			continue
		}
		prev, ok := before[e.Name()]
		if !ok {
			t.Errorf("EnsureMirrorLists created unexpected file %s", e.Name())
			continue
		}
		data, err := os.ReadFile(filepath.Join(mirrorListsDir, e.Name()))
		if err != nil {
			t.Fatalf("re-read %s: %v", e.Name(), err)
		}
		if string(data) != string(prev) {
			t.Errorf("EnsureMirrorLists rewrote %s", e.Name())
		}
	}
}

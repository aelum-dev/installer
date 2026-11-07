package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const mirrorFixture = `##
## Arch Linux repository mirrorlist
## Generated on 2026-07-25
##

## Worldwide
#Server = https://world.example.org/$repo/os/$arch

## Germany
#Server = https://de.example.org/$repo/os/$arch
#Server = https://de2.example.org/$repo/os/$arch

## Atlantis
#Server = https://atlantis.example.org/$repo/os/$arch
`

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mirrors.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestParseMirrorList(t *testing.T) {
	lists, err := ParseMirrorList(writeFixture(t, mirrorFixture))
	if err != nil {
		t.Fatalf("ParseMirrorList: %v", err)
	}

	if lists.GeneratedOn != "2026-07-25" {
		t.Errorf("GeneratedOn = %q, want 2026-07-25", lists.GeneratedOn)
	}
	if len(lists.Worldwide) != 1 || lists.Worldwide[0] != "#Server = https://world.example.org/$repo/os/$arch" {
		t.Errorf("worldwide servers = %v", lists.Worldwide)
	}

	// Every section is returned, mapped or not: an unmapped name is how a country
	// Arch has just added becomes visible instead of quietly disappearing. The
	// file's own title block is not a section.
	if len(lists.Sections) != 2 {
		t.Fatalf("got %d sections, want 2: %+v", len(lists.Sections), lists.Sections)
	}
	if lists.Sections[0].Name != "Germany" || lists.Sections[0].Code != "de" {
		t.Errorf("first section = %+v, want Germany/de", lists.Sections[0])
	}
	if len(lists.Sections[0].Servers) != 2 {
		t.Errorf("German servers = %d, want 2", len(lists.Sections[0].Servers))
	}
	if lists.Sections[1].Name != "Atlantis" || lists.Sections[1].Code != "" {
		t.Errorf("second section = %+v, want Atlantis with no code", lists.Sections[1])
	}
}

func TestParseMirrorList_MissingFile(t *testing.T) {
	if _, err := ParseMirrorList(filepath.Join(t.TempDir(), "nope.txt")); err == nil {
		t.Error("expected an error for a missing mirrorlist file")
	}
}

// TestBuildMirrorList_PutsTheCountryFirst is the behaviour the split exists for:
// a chosen country's mirrors lead, and the worldwide list follows so one dead
// national mirror cannot stall an update.
func TestBuildMirrorList_PutsTheCountryFirst(t *testing.T) {
	country, err := GetEmbeddedMirrorList("ua")
	if err != nil {
		t.Fatalf("ua mirror list: %v", err)
	}
	worldwide, err := GetEmbeddedMirrorList(DefaultRegionCode)
	if err != nil {
		t.Fatalf("worldwide mirror list: %v", err)
	}

	built, err := BuildMirrorList("ua")
	if err != nil {
		t.Fatalf("BuildMirrorList(ua): %v", err)
	}
	body := string(built)

	if !strings.Contains(body, "Arch Linux mirrorlist for Ukraine") {
		t.Errorf("built list does not name the chosen country:\n%s", body)
	}

	lines := serverLines(body)
	if len(lines) < len(serverLines(string(country)))+len(serverLines(string(worldwide))) {
		t.Errorf("built list has %d servers, want at least the %d Ukrainian ones plus the worldwide ones",
			len(lines), len(serverLines(string(country))))
	}

	// The country's mirrors come first, and nothing is listed twice.
	first := lines[0]
	if !strings.Contains(string(country), first) {
		t.Errorf("first server %q is not a Ukrainian mirror", first)
	}
	seen := map[string]bool{}
	for _, l := range lines {
		if seen[l] {
			t.Errorf("server listed twice: %s", l)
		}
		seen[l] = true
	}

	// Everything stays commented: configure.uncommentMirrors is what activates it.
	for _, l := range lines {
		if !strings.HasPrefix(l, MirrorListCommentedServerPrefix) {
			t.Errorf("server line is not commented: %q", l)
		}
	}
}

func TestBuildMirrorList_WorldwideIsItself(t *testing.T) {
	worldwide, err := GetEmbeddedMirrorList(DefaultRegionCode)
	if err != nil {
		t.Fatalf("worldwide mirror list: %v", err)
	}
	built, err := BuildMirrorList(DefaultRegionCode)
	if err != nil {
		t.Fatalf("BuildMirrorList(worldwide): %v", err)
	}
	if string(built) != string(worldwide) {
		t.Error("the worldwide list gained or lost something when built")
	}
}

// TestBuildMirrorList_UnknownCodeFallsBack keeps an unknown location from failing
// an install: the worldwide list is a working mirrorlist.
func TestBuildMirrorList_UnknownCodeFallsBack(t *testing.T) {
	built, err := BuildMirrorList("zz")
	if err != nil {
		t.Fatalf("expected the worldwide fallback for an unknown code, got error: %v", err)
	}
	if len(serverLines(string(built))) == 0 {
		t.Error("fallback mirror list has no servers")
	}
}

// TestEnsureMirrorLists_MatchesCommittedFiles is the drift guard the old
// "skip files that already exist" behaviour made impossible: refreshing from the
// committed upstream list must reproduce exactly what is committed. When it fails,
// the generated lists in the tree are older than the upstream list they come from.
func TestEnsureMirrorLists_MatchesCommittedFiles(t *testing.T) {
	before := map[string][]byte{}
	entries, err := os.ReadDir(mirrorListsDir)
	if err != nil {
		t.Fatalf("read mirrorlist dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".list") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(mirrorListsDir, e.Name()))
		if err != nil {
			t.Fatalf("snapshot %s: %v", e.Name(), err)
		}
		before[e.Name()] = data
	}
	if len(before) < 50 {
		t.Fatalf("only %d generated mirror lists in the tree", len(before))
	}

	if err := EnsureMirrorLists(); err != nil {
		t.Fatalf("EnsureMirrorLists: %v", err)
	}

	for name, prev := range before {
		data, err := os.ReadFile(filepath.Join(mirrorListsDir, name))
		if err != nil {
			t.Fatalf("re-read %s: %v", name, err)
		}
		if string(data) != string(prev) {
			t.Errorf("%s is not what the upstream list generates — regenerate it", name)
		}
	}
}

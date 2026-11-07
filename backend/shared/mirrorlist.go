package shared

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	mirrorListURL     = MirrorListURL
	rawMirrorlistFile = RawMirrorlistFileName
	mirrorListsDir    = MirrorListsRelativeDir // relative to this file (backend/shared/mirrorlist/)
)

//go:embed mirrorlist/*.list
var mirrorListFS embed.FS

// MirrorSection is one country section of the upstream mirrorlist: the section's
// own name, the ISO code it was mapped to, and its server lines.
type MirrorSection struct {
	Name    string // as upstream spells it, e.g. "Ukraine"
	Code    string // ISO 3166-1 alpha-2, e.g. "ua" ("" when the name is unmapped)
	Servers []string
}

// MirrorLists is the parsed upstream mirrorlist: every country section in the
// order upstream lists them, the worldwide servers, and the date upstream says it
// generated its list.
type MirrorLists struct {
	GeneratedOn string // upstream's own "## Generated on" value, "" when absent
	Sections    []MirrorSection
	Worldwide   []string
}

// GetEmbeddedMirrorList returns the generated mirror list for a location code
// (an ISO code, or DefaultRegionCode). Falls back to the worldwide list when the
// code has no file of its own.
func GetEmbeddedMirrorList(code string) ([]byte, error) {
	path := filepath.Join(mirrorListsDir, code+".list")
	data, err := mirrorListFS.ReadFile(path)
	if err == nil {
		return data, nil
	}
	worldwide, werr := mirrorListFS.ReadFile(filepath.Join(mirrorListsDir, DefaultRegionCode+".list"))
	if werr != nil {
		return nil, fmt.Errorf("no mirror list found for %q and no worldwide list available: %w", code, werr)
	}
	return worldwide, nil
}

// BuildMirrorList returns the mirror list to write into an installed system: the
// chosen country's mirrors first, then the worldwide ones, with duplicates
// dropped.
//
// The fallback is the point. A national section can be tiny — Arch lists 7
// mirrors for Ukraine and 1 for Iceland — so a country-only list leaves a fresh
// install one dead mirror away from an update that cannot finish. The lines stay
// commented as upstream writes them; configure.uncommentMirrors activates them
// when the file is written.
func BuildMirrorList(code string) ([]byte, error) {
	country, err := GetEmbeddedMirrorList(code)
	if err != nil {
		return nil, err
	}
	if code == DefaultRegionCode {
		return country, nil
	}

	worldwide, err := mirrorListFS.ReadFile(filepath.Join(mirrorListsDir, DefaultRegionCode+".list"))
	if err != nil {
		// Nothing to fall back to: the country list on its own is still a
		// working mirrorlist, so this is not a reason to refuse the install.
		return country, nil
	}

	servers := append(serverLines(string(country)), extraServers(serverLines(string(worldwide)), serverLines(string(country)))...)
	if len(servers) == 0 {
		return country, nil
	}

	var b strings.Builder
	b.WriteString(headerFor(code))
	for _, s := range servers {
		b.WriteString(s + "\n")
	}
	return []byte(b.String()), nil
}

// headerFor writes the header of a built mirrorlist, naming both halves so the
// file explains itself to whoever opens it later.
func headerFor(code string) string {
	name := code
	if r := GetRegion(code); r.Name != "" {
		name = r.Name
	}
	return fmt.Sprintf(
		"##\n"+
			"## Arch Linux mirrorlist for %s\n"+
			"##\n"+
			"## Mirrors in %s first, then the worldwide mirrors as a fallback for when\n"+
			"## one of them is down. Written by the Aelum installer.\n"+
			"##\n\n",
		name, name,
	)
}

// serverLines returns the commented server lines of a generated list, in order.
func serverLines(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, MirrorListCommentedServerPrefix) {
			out = append(out, line)
		}
	}
	return out
}

// extraServers returns the entries of want that are not already in have.
func extraServers(want, have []string) []string {
	seen := make(map[string]bool, len(have))
	for _, s := range have {
		seen[s] = true
	}
	var out []string
	for _, s := range want {
		if !seen[s] {
			out = append(out, s)
		}
	}
	return out
}

// EnsureMirrorLists refreshes the generated per-country mirror files from the
// upstream list. It is meant to be called at build time (build.sh does, through
// mirrorlist_gen.go).
//
// It always rewrites the files it can: the earlier version skipped every file
// that already existed, which is how the committed lists stayed frozen at the
// date of the first one long after upstream moved on.
func EnsureMirrorLists() error {
	mlDir := mirrorListsDir

	rawPath := filepath.Join(mlDir, rawMirrorlistFile)
	if _, err := os.Stat(rawPath); os.IsNotExist(err) {
		if err := downloadMirrorList(rawPath); err != nil {
			return fmt.Errorf("failed to download mirrorlist: %w", err)
		}
	}

	lists, err := ParseMirrorList(rawPath)
	if err != nil {
		return fmt.Errorf("failed to parse mirrorlist: %w", err)
	}

	if err := writeMirrorListFile(mlDir, DefaultRegionCode, "Worldwide", lists.GeneratedOn, lists.Worldwide); err != nil {
		return fmt.Errorf("failed to write the worldwide mirror list: %w", err)
	}

	var unmapped []string
	written := 0
	for _, section := range lists.Sections {
		if section.Code == "" {
			unmapped = append(unmapped, section.Name)
			continue
		}
		if err := writeMirrorListFile(mlDir, section.Code, section.Name, lists.GeneratedOn, section.Servers); err != nil {
			return fmt.Errorf("failed to write mirror list for %q: %w", section.Name, err)
		}
		written++
	}

	if len(unmapped) > 0 {
		sort.Strings(unmapped)
		return fmt.Errorf(
			"wrote %d mirror lists, but Arch now has sections no ISO code is mapped for: %s "+
				"— add them to ArchCountryISO in backend/shared/countries.go",
			written, strings.Join(unmapped, ", "),
		)
	}
	return nil
}

// downloadMirrorList fetches the full mirrorlist from Arch Linux and saves it to path.
func downloadMirrorList(path string) error {
	resp, err := http.Get(mirrorListURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mirrorlist URL returned status %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(path), MirrorListDirMode); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(f, resp.Body)
	return err
}

// ParseMirrorList reads an upstream mirrorlist into its country sections and its
// worldwide servers. Sections are kept in upstream's order and every section is
// returned, whether or not its name is mapped to an ISO code — an unmapped name
// is how a country Arch has just added becomes visible instead of quietly
// disappearing.
func ParseMirrorList(path string) (MirrorLists, error) {
	f, err := os.Open(path)
	if err != nil {
		return MirrorLists{}, err
	}
	defer func() { _ = f.Close() }()

	var lists MirrorLists
	var currentName string
	var currentServers []string

	flush := func() {
		if currentName == "" {
			return
		}
		if strings.EqualFold(currentName, "Worldwide") {
			lists.Worldwide = currentServers
		} else {
			lists.Sections = append(lists.Sections, MirrorSection{
				Name:    currentName,
				Code:    ArchCountryISO[currentName],
				Servers: currentServers,
			})
		}
	}

	// A country section is a "## Name" line that opens a block of servers. The
	// file's own title block uses the same "## " prefix ("## Arch Linux
	// repository mirrorlist"), so a "## " line directly under another one is part
	// of that block and not a section — every real section header stands on its own
	// after a blank line.
	scanner := bufio.NewScanner(f)
	prev := ""
	for scanner.Scan() {
		line := scanner.Text()

		if lists.GeneratedOn == "" && strings.HasPrefix(line, MirrorListGeneratedOnPrefix) {
			lists.GeneratedOn = strings.TrimSpace(strings.TrimPrefix(line, MirrorListGeneratedOnPrefix))
		} else if strings.HasPrefix(line, MirrorListCountryHeaderPrefix) && !strings.HasPrefix(prev, "##") {
			flush()
			currentName = strings.TrimSpace(strings.TrimPrefix(line, MirrorListCountryHeaderPrefix))
			currentServers = nil
		} else if strings.HasPrefix(line, MirrorListCommentedServerPrefix) {
			currentServers = append(currentServers, line)
		}

		prev = line
	}
	flush()

	return lists, scanner.Err()
}

// writeMirrorListFile writes one generated per-country list. The header carries
// upstream's own generation date, never this build's: build.sh refreshes these
// files on every build, and a build timestamp would make every build dirty the
// tree even when nothing changed.
func writeMirrorListFile(mlDir, code, name, upstreamDate string, servers []string) error {
	if err := os.MkdirAll(mlDir, MirrorListDirMode); err != nil {
		return err
	}
	path := filepath.Join(mlDir, code+".list")

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	_, _ = fmt.Fprintf(w, "##\n## Arch Linux mirrorlist for %s (%s)\n", name, code)
	if upstreamDate != "" {
		_, _ = fmt.Fprintf(w, "## Source: %s (upstream list generated on %s)\n", mirrorListURL, upstreamDate)
	} else {
		_, _ = fmt.Fprintf(w, "## Source: %s\n", mirrorListURL)
	}
	_, _ = fmt.Fprintf(w, "##\n\n")

	for _, server := range servers {
		_, _ = fmt.Fprintln(w, server)
	}

	return w.Flush()
}

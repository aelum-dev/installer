//go:build ignore

// gen-snapshot-prompt-i18n — extracts the snapshot.prompt.* namespace out of the
// installer's UI translations and writes it next to the snapshot-restore dialog,
// so the dialog asks in the user's language with the installer's own strings.
//
// Usage: go run ./backend/scripts/gen-snapshot-prompt-i18n.go
//        AELUM_LIVE_ISO_DIR=../live-iso go run ./backend/scripts/gen-snapshot-prompt-i18n.go
//
// Input:  frontend/src/translations/*.json  (every UI locale, one file each)
// Output: <live-iso>/airootfs/etc/xdg/quickshell/aelum-snapshot/i18n/<locale>.json
//
// The output is a generated baseline: commit it with any change to the copy, and
// never hand-edit it. The dialog reads those files at run time
// (live-iso/airootfs/etc/xdg/quickshell/aelum-snapshot/shell.qml), so the language
// follows the session it runs in rather than anything decided at install time.
//
// Run it from the installer repo root. The live-iso tree is the sibling directory
// by default, the same convention live-iso/scripts/gen-refind-theme.py uses for the
// shared font; AELUM_LIVE_ISO_DIR overrides it when the repos are not side by side.
//
// It is not called by build.sh: it writes into another repository, so it runs when
// the copy changes, not on every build.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	translationsDir = "frontend/src/translations"
	namespace       = "snapshot.prompt."

	// The keys the dialog reads. Adding one here, to the en-US catalog and to every
	// other locale is one change in three places; the parity test in
	// frontend/src/lib/i18n/index.test.ts is what fails the fourth place (a locale
	// that was not updated).
	promptKeys = "title titleUnknown question body restoring restored failed failedUnknown cancel ok close reboot"
)

func main() {
	outDir := filepath.Join(liveISODir(), "airootfs", "etc", "xdg", "quickshell", "aelum-snapshot", "i18n")

	files, err := filepath.Glob(filepath.Join(translationsDir, "*.json"))
	if err != nil || len(files) == 0 {
		fail(fmt.Errorf("no translation files under %s (run this from the installer repo root): %w", translationsDir, err))
	}
	sort.Strings(files)

	required := strings.Fields(promptKeys)
	wanted := make(map[string]bool, len(required))
	for _, key := range required {
		wanted[namespace+key] = true
	}

	written := 0
	for _, path := range files {
		code := strings.TrimSuffix(filepath.Base(path), ".json")
		messages, err := readJSON(path)
		if err != nil {
			fail(fmt.Errorf("%s: %w", path, err))
		}

		// The whole namespace, not just the keys named above: a locale that carries a
		// snapshot.prompt.* key the others do not has drifted, and the shipped set
		// must be the same set for every language.
		var extra []string
		extracted := map[string]string{}
		for key, value := range messages {
			if !strings.HasPrefix(key, namespace) {
				continue
			}
			if value == "" {
				fail(fmt.Errorf("%s: %s is empty", code, key))
			}
			extracted[key] = value
		}
		for _, key := range required {
			if _, ok := extracted[namespace+key]; !ok {
				fail(fmt.Errorf("%s: missing %s%s", code, namespace, key))
			}
		}
		for key := range extracted {
			if !wanted[key] {
				extra = append(extra, key)
			}
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			fail(fmt.Errorf("%s: %s keys the dialog does not read: %s", code, namespace, strings.Join(extra, ", ")))
		}

		if err := writeJSON(filepath.Join(outDir, code+".json"), extracted); err != nil {
			fail(fmt.Errorf("%s: %w", code, err))
		}
		written++
	}

	fmt.Printf("wrote %d locale file(s) of %d keys to %s\n", written, len(required), outDir)
}

// liveISODir is the sibling live-iso tree, overridable for the same reason
// gen-refind-theme.py takes AELUM_NERD_FONT: the four repos sit side by side in
// one workspace, but that is not a fact this script may assume.
func liveISODir() string {
	if dir := os.Getenv("AELUM_LIVE_ISO_DIR"); dir != "" {
		return dir
	}
	return "../live-iso"
}

func readJSON(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// writeJSON writes the namespace in key order, unescaped: these strings are UI
// copy for every language, and HTML escaping would turn a quote in French into
// &#34; inside a Qt string. encoding/json sorts map keys, which is the order the
// UI catalogs themselves are kept in.
func writeJSON(path string, values map[string]string) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(values); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gen-snapshot-prompt-i18n:", err)
	os.Exit(1)
}

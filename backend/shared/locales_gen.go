//go:build ignore

// This file is meant to be run with: go run ./backend/shared/locales_gen.go
// It regenerates locales_table.go — the languages the installer offers.
//
// The list is every locale glibc can generate on this host, not a curated set:
// a person should be able to install in the language their machine reads, and
// "we never thought to add it" is not a reason they cannot. The installer's own
// UI still has 28 translations; a language with no UI translation sets the system
// locale and keeps English chrome, which is a working system, not a broken one.
//
// The sources are not ours to copy: /usr/share/i18n/locales is what locale-gen
// can build, and iso-codes names the languages and territories. What neither
// carries — what a language calls itself — is curated in locales_native.go.
//
// glibc drift: this table is generated from the host's glibc, while the target's
// comes from the ISO's package set. A locale here that the target's glibc cannot
// generate is caught at install time (configure checks the locale exists in the
// target before writing locale.gen and falls back to en_US.UTF-8), so a newer
// host glibc cannot produce an install that fails to boot into its locale.
//
// LOCALES_DIR overrides the glibc locale directory. ISO_CODES_DIR overrides where
// iso-codes' JSON lives.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aelum-installer/backend/shared"
)

// skipLocales are the files in the locale directory that are not languages: glibc
// internals and the collation/transliteration fragments it builds locales from.
var skipLocales = map[string]bool{
	"C": true, "POSIX": true, "i18n": true, "i18n_ctype": true,
	"iso14651_t1": true, "iso14651_t1_common": true, "iso14651_t1_pinyin": true,
}

func main() {
	if err := os.Chdir("backend/shared"); err != nil {
		fmt.Fprintf(os.Stderr, "Error changing directory: %v\n", err)
		os.Exit(1)
	}

	localesDir := os.Getenv("LOCALES_DIR")
	if localesDir == "" {
		localesDir = "/usr/share/i18n/locales"
	}
	isoDir := os.Getenv("ISO_CODES_DIR")
	if isoDir == "" {
		isoDir = "/usr/share/iso-codes/json"
	}

	if err := generate(localesDir, isoDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func generate(localesDir, isoDir string) error {
	enNames, err := readEnglishNames(isoDir)
	if err != nil {
		return fmt.Errorf("read the ISO 639 language names: %w", err)
	}

	entries, err := os.ReadDir(localesDir)
	if err != nil {
		return fmt.Errorf("read the glibc locale directory: %w", err)
	}

	table := make([]shared.Language, 0, len(entries))
	var missing []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || skipLocales[name] || strings.HasPrefix(name, "translit_") {
			continue
		}
		if name[0] < 'a' || name[0] > 'z' {
			continue
		}
		lang := buildLanguage(name, enNames)
		if lang.En == name {
			missing = append(missing, name)
		}
		table = append(table, lang)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		fmt.Fprintf(os.Stderr, "note: no English name for (fell back to the code): %s\n", strings.Join(missing, " "))
	}

	sort.Slice(table, func(i, j int) bool {
		if strings.ToLower(table[i].En) != strings.ToLower(table[j].En) {
			return strings.ToLower(table[i].En) < strings.ToLower(table[j].En)
		}
		return strings.ToLower(table[i].Name) < strings.ToLower(table[j].Name)
	})

	if err := writeTable(table); err != nil {
		return err
	}
	fmt.Printf("Wrote %d languages (from %s).\n", len(table), localesDir)
	return nil
}

// buildLanguage turns one glibc locale file name into a Language row. The name is
// "lang[_TERR][@modifier]"; the display name is the native name plus, in
// parentheses, the territory and the written form that tell the near-identical
// rows apart.
func buildLanguage(file string, enNames map[string]string) shared.Language {
	base, modifier := file, ""
	if i := strings.IndexByte(file, '@'); i >= 0 {
		base, modifier = file[:i], file[i+1:]
	}
	parts := strings.Split(base, "_")
	lang := parts[0]
	territory := ""
	if len(parts) > 1 {
		territory = strings.ToLower(parts[1])
	}

	en := shared.EnglishNameOverride[file]
	if en == "" {
		en = enNames[lang]
	}
	if en == "" {
		en = file
	}
	native := shared.NativeNameByLanguage[lang]
	if native == "" {
		native = en
	}

	locale := base + ".UTF-8"
	if modifier != "" {
		locale += "@" + modifier
	}

	// A territory only shows when it is a real location the installer knows —
	// otherwise the parenthetical would name a code, not a place.
	var paren []string
	if territory != "" && shared.GetRegion(territory).Code != "" {
		paren = append(paren, shared.GetRegion(territory).Name)
	}
	if modifier != "" {
		if spelled, ok := shared.ModifierName[modifier]; ok {
			paren = append(paren, spelled)
		} else {
			paren = append(paren, modifier)
		}
	}
	name := native
	if len(paren) > 0 {
		name += " (" + strings.Join(paren, ", ") + ")"
	}

	return shared.Language{
		Code:      file,
		Name:      name,
		Locale:    locale,
		Native:    native,
		En:        en,
		Territory: territory,
	}
}

// readEnglishNames indexes iso-codes' ISO 639 tables by every code form (alpha-2
// and alpha-3) so a locale's language half resolves whichever way it is spelled.
// The first name seen for a code wins; alpha-2 is read first so the shorter, more
// common code's name is the one kept.
func readEnglishNames(isoDir string) (map[string]string, error) {
	out := map[string]string{}
	for _, f := range []string{"iso_639-2.json", "iso_639-3.json"} {
		data, err := os.ReadFile(filepath.Join(isoDir, f))
		if err != nil {
			return nil, err
		}
		var doc struct {
			Entries []struct {
				Alpha2 string `json:"alpha_2"`
				Alpha3 string `json:"alpha_3"`
				Name   string `json:"name"`
			} `json:"639-2"`
			Entries3 []struct {
				Alpha3 string `json:"alpha_3"`
				Name   string `json:"name"`
			} `json:"639-3"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
		for _, e := range doc.Entries {
			if e.Alpha2 != "" {
				if _, ok := out[e.Alpha2]; !ok {
					out[e.Alpha2] = e.Name
				}
			}
			if e.Alpha3 != "" {
				if _, ok := out[e.Alpha3]; !ok {
					out[e.Alpha3] = e.Name
				}
			}
		}
		for _, e := range doc.Entries3 {
			if e.Alpha3 != "" {
				if _, ok := out[e.Alpha3]; !ok {
					out[e.Alpha3] = e.Name
				}
			}
		}
	}
	return out, nil
}

func writeTable(table []shared.Language) error {
	out, err := os.Create("locales_table.go")
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	w := bufio.NewWriter(out)
	defer func() { _ = w.Flush() }()

	_, _ = w.WriteString(`// Code generated by locales_gen.go. DO NOT EDIT.
//
// The languages the installer offers: every locale glibc can generate on the
// host it was generated on. A language with no UI translation still sets the
// system locale and keeps English chrome. Regenerate with
//
//	go run ./backend/shared/locales_gen.go
//
// Native names (what a language calls itself) are curated in locales_native.go;
// English names and territory names come from iso-codes. Sorted by English name.

package shared

// languageTable is every language the installer can offer.
var languageTable = []Language{
`)
	for _, l := range table {
		_, _ = fmt.Fprintf(w, "\t{Code: %q, Name: %q, Locale: %q, Native: %q, En: %q, Territory: %q},\n",
			l.Code, l.Name, l.Locale, l.Native, l.En, l.Territory)
	}
	_, _ = w.WriteString("}\n")

	return w.Flush()
}

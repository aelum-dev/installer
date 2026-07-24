package shared

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	mirrorListURL     = MirrorListURL
	rawMirrorlistFile = RawMirrorlistFileName
	mirrorListsDir    = MirrorListsRelativeDir // relative to this file (backend/shared/mirrorlist/)
)

//go:embed mirrorlist/*.list
var mirrorListFS embed.FS

// countryNameToLocale maps Arch Linux mirrorlist country names to locale codes.
// These locale codes correspond to Region.Code values.
var countryNameToLocale = map[string]string{
	"Germany":       "de_DE",
	"United States": "en_US",
	"Spain":         "es_ES",
	"France":        "fr_FR",
	"Italy":         "it_IT",
	"Japan":         "ja_JP",
	"South Korea":   "ko_KR",
	"Poland":        "pl_PL",
	"Brazil":        "pt_BR",
	"Russia":        "ru_RU",
	"Turkey":        "tr_TR",
	"Ukraine":       "uk_UA",
	"China":         "zh_CN",
}

// mirrorSection represents a parsed country section from the mirrorlist.
type mirrorSection struct {
	Country string
	Locales []string // locale codes for this country
	Servers []string // raw mirror lines (e.g., "#Server = https://...")
}

// GetEmbeddedMirrorList returns the content of a region's mirror list from the embedded filesystem.
func GetEmbeddedMirrorList(locale string) ([]byte, error) {
	path := filepath.Join("mirrorlist", locale+".list")
	data, err := mirrorListFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mirror list for %q not found in embedded filesystem: %w", locale, err)
	}
	return data, nil
}

// EnsureMirrorLists downloads the full mirrorlist if needed and extracts
// per-region mirror files. This is intended to be called at build time
// (e.g., from build.sh) to regenerate the embedded mirrorlists.
// It is safe to call multiple times — it only downloads once and reuses cached files.
func EnsureMirrorLists() error {
	mlDir := mirrorListsDir
	rawPath := filepath.Join(mlDir, rawMirrorlistFile)

	// Download the raw mirrorlist if it doesn't exist
	if _, err := os.Stat(rawPath); os.IsNotExist(err) {
		if err := downloadMirrorList(rawPath); err != nil {
			return fmt.Errorf("failed to download mirrorlist: %w", err)
		}
	}

	// Parse the raw mirrorlist
	sections, worldwideServers, err := parseMirrorList(rawPath)
	if err != nil {
		return fmt.Errorf("failed to parse mirrorlist: %w", err)
	}

	// Write per-region files only if they don't exist (cached)
	for _, section := range sections {
		for _, locale := range section.Locales {
			regionPath := filepath.Join(mlDir, locale+".list")
			if _, err := os.Stat(regionPath); os.IsNotExist(err) {
				if err := writeRegionMirrorList(locale, section.Servers, mlDir); err != nil {
					return fmt.Errorf("failed to write mirror list for %q: %w", locale, err)
				}
			}
		}
	}

	// Write default.list (worldwide mirrors) if it doesn't exist
	if len(worldwideServers) > 0 {
		defaultPath := filepath.Join(mlDir, "default.list")
		if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
			if err := writeRegionMirrorList("default", worldwideServers, mlDir); err != nil {
				return fmt.Errorf("failed to write default mirror list: %w", err)
			}
		}
	}

	return nil
}

// downloadMirrorList fetches the mirrorlist from Arch Linux and saves it to path.
func downloadMirrorList(path string) error {
	resp, err := http.Get(mirrorListURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// parseMirrorList reads a mirrorlist file and extracts sections for known locales.
// Returns per-region sections, worldwide servers, and any error.
func parseMirrorList(path string) ([]mirrorSection, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	// Build reverse map: locale -> country names
	localeToCountries := make(map[string][]string)
	for country, locale := range countryNameToLocale {
		localeToCountries[locale] = append(localeToCountries[locale], country)
	}

	// Track which locales we've seen servers for
	localeServers := make(map[string][]string)
	var worldwideServers []string
	var currentCountry string
	var currentServers []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Check for country header
		if strings.HasPrefix(line, MirrorListCountryHeaderPrefix) {
			// Process previous section if any
			if currentCountry != "" {
				if strings.EqualFold(currentCountry, "Worldwide") {
					worldwideServers = currentServers
				} else {
					processSection(currentCountry, currentServers, localeToCountries, localeServers)
				}
			}

			country := strings.TrimPrefix(line, MirrorListCountryHeaderPrefix)
			country = strings.TrimSpace(country)
			currentCountry = country
			currentServers = nil
			continue
		}

		// Skip empty lines and non-server lines
		if line == "" || !strings.HasPrefix(line, MirrorListCommentedServerPrefix) {
			continue
		}

		currentServers = append(currentServers, line)
	}

	// Process last section
	if currentCountry != "" {
		if strings.EqualFold(currentCountry, "Worldwide") {
			worldwideServers = currentServers
		} else {
			processSection(currentCountry, currentServers, localeToCountries, localeServers)
		}
	}

	// Build result
	var sections []mirrorSection
	for locale, servers := range localeServers {
		sections = append(sections, mirrorSection{
			Locales: []string{locale},
			Servers: servers,
		})
	}

	return sections, worldwideServers, scanner.Err()
}

// processSection checks if the current country matches any known locale and
// collects servers if so.
func processSection(country string, servers []string, localeToCountries map[string][]string, localeServers map[string][]string) {
	for locale, countries := range localeToCountries {
		for _, c := range countries {
			if strings.EqualFold(country, c) {
				localeServers[locale] = servers
				break
			}
		}
	}
}

// writeRegionMirrorList writes a region's mirror list to a file.
func writeRegionMirrorList(locale string, servers []string, mlDir string) error {
	path := filepath.Join(mlDir, locale+".list")

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "##\n")
	fmt.Fprintf(w, "## Arch Linux mirrorlist for %s\n", locale)
	fmt.Fprintf(w, "##\n\n")

	for _, server := range servers {
		fmt.Fprintln(w, server)
	}

	return w.Flush()
}

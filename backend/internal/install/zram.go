package install

import (
	"fmt"
	"strings"
)

// ConfigureZram sets up zram-based compressed swap in the installed system.
// Uses zram-generator with zram-size = min(ram/2, 16384 MiB) and zstd compression.
// Also adds zram to mkinitcpio so the module is available early in boot.
func ConfigureZram(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	// Step 1: Write zram-generator config
	// zram-size = min(ram / 2, 16384) gives: 8GB RAM→4G, 16GB→8G, 32GB→16G cap
	zramConf := `[zram0]
zram-size = min(ram / 2, 16384)
compression-algorithm = zstd
`
	if err := writeFile(ctx, "/etc/systemd/zram-generator.conf", zramConf); err != nil {
		return fmt.Errorf("failed to write zram-generator config: %w", err)
	}

	// Step 2: Add zram to mkinitcpio modules so it's available in initramfs
	if err := addZramToMkinitcpio(ctx); err != nil {
		return fmt.Errorf("failed to add zram to mkinitcpio: %w", err)
	}

	return nil
}

// addZramToMkinitcpio adds the zram module to mkinitcpio configuration.
// It modifies the archiso.conf file in /etc/mkinitcpio.conf.d/ to include zram
// in the MODULES array.
func addZramToMkinitcpio(ctx *InstallContext) error {
	configPath := "/etc/mkinitcpio.conf.d/archiso.conf"
	cmd := ctx.run(HelperCmdReadFile, configPath)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	currentConfig := string(output)

	// Check if zram is already in MODULES
	if containsModule(currentConfig, "zram") {
		return nil
	}

	// Add zram to the MODULES line
	newConfig := replaceModules(currentConfig, "zram")

	if err := writeFile(ctx, configPath, newConfig); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	return nil
}

// containsModule checks if a module name appears in the MODULES array of mkinitcpio config.
func containsModule(config, module string) bool {
	for _, line := range splitLines(config) {
		if len(line) > 0 && line[0] != '#' && containsModuleInLine(line, module) {
			return true
		}
	}
	return false
}

// containsModuleInLine checks if a module name appears in a single mkinitcpio line.
func containsModuleInLine(line, module string) bool {
	// Look for MODULES=(... module ...)
	idx := findModulesArray(line)
	if idx < 0 {
		return false
	}
	// Skip past the MODULES=( prefix to get just the array contents
	rest := line[idx+len("MODULES=("):]
	if endIdx := findClosingParen(rest); endIdx >= 0 {
		rest = rest[:endIdx]
	} else {
		// No closing paren found — strip any trailing )
		rest = strings.TrimRight(rest, ")")
	}
	for _, m := range splitWords(rest) {
		if m == module {
			return true
		}
	}
	return false
}

// findModulesArray finds the start index of MODULES=( in a line.
func findModulesArray(line string) int {
	idx := 0
	for {
		i := indexOfSubstring(line[idx:], "MODULES=(")
		if i < 0 {
			return -1
		}
		if idx+i > 0 && line[idx+i-1] == '#' {
			// It's commented out, keep searching
			idx = idx + i + len("MODULES=(")
			continue
		}
		return idx + i
	}
}

// findClosingParen finds the matching ) for an opening (.
func findClosingParen(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '(' {
			depth++
		} else if s[i] == ')' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// replaceModules adds a module to the MODULES array in mkinitcpio config.
func replaceModules(config, module string) string {
	lines := splitLines(config)
	for i, line := range lines {
		if len(line) > 0 && line[0] != '#' && containsModuleInLine(line, module) {
			// Module already present
			return config
		}
		idx := findModulesArray(line)
		if idx < 0 {
			continue
		}
		// Find the closing paren on the full rest (starts with MODULES=(...)
		rest := line[idx:]
		end := findClosingParen(rest)
		if end < 0 {
			continue
		}
		// Extract just the array contents (between MODULES=( and ))
		arrayContent := rest[len("MODULES=("):end]
		// Check if module is already there
		for _, m := range splitWords(arrayContent) {
			if m == module {
				return config
			}
		}
		// Add module before the closing paren
		newArray := arrayContent + " " + module
		newLine := line[:idx+len("MODULES=(")] + newArray + line[idx+end+1:]
		lines[i] = newLine
		return joinLines(lines)
	}
	// MODULES line not found — append to a new line
	return config + "\nMODULES=(" + module + ")\n"
}

// splitLines splits config into lines.
func splitLines(s string) []string {
	return splitString(s, "\n")
}

// splitWords splits a string by whitespace.
func splitWords(s string) []string {
	var result []string
	var current string
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// splitString splits by delimiter.
func splitString(s, delim string) []string {
	var result []string
	start := 0
	for {
		idx := indexOfSubstring(s[start:], delim)
		if idx < 0 {
			result = append(result, s[start:])
			break
		}
		result = append(result, s[start:start+idx])
		start += idx + len(delim)
	}
	return result
}

// indexOfSubstring finds the index of substr in s, returning -1 if not found.
func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// joinLines joins lines with newlines.
func joinLines(lines []string) string {
	return joinString(lines, "\n")
}

// joinString joins strings with delimiter.
func joinString(ss []string, delim string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for i := 1; i < len(ss); i++ {
		result += delim + ss[i]
	}
	return result
}



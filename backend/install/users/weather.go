package users

// The weather location.
//
// The Welcome page's location answer says what country the machine is in, and the
// desktop's weather widget asks for one place. The capital is that place — see
// shared.WeatherByISO for why a country's capital, and not its largest city, is
// the answer the installer is allowed to give.
//
// It is written into the target's seeded skeleton, not into the account's home,
// because that is the only place Dux Shell's per-home state can be set before a
// home exists: the file lives in XDG_STATE_HOME, there is no system-side copy of
// it (unlike the input method profile the configure step writes), and `useradd -m`
// copies the skeleton. So the install user gets it from the same copy every later
// account does, and the greeter reads it through the symlink the greetd setup
// points at the home. That is also why this runs here, in SetupUsers: after
// isocopy has put the skeleton in the target, and before the first home is seeded
// out of it.
//
// Unlike every file isocopy carries, this one is edited rather than copied: the
// weather location is the answer to a wizard page, and the shipped skeleton cannot
// know it. Only the two keys are touched — everything else the skeleton's session
// state says (wallpaper, the launcher's hidden entries) stays exactly as the live
// ISO wrote it.

import (
	"bytes"
	"encoding/json"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// seedWeatherLocation writes the location's capital into the seeded desktop
// session state. Best-effort by design: a weather widget is not a reason to fail a
// nearly complete install, and a skeleton the live ISO did not seed is the same
// case as an optional copied file — a warning, and the desktop keeps its own
// default.
func seedWeatherLocation(ctx *shared.InstallContext) {
	place, ok := shared.WeatherForRegion(ctx.Region.Code)
	if !ok {
		// The worldwide location: no country, so no capital to name.
		warn(ctx, "no weather location for the %q location; the desktop keeps its own", ctx.Region.Code)
		return
	}

	path := shared.MountRoot + shared.SkelDmsSessionPath
	data, err := primitives.ReadFile(path)
	if err != nil {
		warn(ctx, "weather location not set: cannot read %s: %v", path, err)
		return
	}

	state, err := decodeSession(data)
	if err != nil {
		warn(ctx, "weather location not set: %s is not readable as JSON: %v", path, err)
		return
	}
	state[shared.DmsWeatherLocationKey] = place.City
	state[shared.DmsWeatherCoordinatesKey] = place.Coordinates

	// The two-space indent is what the shipped file uses; Dux Shell reads the file
	// by key, so the key order this round-trip produces is not something it or the
	// person reading the file depends on. SetEscapeHTML keeps the names as written:
	// a city spelled with & or <> would otherwise come out as an entity.
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(state); err != nil {
		warn(ctx, "weather location not set: could not write %s: %v", path, err)
		return
	}
	if err := primitives.WriteFileMode(path, out.String(), 0o644); err != nil {
		warn(ctx, "weather location not set: %v", err)
		return
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("weather location: %s (%s)", place.City, place.Coordinates)
	}
}

// decodeSession reads Dux Shell's session state without losing anything the
// installer does not touch. UseNumber keeps numbers as they were written: a value
// like configVersion must come back out of the encoder exactly as it went in, not
// through float64.
func decodeSession(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	state := map[string]any{}
	if err := dec.Decode(&state); err != nil {
		return nil, err
	}
	return state, nil
}

// warn logs a non-fatal problem, tolerating a context with no logger (the unit
// tests and --dry-run paths).
func warn(ctx *shared.InstallContext, format string, args ...any) {
	if ctx.Logger != nil {
		ctx.Logger.Error("Warning: "+format, args...)
	}
}

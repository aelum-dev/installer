package greetd

import (
	_ "embed"
)

//go:embed data/greetd/config.toml
var configToml string

// ConfigToml returns the embedded greetd config.toml template.
func ConfigToml() string {
	return configToml
}

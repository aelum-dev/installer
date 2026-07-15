package main

import (
	"flag"
	"fmt"

	"embed"

	backend "aesthetic-os-installer/backend"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Parse command-line flags
	dryRun := flag.Bool("dry-run", false, "Simulate installation without executing real commands")
	flag.Parse()

	if *dryRun {
		fmt.Println("[DRY RUN MODE] Installation steps will be simulated.")
	}

	// Create an instance of the app structure
	app := backend.NewApp(*dryRun)
	// Starting page for debug purposes
	app.SetStartPage(backend.PageInstallation)

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "aesthetic-os-installer",
		Width:     900,
		Height:    600,
		MinWidth:  900,
		MaxWidth:  900,
		MinHeight: 600,
		MaxHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.StartUp,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

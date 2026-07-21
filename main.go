package main

import (
	"flag"
	"fmt"
	"os"

	"embed"

	"aesthetic-os-installer/backend/gui"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Parse command-line flags
	flag.Parse()
	dryRun := false
	if f := flag.Lookup("dry-run"); f != nil && f.Value.String() == "true" {
		dryRun = true
	}

	// Also check environment variable (set by run.sh for wails dev)
	if os.Getenv("AESTHETIC_DRY_RUN") == "1" {
		dryRun = true
	}

	if dryRun {
		fmt.Println("[DRY RUN MODE] Installation steps will be simulated.")
	}

	// Create an instance of the app structure
	app := gui.NewApp(dryRun)
	// Starting page for debug purposes
	app.SetStartPage(gui.PageUserAccount)

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

package main

import (
	"fmt"
	"os"

	"embed"

	"aestheticos-installer/backend/gui"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// StartPage is the page to start on (0=Welcome, 1=User Account, 2=Disk Selection, etc.)
// Set to -1 to start on Welcome page (default behavior)
const StartPage = 2

func main() {
	dryRun := false
	if os.Getenv("AESTHETIC_DRY_RUN") == "1" {
		dryRun = true
	}

	if dryRun {
		fmt.Println("[DRY RUN MODE] Installation steps will be simulated.")
	}

	// Create an instance of the app structure
	app := gui.NewApp(dryRun)

	// Set start page if configured (>= 0)
	if StartPage >= 0 {
		if err := app.SetStartPage(gui.PageID{Value: StartPage}); err != nil {
			fmt.Printf("Warning: invalid start page %d: %v\n", StartPage, err)
		} else {
			fmt.Printf("[START PAGE] Starting on page %d\n", StartPage)
		}
	}

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
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1}, // dark blue-grey
		OnStartup:        app.StartUp,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

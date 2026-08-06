package main

import (
	"embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"aestheticos-installer/backend/gui"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// StartPage is the page to start on (0=Welcome, 1=User Account, 2=Disk Selection, etc.)
// Set to -1 to start on Welcome page (default behavior)
const StartPage = -1

func main() {
	dryRun := os.Getenv("AESTHETIC_DRY_RUN") == "1"

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

	// Catch SIGINT (Ctrl+C) and SIGTERM (window manager close/killactive) so
	// we can run cleanup before exiting. Without this, the Go runtime's default
	// handler prints "shutting down" and hangs because the Wails WebView can't
	// tear down gracefully.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupted — running cleanup...")
		if installer := app.GetInstaller(); installer != nil {
			installer.RunCleanup()
		}
		os.Exit(0)
	}()

	// Also catch SIGHUP (terminal hangup) for completeness.
	signal.Notify(sigChan, syscall.SIGHUP)

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
		OnBeforeClose: app.OnBeforeClose,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

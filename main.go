package main

import (
	"embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aelum-installer/backend/gui"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dryRun := os.Getenv("AELUM_DRY_RUN") == "1"

	if dryRun {
		fmt.Println("[DRY RUN MODE] Installation steps will be simulated.")
	}

	// Create an instance of the app structure
	app := gui.NewApp(dryRun)

	// Set start page from AELUM_START_PAGE env var (0-5) for debugging.
	// Unset or invalid values start on the Welcome page (default).
	if startPageStr := os.Getenv("AELUM_START_PAGE"); startPageStr != "" {
		var startPage int
		if _, err := fmt.Sscanf(startPageStr, "%d", &startPage); err == nil {
			if err := app.SetStartPage(gui.PageID{Value: startPage}); err != nil {
				fmt.Printf("Warning: invalid start page %q: %v\n", startPageStr, err)
			} else {
				fmt.Printf("[START PAGE] Starting on page %d\n", startPage)
			}
		}
	}

	// Catch SIGINT (Ctrl+C) and SIGTERM (window manager close/killactive) so
	// we can run cleanup before exiting. Without this, the Go runtime's default
	// handler prints "shutting down" and hangs because the Wails WebView can't
	// tear down gracefully.
	sigChan := make(chan os.Signal, 1)
	// Catch SIGINT (Ctrl+C), SIGTERM (WM close / killactive) and SIGHUP
	// (terminal hangup) so we can run cleanup before exiting. Without this,
	// the Go runtime's default handler prints "shutting down" and hangs
	// because the Wails WebView can't tear down gracefully.
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupted — cancelling installation...")
		app.CancelInstallation()

		// Wait for the CLI to exit and run its cleanup defers. If it doesn't
		// exit within 10s (e.g. stuck in a step that ignores the cancel
		// channel), force-unmount /mnt ourselves so the user isn't left with
		// stale mounts.
		cleanupDone := make(chan struct{})
		go func() {
			app.WaitForCleanup()
			close(cleanupDone)
		}()
		select {
		case <-cleanupDone:
		case <-time.After(10 * time.Second):
			fmt.Println("CLI did not exit in time — force cleaning mounts...")
			if err := primitives.CleanupMounts(shared.GetLogger()); err != nil {
				fmt.Fprintf(os.Stderr, "Force cleanup error: %v\n", err)
			}
		}
		shared.GetLogger().Close()
		os.Exit(0)
	}()

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "aelum-installer",
		Width:     900,
		Height:    600,
		MinWidth:  900,
		MaxWidth:  900,
		MinHeight: 600,
		MaxHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Create the window hidden and let main.ts reveal it after the first
		// paint: otherwise the window is mapped with WebKit's light pre-page
		// fill still showing (a white flash before the dark page appears).
		// build.sh patches Wails' Linux Run() so StartHidden actually works.
		StartHidden:      true,
		BackgroundColour: &options.RGBA{R: 20, G: 18, B: 24, A: 1}, // #141218, the dark surface
		OnStartup:        app.StartUp,
		OnBeforeClose:    app.OnBeforeClose,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

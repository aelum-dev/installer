package app

import "context"
import "fmt"

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		currentPage: PageWelcome,
	}
}

// StartUp is called when the app starts. The context is saved
func (a *App) StartUp(ctx context.Context) {
	a.ctx = ctx
}

// SetStartPage sets the page to start on (bypasses welcome page).
func (a *App) SetStartPage(pageID PageID) error {
	if pageID < PageWelcome || pageID > PageFinish {
		return fmt.Errorf("invalid start page ID: %d", pageID)
	}
	a.startPage = pageID
	return nil
}

// GetStartPage returns the configured start page, or -1 if none.
func (a *App) GetStartPage() int {
	if a.startPage == PageWelcome {
		return -1
	}
	return int(a.startPage)
}

// ClearStartPage clears the start page so it won't be used again.
func (a *App) ClearStartPage() {
	a.startPage = PageWelcome
}

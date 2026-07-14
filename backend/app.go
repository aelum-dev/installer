package app

import "context"

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

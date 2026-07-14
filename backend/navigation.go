package app

import "fmt"

// GetCurrentPage returns the current page index (0-based).
func (a *App) GetCurrentPage() int {
	return int(a.currentPage)
}

// SetCurrentPage sets the current page after validation.
func (a *App) SetCurrentPage(pageID PageID) error {
	if pageID < PageWelcome || pageID > PageFinish {
		return fmt.Errorf("invalid page ID: %d", pageID)
	}
	a.currentPage = pageID
	return nil
}

// CanGoNext checks if the user can proceed to the next page.
// TODO: Re-enable validation once UI forms are implemented.
func (a *App) CanGoNext() bool {
	return a.currentPage < PageFinish
}

// CanGoBack checks if the user can go back to the previous page.
func (a *App) CanGoBack() bool {
	return a.currentPage > PageWelcome
}

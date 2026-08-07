package install


// RootCheckResult contains the result of a root privilege check.
type RootCheckResult struct {
	Authorized bool   `json:"authorized"`
	Error      string `json:"error,omitempty"`
	Message    string `json:"message"`
	Output     string `json:"output,omitempty"`
}
